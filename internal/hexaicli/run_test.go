package hexaicli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

func floatPtr(v float64) *float64 {
	x := v
	return &x
}

func TestReadInput_Combinations(t *testing.T) {
	// stdin + arg
	restore, f := setStdin(t, "from-stdin")
	defer restore()
	s, err := readInput(f, []string{"from-arg"})
	if err != nil || !strings.HasPrefix(s, "from-arg:\n\nfrom-stdin") {
		t.Fatalf("stdin+arg failed: %q %v", s, err)
	}
	// stdin only
	restore2, f2 := setStdin(t, "from-stdin")
	defer restore2()
	s, err = readInput(f2, nil)
	if err != nil || s != "from-stdin" {
		t.Fatalf("stdin only failed: %q %v", s, err)
	}
	// arg only
	s, err = readInput(strings.NewReader(""), []string{"arg1", "arg2"})
	if err != nil || s != "arg1 arg2" {
		t.Fatalf("arg only failed: %q %v", s, err)
	}
	// no input
	restore3, f3 := setStdin(t, "")
	defer restore3()
	_, err = readInput(f3, nil)
	if err == nil {
		t.Fatalf("expected error for no input")
	}
}

func TestReadInput_PropagatesStdinError(t *testing.T) {
	restore, _ := setStdin(t, "ignored")
	defer restore()
	bad := failingReader{err: io.ErrUnexpectedEOF}
	if _, err := readInput(bad, nil); err == nil || !strings.Contains(err.Error(), "failed to read stdin") {
		t.Fatalf("expected stdin read error, got %v", err)
	}
}

func TestBuildMessages_Explain(t *testing.T) {
	msgs := buildMessages("please explain this")
	if len(msgs) != 2 || msgs[0].Role != "system" || !strings.Contains(strings.ToLower(msgs[0].Content), "explanation") {
		t.Fatalf("unexpected system prompt: %#v", msgs)
	}
}

func TestBuildMessages_Default(t *testing.T) {
	msgs := buildMessages("just do it")
	if len(msgs) != 2 || msgs[0].Role != "system" || strings.Contains(msgs[0].Content, "requested an explanation") {
		t.Fatalf("unexpected system prompt: %#v", msgs)
	}
}

func TestRunChat_StreamAndNonStream(t *testing.T) {
	// stream path
	fc := &fakeStreamer{fakeClient: fakeClient{name: "p", model: "m"}, chunks: []string{"H", "i", "!"}}
	var out, errb bytes.Buffer
	req := requestArgs{model: fc.DefaultModel()}
	if err := runChat(context.Background(), fc, req, buildMessages("hello"), "hello", &out, &errb); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if out.String() != "Hi!" || !strings.Contains(errb.String(), "provider=p model=m") {
		t.Fatalf("bad output or summary: %q %q", out.String(), errb.String())
	}
	// non-stream path
	fc2 := &fakeClient{name: "p2", model: "m2", resp: "Yo"}
	out.Reset()
	errb.Reset()
	if err := runChat(context.Background(), fc2, requestArgs{model: fc2.DefaultModel()}, buildMessages("hello"), "hello", &out, &errb); err != nil {
		t.Fatalf("non-stream: %v", err)
	}
	if out.String() != "Yo" || !strings.Contains(errb.String(), "provider=p2 model=m2") {
		t.Fatalf("bad output or summary (non-stream)")
	}
}

type clientErr struct{ name, model string }

func (c clientErr) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", io.EOF
}
func (c clientErr) Name() string         { return c.name }
func (c clientErr) DefaultModel() string { return c.model }

func TestRunChat_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	if err := runChat(ctx, clientErr{"p", "m"}, requestArgs{model: "m"}, buildMessages("hi"), "hi", out, errb); err == nil {
		t.Fatalf("expected error from Chat")
	}
}

