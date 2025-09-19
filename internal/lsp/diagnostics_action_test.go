package lsp

import (
	"encoding/json"
	"io"
	"log"
	"testing"
)

func TestHandleCodeAction_ListsDiagnosticsActionWhenOverlap(t *testing.T) {
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document)}
	initServerDefaults(s)
	s.llmClient = fakeLLM{resp: "fixed"}
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nvar a=1\n")
	// Selection overlaps line 1
	sel := Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 1, Character: 5}}
	// Provide diagnostics in the action context with one overlapping
	ctx := CodeActionContext{Diagnostics: []Diagnostic{
		{Range: Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 1, Character: 3}}, Message: "in"},
		{Range: Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 1}}, Message: "out"},
	}}
	rawCtx, _ := json.Marshal(ctx)
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: sel, Context: json.RawMessage(rawCtx)}
	ca := s.buildDiagnosticsCodeAction(p, "var a=1")
	if ca == nil {
		t.Fatalf("expected diagnostics action")
	}
	// Resolve should produce an edit
	resolved, ok := s.resolveCodeAction(*ca)
	if !ok || resolved.Edit == nil {
		t.Fatalf("expected resolved edit from diagnostics")
	}
}
