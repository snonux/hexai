package hexailsp

import (
	"bytes"
	"context"
	"io"
	"log"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/lsp"
	"codeberg.org/snonux/hexai/internal/runtimeconfig"
)

type recRunner struct{ ran bool }

func (r *recRunner) Run() error { r.ran = true; return nil }

type applyRunner struct{ opts []lsp.ServerOptions }

func (r *applyRunner) Run() error                          { return nil }
func (r *applyRunner) ApplyOptions(opts lsp.ServerOptions) { r.opts = append(r.opts, opts) }

type stubClient struct{}

func (stubClient) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", nil
}
func (stubClient) Name() string         { return "stub" }
func (stubClient) DefaultModel() string { return "stub-model" }

func TestRunWithFactory_BuildsOptionsAndClient(t *testing.T) {
	var captured lsp.ServerOptions
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		captured = opts
		return &recRunner{}
	}
	var in, out bytes.Buffer
	logger := log.New(&out, "", 0)
	cfg := appconfig.Load(logger)
	// Use ollama to avoid API keys
	cfg.Provider = "ollama"
	cfg.MaxTokens = 123
	cfg.PromptCodeActionRewriteSystem = "RSYS"
	cfg.PromptCodeActionRewriteUser = "RUSER"
	if err := RunWithFactory("", &in, &out, logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if captured.MaxTokens != 123 {
		t.Fatalf("opts not applied: %+v", captured)
	}
	if captured.PromptRewriteSystem != "RSYS" || captured.PromptRewriteUser != "RUSER" {
		t.Fatalf("prompts not mapped: %+v", captured)
	}
	if captured.Client == nil {
		t.Fatalf("expected client to be constructed")
	}
}

func TestRunWithFactory_SubscriptionAppliesUpdates(t *testing.T) {
	var in, out bytes.Buffer
	logger := log.New(io.Discard, "", 0)
	runner := &applyRunner{}
	var capturedStore *runtimeconfig.Store
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		capturedStore = opts.ConfigStore
		runner.opts = append(runner.opts, opts)
		return runner
	}
	cfg := appconfig.Load(nil)
	cfg.StatsWindowMinutes = 0
	cfg.ContextMode = " WINDOW "
	if err := RunWithFactory("", &in, &out, logger, cfg, stubClient{}, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if capturedStore == nil {
		t.Fatal("expected config store to be passed to factory")
	}
	if len(runner.opts) == 0 {
		t.Fatal("expected initial options to be recorded")
	}
	updated := cfg
	updated.MaxTokens = cfg.MaxTokens + 10
	updated.ContextMode = "always-full"
	capturedStore.Set(updated)
	if len(runner.opts) < 2 {
		t.Fatalf("expected ApplyOptions to be invoked on config update, got %d calls", len(runner.opts))
	}
	latest := runner.opts[len(runner.opts)-1]
	if latest.MaxTokens != updated.MaxTokens {
		t.Fatalf("expected updated max tokens, got %+v", latest)
	}
	if latest.ContextMode != "always-full" {
		t.Fatalf("expected normalized context mode, got %+v", latest)
	}
}
