package llm

import "testing"

func TestNewFromConfig_DefaultTemp_ByModel(t *testing.T) {
	// OpenAI, gpt-5.* → default temp 1.0 when not provided
	cfg := Config{Provider: "openai", OpenAIModel: "gpt-5.0-preview"}
	c, err := NewFromConfig(cfg, "key", "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	oc, ok := c.(openAIClient)
	if !ok {
		t.Fatalf("expected openAIClient")
	}
	if oc.defaultTemperature == nil || *oc.defaultTemperature != 1.0 {
		t.Fatalf("expected default temp 1.0 for gpt-5, got %#v", oc.defaultTemperature)
	}
	// OpenAI, gpt-4.* → default temp 0.2 when not provided
	cfg2 := Config{Provider: "openai", OpenAIModel: "gpt-4.1"}
	c2, err := NewFromConfig(cfg2, "key", "")
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	oc2 := c2.(openAIClient)
	if oc2.defaultTemperature == nil || *oc2.defaultTemperature != 0.2 {
		t.Fatalf("expected default temp 0.2 for gpt-4.*, got %#v", oc2.defaultTemperature)
	}
}

func TestNewFromConfig_DefaultTemp_UpgradeWhenGpt5AndDefault02(t *testing.T) {
	// Simulate app-default of 0.2 while selecting a gpt-5 model: should upgrade to 1.0
	v := 0.2
	cfg := Config{Provider: "openai", OpenAIModel: "gpt-5.0", OpenAITemperature: &v}
	c, err := NewFromConfig(cfg, "key", "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	oc := c.(openAIClient)
	if oc.defaultTemperature == nil || *oc.defaultTemperature != 1.0 {
		t.Fatalf("expected upgraded default temp 1.0 for gpt-5 with default 0.2, got %#v", oc.defaultTemperature)
	}
}
