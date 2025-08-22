package llm

import "testing"

func TestBuildCopilotChatRequest_FieldsAndDefaults(t *testing.T) {
	o := Options{
		Model:       "gpt-x",
		Temperature: 0,
		MaxTokens:   123,
		Stop:        []string{"X"},
	}

	msgs := []Message{{Role: "user", Content: "q"}}
	req := buildCopilotChatRequest(o, msgs, f64p(0.5))

	if req.Model != "gpt-x" {
		t.Fatalf("model mismatch: %q", req.Model)
	}

	if req.Temperature == nil || *req.Temperature != 0.5 {
		t.Fatalf("default temp not applied")
	}

	if req.MaxTokens == nil || *req.MaxTokens != 123 {
		t.Fatalf("max_tokens not applied")
	}

	if len(req.Stop) != 1 || req.Stop[0] != "X" {
		t.Fatalf("stop not applied")
	}

	if len(req.Messages) != 1 || req.Messages[0].Content != "q" {
		t.Fatalf("messages not copied")
	}
}
