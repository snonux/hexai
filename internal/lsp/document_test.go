// Summary: Tests for LSP document model (line management, edits, and transformations).
package lsp

import (
	"io"
	"log"
	"strings"
	"testing"
)

func newTestServer() *Server {
	s := &Server{
		logger:       log.New(io.Discard, "", 0),
		docs:         make(map[string]*document),
		inlineOpen:   ">",
		inlineClose:  ">",
		chatSuffix:   ">",
		chatPrefixes: []string{"?", "!", ":", ";"},
	}
	// Default prompt templates (mirror app defaults)
	s.promptCompSysParams = "You are a code completion engine for function signatures. Return only the parameter list contents (without parentheses), no braces, no prose. Prefer idiomatic names and types."
	s.promptCompUserParams = "Cursor is inside the function parameter list. Suggest only the parameter list (no parentheses).\nFunction line: {{function}}\nCurrent line (cursor at {{char}}): {{current}}"
	s.promptCompSysGeneral = "You are a terse code completion engine. Return only the code to insert, no surrounding prose or backticks. Only continue from the cursor; never repeat characters already present to the left of the cursor on the current line (e.g., if 'name :=' is already typed, only return the right-hand side expression)."
	s.promptCompUserGeneral = "Provide the next likely code to insert at the cursor.\nFile: {{file}}\nFunction/context: {{function}}\nAbove line: {{above}}\nCurrent line (cursor at character {{char}}): {{current}}\nBelow line: {{below}}\nOnly return the completion snippet."
	s.promptCompSysInline = "You are a precise code completion/refactoring engine. Output only the code to insert with no prose, no comments, and no backticks. Return raw code only."
	s.promptCompExtraHeader = "Additional context:\n{{context}}"
	s.promptNativeCompletion = "// Path: {{path}}\n{{before}}"
	s.promptChatSystem = "You are a helpful coding assistant. Answer concisely and clearly."
	s.promptRewriteSystem = "You are a precise code refactoring engine. Rewrite the given code strictly according to the instruction. Return only the updated code with no prose or backticks. Preserve formatting where reasonable."
	s.promptDiagnosticsSystem = "You are a precise code fixer. Resolve the given diagnostics by editing only the selected code. Return only the corrected code with no prose or backticks. Keep behavior and style, and avoid unrelated changes."
	s.promptDocumentSystem = "You are a precise code documentation engine. Add idiomatic documentation comments to the given code. Preserve exact behavior and formatting as much as possible. Return only the updated code with comments, no prose or backticks."
	s.promptRewriteUser = "Instruction: {{instruction}}\n\nSelected code to transform:\n{{selection}}"
	s.promptDiagnosticsUser = "Diagnostics to resolve (selection only):\n{{diagnostics}}\n\nSelected code:\n{{selection}}"
	s.promptDocumentUser = "Add documentation comments to this code:\n{{selection}}"
	s.promptGoTestSystem = "You are a precise Go unit test generator. Given a Go function, write one or more Test* functions using the testing package. Do NOT include package or imports, only the test function(s). Prefer table-driven tests. Keep it minimal and idiomatic."
	s.promptGoTestUser = "Function under test:\n{{function}}"
	// Keep package-level helpers in sync for tests using free functions
	inlineOpenChar = '>'
	inlineCloseChar = '>'
	chatSuffixChar = '>'
	chatPrefixSingles = []string{"?", "!", ":", ";"}
	return s
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

func TestDocBeforeAfter_ClampsIndices(t *testing.T) {
	s := newTestServer()
	uri := "file:///clamp.go"
	s.setDocument(uri, "abc\nxyz")
	// Position beyond document length should be clamped safely
	before, after := s.docBeforeAfter(uri, Position{Line: 99, Character: 99})
	if before == "" && after == "" {
		t.Fatalf("expected some text with clamped indices")
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
