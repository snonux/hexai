// Tests for the Hexai LSP runner using a fake server factory and environment keys.
package hexailsp

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/lsp"
)

// TestMain registers all built-in LLM providers before tests run, mirroring
// the explicit registration done in production binaries via RunWithConfig.
func TestMain(m *testing.M) {
	if err := llm.RegisterAllProviders(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// fake server capturing options and recording run calls
type fakeServer struct {
	ran  bool
	opts lsp.ServerOptions
}

func (f *fakeServer) Run() error { f.ran = true; return nil }

func TestRunWithFactory_UsesDefaultsAndCallsServer(t *testing.T) {
	old := os.Getenv("OPENAI_API_KEY")
	t.Cleanup(func() { _ = os.Setenv("OPENAI_API_KEY", old) })
	_ = os.Setenv("OPENAI_API_KEY", "")

	var stderr bytes.Buffer
	logger := log.New(&stderr, "hexai-lsp-server ", 0)
	cfg := appconfig.Load(nil) // defaults
	// Pin provider to openai: the in-code default is now ollama, which would
	// happily build a client without a key and short-circuit the missing-key
	// assertion below. Load(nil) returns raw defaults and ignores env vars,
	// so set the field directly on the struct.
	cfg.Provider = "openai"
	var gotOpts lsp.ServerOptions
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		gotOpts = opts
		return &fakeServer{opts: opts}
	}
	if err := RunWithFactory("", "", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if gotOpts.Config == nil {
		t.Fatalf("expected Config to be set in ServerOptions")
	}
	if gotOpts.Config.MaxTokens != cfg.MaxTokens {
		t.Fatalf("MaxTokens want %d got %d", cfg.MaxTokens, gotOpts.Config.MaxTokens)
	}
	if gotOpts.Config.ContextMode != cfg.ContextMode {
		t.Fatalf("ContextMode want %q got %q", cfg.ContextMode, gotOpts.Config.ContextMode)
	}
	if gotOpts.Config.ContextWindowLines != cfg.ContextWindowLines {
		t.Fatalf("ContextWindowLines want %d got %d", cfg.ContextWindowLines, gotOpts.Config.ContextWindowLines)
	}
	if gotOpts.Config.MaxContextTokens != cfg.MaxContextTokens {
		t.Fatalf("MaxContextTokens want %d got %d", cfg.MaxContextTokens, gotOpts.Config.MaxContextTokens)
	}

	if gotOpts.Client != nil { // with no env, openai client fails to build
		t.Fatalf("expected nil client when API key missing")
	}
}

func TestRunWithFactory_BuildsClientWhenKeysPresent(t *testing.T) {
	// Set a dummy OpenAI key to allow client creation
	old := os.Getenv("OPENAI_API_KEY")
	t.Cleanup(func() { _ = os.Setenv("OPENAI_API_KEY", old) })
	_ = os.Setenv("OPENAI_API_KEY", "dummy")

	var stderr bytes.Buffer
	logger := log.New(&stderr, "hexai-lsp-server ", 0)
	cfg := appconfig.Load(nil) // defaults
	// Pin provider to openai (the in-code default is now ollama). Load(nil)
	// returns raw defaults and ignores env vars, so set this on the struct.
	cfg.Provider = "openai"
	var got llm.Client
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		got = opts.Client
		return &fakeServer{opts: opts}
	}
	if err := RunWithFactory("", "", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil client when OPENAI_API_KEY is set")
	}
}

func TestRun_RespectsLogPathFlag(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "hexai-lsp-server.log")
	// Run with real Run but nil env key so client disabled; ensure no panic and file created
	if err := Run(logFile, bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("expected log file to be created: %v", err)
	}
}

func TestRunWithFactory_NormalizesContextMode_AndSetsPreviewLimit(t *testing.T) {
	t.Cleanup(func() { logging.SetLogPreviewLimit(0) })
	var stderr bytes.Buffer
	logger := log.New(&stderr, "hexai-lsp-server ", 0)
	cfg := appconfig.App{
		CoreConfig: appconfig.CoreConfig{
			ContextMode:     "  File-On-New-Func  ",
			LogPreviewLimit: 3,
		},
	}
	var gotOpts lsp.ServerOptions
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		gotOpts = opts
		return &fakeServer{opts: opts}
	}
	if err := RunWithFactory("", "", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if gotOpts.Config == nil {
		t.Fatalf("expected Config to be set in ServerOptions")
	}
	if gotOpts.Config.ContextMode != "file-on-new-func" {
		t.Fatalf("ContextMode not normalized: %q", gotOpts.Config.ContextMode)
	}
	if logging.PreviewForLog("abcdef") != "abc…" {
		t.Fatalf("PreviewForLog not respecting limit: %q", logging.PreviewForLog("abcdef"))
	}
}

func TestRunWithFactory_LogContextFlag(t *testing.T) {
	var stderr bytes.Buffer
	logger := log.New(&stderr, "hexai-lsp-server ", 0)
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
	if err := RunWithFactory("/tmp/some.log", "", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if !got1.LogContext {
		t.Fatalf("expected LogContext true when logPath is non-empty")
	}
	if err := RunWithFactory("", "", bytes.NewBuffer(nil), bytes.NewBuffer(nil), logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if got2.LogContext {
		t.Fatalf("expected LogContext false when logPath is empty")
	}
}
