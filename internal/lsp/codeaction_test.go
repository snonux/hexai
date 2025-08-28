package lsp

import (
	"context"
	"encoding/json"
	"hexai/internal/llm"
	"testing"
)

type fakeLLM struct {
	resp string
	err  error
}

func (f fakeLLM) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return f.resp, f.err
}
func (f fakeLLM) Name() string         { return "fake" }
func (f fakeLLM) DefaultModel() string { return "fake-model" }

func TestBuildRewriteCodeAction_LazyAndResolves(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: "REWRITTEN"}
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: "file:///t.go"}, Range: Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 3, Character: 4}}}
	sel := ";rewrite;\nold code"
	ca := s.buildRewriteCodeAction(p, sel)
	if ca == nil {
		t.Fatalf("expected code action")
	}
	// Should be lazy (no edit yet)
	if ca.Edit != nil {
		t.Fatalf("expected nil Edit before resolve")
	}
	if len(ca.Data) == 0 {
		t.Fatalf("expected data payload for lazy resolve")
	}
	// Resolve now
	resolved, ok := s.resolveCodeAction(*ca)
	if !ok || resolved.Edit == nil {
		t.Fatalf("expected resolve to produce edit")
	}
	edits := resolved.Edit.Changes[p.TextDocument.URI]
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].Range != p.Range {
		t.Fatalf("edit range mismatch: got %+v want %+v", edits[0].Range, p.Range)
	}
	if edits[0].NewText == "" {
		t.Fatalf("expected non-empty replacement text")
	}
}

func TestBuildRewriteCodeAction_NoInstruction(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: "IGNORED"}
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: "file:///t.go"}, Range: Range{}}
	sel := "no instruction here"
	if ca := s.buildRewriteCodeAction(p, sel); ca != nil {
		t.Fatalf("expected nil action when no instruction present")
	}
}

func TestBuildDiagnosticsCodeAction_LazyAndResolves(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: "FIXED"}
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: "file:///t.go"}, Range: Range{Start: Position{Line: 10}, End: Position{Line: 12, Character: 5}}}
	ctx := CodeActionContext{Diagnostics: []Diagnostic{
		{Range: Range{Start: Position{Line: 11}, End: Position{Line: 11, Character: 10}}, Message: "inside"},
		{Range: Range{Start: Position{Line: 2}, End: Position{Line: 3}}, Message: "outside"},
	}}
	raw, _ := json.Marshal(ctx)
	p.Context = json.RawMessage(raw)
	sel := "some selected code"
	ca := s.buildDiagnosticsCodeAction(p, sel)
	if ca == nil {
		t.Fatalf("expected diagnostics code action")
	}
	if ca.Edit != nil {
		t.Fatalf("expected lazy action without edit")
	}
	if len(ca.Data) == 0 {
		t.Fatalf("expected data payload for lazy diagnostics action")
	}
	resolved, ok := s.resolveCodeAction(*ca)
	if !ok || resolved.Edit == nil {
		t.Fatalf("expected resolve to produce edit")
	}
}

func TestBuildDiagnosticsCodeAction_NoDiagnostics(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: "FIXED"}
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: "file:///t.go"}, Range: Range{}}
	// empty context
	p.Context = json.RawMessage(nil)
	if ca := s.buildDiagnosticsCodeAction(p, "sel"); ca != nil {
		t.Fatalf("expected nil action when no diagnostics")
	}
}
