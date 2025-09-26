// Summary: Hexai CLI runner; reads input, creates an LLM client, builds messages,
// streams or collects the model output, and prints a short summary to stderr.
package hexaicli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/tmux"
)

type requestArgs struct {
	model   string
	options []llm.RequestOption
}

type cliJob struct {
	index    int
	provider string
	entry    appconfig.SurfaceConfig
	client   llm.Client
	req      requestArgs
}

func buildCLIJobs(cfg appconfig.App) ([]cliJob, error) {
	entries := cfg.CLIConfigs
	if len(entries) == 0 {
		entries = []appconfig.SurfaceConfig{{}}
	}
	jobs := make([]cliJob, 0, len(entries))
	for i, raw := range entries {
		entry := appconfig.SurfaceConfig{Provider: strings.TrimSpace(raw.Provider), Model: strings.TrimSpace(raw.Model), Temperature: raw.Temperature}
		provider := entry.Provider
		if provider == "" {
			provider = cfg.Provider
		}
		provider = canonicalProvider(provider)
		derived := cfg
		derived.Provider = provider
		switch provider {
		case "openai":
			if entry.Model != "" {
				derived.OpenAIModel = entry.Model
			}
		case "copilot":
			if entry.Model != "" {
				derived.CopilotModel = entry.Model
			}
		case "ollama":
			if entry.Model != "" {
				derived.OllamaModel = entry.Model
			}
		}
		client, err := newClientFromApp(derived)
		if err != nil {
			return nil, err
		}
		req := buildCLIRequest(entry, provider, cfg, client)
		if strings.TrimSpace(req.model) == "" {
			req.model = strings.TrimSpace(client.DefaultModel())
		}
		jobs = append(jobs, cliJob{index: i, provider: provider, entry: entry, client: client, req: req})
	}
	return jobs, nil
}

func buildCLIRequest(entry appconfig.SurfaceConfig, provider string, cfg appconfig.App, client llm.Client) requestArgs {
	opts := make([]llm.RequestOption, 0, 2)
	if cfg.MaxTokens > 0 {
		opts = append(opts, llm.WithMaxTokens(cfg.MaxTokens))
	}
	model := strings.TrimSpace(entry.Model)
	if model == "" {
		if client != nil {
			model = strings.TrimSpace(client.DefaultModel())
		}
		if model == "" {
			model = strings.TrimSpace(defaultModelForProvider(cfg, provider))
		}
	}
	if entry.Model != "" {
		opts = append(opts, llm.WithModel(entry.Model))
	}
	if temp, ok := cliTemperatureFromEntry(cfg, provider, entry, model); ok {
		opts = append(opts, llm.WithTemperature(temp))
	}
	return requestArgs{model: model, options: opts}
}

func cliTemperatureFromEntry(cfg appconfig.App, provider string, entry appconfig.SurfaceConfig, model string) (float64, bool) {
	if entry.Temperature != nil {
		return *entry.Temperature, true
	}
	if cfg.CodingTemperature != nil {
		temp := *cfg.CodingTemperature
		if provider == "openai" && strings.HasPrefix(strings.ToLower(model), "gpt-5") && temp == 0.2 {
			temp = 1.0
		}
		return temp, true
	}
	if provider == "openai" && strings.HasPrefix(strings.ToLower(model), "gpt-5") {
		return 1.0, true
	}
	return 0, false
}

func canonicalProvider(name string) string {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return "openai"
	}
	return p
}

func defaultModelForProvider(cfg appconfig.App, provider string) string {
	switch provider {
	case "ollama":
		return cfg.OllamaModel
	case "copilot":
		return cfg.CopilotModel
	default:
		return cfg.OpenAIModel
	}
}

