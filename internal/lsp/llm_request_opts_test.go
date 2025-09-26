package lsp

import (
	"context"
	"testing"

	"codeberg.org/snonux/hexai/internal/llm"
)

type fakeClient struct{ name, model string }

func (f fakeClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "", nil
}
func (f fakeClient) Name() string         { return f.name }
func (f fakeClient) DefaultModel() string { return f.model }

func TestRequestSpec_Gpt5_ForcesTemp1(t *testing.T) {
	s := newTestServer()
	one := 0.2
	s.cfg.CodingTemperature = &one
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
	if model := spec.effectiveModel(); model != "gpt-5.0" {
		t.Fatalf("expected fallback model gpt-5.0, got %q", model)
	}
}
