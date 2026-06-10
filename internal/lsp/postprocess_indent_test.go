package lsp

import "testing"

func TestPostProcessCompletion_IndentWithDoubleOpen(t *testing.T) {
	s := newTestServer()
	cleaned := s.completion.postProcessCompletion("a\nb", "", "  >>!gen>")
	// Expect each non-empty line to be indented by two spaces
	want := "  a\n  b"
	if cleaned != want {
		t.Fatalf("got %q want %q", cleaned, want)
	}
}
