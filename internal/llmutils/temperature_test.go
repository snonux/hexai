package llmutils

import "testing"

func floatPtr(v float64) *float64 { return &v }

func TestResolveTemperature_EntryTempTakesPrecedence(t *testing.T) {
	temp, ok := ResolveTemperature("openai", "gpt-5.0", floatPtr(0.5), floatPtr(0.2))
	if !ok || temp != 0.5 {
		t.Fatalf("expected entry temp 0.5, got %v ok=%v", temp, ok)
	}
}

func TestResolveTemperature_CodingTempUsed(t *testing.T) {
	temp, ok := ResolveTemperature("openai", "gpt-4.1", nil, floatPtr(0.3))
	if !ok || temp != 0.3 {
		t.Fatalf("expected coding temp 0.3, got %v ok=%v", temp, ok)
	}
}

func TestResolveTemperature_GPT5UpgradesDefault02(t *testing.T) {
	temp, ok := ResolveTemperature("openai", "gpt-5.1", nil, floatPtr(0.2))
	if !ok || temp != 1.0 {
		t.Fatalf("expected upgraded temp 1.0 for gpt-5 with 0.2, got %v ok=%v", temp, ok)
	}
}

func TestResolveTemperature_GPT5DefaultWhenNoTemp(t *testing.T) {
	temp, ok := ResolveTemperature("openai", "gpt-5.0-preview", nil, nil)
	if !ok || temp != 1.0 {
		t.Fatalf("expected default 1.0 for gpt-5, got %v ok=%v", temp, ok)
	}
}

func TestResolveTemperature_NoTempNonGPT5(t *testing.T) {
	_, ok := ResolveTemperature("openai", "gpt-4.1", nil, nil)
	if ok {
		t.Fatal("expected no temperature for non-gpt-5 with no config")
	}
}

func TestResolveTemperature_NonOpenAIProviderNoUpgrade(t *testing.T) {
	temp, ok := ResolveTemperature("anthropic", "gpt-5.0", nil, floatPtr(0.2))
	if !ok || temp != 0.2 {
		t.Fatalf("expected 0.2 for non-openai provider, got %v ok=%v", temp, ok)
	}
}

func TestResolveTemperature_GPT5CodingTempNon02NotUpgraded(t *testing.T) {
	temp, ok := ResolveTemperature("openai", "gpt-5.0", nil, floatPtr(0.7))
	if !ok || temp != 0.7 {
		t.Fatalf("expected 0.7 (no upgrade for non-0.2), got %v ok=%v", temp, ok)
	}
}

func TestResolveTemperature_CaseInsensitiveModel(t *testing.T) {
	temp, ok := ResolveTemperature("openai", "GPT-5.0", nil, nil)
	if !ok || temp != 1.0 {
		t.Fatalf("expected 1.0 for case-insensitive GPT-5, got %v ok=%v", temp, ok)
	}
}

func TestIsGPT5(t *testing.T) {
	tests := []struct {
		provider, model string
		want            bool
	}{
		{"openai", "gpt-5.0", true},
		{"openai", "GPT-5.1-preview", true},
		{"openai", "gpt-4.1", false},
		{"anthropic", "gpt-5.0", false},
		{"openai", "", false},
	}
	for _, tt := range tests {
		if got := isGPT5(tt.provider, tt.model); got != tt.want {
			t.Errorf("isGPT5(%q, %q) = %v, want %v", tt.provider, tt.model, got, tt.want)
		}
	}
}
