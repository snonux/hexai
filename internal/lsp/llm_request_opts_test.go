package lsp

import (
	"context"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
)

type fakeClient struct{ name, model string }

func (f fakeClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "", nil
}
func (f fakeClient) Name() string         { return f.name }
func (f fakeClient) DefaultModel() string { return f.model }

func floatPtr(v float64) *float64 {
	return &v
}

func TestRequestSpec_Gpt5_ForcesTemp1(t *testing.T) {
	s := newTestServer()
	one := 0.2
	s.cfg.CodingTemperature = &one
	// Pin Provider explicitly: the in-code default is now ollama, but the
	// gpt-5 temperature-force rule only fires for openai.
	s.cfg.Provider = "openai"
	s.llmClient = fakeClient{name: "openai", model: "gpt-5.0"}
	s.cfg.OpenAIModel = "gpt-5.0"

	spec := s.buildRequestSpec(surfaceCompletion)
	var got llm.Options
	for _, o := range spec.options {
		o(&got)
	}
	if got.Temperature != 1.0 {
		t.Fatalf("expected temp 1.0 for gpt-5, got %v", got.Temperature)
	}
	if model := spec.effectiveModel(s.llmClient.DefaultModel()); model != "gpt-5.0" {
		t.Fatalf("expected fallback model gpt-5.0, got %q", model)
	}
}

func TestBuildRequestSpecs_MultiEntries(t *testing.T) {
	s := newTestServer()
	s.cfg.CompletionConfigs = []appconfig.SurfaceConfig{
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "anthropic", Model: "claude", Temperature: floatPtr(0.4)},
	}
	s.cfg.OpenAIModel = "gpt-3.5"
	s.cfg.AnthropicModel = "claude-base"
	s.cfg.MaxTokens = 256
	specs := s.buildRequestSpecs(surfaceCompletion)
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].provider != "openai" || specs[0].index != 0 {
		t.Fatalf("unexpected first spec: %+v", specs[0])
	}
	if specs[1].provider != "anthropic" || specs[1].index != 1 {
		t.Fatalf("unexpected second spec: %+v", specs[1])
	}
	var opts1, opts2 llm.Options
	for _, opt := range specs[0].options {
		opt(&opts1)
	}
	for _, opt := range specs[1].options {
		opt(&opts2)
	}
	if opts1.Model != "gpt-4o" || opts1.MaxTokens != 256 {
		t.Fatalf("unexpected opts1: %+v", opts1)
	}
	if opts2.Model != "claude" || opts2.Temperature != 0.4 {
		t.Fatalf("unexpected opts2: %+v", opts2)
	}
}
