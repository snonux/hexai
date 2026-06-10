// Package hexaicli is the Hexai CLI runner; reads input, creates an LLM client, builds messages,
// streams or collects the model output, and prints a short summary to stderr.
package hexaicli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/termprint"
)

type requestArgs struct {
	model       string
	maxTokens   int
	temperature *float64
	options     []llm.RequestOption
}

type cliJob struct {
	index    int
	provider string
	entry    appconfig.SurfaceConfig
	cfg      appconfig.App
	req      requestArgs
}

type (
	selectionContextKey  struct{}
	configPathContextKey struct{}
)

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
		provider = llmutils.CanonicalProvider(provider)
		derived := llmutils.ConfigForProvider(cfg, provider, entry.Model)
		req := buildCLIRequest(entry, provider, derived)
		jobs = append(jobs, cliJob{index: i, provider: provider, entry: entry, cfg: derived, req: req})
	}
	return jobs, nil
}

func buildCLIRequest(entry appconfig.SurfaceConfig, provider string, cfg appconfig.App) requestArgs {
	opts := make([]llm.RequestOption, 0, 2)
	req := requestArgs{maxTokens: cfg.MaxTokens}
	if cfg.MaxTokens > 0 {
		opts = append(opts, llm.WithMaxTokens(cfg.MaxTokens))
	}
	model := strings.TrimSpace(entry.Model)
	if model == "" {
		model = strings.TrimSpace(llmutils.DefaultModelForProvider(cfg, provider))
	}
	if entry.Model != "" {
		opts = append(opts, llm.WithModel(entry.Model))
	}
	if temp, ok := cliTemperatureFromEntry(cfg, provider, entry, model); ok {
		tempValue := temp
		req.temperature = &tempValue
		opts = append(opts, llm.WithTemperature(temp))
	}
	req.model = model
	req.options = opts
	return req
}

// cliTemperatureFromEntry resolves the effective temperature for a CLI request,
// delegating GPT-5 override logic to llmutils.ResolveTemperature.
func cliTemperatureFromEntry(cfg appconfig.App, provider string, entry appconfig.SurfaceConfig, model string) (float64, bool) {
	return llmutils.ResolveTemperature(provider, model, entry.Temperature, cfg.CodingTemperature)
}

// Run executes the Hexai CLI behavior given arguments and I/O streams.
// It assumes flags have already been parsed by the caller.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := llm.RegisterAllProviders(); err != nil {
		return fmt.Errorf("failed to register LLM providers: %w", err)
	}
	return NewRunner().Run(ctx, args, stdin, stdout, stderr)
}

// RunWithClient executes the CLI flow using an already-constructed client.
// Useful for testing and embedding.
func RunWithClient(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, client llm.Client) error {
	return NewRunner().RunWithClient(ctx, args, stdin, stdout, stderr, client)
}

type cliJobResult struct {
	provider string
	model    string
	output   string
	summary  string
	err      error
}

type chatRunSummary struct {
	snapshot stats.Snapshot
	sent     int
	recv     int
	scopeReq int64
	scopeRPM float64
}

func runCLIJobs(ctx context.Context, jobs []cliJob, msgs []llm.Message, input string, stdout, stderr io.Writer, clientFactory cliClientFactory, statusSink cliStatusSink) error {
	streamSingle := len(jobs) == 1
	results, printer := executeCLIJobs(ctx, jobs, msgs, input, stdout, stderr, streamSingle, clientFactory, statusSink)
	if printer == nil && !streamSingle {
		if err := writeCLIJobOutputs(stdout, results); err != nil {
			return err
		}
	}
	return writeCLIJobSummaries(stderr, results)
}

func executeCLIJobs(ctx context.Context, jobs []cliJob, msgs []llm.Message, input string, stdout io.Writer, stderr io.Writer, streamSingle bool, clientFactory cliClientFactory, statusSink cliStatusSink) ([]*cliJobResult, *termprint.ColumnPrinter) {
	results := make([]*cliJobResult, len(jobs))
	printer := setupCLIPrinter(stdout, jobs)
	printCLIHeader(stderr, jobs, printer)
	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[job.index] = runSingleCLIJob(ctx, job, msgs, input, stdout, printer, streamSingle, clientFactory, statusSink)
		}()
	}
	wg.Wait()
	return results, printer
}

