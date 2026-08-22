package lsp

import (
	"errors"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
)

func TestLLMClientRegistryClientFor_CachesAlternateProviders(t *testing.T) {
	registry := newLLMClientRegistry()
	registry.applyOptions(fakeClient{name: "openai", model: "gpt-5.0"}, "openai")

	cfg := appconfig.App{}
	spec := requestSpec{
		provider:      "anthropic",
		entry:         appconfig.SurfaceConfig{Model: "claude-3-7-sonnet"},
		fallbackModel: "claude-3-7-sonnet",
	}

	buildCalls := 0
	builder := func(_ appconfig.App, provider, modelOverride string) (llm.Client, error) {
		buildCalls++
		return fakeClient{name: provider, model: modelOverride}, nil
	}

	first := registry.clientFor(spec, cfg, builder)
	second := registry.clientFor(spec, cfg, builder)
	if first == nil || second == nil {
		t.Fatal("expected alternate provider client")
	}
	if buildCalls != 1 {
		t.Fatalf("expected one build for cached alternate client, got %d", buildCalls)
	}
	if first.Name() != "anthropic" || second.Name() != "anthropic" {
		t.Fatalf("expected anthropic client, got %q and %q", first.Name(), second.Name())
	}
}

func TestLLMClientRegistryClientFor_FallsBackToBaseClientOnBuildError(t *testing.T) {
	base := fakeClient{name: "openai", model: "gpt-5.0"}
	registry := newLLMClientRegistry()
	registry.applyOptions(base, "openai")

	spec := requestSpec{provider: "anthropic"}
	builder := func(appconfig.App, string, string) (llm.Client, error) {
		return nil, errors.New("boom")
	}

	got := registry.clientFor(spec, appconfig.App{}, builder)
	if got != base {
		t.Fatalf("expected base client fallback, got %#v", got)
	}
}

func TestLLMClientRegistryCurrent_ReturnsConfiguredClient(t *testing.T) {
	registry := newLLMClientRegistry()
	client := fakeClient{name: "openrouter", model: "gpt-4.1-mini"}
	registry.applyOptions(client, "openrouter")

	if got := registry.current(); got != client {
		t.Fatalf("expected configured client, got %#v", got)
	}
}
