package llm

import (
	"encoding/json"
	"testing"
)

func TestBuildOAChatRequest_MaxTokensKeyByModel(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	mt := 123
	// Legacy model: use max_tokens
	r1 := buildOAChatRequest(Options{Model: "gpt-4.1", MaxTokens: mt}, msgs, nil, false, "llm/test ")
	b1, _ := json.Marshal(r1)
	if !contains(string(b1), "max_tokens") || contains(string(b1), "max_completion_tokens") {
		t.Fatalf("expected max_tokens only, got %s", string(b1))
	}
	// gpt-5 family: use max_completion_tokens
	r2 := buildOAChatRequest(Options{Model: "gpt-5.0-preview", MaxTokens: mt}, msgs, nil, false, "llm/test ")
	b2, _ := json.Marshal(r2)
	if !contains(string(b2), "max_completion_tokens") || contains(string(b2), "max_tokens\":") {
		t.Fatalf("expected max_completion_tokens only, got %s", string(b2))
	}
}

func TestBuildOAChatRequest_TemperatureForcedForGpt5(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	// Explicit temp 0.2 → should be forced to 1.0 for gpt-5
	r := buildOAChatRequest(Options{Model: "gpt-5.0", Temperature: 0.2, MaxTokens: 50}, msgs, nil, false, "llm/test ")
	b, _ := json.Marshal(r)
	if !contains(string(b), "\"temperature\":1") {
		t.Fatalf("expected forced temperature 1.0 for gpt-5, got %s", string(b))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
