package lsp

import "testing"

func TestBuildPrompts_Table(t *testing.T) {
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x.go"}, Position: Position{Line: 5, Character: 7}}
	cases := []struct {
		name     string
		inParams bool
	}{
		{"generic", false},
		{"in_params", true},
	}
	for _, c := range cases {
		s := newTestServer()
		msgs := s.buildCompletionMessages(false, false, "", c.inParams, p, "above", "current", "below", "func ctx")
		if len(msgs) < 2 || msgs[0].Role != "system" || msgs[1].Role != "user" {
			t.Fatalf("%s: unexpected messages", c.name)
		}
		if msgs[0].Content == "" || msgs[1].Content == "" {
			t.Fatalf("%s: prompts empty", c.name)
		}
	}
}
