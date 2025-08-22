// Summary: Tests for context-building logic (window, full-file) and truncation behavior.
package lsp

import (
	"strconv"
	"strings"
	"testing"
)

func TestWindowContext_Bounds(t *testing.T) {
	s := newTestServer()
	s.windowLines = 4 // half=2
	s.maxContextTokens = 9999
	lines := make([]string, 10)
	for i := 0; i < 10; i++ {
		lines[i] = "L" + strconv.Itoa(i)
	}
	text := strings.Join(lines, "\n")
	uri := "file:///w.go"
	s.setDocument(uri, text)
	got := s.windowContext(uri, Position{Line: 5, Character: 0})
	// expect lines 3..7 inclusive
	want := strings.Join(lines[3:8], "\n")
	if got != want {
		t.Fatalf("window context got %q want %q", got, want)
	}
}

func TestBuildAdditionalContext_Minimal(t *testing.T) {
	s := newTestServer()
	s.contextMode = "minimal"
	if ctx, ok := s.buildAdditionalContext(false, "file:///x.go", Position{}); ok || ctx != "" {
		t.Fatalf("expected no context in minimal mode; got ok=%v ctx=%q", ok, ctx)
	}
}

func TestBuildAdditionalContext_FileOnNewFunc(t *testing.T) {
	s := newTestServer()
	s.contextMode = "file-on-new-func"
	s.maxContextTokens = 9999
	uri := "file:///x.go"
	body := "package x\n\nfunc a(){}\n"
	s.setDocument(uri, body)
	if ctx, ok := s.buildAdditionalContext(true, uri, Position{}); !ok || ctx == "" {
		t.Fatalf("expected full context when new func; ok=%v ctx=%q", ok, ctx)
	}
	if ctx, ok := s.buildAdditionalContext(false, uri, Position{}); ok || ctx != "" {
		t.Fatalf("expected no context when not new func; ok=%v ctx=%q", ok, ctx)
	}
}

func TestBuildAdditionalContext_AlwaysFull(t *testing.T) {
	s := newTestServer()
	s.contextMode = "always-full"
	s.maxContextTokens = 9999
	uri := "file:///x.go"
	body := "line1\nline2\n"
	s.setDocument(uri, body)
	if ctx, ok := s.buildAdditionalContext(false, uri, Position{}); !ok || ctx == "" {
		t.Fatalf("expected context in always-full; ok=%v ctx=%q", ok, ctx)
	}
}

func TestTruncateToApproxTokens(t *testing.T) {
	text := strings.Repeat("abcd", 10)     // 40 chars
	got := truncateToApproxTokens(text, 5) // ~20 chars
	if len(got) > 5*4 {
		t.Fatalf("truncate exceeded budget: got len=%d budget=%d", len(got), 5*4)
	}
}