// Run executes the Hexai CLI behavior given arguments and I/O streams.
// It assumes flags have already been parsed by the caller.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// Load configuration with a logger so file-based config is respected.
	logger := log.New(stderr, "hexai ", log.LstdFlags|log.Lmsgprefix)
	cfg := appconfig.Load(logger)
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	jobs, err := buildCLIJobs(cfg)
	if err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	if len(jobs) == 0 {
		return fmt.Errorf("hexai: no CLI providers configured")
	}
	// Prefer piped stdin when present; only open the editor when there are no args
	// and no stdin content available.
	input, rerr := readInput(stdin, args)
	if rerr != nil && len(args) == 0 {
		if prompt, eerr := editor.OpenTempAndEdit(nil); eerr == nil && strings.TrimSpace(prompt) != "" {
			args = []string{prompt}
			input, rerr = readInput(stdin, args)
		}
	}
	if rerr != nil {
		fmt.Fprintln(stderr, logging.AnsiBase+rerr.Error()+logging.AnsiReset)
		return rerr
	}
	msgs := buildMessagesFromConfig(cfg, input)
	if err := runCLIJobs(ctx, jobs, msgs, input, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	return nil
}

// RunWithClient executes the CLI flow using an already-constructed client.
// Useful for testing and embedding.
func RunWithClient(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, client llm.Client) error {
	input, err := readInput(stdin, args)
	if err != nil {
		fmt.Fprintln(stderr, logging.AnsiBase+err.Error()+logging.AnsiReset)
		return err
	}
	req := requestArgs{model: strings.TrimSpace(client.DefaultModel())}
	printProviderInfo(stderr, client, req.model)
	msgs := buildMessages(input)
	if err := runChat(ctx, client, req, msgs, input, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	return nil
}

type cliJobResult struct {
	provider string
	model    string
	output   string
	summary  string
	err      error
}

func runCLIJobs(ctx context.Context, jobs []cliJob, msgs []llm.Message, input string, stdout, stderr io.Writer) error {
	results := make([]*cliJobResult, len(jobs))
	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		wg.Add(1)
		printProviderInfo(stderr, job.client, job.req.model)
		go func() {
			defer wg.Done()
			var outBuf, errBuf bytes.Buffer
			jobMsgs := make([]llm.Message, len(msgs))
			copy(jobMsgs, msgs)
			err := runChat(ctx, job.client, job.req, jobMsgs, input, &outBuf, &errBuf)
			results[job.index] = &cliJobResult{
				provider: job.client.Name(),
				model:    job.req.model,
				output:   outBuf.String(),
				summary:  errBuf.String(),
				err:      err,
			}
		}()
	}
	wg.Wait()
	var firstErr error
	printed := false
	for _, res := range results {
		if res == nil {
			continue
		}
		if printed {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				return err
			}
		}
		heading := fmt.Sprintf("=== %s:%s ===\n", res.provider, res.model)
		if _, err := io.WriteString(stdout, heading); err != nil {
			return err
		}
		if res.output != "" {
			if _, err := io.WriteString(stdout, res.output); err != nil {
				return err
			}
			if !strings.HasSuffix(res.output, "\n") {
				if _, err := io.WriteString(stdout, "\n"); err != nil {
					return err
				}
			}
		}
		printed = true
		if res.summary != "" {
			if _, err := io.WriteString(stderr, res.summary); err != nil {
				return err
			}
		}
		if res.err != nil {
			if _, err := fmt.Fprintf(stderr, logging.AnsiBase+"hexai: provider=%s model=%s error: %v"+logging.AnsiReset+"\n", res.provider, res.model, res.err); err != nil {
				return err
			}
			if firstErr == nil {
				firstErr = res.err
			}
		}
	}
	return firstErr
}

// readInput reads from stdin and args, then combines them per CLI rules.
func readInput(stdin io.Reader, args []string) (string, error) {
	var stdinData string
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		data, readErr := io.ReadAll(stdin)
		if readErr != nil {
			return "", fmt.Errorf("hexai: failed to read stdin: %w", readErr)
		}
		stdinData = strings.TrimSpace(string(data))
	}
	argData := strings.TrimSpace(strings.Join(args, " "))
	switch {
	case stdinData != "" && argData != "":
		return fmt.Sprintf("%s:\n\n%s", argData, stdinData), nil
	case stdinData != "":
		return stdinData, nil
	case argData != "":
		return argData, nil
	default:
		return "", fmt.Errorf("hexai: no input provided; pass text as an argument or via stdin")
	}
}

