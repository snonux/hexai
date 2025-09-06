package lsp

import (
	"testing"
)

func TestBuildCompletionMessages_InlinePromptOverridesSys(t *testing.T) {
	s := newTestServer()
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line: 0, Character: 1}}
	msgs := s.buildCompletionMessages(true, false, "", false, p, "above", "current", "below", "func f")
	if len(msgs) < 2 {
		t.Fatalf("expected messages")
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("unexpected roles")
	}
	if want := "precise code completion/refactoring engine"; !contains(msgs[0].Content, want) {
		t.Fatalf("inline sys not applied")
	}
}

func TestBuildCompletionMessages_ExtraContextIncluded(t *testing.T) {
	s := newTestServer()
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line: 0, Character: 1}}
	msgs := s.buildCompletionMessages(false, true, "EXTRA", false, p, "a", "b", "c", "f")
	found := false
	for _, m := range msgs {
		if m.Role == "user" && contains(m.Content, "Additional context:") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing extra context message")
	}
}

func TestPrefixHeuristic_AllVariants(t *testing.T) {
	s := newTestServer()
	// manual invoke requires at least min prefix; set to 2
	s.manualInvokeMinPrefix = 2
	cur := "a"
	p := CompletionParams{Position: Position{Line: 0, Character: 1}}
	if s.prefixHeuristicAllows(false, cur, p, true) {
		t.Fatalf("should require >=2 prefix on manual invoke")
	}
	// structural triggers allow without prefix
	if !s.prefixHeuristicAllows(false, "fmt.", CompletionParams{Position: Position{Line: 0, Character: 4}}, false) {
		t.Fatalf("dot trigger should allow")
	}
}

func TestBuildDocString_Contents(t *testing.T) {
	s := newTestServer()
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line: 3, Character: 7}}
	got := s.buildDocString(p, "above", "current", "below", "func ctx")
	if !contains(got, "file: file:///x") || !contains(got, "line: 3") || !contains(got, "function: func ctx") {
		t.Fatalf("unexpected doc string: %q", got)
	}
}

func TestBuildCompletionMessages_InParams_UsesParamPrompts(t *testing.T) {
	s := newTestServer()
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line: 0, Character: 5}}
	msgs := s.buildCompletionMessages(false, false, "", true, p, "a", "func f(x)", "c", "func f(x)")
	if len(msgs) < 2 || msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("unexpected messages")
	}
	if !contains(msgs[0].Content, "function signatures") || !contains(msgs[1].Content, "parameter list") {
		t.Fatalf("unexpected in-params prompts: %#v", msgs)
	}
}

func TestPostProcessCompletion_CodeFencesAndDuplicates(t *testing.T) {
	s := newTestServer()
	// code fences
	cleaned := s.postProcessCompletion("```go\nname := value\n```", "", "")
	if cleaned == "" {
		t.Fatalf("expected non-empty after fence removal")
	}
	// duplicate assignment prefix strip
	cleaned2 := s.postProcessCompletion("name := other", "name := ", "name := ")
	if cleaned2 == "" || cleaned2 == "name := other" {
		t.Fatalf("expected duplicate assignment prefix stripped: %q", cleaned2)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && (stringIndex(s, sub) >= 0)))
}

func stringIndex(s, sub string) int {
	return len([]rune(s[:])) - len([]rune(s[:])) + (func() int { return intIndex(s, sub) })()
}
func intIndex(s, sub string) int { return Index(s, sub) }

// Go's strings.Index is fine; wrapped to avoid extra imports in this small test.
func Index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
