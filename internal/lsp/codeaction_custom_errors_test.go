package lsp

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "testing"

    "codeberg.org/snonux/hexai/internal/llm"
)

func TestResolveCodeAction_Custom_UnknownID(t *testing.T) {
    s := newTestServer()
    // No matching custom action configured
    s.customActions = []CustomAction{{ID: "known", Title: "Known", Instruction: "x"}}
    uri := "file:///t.go"
    payload := struct {
        Type      string `json:"type"`
        ID        string `json:"id"`
        URI       string `json:"uri"`
        Range     Range  `json:"range"`
        Selection string `json:"selection"`
    }{Type: "custom", ID: "missing", URI: uri, Range: Range{}, Selection: "abc"}
    raw, _ := json.Marshal(payload)
    ca := CodeAction{Title: "Hexai: X", Data: raw}
    if _, ok := s.resolveCodeAction(ca); ok {
        t.Fatalf("expected resolve to fail for unknown custom id")
    }
}

type errLLM struct{}
func (errLLM) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) { return "", errors.New("boom") }
func (errLLM) Name() string         { return "prov" }
func (errLLM) DefaultModel() string { return "m" }

func TestResolveCodeAction_Custom_EmptyAndError(t *testing.T) {
    // empty output case
    s1 := newTestServer()
    s1.llmClient = fakeLLM{resp: "   \n\n"}
    s1.customActions = []CustomAction{{ID: "empty", Title: "Empty", Instruction: "x"}}
    raw1, _ := json.Marshal(struct{ Type, ID, URI, Selection string; Range Range }{"custom", "empty", "file:///t.go", "sel", Range{}})
    if resolved, ok := s1.resolveCodeAction(CodeAction{Data: raw1}); ok || resolved.Edit != nil {
        t.Fatalf("expected no edit for empty llm output")
    }

    // error case
    s2 := newTestServer()
    s2.llmClient = errLLM{}
    s2.customActions = []CustomAction{{ID: "err", Title: "Err", Instruction: "x"}}
    raw2, _ := json.Marshal(struct{ Type, ID, URI, Selection string; Range Range }{"custom", "err", "file:///t.go", "sel", Range{}})
    if resolved, ok := s2.resolveCodeAction(CodeAction{Data: raw2}); ok || resolved.Edit != nil {
        t.Fatalf("expected no edit for llm error")
    }
}

func TestHandleCodeAction_Custom_SelectionSuppressedWhenEmpty(t *testing.T) {
    s := newTestServer()
    s.llmClient = fakeLLM{resp: "IGN"}
    // One selection-scoped and one diagnostics-scoped custom
    s.customActions = []CustomAction{
        {ID: "sel", Title: "Sel", Scope: "selection", Instruction: "x"},
        {ID: "diag", Title: "Diag", Scope: "diagnostics", User: "{{diagnostics}}"},
    }
    uri := "file:///t.go"
    s.setDocument(uri, "package p\nfunc f(){}\n")
    // Empty selection range (start==end)
    p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1}}}
    // include a diagnostic so diagnostics action is allowed
    ctx := CodeActionContext{Diagnostics: []Diagnostic{{Range: Range{Start: Position{Line: 1}}, Message: "x"}}}
    rawCtx, _ := json.Marshal(ctx)
    p.Context = json.RawMessage(rawCtx)
    // Build request
    req := Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "textDocument/codeAction"}
    req.Params, _ = json.Marshal(p)
    // capture
    var out bytes.Buffer
    s.out = &out
    s.handleCodeAction(req)
    resp := captureResponse(t, &out)
    rb, _ := json.Marshal(resp.Result)
    var actions []CodeAction
    _ = json.Unmarshal(rb, &actions)
    seenSel, seenDiag := false, false
    for _, a := range actions {
        if a.Title == "Hexai: Sel" { seenSel = true }
        if a.Title == "Hexai: Diag" { seenDiag = true }
    }
    if seenSel || !seenDiag {
        t.Fatalf("expected only diagnostics custom when selection is empty; got %+v", actions)
    }
}
