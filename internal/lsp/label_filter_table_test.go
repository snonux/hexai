package lsp

import "testing"

func TestLabelForCompletion_Table(t *testing.T) {
	cases := []struct{ cleaned, filter, want string }{
		{"line one\nline two", "zzz", "zzz"},
		{"result", "re", "result"},
		{"hello world", "he", "hello world"},
	}
	for _, c := range cases {
		if got := labelForCompletion(c.cleaned, c.filter); got != c.want {
			t.Fatalf("cleaned=%q filter=%q got %q want %q", c.cleaned, c.filter, got, c.want)
		}
	}
}
