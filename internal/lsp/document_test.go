// Summary: Tests for LSP document model (line management, edits, and transformations).
package lsp

import (
	"io"
	"log"
	"strings"
	"testing"
)

func newTestServer() *Server {
	return &Server{
		logger: log.New(io.Discard, "", 0),
		docs:   make(map[string]*document),
	}
}

func TestSplitLines(t *testing.T) {
	in := "a\r\nb\nc"
	got := splitLines(in)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestLineContext(t *testing.T) {
	s := newTestServer()
	src := "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n"
	uri := "file:///test.go"
	s.setDocument(uri, src)

	// Position on the return line (line 3, zero-based)
	above, current, below, funcCtx := s.lineContext(uri, Position{Line: 3, Character: 0})

	if want := "func add(a, b int) int {"; funcCtx != want {
		t.Fatalf("funcCtx got %q want %q", funcCtx, want)
	}
	if want := "func add(a, b int) int {"; above != want {
		t.Fatalf("above got %q want %q", above, want)
	}
	if want := "\treturn a + b"; current != want {
		t.Fatalf("current got %q want %q", current, want)
	}
	if want := "}"; below != want {
		t.Fatalf("below got %q want %q", below, want)
	}
}

func TestLineContext_EmptyDoc(t *testing.T) {
	s := newTestServer()
	a, c, b, f := s.lineContext("file:///missing.go", Position{Line: 0, Character: 0})
	if a != "" || b != "" || c != "" || f != "" {
		t.Fatalf("expected all empty for missing doc; got above=%q current=%q below=%q func=%q", a, c, b, f)
	}
}

func TestTrimLen(t *testing.T) {
	long := strings.Repeat("a", 205)
	got := trimLen(long)
	want := strings.Repeat("a", 200) + "…"
	if got != want {
		t.Fatalf("trimLen got %q want %q", got, want)
	}
}

func TestFirstLine(t *testing.T) {
	s := "first line\r\nsecond line"
	if got := firstLine(s); got != "first line" {
		t.Fatalf("firstLine got %q want %q", got, "first line")
	}
}
