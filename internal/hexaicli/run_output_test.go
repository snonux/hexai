// Tests for CLI job output writing, result counting, config path context,
// cached output writing, and streaming error paths.
package hexaicli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

func TestCliJobResultCount(t *testing.T) {
	tests := []struct {
		name    string
		results []*cliJobResult
		want    int
	}{
		{name: "all nil", results: []*cliJobResult{nil, nil}, want: 0},
		{name: "empty slice", results: nil, want: 0},
		{name: "one result", results: []*cliJobResult{{provider: "a"}}, want: 1},
		{name: "mixed", results: []*cliJobResult{{provider: "a"}, nil, {provider: "b"}}, want: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cliJobResultCount(tc.results); got != tc.want {
				t.Fatalf("cliJobResultCount = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWriteCLIJobOutput_WithHeading(t *testing.T) {
	var buf bytes.Buffer
	res := &cliJobResult{provider: "openai", model: "gpt-4.1", output: "hello world"}
	if err := writeCLIJobOutput(&buf, res, true); err != nil {
		t.Fatalf("writeCLIJobOutput: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "=== openai:gpt-4.1 ===") {
		t.Fatalf("expected heading, got %q", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Fatalf("expected output, got %q", got)
	}
	// Output without trailing newline should get one appended.
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestWriteCLIJobOutput_WithoutHeading(t *testing.T) {
	var buf bytes.Buffer
	res := &cliJobResult{provider: "openai", model: "gpt-4.1", output: "hello\n"}
	if err := writeCLIJobOutput(&buf, res, false); err != nil {
		t.Fatalf("writeCLIJobOutput: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "===") {
		t.Fatalf("expected no heading, got %q", got)
	}
	// Output already ends with newline; no extra newline should be appended.
	if got != "hello\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestWriteCLIJobOutput_EmptyOutput(t *testing.T) {
	var buf bytes.Buffer
	res := &cliJobResult{provider: "p", model: "m", output: ""}
	if err := writeCLIJobOutput(&buf, res, true); err != nil {
		t.Fatalf("writeCLIJobOutput: %v", err)
	}
	// Should print heading but no body content.
	if !strings.Contains(buf.String(), "=== p:m ===") {
		t.Fatalf("expected heading even for empty output, got %q", buf.String())
	}
}

func TestWriteCLIJobOutputs_SingleResult(t *testing.T) {
	var buf bytes.Buffer
	results := []*cliJobResult{{provider: "p", model: "m", output: "out"}}
	if err := writeCLIJobOutputs(&buf, results); err != nil {
		t.Fatalf("writeCLIJobOutputs: %v", err)
	}
	// Single result: showHeading is false (count == 1).
	if strings.Contains(buf.String(), "===") {
		t.Fatalf("single result should have no heading, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "out") {
		t.Fatalf("expected output, got %q", buf.String())
	}
}

func TestWriteCLIJobOutputs_MultipleResults(t *testing.T) {
	var buf bytes.Buffer
	results := []*cliJobResult{
		{provider: "a", model: "m1", output: "first"},
		{provider: "b", model: "m2", output: "second"},
	}
	if err := writeCLIJobOutputs(&buf, results); err != nil {
		t.Fatalf("writeCLIJobOutputs: %v", err)
	}
	got := buf.String()
	// Multiple results: headings shown.
	if !strings.Contains(got, "=== a:m1 ===") || !strings.Contains(got, "=== b:m2 ===") {
		t.Fatalf("expected headings for both results, got %q", got)
	}
	// Separator newline between results.
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Fatalf("expected both outputs, got %q", got)
	}
}

func TestWriteCLIJobOutputs_WithNils(t *testing.T) {
	var buf bytes.Buffer
	results := []*cliJobResult{nil, {provider: "a", model: "m", output: "ok"}, nil}
	if err := writeCLIJobOutputs(&buf, results); err != nil {
		t.Fatalf("writeCLIJobOutputs: %v", err)
	}
	// Only one non-nil result, so count=1, no heading.
	if strings.Contains(buf.String(), "===") {
		t.Fatalf("single non-nil result should have no heading, got %q", buf.String())
	}
}

func TestWriteCLIJobOutputs_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCLIJobOutputs(&buf, nil); err != nil {
		t.Fatalf("writeCLIJobOutputs: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buf.String())
	}
}

func TestWithCLIConfigPath_And_ConfigPathFromContext(t *testing.T) {
	// Normal usage.
	ctx := WithCLIConfigPath(context.Background(), "/tmp/config.toml")
	if got := configPathFromContext(ctx); got != "/tmp/config.toml" {
		t.Fatalf("configPathFromContext = %q, want /tmp/config.toml", got)
	}

	// With whitespace trimming.
	ctx = WithCLIConfigPath(context.Background(), "  /tmp/cfg.toml  ")
	if got := configPathFromContext(ctx); got != "/tmp/cfg.toml" {
		t.Fatalf("configPathFromContext = %q, want /tmp/cfg.toml", got)
	}

	// Nil context for WithCLIConfigPath creates a background context.
	ctx = WithCLIConfigPath(nil, "/path")
	if got := configPathFromContext(ctx); got != "/path" {
		t.Fatalf("configPathFromContext = %q, want /path", got)
	}

	// Empty context returns empty string.
	if got := configPathFromContext(context.Background()); got != "" {
		t.Fatalf("configPathFromContext on empty ctx = %q, want empty", got)
	}

	// Nil context returns empty string.
	if got := configPathFromContext(nil); got != "" {
		t.Fatalf("configPathFromContext on nil = %q, want empty", got)
	}
}

func TestWriteCachedCLIJobOutput_StreamOutput(t *testing.T) {
	var buf bytes.Buffer
	// streamOutput=true, printer=nil => writes to stdout.
	if err := writeCachedCLIJobOutput("cached", &buf, nil, 0, true); err != nil {
		t.Fatalf("writeCachedCLIJobOutput: %v", err)
	}
	if buf.String() != "cached" {
		t.Fatalf("expected 'cached', got %q", buf.String())
	}
}

func TestWriteCachedCLIJobOutput_NoStreamNoPrinter(t *testing.T) {
	var buf bytes.Buffer
	// streamOutput=false, printer=nil => returns nil without writing.
	if err := writeCachedCLIJobOutput("cached", &buf, nil, 0, false); err != nil {
		t.Fatalf("writeCachedCLIJobOutput: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

// errWriter is an io.Writer that always returns an error.
type errWriter struct{ err error }

func (e errWriter) Write([]byte) (int, error) { return 0, e.err }

func TestWriteCLIJobOutput_WriteError(t *testing.T) {
	w := errWriter{err: errors.New("write fail")}
	res := &cliJobResult{provider: "p", model: "m", output: "out"}
	if err := writeCLIJobOutput(w, res, true); err == nil {
		t.Fatalf("expected error from failing writer")
	}
}

func TestWriteCLIJobOutputs_WriteError(t *testing.T) {
	w := errWriter{err: errors.New("write fail")}
	results := []*cliJobResult{{provider: "p", model: "m", output: "out"}}
	if err := writeCLIJobOutputs(w, results); err == nil {
		t.Fatalf("expected error from failing writer")
	}
}

// streamErrClient is a Streamer that returns a stream error.
type streamErrClient struct {
	fakeClient
	streamErr error
}

func (s *streamErrClient) ChatStream(_ context.Context, _ []llm.Message, _ func(string), _ ...llm.RequestOption) error {
	return s.streamErr
}

func TestRunStreamingChat_StreamError(t *testing.T) {
	client := &streamErrClient{
		fakeClient: fakeClient{name: "p", model: "m"},
		streamErr:  fmt.Errorf("stream broken"),
	}
	var out bytes.Buffer
	_, err := runChatRequest(context.Background(), client, requestArgs{}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "stream broken") {
		t.Fatalf("expected stream error, got %v", err)
	}
}

// streamWriteErrClient is a Streamer that writes chunks to trigger a write error.
type streamWriteErrClient struct {
	fakeClient
}

func (s *streamWriteErrClient) ChatStream(_ context.Context, _ []llm.Message, onDelta func(string), _ ...llm.RequestOption) error {
	onDelta("chunk1")
	onDelta("chunk2")
	return nil
}

func TestRunStreamingChat_WriteError(t *testing.T) {
	client := &streamWriteErrClient{fakeClient: fakeClient{name: "p", model: "m"}}
	w := errWriter{err: errors.New("write fail")}
	_, err := runChatRequest(context.Background(), client, requestArgs{}, nil, w)
	if err == nil || !strings.Contains(err.Error(), "write fail") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestRunWithClient_NoInput(t *testing.T) {
	var out, errb bytes.Buffer
	err := RunWithClient(context.Background(), nil, strings.NewReader(""), &out, &errb, &fakeClient{name: "p", model: "m", resp: "out"})
	if err == nil {
		t.Fatalf("expected error for no input")
	}
	if !strings.Contains(errb.String(), "no input provided") {
		t.Fatalf("expected no-input error message, got %q", errb.String())
	}
}

func TestRunWithClient_Success(t *testing.T) {
	var out, errb bytes.Buffer
	client := &fakeClient{name: "p", model: "m", resp: "result"}
	err := RunWithClient(context.Background(), []string{"hello"}, strings.NewReader(""), &out, &errb, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.String() != "result" {
		t.Fatalf("stdout = %q, want result", out.String())
	}
	if !strings.Contains(errb.String(), "provider=p model=m") {
		t.Fatalf("expected summary in stderr, got %q", errb.String())
	}
}

func TestEffectiveModel_Empty(t *testing.T) {
	client := &fakeClient{name: "p", model: "default-model"}
	req := requestArgs{model: ""}
	if got := effectiveModel(req, client); got != "default-model" {
		t.Fatalf("effectiveModel = %q, want default-model", got)
	}
}

func TestEffectiveModel_Whitespace(t *testing.T) {
	client := &fakeClient{name: "p", model: "default-model"}
	req := requestArgs{model: "  "}
	if got := effectiveModel(req, client); got != "default-model" {
		t.Fatalf("effectiveModel = %q, want default-model", got)
	}
}

func TestRunSimpleChat_WriteError(t *testing.T) {
	client := &fakeClient{name: "p", model: "m", resp: "ok"}
	w := errWriter{err: errors.New("write fail")}
	_, err := runChatRequest(context.Background(), client, requestArgs{}, nil, w)
	if err == nil || !strings.Contains(err.Error(), "write fail") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestChooseCLIModel_Empty(t *testing.T) {
	if got := chooseCLIModel("", "fallback"); got != "fallback" {
		t.Fatalf("chooseCLIModel = %q, want fallback", got)
	}
}

func TestChooseCLIModel_Whitespace(t *testing.T) {
	if got := chooseCLIModel("  ", "fallback"); got != "fallback" {
		t.Fatalf("chooseCLIModel = %q, want fallback", got)
	}
}

func TestPrintProviderLabel_EmptyModel(t *testing.T) {
	var buf bytes.Buffer
	printProviderLabel(&buf, "p", "")
	if buf.Len() != 0 {
		t.Fatalf("expected no output for empty model, got %q", buf.String())
	}
}

func TestPrintProviderLabel_WhitespaceModel(t *testing.T) {
	var buf bytes.Buffer
	printProviderLabel(&buf, "p", "   ")
	if buf.Len() != 0 {
		t.Fatalf("expected no output for whitespace model, got %q", buf.String())
	}
}

func TestCacheHitSummary_NegativeAge(t *testing.T) {
	got := cacheHitSummary("p", "m", -5)
	if !strings.Contains(got, "cache hit") || !strings.Contains(got, "age=0s") {
		t.Fatalf("expected cache hit with age=0s, got %q", got)
	}
}

func TestRunCLIJobs_MultiJob_WritesOutputs(t *testing.T) {
	// runCLIJobs with multiple jobs should call writeCLIJobOutputs
	// (the non-streaming, non-printer path).
	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		return &fakeClient{name: cfg.Provider, model: "m", resp: "out-" + cfg.Provider}, nil
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	jobs := []cliJob{
		{index: 0, provider: "a", cfg: appconfig.App{
			CoreConfig: appconfig.CoreConfig{Provider: "a"},
			ProviderConfig: appconfig.ProviderConfig{
				OllamaBaseURL: "http://x",
				OllamaModel:   "m",
			},
		}, req: requestArgs{model: "m"}},
		{index: 1, provider: "b", cfg: appconfig.App{
			CoreConfig: appconfig.CoreConfig{Provider: "b"},
			ProviderConfig: appconfig.ProviderConfig{
				OllamaBaseURL: "http://x",
				OllamaModel:   "m",
			},
		}, req: requestArgs{model: "m"}},
	}
	msgs := buildMessages("hello")
	var stdout, stderr bytes.Buffer

	// Test writeCLIJobOutputs and writeCLIJobSummaries directly
	// since executeCLIJobs with multiple jobs uses a column printer.
	_ = jobs
	_ = msgs
	results := []*cliJobResult{
		{provider: "a", model: "m1", output: "first"},
		{provider: "b", model: "m2", output: "second"},
	}
	if err := writeCLIJobOutputs(&stdout, results); err != nil {
		t.Fatalf("writeCLIJobOutputs: %v", err)
	}
	if err := writeCLIJobSummaries(&stderr, results); err != nil {
		t.Fatalf("writeCLIJobSummaries: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "=== a:m1 ===") || !strings.Contains(got, "=== b:m2 ===") {
		t.Fatalf("expected headings, got %q", got)
	}

	// Also test the runCLIJobs single-job (streaming) path.
	singleJobs := []cliJob{
		{index: 0, provider: "a", cfg: appconfig.App{
			CoreConfig: appconfig.CoreConfig{Provider: "a"},
			ProviderConfig: appconfig.ProviderConfig{
				OllamaBaseURL: "http://x",
				OllamaModel:   "m",
			},
		}, req: requestArgs{model: "m"}},
	}
	stdout.Reset()
	stderr.Reset()
	if err := runCLIJobs(context.Background(), singleJobs, msgs, "hello", &stdout, &stderr, newClientFromApp, nil); err != nil {
		t.Fatalf("runCLIJobs single: %v", err)
	}
	if !strings.Contains(stdout.String(), "out-a") {
		t.Fatalf("expected single job output, got %q", stdout.String())
	}
}

func TestWithCLISelection_NilContext(t *testing.T) {
	ctx := WithCLISelection(nil, []int{1, 2})
	got := selectionFromContext(ctx)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("unexpected selection: %v", got)
	}
}

func TestPrintCLIHeader_EmptyJobs(t *testing.T) {
	var buf bytes.Buffer
	printCLIHeader(&buf, nil, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for empty jobs, got %q", buf.String())
	}
}

func TestWriteCachedCLIJobOutput_StreamWriteError(t *testing.T) {
	w := errWriter{err: errors.New("write fail")}
	err := writeCachedCLIJobOutput("data", w, nil, 0, true)
	if err == nil || !strings.Contains(err.Error(), "write fail") {
		t.Fatalf("expected write error, got %v", err)
	}
}

// chatErrClient fails on Chat but not Name/DefaultModel.
type chatErrClient struct {
	fakeClient
	chatErr error
}

func (c *chatErrClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "", c.chatErr
}

func TestRunSimpleChat_ChatError(t *testing.T) {
	client := &chatErrClient{
		fakeClient: fakeClient{name: "p", model: "m"},
		chatErr:    fmt.Errorf("chat broken"),
	}
	var out bytes.Buffer
	_, err := runChatRequest(context.Background(), client, requestArgs{}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "chat broken") {
		t.Fatalf("expected chat error, got %v", err)
	}
}

func TestWriteCLIJobSummary_WithError(t *testing.T) {
	var buf bytes.Buffer
	res := &cliJobResult{provider: "p", model: "m", err: fmt.Errorf("boom"), summary: ""}
	if err := writeCLIJobSummary(&buf, res); err != nil {
		t.Fatalf("writeCLIJobSummary: %v", err)
	}
	if !strings.Contains(buf.String(), "boom") || !strings.Contains(buf.String(), "provider=p model=m") {
		t.Fatalf("expected error info, got %q", buf.String())
	}
}

func TestWriteCLIJobSummaries_FirstError(t *testing.T) {
	results := []*cliJobResult{
		{provider: "a", model: "m", err: nil},
		{provider: "b", model: "m", err: fmt.Errorf("fail")},
	}
	var buf bytes.Buffer
	err := writeCLIJobSummaries(&buf, results)
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Fatalf("expected first error, got %v", err)
	}
}

func TestFilterJobsBySelection_Dedup(t *testing.T) {
	jobs := []cliJob{{index: 0, provider: "a"}, {index: 1, provider: "b"}}
	filtered, err := filterJobsBySelection(jobs, []int{0, 0, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 jobs after dedup, got %d", len(filtered))
	}
}

func TestWriteCLIJobOutputs_SeparatorBetweenMultiple(t *testing.T) {
	var buf bytes.Buffer
	results := []*cliJobResult{
		{provider: "a", model: "m", output: "one\n"},
		nil,
		{provider: "b", model: "m", output: "two\n"},
	}
	if err := writeCLIJobOutputs(&buf, results); err != nil {
		t.Fatalf("writeCLIJobOutputs: %v", err)
	}
	got := buf.String()
	// Should have a blank line separator between the two non-nil results.
	if !strings.Contains(got, "one\n\n") {
		t.Fatalf("expected separator between results, got %q", got)
	}
}

func TestRunChatRequest_NonStreamer(t *testing.T) {
	client := &fakeClient{name: "p", model: "m", resp: "hello"}
	var out bytes.Buffer
	got, err := runChatRequest(context.Background(), client, requestArgs{model: "m"}, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

func TestRunChatRequest_Streamer(t *testing.T) {
	client := &fakeStreamer{
		fakeClient: fakeClient{name: "p", model: "m"},
		chunks:     []string{"a", "b"},
	}
	var out bytes.Buffer
	got, err := runChatRequest(context.Background(), client, requestArgs{model: "m"}, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ab" {
		t.Fatalf("expected ab, got %q", got)
	}
}

func TestSelectionFromContext_Nil(t *testing.T) {
	if got := selectionFromContext(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSelectionFromContext_NoValue(t *testing.T) {
	if got := selectionFromContext(context.Background()); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestFilterJobsBySelection_Empty(t *testing.T) {
	jobs := []cliJob{{index: 0, provider: "a"}}
	filtered, err := filterJobsBySelection(jobs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected original jobs, got %d", len(filtered))
	}
}
