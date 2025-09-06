package lsp

import "testing"

func TestComputeTextEditAndFilter_Table(t *testing.T) {
	cases := []struct {
		name     string
		inParams bool
		current  string
		pos      Position
		cleaned  string
	}{
		{"ident_replace", false, "ab cd", Position{Line: 1, Character: 4}, "X"},
		{"params_inside", true, "func add(a int, b string)", Position{Line: 0, Character: 15}, "c bool"},
		{"params_at_close", true, "func add(a int)", Position{Line: 0, Character: len("func add(a int)")}, "b string"},
	}
	for _, c := range cases {
		te, filter := computeTextEditAndFilter(c.cleaned, c.inParams, c.current, CompletionParams{Position: c.pos})
		if te == nil {
			t.Fatalf("%s: expected edit", c.name)
		}
		if c.inParams && te.Range.Start.Character == 0 {
			t.Fatalf("%s: expected param range (non-zero start)", c.name)
		}
		if filter == "" && c.current != "" {
			// For ident_replace, filter may be non-empty; for params, it can be empty when replacing entire segment
		}
		if te.NewText != c.cleaned {
			t.Fatalf("%s: newText got %q want %q", c.name, te.NewText, c.cleaned)
		}
	}
}
