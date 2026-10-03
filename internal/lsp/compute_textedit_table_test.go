package lsp

import "testing"

func TestComputeTextEditAndFilter_Table(t *testing.T) {
	cases := []struct {
		name     string
		inParams bool
		current  string
		pos      Position
		cleaned  string
		want     string
	}{
		// The typed word "c" is replaced and re-inserted ahead of the completion.
		{"ident_replace", false, "ab cd", Position{Line: 1, Character: 4}, "X", "cX"},
		{"ident_continuation", false, "\tfmt.P", Position{Line: 0, Character: 6}, "rintln()", "Println()"},
		{"params_inside", true, "func add(a int, b string)", Position{Line: 0, Character: 15}, "c bool", "a int, c bool"},
		{"params_at_close", true, "func add(a int)", Position{Line: 0, Character: len("func add(a int)")}, "b string", "b string"},
		{"utf16_range", false, "s := \"ü\"; fmt.", Position{Line: 0, Character: 15}, "Println()", "Println()"},
	}
	for _, c := range cases {
		te, _ := computeTextEditAndFilter(c.cleaned, c.inParams, c.current, CompletionParams{Position: c.pos})
		if te == nil {
			t.Fatalf("%s: expected edit", c.name)
		}
		if c.name == "params_inside" && te.Range.Start.Character == 0 {
			t.Fatalf("%s: expected param range (non-zero start)", c.name)
		}

		if te.NewText != c.want {
			t.Fatalf("%s: newText got %q want %q", c.name, te.NewText, c.want)
		}
		if te.Range.Start.Character > te.Range.End.Character {
			t.Fatalf("%s: inverted range %+v", c.name, te.Range)
		}
	}
}