func setupCLIPrinter(stdout io.Writer, jobs []cliJob) *termprint.ColumnPrinter {
	if len(jobs) < 2 {
		return nil
	}
	return newColumnPrinter(stdout, jobs)
}

func runSingleCLIJob(ctx context.Context, job cliJob, msgs []llm.Message, input string, stdout io.Writer, printer *termprint.ColumnPrinter, streamOutput bool, clientFactory cliClientFactory, statusSink cliStatusSink) *cliJobResult {
	if res := cachedCLIJobResult(job, msgs, stdout, printer, streamOutput); res != nil {
		return res
	}

	client, err := clientFactory(job.cfg)
	if err != nil {
		return &cliJobResult{provider: job.provider, model: job.req.model, err: err}
	}
	model := effectiveModel(job.req, client)

	var errBuf bytes.Buffer
	var outBuf bytes.Buffer
	jobMsgs := append([]llm.Message(nil), msgs...)
	writer := io.Writer(&outBuf)
	if printer != nil {
		writer = io.MultiWriter(printer.Writer(job.index), &outBuf)
	} else if streamOutput {
		writer = io.MultiWriter(stdout, &outBuf)
	}
	err = runChatWithStatus(statusSink, ctx, client, job.req, jobMsgs, input, writer, &errBuf)
	if printer != nil {
		printer.Flush(job.index)
	}
	if err == nil {
		storeCLIResponseCache(newCLIResponseCacheKey(job.provider, model, job.req, jobMsgs), outBuf.String())
	}
	return &cliJobResult{
		provider: job.provider,
		model:    model,
		output:   outBuf.String(),
		summary:  errBuf.String(),
		err:      err,
	}
}

func cachedCLIJobResult(job cliJob, msgs []llm.Message, stdout io.Writer, printer *termprint.ColumnPrinter, streamOutput bool) *cliJobResult {
	output, age, ok := lookupCLIResponseCache(newCLIResponseCacheKey(job.provider, job.req.model, job.req, msgs))
	if !ok {
		return nil
	}
	if err := writeCachedCLIJobOutput(output, stdout, printer, job.index, streamOutput); err != nil {
		return &cliJobResult{provider: job.provider, model: job.req.model, err: err}
	}
	return &cliJobResult{
		provider: job.provider,
		model:    job.req.model,
		output:   output,
		summary:  cacheHitSummary(job.provider, job.req.model, age),
	}
}

func writeCachedCLIJobOutput(output string, stdout io.Writer, printer *termprint.ColumnPrinter, idx int, streamOutput bool) error {
	if printer != nil {
		if _, err := io.WriteString(printer.Writer(idx), output); err != nil {
			return err
		}
		printer.Flush(idx)
		return nil
	}
	if !streamOutput {
		return nil
	}
	_, err := io.WriteString(stdout, output)
	return err
}

func writeCLIJobOutputs(stdout io.Writer, results []*cliJobResult) error {
	printed := false
	showHeading := cliJobResultCount(results) > 1
	for _, res := range results {
		if res == nil {
			continue
		}
		if printed {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				return err
			}
		}
		if err := writeCLIJobOutput(stdout, res, showHeading); err != nil {
			return err
		}
		printed = true
	}
	return nil
}

func cliJobResultCount(results []*cliJobResult) int {
	count := 0
	for _, res := range results {
		if res != nil {
			count++
		}
	}
	return count
}

func writeCLIJobOutput(stdout io.Writer, res *cliJobResult, showHeading bool) error {
	if showHeading {
		heading := fmt.Sprintf("=== %s:%s ===\n", res.provider, res.model)
		if _, err := io.WriteString(stdout, heading); err != nil {
			return err
		}
	}
	if res.output == "" {
		return nil
	}
	if _, err := io.WriteString(stdout, res.output); err != nil {
		return err
	}
	if strings.HasSuffix(res.output, "\n") {
		return nil
	}
	_, err := io.WriteString(stdout, "\n")
	return err
}

func writeCLIJobSummaries(stderr io.Writer, results []*cliJobResult) error {
	var firstErr error
	for _, res := range results {
		if res == nil {
			continue
		}
		if err := writeCLIJobSummary(stderr, res); err != nil {
			return err
		}
		if firstErr == nil && res.err != nil {
			firstErr = res.err
		}
	}
	return firstErr
}

