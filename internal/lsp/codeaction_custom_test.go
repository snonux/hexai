package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
)

// local copy of captureResponse for this test file
func capResp(t *testing.T, buf *bytes.Buffer) Response {
	t.Helper()
	raw := buf.String()
	idx := strings.Index(raw, "\r\n\r\n")
	if idx < 0 {
		t.Fatalf("no header/body separator in %q", raw)
	}
	body := raw[idx+4:]
	var resp Response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func TestHandleCodeAction_ListsCustomActions(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	s.llmClient = fakeLLM{resp: "IGN"}
	// Inject two custom actions
	s.customActions = []CustomAction{
		{ID: "extract", Title: "Extract function", Scope: "selection", Kind: "refactor.extract", Instruction: "Extract into function"},
		{ID: "fix", Title: "Fix diagnostics", Scope: "diagnostics", Kind: "quickfix", User: "Fix:\n{{diagnostics}}\n\n{{selection}}"},
	}
	// Prepare document and params
	uri := "file:///t.go"
	s.setDocument(uri, "package x\n\nfunc f(){}\n")
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 2}, End: Position{Line: 2, Character: 5}}}
	// Include diagnostics context so diagnostics-scoped action appears
	ctx := CodeActionContext{Diagnostics: []Diagnostic{{Range: Range{Start: Position{Line: 2}}, Message: "warn"}}}
	raw, _ := json.Marshal(ctx)
	p.Context = json.RawMessage(raw)

	// Call handler
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "textDocument/codeAction"}
	req.Params, _ = json.Marshal(p)
	out.Reset()
	s.handleCodeAction(req)
	resp := capResp(t, &out)
	var actions []CodeAction
	rb, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(rb, &actions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var seenSel, seenDiag bool
	for _, a := range actions {
		if a.Title == "Hexai: Extract function" {
			seenSel = true
		}
		if a.Title == "Hexai: Fix diagnostics" {
			seenDiag = true
		}
	}
	if !seenSel || !seenDiag {
		t.Fatalf("expected both custom actions, got %+v", actions)
	}
}

func TestResolveCodeAction_CustomInstructionAndUser(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeLLM{resp: "REPLACED"}
	// one instruction-based and one user-based
	s.customActions = []CustomAction{
		{ID: "extract", Title: "Extract function", Scope: "selection", Kind: "refactor.extract", Instruction: "Extract into function"},
		{ID: "fix", Title: "Fix diagnostics", Scope: "diagnostics", Kind: "quickfix", User: "Fix: {{diagnostics}}\n{{selection}}"},
	}
	uri := "file:///t.go"
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 3}}}

	// Build selection-scoped custom action payload
	selPayload := struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		URI       string `json:"uri"`
		Range     Range  `json:"range"`
		Selection string `json:"selection"`
	}{Type: "custom", ID: "extract", URI: uri, Range: p.Range, Selection: "abc"}
	raw1, _ := json.Marshal(selPayload)
	ca1 := CodeAction{Title: "Hexai: Extract function", Data: raw1}
	if resolved, ok := s.resolveCodeAction(ca1); !ok || resolved.Edit == nil {
		t.Fatalf("expected resolve for instruction-based custom action")
	}

	// Build diagnostics-scoped custom action payload
	diagPayload := struct {
		Type        string       `json:"type"`
		ID          string       `json:"id"`
		URI         string       `json:"uri"`
		Range       Range        `json:"range"`
		Selection   string       `json:"selection"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}{Type: "custom", ID: "fix", URI: uri, Range: p.Range, Selection: "abc", Diagnostics: []Diagnostic{{Message: "lint"}}}
	raw2, _ := json.Marshal(diagPayload)
	ca2 := CodeAction{Title: "Hexai: Fix diagnostics", Data: raw2}
	if resolved, ok := s.resolveCodeAction(ca2); !ok || resolved.Edit == nil {
		t.Fatalf("expected resolve for user-based custom action")
	}
}
