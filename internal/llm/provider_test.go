package llm

import (
	"context"
	"testing"
)

func TestNewFromConfig_DefaultsAndErrors(t *testing.T) {
	// Unknown provider
	if _, err := NewFromConfig(Config{Provider: "bogus"}, "", ""); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
	// OpenAI missing key
	if _, err := NewFromConfig(Config{Provider: "openai", OpenAIModel: "g"}, "", ""); err == nil {
		t.Fatalf("expected key error")
	}
	// Copilot missing key
	if _, err := NewFromConfig(Config{Provider: "copilot", CopilotModel: "m"}, "", ""); err == nil {
		t.Fatalf("expected key error")
	}
}

type fakeClientMin struct{}

func (fakeClientMin) Chat(context.Context, []Message, ...RequestOption) (string, error) {
	return "", nil
}
func (fakeClientMin) Name() string         { return "x" }
func (fakeClientMin) DefaultModel() string { return "m" }
