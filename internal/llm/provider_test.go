package llm

import (
	"strings"
	"testing"
)

func TestNewFromConfig_DefaultsAndErrors(t *testing.T) {
	// Unknown provider
	if _, err := NewFromConfig(Config{Provider: "bogus"}, "", "", "", ""); err == nil {
		t.Fatalf("expected error for unknown provider")
	} else if !strings.Contains(err.Error(), "supported providers:") {
		t.Fatalf("expected supported providers hint, got %q", err.Error())
	}
	// OpenAI missing key
	if _, err := NewFromConfig(Config{Provider: "openai", OpenAIModel: "g"}, "", "", "", ""); err == nil {
		t.Fatalf("expected key error")
	} else if !strings.Contains(err.Error(), "OPENAI_API_KEY") || !strings.Contains(err.Error(), "HEXAI_OPENAI_API_KEY") {
		t.Fatalf("expected actionable API key hint, got %q", err.Error())
	}
}
