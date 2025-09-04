package lsp

import (
    "encoding/json"
    "testing"
)

func TestResolveRewrite_MultiLine_PreservesRange(t *testing.T) {
    s := newTestServer()
    s.llmClient = fakeLLM{resp: "line1\nline2"}
    uri := "file:///x.go"
    s.setDocument(uri, "package p\nvar a=1\n")
    r := Range{Start: Position{Line:1, Character:0}, End: Position{Line:1, Character:5}}
    payload := struct {
        Type        string `json:"type"`
        URI         string `json:"uri"`
        Range       Range  `json:"range"`
        Instruction string `json:"instruction"`
        Selection   string `json:"selection"`
    }{Type: "rewrite", URI: uri, Range: r, Instruction: "expand", Selection: "var a"}
    raw, _ := json.Marshal(payload)
    ca := CodeAction{Title: "Hexai: rewrite selection", Data: raw}
    resolved, ok := s.resolveCodeAction(ca)
    if !ok || resolved.Edit == nil { t.Fatalf("expected resolved rewrite edit") }
    edits := resolved.Edit.Changes[uri]
    if len(edits) != 1 { t.Fatalf("expected 1 edit") }
    if edits[0].Range != r { t.Fatalf("range mismatch: got %+v want %+v", edits[0].Range, r) }
    if edits[0].NewText == "" || !containsNewline(edits[0].NewText) {
        t.Fatalf("expected multi-line replacement text, got %q", edits[0].NewText)
    }
}

func TestResolveDiagnostics_MultiLine_PreservesRange(t *testing.T) {
    s := newTestServer()
    s.llmClient = fakeLLM{resp: "fixed\nvalue"}
    uri := "file:///x.go"
    s.setDocument(uri, "package p\nvar x = 1\n")
    r := Range{Start: Position{Line:1, Character:0}, End: Position{Line:1, Character:10}}
    payload := struct {
        Type        string       `json:"type"`
        URI         string       `json:"uri"`
        Range       Range        `json:"range"`
        Selection   string       `json:"selection"`
        Diagnostics []Diagnostic `json:"diagnostics"`
    }{Type: "diagnostics", URI: uri, Range: r, Selection: "var x = 1", Diagnostics: []Diagnostic{{Range: Range{Start: Position{Line:1}, End: Position{Line:1, Character:5}}, Message: "msg"}}}
    raw, _ := json.Marshal(payload)
    ca := CodeAction{Title: "Hexai: resolve diagnostics", Data: raw}
    resolved, ok := s.resolveCodeAction(ca)
    if !ok || resolved.Edit == nil { t.Fatalf("expected resolved diagnostics edit") }
    edits := resolved.Edit.Changes[uri]
    if len(edits) != 1 { t.Fatalf("expected 1 edit") }
    if edits[0].Range != r { t.Fatalf("range mismatch: got %+v want %+v", edits[0].Range, r) }
    if edits[0].NewText == "" || !containsNewline(edits[0].NewText) {
        t.Fatalf("expected multi-line replacement text, got %q", edits[0].NewText)
    }
}

func containsNewline(s string) bool {
    for i := 0; i < len(s); i++ { if s[i] == '\n' { return true } }
    return false
}