// writeCLIJobSummary writes the per-job summary (stats or cache-hit note) to
// stderr. It always starts on a new line so that streaming output that does
// not end with a newline is not run together with the meta text.
func writeCLIJobSummary(stderr io.Writer, res *cliJobResult) error {
	summary := strings.TrimLeft(res.summary, "\n")
	if summary != "" {
		if _, err := fmt.Fprintf(stderr, "\n%s", summary); err != nil {
			return err
		}
	}
	if res.err == nil {
		return nil
	}
	_, err := fmt.Fprintf(stderr, logging.AnsiBase+"hexai: provider=%s model=%s error: %v"+logging.AnsiReset+"\n", res.provider, res.model, res.err)
	return err
}

func newColumnPrinter(stdout io.Writer, jobs []cliJob) *termprint.ColumnPrinter {
	providers := make([]string, len(jobs))
	models := make([]string, len(jobs))
	for _, job := range jobs {
		providers[job.index] = job.provider
		models[job.index] = job.req.model
	}
	return termprint.NewColumnPrinter(stdout, providers, models)
}

func printCLIHeader(stderr io.Writer, jobs []cliJob, printer *termprint.ColumnPrinter) {
	if len(jobs) == 0 {
		return
	}
	if printer != nil {
		printer.PrintHeaderTo(stderr)
		return
	}
	job := jobs[0]
	printProviderLabel(stderr, job.provider, job.req.model)
}

// WithCLISelection injects provider indices into the context so Run only executes those jobs.
func WithCLISelection(ctx context.Context, indices []int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	cpy := make([]int, len(indices))
	copy(cpy, indices)
	return context.WithValue(ctx, selectionContextKey{}, cpy)
}

// WithCLIConfigPath returns a context that carries the config file path override.
func WithCLIConfigPath(ctx context.Context, path string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, configPathContextKey{}, strings.TrimSpace(path))
}

func configPathFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(configPathContextKey{}).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func selectionFromContext(ctx context.Context) []int {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(selectionContextKey{}).([]int); ok {
		cpy := make([]int, len(v))
		copy(cpy, v)
		return cpy
	}
	return nil
}

func filterJobsBySelection(jobs []cliJob, indices []int) ([]cliJob, error) {
	if len(indices) == 0 {
		return jobs, nil
	}
	filtered := make([]cliJob, 0, len(indices))
	seen := make(map[int]struct{}, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(jobs) {
			return nil, fmt.Errorf("provider index %d out of range (0-%d)", idx, len(jobs)-1)
		}
		if _, ok := seen[idx]; ok {
			continue
		}
		clone := jobs[idx]
		filtered = append(filtered, clone)
		seen[idx] = struct{}{}
	}
	for i := range filtered {
		filtered[i].index = i
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no CLI providers matched selection")
	}
	return filtered, nil
}

// stater is the subset of *os.File needed to detect piped vs terminal input.
type stater interface {
	Stat() (os.FileInfo, error)
}

// isPipedInput reports whether stdin is a pipe or file (not a terminal).
// For *os.File it checks the file mode; for any other io.Reader it
// optimistically returns true since non-file readers are typically
// in-memory buffers used in tests.
func isPipedInput(stdin io.Reader) bool {
	if f, ok := stdin.(stater); ok {
		fi, err := f.Stat()
		return err == nil && (fi.Mode()&os.ModeCharDevice) == 0
	}
	// Non-file readers (e.g. strings.NewReader in tests) always have data.
	return true
}

