// Summary: Tests for the Hexai LSP runner using a fake server factory and environment keys.
// Not yet reviewed by a human
package hexailsp

import (
    "bytes"
    "log"
    "io"
    "os"
    "path/filepath"
    "testing"

    "hexai/internal/appconfig"
    "hexai/internal/llm"
    "hexai/internal/lsp"
    "hexai/internal/logging"
)

// fake server capturing options and recording run calls
type fakeServer struct{
    ran bool
    opts lsp.ServerOptions
}
func (f *fakeServer) Run() error { f.ran = true; return nil }

func TestRunWithFactory_UsesDefaultsAndCallsServer(t *testing.T) {
    old := os.Getenv("OPENAI_API_KEY")
    t.Cleanup(func(){ _ = os.Setenv("OPENAI_API_KEY", old) })
    _ = os.Setenv("OPENAI_API_KEY", "")

    var stderr bytes.Buffer
    logger := log.New(&stderr, "hexai-lsp ", 0)
    cfg := appconfig.Load(nil) // defaults
    var gotOpts lsp.ServerOptions
    factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
        gotOpts = opts
        return &fakeServer{opts: opts}
    }
    if err := RunWithFactory("", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
        t.Fatalf("RunWithFactory error: %v", err)
    }
    if gotOpts.MaxTokens != cfg.MaxTokens {
        t.Fatalf("MaxTokens want %d got %d", cfg.MaxTokens, gotOpts.MaxTokens)
    }
    if gotOpts.ContextMode != cfg.ContextMode {
        t.Fatalf("ContextMode want %q got %q", cfg.ContextMode, gotOpts.ContextMode)
    }
    if gotOpts.WindowLines != cfg.ContextWindowLines {
        t.Fatalf("WindowLines want %d got %d", cfg.ContextWindowLines, gotOpts.WindowLines)
    }
    if gotOpts.MaxContextTokens != cfg.MaxContextTokens {
        t.Fatalf("MaxContextTokens want %d got %d", cfg.MaxContextTokens, gotOpts.MaxContextTokens)
    }
    if gotOpts.NoDiskIO != cfg.NoDiskIO {
        t.Fatalf("NoDiskIO want %v got %v", cfg.NoDiskIO, gotOpts.NoDiskIO)
    }
    if gotOpts.Client != nil { // with no env, openai client fails to build
        t.Fatalf("expected nil client when API key missing")
    }
}

func TestRunWithFactory_BuildsClientWhenKeysPresent(t *testing.T) {
    // Set a dummy OpenAI key to allow client creation
    old := os.Getenv("OPENAI_API_KEY")
    t.Cleanup(func(){ _ = os.Setenv("OPENAI_API_KEY", old) })
    _ = os.Setenv("OPENAI_API_KEY", "dummy")

    var stderr bytes.Buffer
    logger := log.New(&stderr, "hexai-lsp ", 0)
    cfg := appconfig.Load(nil) // defaults, provider=openai by default
    var got llm.Client
    factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
        got = opts.Client
        return &fakeServer{opts: opts}
    }
    if err := RunWithFactory("", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
        t.Fatalf("RunWithFactory error: %v", err)
    }
    if got == nil {
        t.Fatalf("expected non-nil client when OPENAI_API_KEY is set")
    }
}

func TestRun_RespectsLogPathFlag(t *testing.T) {
    tmp := t.TempDir()
    logFile := filepath.Join(tmp, "hexai-lsp.log")
    // Run with real Run but nil env key so client disabled; ensure no panic and file created
    if err := Run(logFile, bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err != nil {
        t.Fatalf("Run error: %v", err)
    }
    if _, err := os.Stat(logFile); err != nil {
        t.Fatalf("expected log file to be created: %v", err)
    }
}

func TestRunWithFactory_NormalizesContextMode_AndSetsPreviewLimit(t *testing.T) {
    t.Cleanup(func(){ logging.SetLogPreviewLimit(0) })
    var stderr bytes.Buffer
    logger := log.New(&stderr, "hexai-lsp ", 0)
    cfg := appconfig.App{
        ContextMode:     "  File-On-New-Func  ",
        LogPreviewLimit: 3,
    }
    var gotOpts lsp.ServerOptions
    factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
        gotOpts = opts
        return &fakeServer{opts: opts}
    }
    if err := RunWithFactory("", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
        t.Fatalf("RunWithFactory error: %v", err)
    }
    if gotOpts.ContextMode != "file-on-new-func" {
        t.Fatalf("ContextMode not normalized: %q", gotOpts.ContextMode)
    }
    if logging.PreviewForLog("abcdef") != "abc…" {
        t.Fatalf("PreviewForLog not respecting limit: %q", logging.PreviewForLog("abcdef"))
    }
}

func TestRunWithFactory_LogContextFlag(t *testing.T) {
    var stderr bytes.Buffer
    logger := log.New(&stderr, "hexai-lsp ", 0)
    cfg := appconfig.App{}
    var got1, got2 lsp.ServerOptions
    first := true
    factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
        if first {
            got1 = opts
            first = false
        } else {
            got2 = opts
        }
        return &fakeServer{opts: opts}
    }
    if err := RunWithFactory("/tmp/some.log", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
        t.Fatalf("RunWithFactory error: %v", err)
    }
    if !got1.LogContext {
        t.Fatalf("expected LogContext true when logPath is non-empty")
    }
    if err := RunWithFactory("", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
        t.Fatalf("RunWithFactory error: %v", err)
    }
    if got2.LogContext {
        t.Fatalf("expected LogContext false when logPath is empty")
    }
}
