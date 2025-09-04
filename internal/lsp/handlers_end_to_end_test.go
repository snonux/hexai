package lsp

import (
    "bytes"
    "encoding/json"
    "io"
    "log"
    "strings"
    "testing"
    "time"
)

// captureResponse decodes a single LSP Response from the server's output buffer.
func captureResponse(t *testing.T, buf *bytes.Buffer) Response {
    t.Helper()
    raw := buf.String()
    // strip Content-Length header framing
    idx := strings.Index(raw, "\r\n\r\n")
    if idx < 0 { t.Fatalf("no header/body separator in %q", raw) }
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
    idx := strings.Index(raw, "\r\n\r\n")
    if idx < 0 { t.Fatalf("no header/body separator in %q", raw) }
    body := raw[idx+4:]
    var req Request
    if err := json.Unmarshal([]byte(body), &req); err != nil {
        t.Fatalf("unmarshal request: %v", err)
    }
    return req
}

func TestHandleCodeAction_ListsHexaiActions(t *testing.T) {
    // Prepare server
    var out bytes.Buffer
    s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
    s.llmClient = fakeLLM{resp: "// doc\nfunc add(a,b int) int { return a+b }"}

    // Document with a function
    uri := "file:///x.go"
    src := "package p\n\nfunc add(a,b int) int { return a+b }\n"
    s.setDocument(uri, src)

    // Select the function line
    p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line:2, Character:0}, End: Position{Line:2, Character:len("func add(a,b int) int { return a+b }")}}}
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
    if len(actions) == 0 { t.Fatalf("expected some actions") }
    // Ensure our Hexai actions are present
    hasDoc := false
    hasGoTest := false
    for _, a := range actions {
        if strings.Contains(strings.ToLower(a.Title), "hexai:") {
            if strings.Contains(a.Title, "document code") { hasDoc = true }
            if strings.Contains(a.Title, "implement unit test") { hasGoTest = true }
        }
    }
    if !hasDoc || !hasGoTest {
        t.Fatalf("expected both Hexai actions, got %+v", actions)
    }
}

func TestHandleCodeActionResolve_Document(t *testing.T) {
    var out bytes.Buffer
    s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
    s.llmClient = fakeLLM{resp: "// doc\nfunc f(){}"}
    uri := "file:///x.go"
    s.setDocument(uri, "package p\nfunc f(){}\n")
    // Build a document code action payload
    payload := struct {
        Type      string `json:"type"`
        URI       string `json:"uri"`
        Range     Range  `json:"range"`
        Selection string `json:"selection"`
    }{Type: "document", URI: uri, Range: Range{Start: Position{Line:1}, End: Position{Line:1, Character: 10}}, Selection: "func f(){}"}
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
    if resolved.Edit == nil { t.Fatalf("expected resolved edit") }
}

func TestHandleCodeAction_NoLLMOrEmptySelection_ReturnsEmpty(t *testing.T) {
    var out bytes.Buffer
    s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
    uri := "file:///x.go"
    s.setDocument(uri, "package p\n\n")
    // Empty selection
    p := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line:1}, End: Position{Line:1}}}
    b, _ := json.Marshal(p)
    req := Request{JSONRPC: "2.0", ID: json.RawMessage("4"), Method: "textDocument/codeAction", Params: b}
    out.Reset()
    s.handleCodeAction(req)
    resp := captureResponse(t, &out)
    var actions []CodeAction
    rb, _ := json.Marshal(resp.Result)
    _ = json.Unmarshal(rb, &actions)
    if len(actions) != 0 { t.Fatalf("expected no actions for empty selection, got %d", len(actions)) }

    // No llm client: should also return empty even if selection non-empty
    p2 := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Range: Range{Start: Position{Line:0}, End: Position{Line:0, Character:7}}}
    out.Reset()
    req2 := Request{JSONRPC: "2.0", ID: json.RawMessage("5"), Method: "textDocument/codeAction", Params: mustJSON(p2)}
    s.handleCodeAction(req2)
    resp2 := captureResponse(t, &out)
    var actions2 []CodeAction
    rb2, _ := json.Marshal(resp2.Result)
    _ = json.Unmarshal(rb2, &actions2)
    if len(actions2) != 0 { t.Fatalf("expected no actions when llm is nil") }
}

func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestDetectAndHandleChat_InsertsReply(t *testing.T) {
    var out bytes.Buffer
    s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
    s.llmClient = fakeLLM{resp: "Hello"}
    uri := "file:///chat.go"
    // Place a prompt line with a supported trigger at EOL, then a blank line
    s.setDocument(uri, "What time?>\n\n")
    out.Reset()
    s.detectAndHandleChat(uri)
    // Allow async goroutine to write the request
    for i := 0; i < 20 && out.Len() == 0; i++ { time.Sleep(10 * time.Millisecond) }
    if out.Len() == 0 { t.Fatalf("no output written by detectAndHandleChat") }
    // Expect a workspace/applyEdit request to be written
    req := captureRequest(t, &out)
    if req.Method != "workspace/applyEdit" { t.Fatalf("expected workspace/applyEdit, got %s", req.Method) }
    var params ApplyWorkspaceEditParams
    if err := json.Unmarshal(req.Params, &params); err != nil { t.Fatalf("decode params: %v", err) }
    we := params.Edit
    if len(we.Changes) == 0 { t.Fatalf("expected changes in edit") }
    edits := we.Changes[uri]
    if len(edits) != 2 { t.Fatalf("expected 2 edits (delete+insert), got %d", len(edits)) }
    if !strings.Contains(edits[1].NewText, "> Hello") { t.Fatalf("expected reply insertion with '> Hello', got %q", edits[1].NewText) }
}

func TestHandleCodeActionResolve_Diagnostics(t *testing.T) {
    var out bytes.Buffer
    s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
    s.llmClient = fakeLLM{resp: "fixed"}
    uri := "file:///x.go"
    s.setDocument(uri, "package p\nvar x = 1\n")
    payload := struct {
        Type        string       `json:"type"`
        URI         string       `json:"uri"`
        Range       Range        `json:"range"`
        Selection   string       `json:"selection"`
        Diagnostics []Diagnostic `json:"diagnostics"`
    }{Type: "diagnostics", URI: uri, Range: Range{Start: Position{Line:1}, End: Position{Line:1, Character: 10}}, Selection: "var x = 1", Diagnostics: []Diagnostic{{Range: Range{Start: Position{Line:1}, End: Position{Line:1, Character:5}}, Message: "bad"}}}
    raw, _ := json.Marshal(payload)
    ca := CodeAction{Title: "Hexai: resolve diagnostics", Data: raw}
    b, _ := json.Marshal(ca)
    req := Request{JSONRPC: "2.0", ID: json.RawMessage("3"), Method: "codeAction/resolve", Params: b}
    out.Reset()
    s.handleCodeActionResolve(req)
    resp := captureResponse(t, &out)
    var resolved CodeAction
    rb, _ := json.Marshal(resp.Result)
    if err := json.Unmarshal(rb, &resolved); err != nil { t.Fatalf("decode resolved: %v", err) }
    if resolved.Edit == nil { t.Fatalf("expected resolved edit for diagnostics") }
}
