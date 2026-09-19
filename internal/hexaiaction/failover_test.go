package hexaiaction

import (
	"context"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/stats"
)

type actionFailoverClient struct {
	name string
	err  error
	text string
}

func (c actionFailoverClient) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return c.text, c.err
}
func (c actionFailoverClient) Name() string         { return c.name }
func (c actionFailoverClient) DefaultModel() string { return c.name + "-model" }

func TestPrepareActionClientFallsBack(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CodeActionConfigs: []appconfig.SurfaceConfig{{Provider: "openai", FallbackProvider: "ollama", FallbackModel: "local"}},
	}}
	factory := func(cfg appconfig.App) (actionClient, error) {
		if cfg.Provider == "openai" {
			return actionFailoverClient{name: "openai", err: &llm.HTTPError{Provider: "openai", Status: 503}}, nil
		}
		return actionFailoverClient{name: "ollama", text: "rewritten"}, nil
	}
	client, err := prepareActionClient(nilWriter{}, cfg, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "code"}})
	if err != nil || got != "rewritten" || client.DefaultModel() != "local" {
		t.Fatalf("fallback result=%q model=%q err=%v", got, client.DefaultModel(), err)
	}
}

func TestPrepareActionClientReportsBothFailures(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CodeActionConfigs: []appconfig.SurfaceConfig{{Provider: "openai", FallbackProvider: "ollama"}},
	}}
	factory := func(cfg appconfig.App) (actionClient, error) {
		return actionFailoverClient{name: cfg.Provider, err: &llm.HTTPError{Provider: cfg.Provider, Status: 503}}, nil
	}
	client, err := prepareActionClient(nilWriter{}, cfg, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "openai") || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("expected both target failures, got %v", err)
	}
}

func TestRunOnceAccountsResolvedFallbackModel(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HEXAI_TMUX_STATUS", "0")
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CodeActionConfigs: []appconfig.SurfaceConfig{{Provider: "openai", Model: "primary", FallbackProvider: "ollama", FallbackModel: "fallback"}},
	}}
	factory := func(cfg appconfig.App) (actionClient, error) {
		if cfg.Provider == "openai" {
			return actionFailoverClient{name: "openai", err: &llm.HTTPError{Provider: "openai", Status: 503}}, nil
		}
		return actionFailoverClient{name: "ollama", text: "rewritten"}, nil
	}
	client, err := prepareActionClient(nilWriter{}, cfg, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runOnce(context.Background(), client, "system", "selection", requestArgs{model: "primary", options: []llm.RequestOption{llm.WithModel("primary")}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := stats.TakeSnapshot()
	if err != nil || snapshot.ScopeReqs("ollama", "fallback") != 1 || snapshot.ScopeReqs("ollama", "primary") != 0 {
		t.Fatalf("unexpected fallback stats: snapshot=%+v err=%v", snapshot, err)
	}
}

type nilWriter struct{}

func (nilWriter) Write(p []byte) (int, error) { return len(p), nil }