func TestRunWithClient_ErrorPrint(t *testing.T) {
	var out, errb bytes.Buffer
	err := RunWithClient(context.Background(), []string{"hi"}, strings.NewReader(""), &out, &errb, clientErr{"p", "m"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(errb.String(), "hexai: error:") {
		t.Fatalf("expected error line, got %q", errb.String())
	}
}

func TestRun_OpenAI_NoKey_ShowsError(t *testing.T) {
	dir := testingTempDir(t)
	// write config with provider=openai using sectioned tables
	configPath := filepath.Join(dir, "hexai", "config.toml")
	writeConfigString(t, configPath, `
[provider]
name = "openai"

[openai]
model = "gpt-x"
`)
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Ensure no OpenAI API key is present in environment
	t.Setenv("HEXAI_OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("HEXAI_PROVIDER", "")
	var out, errb bytes.Buffer
	// Run expects parsed flags; here args irrelevant
	err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &out, &errb)
	if err == nil {
		t.Fatalf("expected error due to missing API key")
	}
	// Accept either explicit "LLM disabled" or a generic provider error emitted by Run.
	if !strings.Contains(errb.String(), "LLM disabled") && !strings.Contains(errb.String(), "openai error") && !strings.Contains(errb.String(), "hexai: error:") {
		t.Fatalf("expected disabled-or-error message, got %q", errb.String())
	}
}

func TestPrintProviderInfo(t *testing.T) {
	var b bytes.Buffer
	printProviderInfo(&b, &fakeClient{name: "x", model: "y"}, "y")
	// Expect compact "x:y:" label with no divider line.
	if !strings.Contains(b.String(), "x:y:") {
		t.Fatalf("missing provider label: %q", b.String())
	}
	if strings.Contains(b.String(), "─") {
		t.Fatalf("unexpected divider in single-provider header: %q", b.String())
	}
	if strings.Contains(b.String(), "provider=") {
		t.Fatalf("unexpected legacy provider line: %q", b.String())
	}
}

func TestRun_SingleProviderHeaderUsesStderr(t *testing.T) {
	// This test asserts an "openai:gpt-4.1:" header, so pin the provider/model
	// explicitly (the in-code default switched to ollama/gemma4).
	t.Setenv("HEXAI_PROVIDER", "openai")
	t.Setenv("HEXAI_OPENAI_MODEL", "gpt-4.1")
	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()
	newClientFromApp = func(_ appconfig.App) (llm.Client, error) {
		return &fakeClient{name: "openai", model: "gpt-4.1", resp: "OUT"}, nil
	}

	restore, f := setStdin(t, "hello")
	defer restore()

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), nil, f, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "OUT" {
		t.Fatalf("stdout = %q, want %q", got, "OUT")
	}
	// Single-provider header is now a compact "openai:gpt-4.1:" label with no divider.
	if !strings.Contains(stderr.String(), "openai:gpt-4.1:") {
		t.Fatalf("stderr missing provider label: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "─") {
		t.Fatalf("unexpected divider in single-provider stderr: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "openai:gpt-4.1") {
		t.Fatalf("stdout should not contain provider label: %q", stdout.String())
	}
}

func TestExecuteCLIJobs_MultiProviderHeaderUsesStderr(t *testing.T) {
	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		switch cfg.Provider {
		case "anthropic":
			return &fakeClient{name: "anthropic", model: "claude", resp: "RIGHT"}, nil
		default:
			return &fakeClient{name: "openai", model: "gpt-4.1", resp: "LEFT"}, nil
		}
	}

	jobs := []cliJob{
		{
			index:    0,
			provider: "openai",
			cfg: appconfig.App{
				CoreConfig:     appconfig.CoreConfig{Provider: "openai"},
				ProviderConfig: appconfig.ProviderConfig{OpenAIModel: "gpt-4.1"},
			},
			req: requestArgs{model: "gpt-4.1"},
		},
		{
			index:    1,
			provider: "anthropic",
			cfg: appconfig.App{
				CoreConfig:     appconfig.CoreConfig{Provider: "anthropic"},
				ProviderConfig: appconfig.ProviderConfig{AnthropicModel: "claude"},
			},
			req: requestArgs{model: "claude"},
		},
	}

	var stdout, stderr bytes.Buffer
	results, printer := executeCLIJobs(context.Background(), jobs, buildMessages("hello"), "hello", &stdout, &stderr, false, newClientFromApp, nil)
	if printer == nil {
		t.Fatalf("expected column printer for multi-provider run")
	}
	if len(results) != 2 || results[0] == nil || results[1] == nil {
		t.Fatalf("expected results for both jobs, got %#v", results)
	}
	if !strings.Contains(stderr.String(), "openai:gpt-4.1") || !strings.Contains(stderr.String(), "anthropic:claude") {
		t.Fatalf("stderr missing multi-provider header: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "openai:gpt-4.1") || strings.Contains(stdout.String(), "anthropic:claude") {
		t.Fatalf("stdout should not contain provider header: %q", stdout.String())
	}
}

func TestBuildCLIRequest_Override(t *testing.T) {
	cfg := appconfig.App{
		CoreConfig:     appconfig.CoreConfig{Provider: "openai"},
		ProviderConfig: appconfig.ProviderConfig{AnthropicModel: "claude-3-5-sonnet"},
	}
	entry := appconfig.SurfaceConfig{Provider: "anthropic", Model: "override", Temperature: floatPtr(0.7)}
	req := buildCLIRequest(entry, "anthropic", cfg)
	if req.model != "override" {
		t.Fatalf("expected model override, got %q", req.model)
	}
	var opts llm.Options
	for _, o := range req.options {
		o(&opts)
	}
	if opts.Model != "override" || opts.Temperature != 0.7 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestBuildCLIRequest_Gpt5Temp(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai", CodingTemperature: floatPtr(0.2)}}
	entry := appconfig.SurfaceConfig{}
	cfg.OpenAIModel = "gpt-5.1"
	req := buildCLIRequest(entry, "openai", cfg)
	if req.model != "gpt-5.1" {
		t.Fatalf("expected fallback model, got %q", req.model)
	}
	var opts llm.Options
	for _, o := range req.options {
		o(&opts)
	}
	if opts.Temperature != 1.0 {
		t.Fatalf("expected temp 1.0, got %v", opts.Temperature)
	}
}

func TestBuildCLIJobs_MultiEntries(t *testing.T) {
	cfg := appconfig.App{
		CoreConfig: appconfig.CoreConfig{Provider: "ollama"},
		ProviderConfig: appconfig.ProviderConfig{
			OllamaModel: "llama3",
			CLIConfigs: []appconfig.SurfaceConfig{
				{Provider: "openai", Model: "gpt-4o"},
				{Provider: "anthropic", Model: "claude"},
			},
		},
	}
	jobs, err := buildCLIJobs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].provider != "openai" || jobs[0].req.model != "gpt-4o" {
		t.Fatalf("unexpected first job: %+v", jobs[0])
	}
	if jobs[1].provider != "anthropic" || jobs[1].req.model != "claude" {
		t.Fatalf("unexpected second job: %+v", jobs[1])
	}
	if jobs[0].cfg.Provider != "openai" || jobs[1].cfg.Provider != "anthropic" {
		t.Fatalf("unexpected derived configs: %+v", jobs)
	}
}

func TestFilterJobsBySelection(t *testing.T) {
	jobs := []cliJob{{index: 0, provider: "openai"}, {index: 1, provider: "ollama"}, {index: 2, provider: "anthropic"}}
	filtered, err := filterJobsBySelection(jobs, []int{2, 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filtered) != 2 || filtered[0].provider != "anthropic" || filtered[1].provider != "openai" {
		t.Fatalf("unexpected filtered order: %+v", filtered)
	}
	if filtered[0].index != 0 || filtered[1].index != 1 {
		t.Fatalf("expected reindexed jobs, got %+v", filtered)
	}
	if _, err := filterJobsBySelection(jobs, []int{5}); err == nil {
		t.Fatalf("expected out-of-range error")
	}
}

func TestNewClientFromConfig_Ollama(t *testing.T) {
	cfg := appconfig.App{
		CoreConfig: appconfig.CoreConfig{Provider: "ollama"},
		ProviderConfig: appconfig.ProviderConfig{
			OllamaBaseURL: "http://x",
			OllamaModel:   "m",
		},
	}
	c, err := newClientFromConfig(cfg)
	if err != nil || c == nil {
		t.Fatalf("expected client: %v %v", c, err)
	}
}

func TestNewClientFromConfig_OpenAI_MissingKey(t *testing.T) {
	cfg := appconfig.App{
		CoreConfig: appconfig.CoreConfig{Provider: "openai"},
		ProviderConfig: appconfig.ProviderConfig{
			OpenAIBaseURL: "https://api",
			OpenAIModel:   "gpt",
		},
	}
	t.Setenv("HEXAI_OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := newClientFromConfig(cfg); err == nil {
		t.Fatalf("expected error for missing openai key")
	}
}