// readInput reads from stdin and args, then combines them per CLI rules.
// It uses the passed stdin reader (not os.Stdin) to detect piped input.
func readInput(stdin io.Reader, args []string) (string, error) {
	var stdinData string
	if isPipedInput(stdin) {
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
	return runChatWithStatus(tmuxCLIStatusSink{}, ctx, client, req, msgs, input, out, errw)
}

func runChatWithStatus(statusSink cliStatusSink, ctx context.Context, client llm.Client, req requestArgs, msgs []llm.Message, input string, out io.Writer, errw io.Writer) error {
	start := time.Now()
	model := effectiveModel(req, client)
	if statusSink != nil {
		_ = statusSink.SetLLMStart(client.Name(), model)
	}

	output, err := runChatRequest(ctx, client, req, msgs, out)
	if err != nil {
		return err
	}

	dur := time.Since(start)
	summary := summarizeChatRun(ctx, client, model, msgs, output)
	if _, err := fmt.Fprintf(errw, "\n"+logging.AnsiBase+"done provider=%s model=%s time=%s in_bytes=%d out_bytes=%d | global Σ reqs=%d rpm=%.2f"+logging.AnsiReset+"\n",
		client.Name(), model, dur.Round(time.Millisecond), summary.sent, summary.recv, summary.snapshot.Global.Reqs, summary.snapshot.RPM); err != nil {
		return err
	}
	if statusSink != nil {
		_ = statusSink.SetGlobal(summary.snapshot, client.Name(), model, summary.scopeRPM, summary.scopeReq)
	}
	return nil
}

func effectiveModel(req requestArgs, client llm.Client) string {
	model := strings.TrimSpace(req.model)
	if model == "" {
		model = client.DefaultModel()
	}
	return model
}

func runChatRequest(ctx context.Context, client llm.Client, req requestArgs, msgs []llm.Message, out io.Writer) (string, error) {
	if streamer, ok := client.(llm.Streamer); ok {
		return runStreamingChat(ctx, streamer, msgs, req.options, out)
	}
	return runSimpleChat(ctx, client, msgs, req.options, out)
}

func runStreamingChat(ctx context.Context, client llm.Streamer, msgs []llm.Message, options []llm.RequestOption, out io.Writer) (string, error) {
	var output strings.Builder
	var writeErr error
	if err := client.ChatStream(ctx, msgs, func(chunk string) {
		if writeErr != nil {
			return
		}
		output.WriteString(chunk)
		if _, err := fmt.Fprint(out, chunk); err != nil {
			writeErr = err
		}
	}, options...); err != nil {
		return "", err
	}
	if writeErr != nil {
		return "", writeErr
	}
	return output.String(), nil
}

func runSimpleChat(ctx context.Context, client llm.Client, msgs []llm.Message, options []llm.RequestOption, out io.Writer) (string, error) {
	output, err := client.Chat(ctx, msgs, options...)
	if err != nil {
		return "", err
	}
	if _, err := fmt.Fprint(out, output); err != nil {
		return "", err
	}
	return output, nil
}

func summarizeChatRun(ctx context.Context, client llm.Client, model string, msgs []llm.Message, output string) chatRunSummary {
	summary := chatRunSummary{snapshot: stats.Snapshot{Window: time.Hour}}
	for _, m := range msgs {
		summary.sent += len(m.Content)
	}
	summary.recv = len(output)
	_ = stats.Update(ctx, client.Name(), model, summary.sent, summary.recv)
	snap, err := stats.TakeSnapshot()
	if err == nil {
		summary.snapshot = snap
	}
	summary.scopeReq = summary.snapshot.ScopeReqs(client.Name(), model)
	summary.scopeRPM = summary.snapshot.ScopeRPM(client.Name(), model)
	return summary
}

// printProviderInfo writes the provider:model header and divider to stderr.
func printProviderInfo(errw io.Writer, client llm.Client, model string) {
	printProviderLabel(errw, client.Name(), chooseCLIModel(model, client.DefaultModel()))
}

// printProviderLabel writes a compact "provider:model: " label to errw using
// the same ANSI styling as other meta output. No divider line is emitted so
// the label stays out of the way of the actual response text.
func printProviderLabel(errw io.Writer, provider, model string) {
	if strings.TrimSpace(model) == "" {
		return
	}
	label := strings.TrimSpace(provider) + ":" + strings.TrimSpace(model) + ":"
	_, _ = fmt.Fprintf(errw, logging.AnsiBase+"%s"+logging.AnsiReset+"\n", label)
}

func chooseCLIModel(model, fallback string) string {
	model = strings.TrimSpace(model)
	if model != "" {
		return model
	}
	return strings.TrimSpace(fallback)
}

func cacheHitSummary(provider, model string, age time.Duration) string {
	if age < 0 {
		age = 0
	}
	return fmt.Sprintf(logging.AnsiBase+"cache hit provider=%s model=%s age=%s"+logging.AnsiReset+"\n", provider, model, age.Round(time.Second))
}

// newClientFromConfig is kept for tests; delegates to llmutils.
var newClientFromApp = llmutils.NewClientFromApp

// Backcompat for tests referencing the older helper name.
func newClientFromConfig(cfg appconfig.App) (llm.Client, error) { return newClientFromApp(cfg) }