// newClientFromConfig builds an LLM client from the app config and env keys.
// client construction moved to internal/llmutils

// buildMessages creates system and user messages based on input content.
func buildMessages(input string) []llm.Message {
	lower := strings.ToLower(input)
	system := "You are Hexai CLI. Default to very short, concise answers. If the user asks for commands, output only the commands (one per line) with no commentary or explanation. Only when the word 'explain' appears in the prompt, produce a verbose explanation."
	if strings.Contains(lower, "explain") {
		system = "You are Hexai CLI. The user requested an explanation. Provide a clear, verbose explanation with reasoning and details. If commands are needed, include them with brief context."
	}
	return []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: input},
	}
}

// buildMessagesFromConfig uses configured CLI system prompts.
func buildMessagesFromConfig(cfg appconfig.App, input string) []llm.Message {
	lower := strings.ToLower(input)
	system := cfg.PromptCLIDefaultSystem
	if strings.Contains(lower, "explain") {
		if strings.TrimSpace(cfg.PromptCLIExplainSystem) != "" {
			system = cfg.PromptCLIExplainSystem
		}
	}
	return []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: input},
	}
}

// runChat executes the chat request, handling streaming and summary output.
func runChat(ctx context.Context, client llm.Client, req requestArgs, msgs []llm.Message, input string, out io.Writer, errw io.Writer) error {
	start := time.Now()
	// Best-effort tmux status update (colored start heartbeat)
	model := strings.TrimSpace(req.model)
	if model == "" {
		model = client.DefaultModel()
	}
	_ = tmux.SetStatus(tmux.FormatLLMStartStatus(client.Name(), model))
	var output string
	if s, ok := client.(llm.Streamer); ok {
		var b strings.Builder
		if err := s.ChatStream(ctx, msgs, func(chunk string) {
			b.WriteString(chunk)
			fmt.Fprint(out, chunk)
		}, req.options...); err != nil {
			return err
		}
		output = b.String()
	} else {
		txt, err := client.Chat(ctx, msgs, req.options...)
		if err != nil {
			return err
		}
		output = txt
		fmt.Fprint(out, output)
	}
	dur := time.Since(start)
	// Contribute to global stats and update tmux status
	sent := 0
	for _, m := range msgs {
		sent += len(m.Content)
	}
	recv := len(output)
	_ = stats.Update(ctx, client.Name(), model, sent, recv)
	snap, _ := stats.TakeSnapshot()
	minsWin := snap.Window.Minutes()
	if minsWin <= 0 {
		minsWin = 0.001
	}
	scopeReqs := int64(0)
	if pe, ok := snap.Providers[client.Name()]; ok {
		if mc, ok2 := pe.Models[model]; ok2 {
			scopeReqs = mc.Reqs
		}
	}
	scopeRPM := float64(scopeReqs) / minsWin
	fmt.Fprintf(errw, "\n"+logging.AnsiBase+"done provider=%s model=%s time=%s in_bytes=%d out_bytes=%d | global Σ reqs=%d rpm=%.2f"+logging.AnsiReset+"\n",
		client.Name(), model, dur.Round(time.Millisecond), sent, recv, snap.Global.Reqs, snap.RPM)
	_ = tmux.SetStatus(tmux.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, client.Name(), model, scopeRPM, scopeReqs, snap.Window))
	return nil
}

// printProviderInfo writes the provider/model line to stderr.
func printProviderInfo(errw io.Writer, client llm.Client, model string) {
	if strings.TrimSpace(model) == "" {
		model = client.DefaultModel()
	}
	fmt.Fprintf(errw, logging.AnsiBase+"provider=%s model=%s"+logging.AnsiReset+"\n", client.Name(), model)
}

// newClientFromConfig is kept for tests; delegates to llmutils.
var newClientFromApp = llmutils.NewClientFromApp

// Backcompat for tests referencing the older helper name.
func newClientFromConfig(cfg appconfig.App) (llm.Client, error) { return newClientFromApp(cfg) }
