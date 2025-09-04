package lsp

import (
    "bytes"
    "encoding/json"
    "io"
    "log"
    "strings"
    "testing"
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

