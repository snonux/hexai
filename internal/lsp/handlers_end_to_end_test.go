package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"

	tut "github.com/snonux/hexai/internal/testutil"
)

// captureResponse decodes a single LSP Response from the server's output buffer.
func captureResponse(t *testing.T, buf *bytes.Buffer) Response {
	t.Helper()
	raw := buf.String()
	// strip Content-Length header framing
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

// captureRequest decodes a single JSON-RPC Request from the server's output buffer.
func captureRequest(t *testing.T, buf *bytes.Buffer) Request {
	t.Helper()
	raw := buf.String()
	// There may be multiple framed messages concatenated; scan for each
	off := 0
	for off < len(raw) {
		rest := raw[off:]
		idx := strings.Index(rest, "\r\n\r\n")
		if idx < 0 {
			break
		}
		body := rest[idx+4:]
		// Content-Length header indicates body length; parse length from header
		hdr := rest[:idx]
		clen := 0
		for _, line := range strings.Split(hdr, "\r\n") {
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				var n int
				_, _ = fmt.Sscanf(line, "Content-Length: %d", &n)
				clen = n
				break
			}
		}
		if clen <= 0 || clen > len(body) {
			clen = len(body)
		}
		piece := body[:clen]
		var req Request
		_ = json.Unmarshal([]byte(piece), &req)
		if req.Method != "" {
			return req
		}
		off += idx + 4 + clen
	}
	t.Fatalf("no request found in output")
	return Request{}
}

func TestHandleCodeAction_ListsHexaiActions(t *testing.T) {
	// Prepare server
	var out bytes.Buffer
	s := newTestServer()
	s.out = &out
	s.llmClient = fakeLLM{resp: "// doc\nfunc add(a,b int) int { return a+b }"}

	// Document with a function
	uri := "file:///x.go"
	src := "package p\n\nfunc add(a,b int) int { return a+b }\n"
	s.setDocument(uri, src)

	// Select the function line
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 2, Character: len("func add(a,b int) int { return a+b }")}}}
	b, _ := json.Marshal(p)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "textDocument/codeAction", Params: b}

	// Invoke directly
	out.Reset()
	s.handleCodeAction(req)
	resp := captureResponse(t, &out)
	// Decode result into []CodeAction
	var actions []CodeAction
	rb, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(rb, &actions); err != nil {
		t.Fatalf("decode actions: %v", err)
	}
	if len(actions) == 0 {
		t.Fatalf("expected some actions")
	}
	// Ensure our Hexai actions are present
	hasDoc := false
	hasGoTest := false
	for _, a := range actions {
		if strings.Contains(strings.ToLower(a.Title), "hexai:") {
			if strings.Contains(a.Title, "document code") {
				hasDoc = true
			}
			if strings.Contains(a.Title, "implement unit test") {
				hasGoTest = true
			}
		}
	}
	if !hasDoc || !hasGoTest {
		t.Fatalf("expected both Hexai actions, got %+v", actions)
	}
}

func TestHandleCodeActionResolve_Document(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	initServerDefaults(s)
	s.llmClient = fakeLLM{resp: "// doc\nfunc f(){}"}
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nfunc f(){}\n")
	// Build a document code action payload
	payload := struct {
		Type      string `json:"type"`
		URI       string `json:"uri"`
		Range     Range  `json:"range"`
		Selection string `json:"selection"`
	}{Type: "document", URI: uri, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 10}}, Selection: "func f(){}"}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: document code", Data: raw}
	b, _ := json.Marshal(ca)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("2"), Method: "codeAction/resolve", Params: b}

	out.Reset()
	s.handleCodeActionResolve(req)
	resp := captureResponse(t, &out)
	var resolved CodeAction
	rb, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(rb, &resolved); err != nil {
		t.Fatalf("decode resolved: %v", err)
	}
	if resolved.Edit == nil {
		t.Fatalf("expected resolved edit")
	}
}

