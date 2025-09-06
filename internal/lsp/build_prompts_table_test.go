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
		sys, user := buildPrompts(c.inParams, p, "above", "current", "below", "func ctx")
		if sys == "" || user == "" {
			t.Fatalf("%s: prompts empty", c.name)
		}
	}
}
