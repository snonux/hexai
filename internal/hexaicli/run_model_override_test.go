package hexaicli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type fakeClientModelEnv struct{ name, model string }

func (f fakeClientModelEnv) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "ok", nil
}
func (f fakeClientModelEnv) Name() string         { return f.name }
func (f fakeClientModelEnv) DefaultModel() string { return f.model }

// Ensure that HEXAI_MODEL overrides config for CLI runs.
func TestRun_ModelEnvOverride_FlowsIntoClient(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HEXAI_MODEL", "gpt-5-codex")
	t.Setenv("HEXAI_PROVIDER", "openai")
	// Replace client constructor to assert model was overridden
	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()
	var seenModel string
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		seenModel = strings.TrimSpace(cfg.OpenAIModel)
		return fakeClientModelEnv{name: "openai", model: cfg.OpenAIModel}, nil
	}

	var out, errb bytes.Buffer
	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if seenModel != "gpt-5-codex" {
		t.Fatalf("expected cfg.OpenAIModel=gpt-5-codex, got %q", seenModel)
	}
	if !strings.Contains(errb.String(), "openai:gpt-5-codex") {
		t.Fatalf("stderr should print effective model, got: %s", errb.String())
	}
}