func TestHandleCodeAction_NoLLMOrEmptySelection_ReturnsEmpty(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	initServerDefaults(s)
	uri := "file:///x.go"
	s.setDocument(uri, "package p\n\n")
	// Empty selection
	p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1}}}
	b, _ := json.Marshal(p)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("4"), Method: "textDocument/codeAction", Params: b}
	out.Reset()
	s.handleCodeAction(req)
	resp := captureResponse(t, &out)
	var actions []CodeAction
	rb, _ := json.Marshal(resp.Result)
	_ = json.Unmarshal(rb, &actions)
	if len(actions) != 0 {
		t.Fatalf("expected no actions for empty selection, got %d", len(actions))
	}

	// No llm client: should also return empty even if selection non-empty
	p2 := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line: 0}, End: Position{Line: 0, Character: 7}}}
	out.Reset()
	req2 := Request{JSONRPC: "2.0", ID: json.RawMessage("5"), Method: "textDocument/codeAction", Params: mustJSON(p2)}
	s.handleCodeAction(req2)
	resp2 := captureResponse(t, &out)
	var actions2 []CodeAction
	rb2, _ := json.Marshal(resp2.Result)
	_ = json.Unmarshal(rb2, &actions2)
	if len(actions2) != 0 {
		t.Fatalf("expected no actions when llm is nil")
	}
}

func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestHandle_UnknownMethod_ReturnsError(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out, handlers: map[string]func(Request){}}
	initServerDefaults(s)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("9"), Method: "no/such"}
	out.Reset()
	s.handle(req)
	resp := captureResponse(t, &out)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected method not found error, got %+v", resp.Error)
	}
}

func TestHandle_Dispatch_Initialize(t *testing.T) {
	var out bytes.Buffer
	// Build a server via constructor to ensure handlers map is populated
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{})
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("13"), Method: "initialize"}
	out.Reset()
	s.handle(req)
	resp := captureResponse(t, &out)
	var init InitializeResult
	b, _ := json.Marshal(resp.Result)
	_ = json.Unmarshal(b, &init)
	if init.Capabilities.CodeActionProvider == nil || init.Capabilities.CompletionProvider == nil {
		t.Fatalf("missing capabilities")
	}
}

func TestDetectAndHandleChat_InsertsReply(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{})
	cfg := s.cfg
	if strings.TrimSpace(cfg.ChatSuffix) == "" {
		cfg.ChatSuffix = ">"
		cfg.ChatPrefixes = []string{"?", "!", ":", ";"}
		s.cfg = cfg
	}
	s.llmClient = fakeLLM{resp: tut.MultilineChatReply()}
	uri := "file:///chat.go"
	// Place a prompt line with a supported trigger at EOL, then a blank line
	s.setDocument(uri, "What time?>\n\n")
	out.Reset()
	s.chatSvc().detectAndHandleChat(uri)
	// Wait for the background chat goroutine to finish writing.
	s.inflight.Wait()
	if out.Len() == 0 {
		t.Fatalf("no output written by detectAndHandleChat")
	}
	// Expect a workspace/applyEdit request to be written
	req := captureRequest(t, &out)
	if req.Method != "workspace/applyEdit" {
		t.Fatalf("expected workspace/applyEdit, got %s", req.Method)
	}
	var params ApplyWorkspaceEditParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	we := params.Edit
	if len(we.Changes) == 0 {
		t.Fatalf("expected changes in edit")
	}
	edits := we.Changes[uri]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (delete+insert), got %d", len(edits))
	}
	if !strings.Contains(edits[1].NewText, "> Hello") || !strings.Contains(edits[1].NewText, "multi-line reply") {
		t.Fatalf("expected multi-line reply insertion, got %q", edits[1].NewText)
	}
}

func TestHandleCodeActionResolve_Diagnostics(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	initServerDefaults(s)
	s.llmClient = fakeLLM{resp: "fixed"}
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nvar x = 1\n")
	payload := struct {
		Type        string       `json:"type"`
		URI         string       `json:"uri"`
		Range       Range        `json:"range"`
		Selection   string       `json:"selection"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}{Type: "diagnostics", URI: uri, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 10}}, Selection: "var x = 1", Diagnostics: []Diagnostic{{Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 5}}, Message: "bad"}}}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: resolve diagnostics", Data: raw}
	b, _ := json.Marshal(ca)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("3"), Method: "codeAction/resolve", Params: b}
	out.Reset()
	s.handleCodeActionResolve(req)
	resp := captureResponse(t, &out)
	var resolved CodeAction
	rb, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(rb, &resolved); err != nil {
		t.Fatalf("decode resolved: %v", err)
	}
	if resolved.Edit == nil {
		t.Fatalf("expected resolved edit for diagnostics")
	}
}
