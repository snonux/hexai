package lsp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	tut "github.com/snonux/hexai/internal/testutil"
)

func TestBuildDocumentCodeAction_AndResolve(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: tut.MultilineDocBlock() + "\n" + "func add(a,b int) int { return a+b }"}
	uri := "file:///doc.go"
	s.setDocument(uri, "package x\nfunc add(a,b int) int {return a+b}")
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 1, Character: 10}}}
	sel := "func add(a,b int) int {return a+b}"
	ca := s.buildDocumentCodeAction(p, sel)
	if ca == nil {
		t.Fatalf("expected document code action")
	}
	resolved, ok := s.resolveCodeAction(*ca)
	if !ok || resolved.Edit == nil {
		t.Fatalf("expected resolved edit")
	}
	edits := resolved.Edit.Changes[uri]
	if len(edits) != 1 || strings.TrimSpace(edits[0].NewText) == "" {
		t.Fatalf("expected replacement text")
	}
}

func TestResolveCodeAction_Rewrite(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: "rewritten"}
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nvar a=1\n")
	payload := struct {
		Type        string `json:"type"`
		URI         string `json:"uri"`
		Range       Range  `json:"range"`
		Instruction string `json:"instruction"`
		Selection   string `json:"selection"`
	}{Type: "rewrite", URI: uri, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 5}}, Instruction: "do it", Selection: "var a"}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: rewrite selection", Data: raw}
	if resolved, ok := s.resolveCodeAction(ca); !ok || resolved.Edit == nil {
		t.Fatalf("expected resolved rewrite edit")
	}
}

func TestBuildGoUnitTestCodeAction_AndResolveCreate(t *testing.T) {
	s := newTestServer()
	// place files under a temp dir to avoid collisions
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "calc.go")
	uri := "file://" + srcPath
	src := "package calc\n\nfunc Sum(a, b int) int { return a+b }\n"
	s.setDocument(uri, src)
	// Offer action (not a _test.go)
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 2}}}
	if a := s.buildGoUnitTestCodeAction(p); a == nil {
		t.Fatalf("expected go unit test action")
	}
	// Resolve should create new test file with package+import and a test function
	we, testURI, _, ok := s.resolveGoTest(uri, Position{Line: 2})
	if !ok {
		t.Fatalf("resolveGoTest failed")
	}
	if len(we.DocumentChanges) != 2 {
		t.Fatalf("expected create + edits, got %d", len(we.DocumentChanges))
	}
	if !strings.HasSuffix(testURI, "_test.go") {
		t.Fatalf("unexpected test URI: %s", testURI)
	}
}

func TestBuildGoUnitTestCodeAction_SkipOnTestFile(t *testing.T) {
	s := newTestServer()
	uri := "file:///tmp/x_test.go"
	s.setDocument(uri, "package p\nfunc T(){}")
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}}
	if a := s.buildGoUnitTestCodeAction(p); a != nil {
		t.Fatalf("expected no action on _test.go")
	}
}

func TestDiagnosticsInRange(t *testing.T) {
	s := newTestServer()
	ctx := CodeActionContext{Diagnostics: []Diagnostic{
		{Range: Range{Start: Position{Line: 3}, End: Position{Line: 3, Character: 5}}, Message: "in"},
		{Range: Range{Start: Position{Line: 10}, End: Position{Line: 11}}, Message: "out"},
	}}
	raw, _ := json.Marshal(ctx)
	got := s.diagnosticsInRange(json.RawMessage(raw), Range{Start: Position{Line: 2}, End: Position{Line: 4}})
	if len(got) != 1 || got[0].Message != "in" {
		t.Fatalf("unexpected diags: %+v", got)
	}
}

func TestDocBeforeAfter(t *testing.T) {
	s := newTestServer()
	uri := "file:///d.go"
	s.setDocument(uri, "ab\ncd\nef")
	before, after := s.docBeforeAfter(uri, Position{Line: 1, Character: 1})
	if before != "ab\nc" || after != "d\nef" {
		t.Fatalf("before=%q after=%q", before, after)
	}
}
