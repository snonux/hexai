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

func TestReadInput_Combinations(t *testing.T) {
    // stdin + arg
    restore, f := setStdin(t, "from-stdin")
    defer restore()
    s, err := readInput(f, []string{"from-arg"})
    if err != nil || !strings.HasPrefix(s, "from-arg:\n\nfrom-stdin") { t.Fatalf("stdin+arg failed: %q %v", s, err) }
    // stdin only
    restore2, f2 := setStdin(t, "from-stdin")
    defer restore2()
    s, err = readInput(f2, nil)
    if err != nil || s != "from-stdin" { t.Fatalf("stdin only failed: %q %v", s, err) }
    // arg only
    s, err = readInput(strings.NewReader(""), []string{"arg1","arg2"})
    if err != nil || s != "arg1 arg2" { t.Fatalf("arg only failed: %q %v", s, err) }
    // no input
    restore3, f3 := setStdin(t, "")
    defer restore3()
    _, err = readInput(f3, nil)
    if err == nil { t.Fatalf("expected error for no input") }
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
    fc := &fakeStreamer{fakeClient: fakeClient{name: "p", model: "m"}, chunks: []string{"H","i","!"}}
    var out, errb bytes.Buffer
    if err := runChat(context.Background(), fc, buildMessages("hello"), "hello", &out, &errb); err != nil { t.Fatalf("stream: %v", err) }
    if out.String() != "Hi!" || !strings.Contains(errb.String(), "provider=p model=m") { t.Fatalf("bad output or summary: %q %q", out.String(), errb.String()) }
    // non-stream path
    fc2 := &fakeClient{name: "p2", model: "m2", resp: "Yo"}
    out.Reset(); errb.Reset()
    if err := runChat(context.Background(), fc2, buildMessages("hello"), "hello", &out, &errb); err != nil { t.Fatalf("non-stream: %v", err) }
    if out.String() != "Yo" || !strings.Contains(errb.String(), "provider=p2 model=m2") { t.Fatalf("bad output or summary (non-stream)") }
}

type clientErr struct{ name, model string }
func (c clientErr) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) { return "", io.EOF }
func (c clientErr) Name() string { return c.name }
func (c clientErr) DefaultModel() string { return c.model }

func TestRunChat_ErrorPaths(t *testing.T) {
    ctx := context.Background()
    out, errb := &bytes.Buffer{}, &bytes.Buffer{}
    if err := runChat(ctx, clientErr{"p","m"}, buildMessages("hi"), "hi", out, errb); err == nil {
        t.Fatalf("expected error from Chat")
    }
}

func TestRunWithClient_ErrorPrint(t *testing.T) {
    var out, errb bytes.Buffer
    err := RunWithClient(context.Background(), []string{"hi"}, strings.NewReader(""), &out, &errb, clientErr{"p","m"})
    if err == nil { t.Fatalf("expected error") }
    if !strings.Contains(errb.String(), "hexai: error:") {
        t.Fatalf("expected error line, got %q", errb.String())
    }
}

func TestRun_OpenAI_NoKey_ShowsError(t *testing.T) {
    dir := testingTempDir(t)
    // write config with provider=openai
    writeJSON(t, filepath.Join(dir, "hexai", "config.json"), map[string]any{"provider":"openai", "openai_model":"gpt-x"})
    t.Setenv("XDG_CONFIG_HOME", dir)
    // Ensure no OpenAI API key is present in environment
    t.Setenv("HEXAI_OPENAI_API_KEY", "")
    t.Setenv("OPENAI_API_KEY", "")
    var out, errb bytes.Buffer
    // Run expects parsed flags; here args irrelevant
    err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &out, &errb)
    if err == nil { t.Fatalf("expected error due to missing API key") }
    // Accept either explicit "LLM disabled" or a generic provider error emitted by Run.
    if !(strings.Contains(errb.String(), "LLM disabled") || strings.Contains(errb.String(), "openai error") || strings.Contains(errb.String(), "hexai: error:")) {
        t.Fatalf("expected disabled-or-error message, got %q", errb.String())
    }
}

func TestPrintProviderInfo(t *testing.T) {
    var b bytes.Buffer
    printProviderInfo(&b, &fakeClient{name:"x", model:"y"})
    if !strings.Contains(b.String(), "provider=x model=y") { t.Fatalf("missing provider line: %q", b.String()) }
}

func TestNewClientFromConfig_Ollama(t *testing.T) {
    cfg := appconfig.App{ Provider: "ollama", OllamaBaseURL: "http://x", OllamaModel: "m" }
    c, err := newClientFromConfig(cfg)
    if err != nil || c == nil { t.Fatalf("expected client: %v %v", c, err) }
}

func TestNewClientFromConfig_OpenAI_MissingKey(t *testing.T) {
    cfg := appconfig.App{ Provider: "openai", OpenAIBaseURL: "https://api", OpenAIModel: "gpt" }
    t.Setenv("HEXAI_OPENAI_API_KEY", "")
    t.Setenv("OPENAI_API_KEY", "")
    if _, err := newClientFromConfig(cfg); err == nil {
        t.Fatalf("expected error for missing openai key")
    }
}
