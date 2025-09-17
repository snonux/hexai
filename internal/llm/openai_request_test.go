package llm

import (
	"encoding/json"
	"testing"
)

func TestBuildOAChatRequest_MaxTokensKeyByModel(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	mt := 123
	// Legacy model: use max_tokens
	r1 := buildOAChatRequest(Options{Model: "gpt-4.1", MaxTokens: mt}, msgs, nil, false)
	b1, _ := json.Marshal(r1)
	if !contains(string(b1), "max_tokens") || contains(string(b1), "max_completion_tokens") {
		t.Fatalf("expected max_tokens only, got %s", string(b1))
	}
	// gpt-5 family: use max_completion_tokens
	r2 := buildOAChatRequest(Options{Model: "gpt-5.0-preview", MaxTokens: mt}, msgs, nil, false)
	b2, _ := json.Marshal(r2)
	if !contains(string(b2), "max_completion_tokens") || contains(string(b2), "max_tokens\":") {
		t.Fatalf("expected max_completion_tokens only, got %s", string(b2))
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
