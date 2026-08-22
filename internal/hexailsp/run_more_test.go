package hexailsp

import (
	"bytes"
	"context"
	"io"
	"log"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/lsp"
	"github.com/snonux/hexai/internal/runtimeconfig"
)

type recRunner struct{ ran bool }

func (r *recRunner) Run(context.Context) error { r.ran = true; return nil }

type applyRunner struct{ opts []lsp.ServerOptions }

func (r *applyRunner) Run(context.Context) error           { return nil }
func (r *applyRunner) ApplyOptions(opts lsp.ServerOptions) { r.opts = append(r.opts, opts) }

type stubClient struct{}

func (stubClient) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", nil
}
func (stubClient) Name() string         { return "stub" }
func (stubClient) DefaultModel() string { return "stub-model" }

type recordingStatusSink struct{}

func (recordingStatusSink) SetLLMStart(string, string) error { return nil }
func (recordingStatusSink) SetGlobal(lsp.GlobalStatus) error { return nil }

func TestRunWithFactory_BuildsOptionsAndClient(t *testing.T) {
	var captured lsp.ServerOptions
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		captured = opts
		return &recRunner{}
	}
	var in, out bytes.Buffer
	logger := log.New(&out, "", 0)
	cfg := appconfig.Load(context.Background(), logger)
	// Use ollama to avoid API keys
	cfg.Provider = "ollama"
	cfg.MaxTokens = 123
	cfg.PromptCodeActionRewriteSystem = "RSYS"
	cfg.PromptCodeActionRewriteUser = "RUSER"
	if err := RunWithFactory(context.Background(), "", "", &in, &out, logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if captured.Config == nil {
		t.Fatalf("expected Config to be set in ServerOptions")
	}
	if captured.Config.MaxTokens != 123 {
		t.Fatalf("opts not applied: %+v", captured)
	}
	if captured.Config.PromptCodeActionRewriteSystem != "RSYS" || captured.Config.PromptCodeActionRewriteUser != "RUSER" {
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
	cfg := appconfig.Load(context.Background(), nil)
	cfg.StatsWindowMinutes = 0
	cfg.ContextMode = " WINDOW "
	if err := RunWithFactory(context.Background(), "", "", &in, &out, logger, cfg, stubClient{}, factory); err != nil {
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
	if latest.Config == nil {
		t.Fatalf("expected Config on latest options")
	}
	if latest.Config.MaxTokens != updated.MaxTokens {
		t.Fatalf("expected updated max tokens, got %+v", latest)
	}
	if latest.Config.ContextMode != "always-full" {
		t.Fatalf("expected normalized context mode, got %+v", latest)
	}
}

func TestRunWithDependencies_UsesInjectedClientBuilderAndStatusSink(t *testing.T) {
	var captured lsp.ServerOptions
	sink := &recordingStatusSink{}
	buildCalls := 0
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		captured = opts
		return &recRunner{}
	}
	cfg := appconfig.Load(context.Background(), nil)
	if err := runWithDependencies(context.Background(), "", "", bytes.NewBuffer(nil), bytes.NewBuffer(nil), log.New(io.Discard, "", 0), cfg, nil, factory, runDependencies{
		buildClient: func(appconfig.App, llm.Client) llm.Client {
			buildCalls++
			return stubClient{}
		},
		newConfigStore:   runtimeconfig.New,
		newIgnoreChecker: defaultIgnoreCheckerFactory,
		statusSink:       sink,
	}); err != nil {
		t.Fatalf("runWithDependencies error: %v", err)
	}
	if buildCalls != 1 {
		t.Fatalf("expected one client build, got %d", buildCalls)
	}
	if captured.Client == nil {
		t.Fatal("expected injected client to be passed through")
	}
	if captured.StatusSink != sink {
		t.Fatal("expected injected status sink to be passed through")
	}
}
