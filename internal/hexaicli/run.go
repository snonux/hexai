// Summary: Hexai CLI runner; reads input, creates an LLM client, builds messages,
// streams or collects the model output, and prints a short summary to stderr.
package hexaicli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/tmux"
)

// Run executes the Hexai CLI behavior given arguments and I/O streams.
// It assumes flags have already been parsed by the caller.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// Load configuration with a logger so file-based config is respected.
	logger := log.New(stderr, "hexai ", log.LstdFlags|log.Lmsgprefix)
	cfg := appconfig.Load(logger)
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	client, err := newClientFromApp(cfg)
	if err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	// No args: open editor to capture a prompt, then combine with stdin as usual.
	if len(args) == 0 {
		if prompt, eerr := editor.OpenTempAndEdit(nil); eerr == nil && strings.TrimSpace(prompt) != "" {
			args = []string{prompt}
		} else {
			// If editor fails or empty, continue; readInput will likely error if no stdin either.
		}
	}
	// Inline the flow here to use configured CLI prompts.
	input, rerr := readInput(stdin, args)
	if rerr != nil {
		fmt.Fprintln(stderr, logging.AnsiBase+rerr.Error()+logging.AnsiReset)
		return rerr
	}
	printProviderInfo(stderr, client)
	msgs := buildMessagesFromConfig(cfg, input)
	if err := runChat(ctx, client, msgs, input, stdout, stderr); err != nil {
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
	printProviderInfo(stderr, client)
	msgs := buildMessages(input)
	if err := runChat(ctx, client, msgs, input, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	return nil
}

// readInput reads from stdin and args, then combines them per CLI rules.
func readInput(stdin io.Reader, args []string) (string, error) {
	var stdinData string
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		b, _ := io.ReadAll(bufio.NewReader(stdin))
		stdinData = strings.TrimSpace(string(b))
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
func runChat(ctx context.Context, client llm.Client, msgs []llm.Message, input string, out io.Writer, errw io.Writer) error {
	start := time.Now()
	// Best-effort tmux status update (colored start heartbeat)
	_ = tmux.SetStatus(tmux.FormatLLMStartStatus(client.Name(), client.DefaultModel()))
	var output string
	if s, ok := client.(llm.Streamer); ok {
		var b strings.Builder
		if err := s.ChatStream(ctx, msgs, func(chunk string) {
			b.WriteString(chunk)
			fmt.Fprint(out, chunk)
		}); err != nil {
			return err
		}
		output = b.String()
	} else {
		txt, err := client.Chat(ctx, msgs)
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
	_ = stats.Update(ctx, client.Name(), client.DefaultModel(), sent, recv)
	snap, _ := stats.TakeSnapshot()
	minsWin := snap.Window.Minutes()
	if minsWin <= 0 {
		minsWin = 0.001
	}
	scopeReqs := int64(0)
	if pe, ok := snap.Providers[client.Name()]; ok {
		if mc, ok2 := pe.Models[client.DefaultModel()]; ok2 {
			scopeReqs = mc.Reqs
		}
	}
	scopeRPM := float64(scopeReqs) / minsWin
	fmt.Fprintf(errw, "\n"+logging.AnsiBase+"done provider=%s model=%s time=%s in_bytes=%d out_bytes=%d | global Σ reqs=%d rpm=%.2f"+logging.AnsiReset+"\n",
		client.Name(), client.DefaultModel(), dur.Round(time.Millisecond), sent, recv, snap.Global.Reqs, snap.RPM)
	_ = tmux.SetStatus(tmux.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, client.Name(), client.DefaultModel(), scopeRPM, scopeReqs, snap.Window))
	return nil
}

// printProviderInfo writes the provider/model line to stderr.
func printProviderInfo(errw io.Writer, client llm.Client) {
	fmt.Fprintf(errw, logging.AnsiBase+"provider=%s model=%s"+logging.AnsiReset+"\n", client.Name(), client.DefaultModel())
}

// newClientFromConfig is kept for tests; delegates to llmutils.
var newClientFromApp = llmutils.NewClientFromApp

// Backcompat for tests referencing the older helper name.
func newClientFromConfig(cfg appconfig.App) (llm.Client, error) { return newClientFromApp(cfg) }
