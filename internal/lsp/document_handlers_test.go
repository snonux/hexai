package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"testing"
	"time"
)

func TestDidOpenChangeClose_UpdateDocs(t *testing.T) {
	s := newTestServer()
	uri := "file:///x.go"
	// didOpen
	open := DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "a\n"}}
	s.handleDidOpen(Request{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: mustJSON(open)})
	if s.getDocument(uri) == nil {
		t.Fatalf("doc not opened")
	}
	// didChange
	ch := DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri}, ContentChanges: []TextDocumentContentChangeEvent{{Text: "b\n"}}}
	s.handleDidChange(Request{JSONRPC: "2.0", Method: "textDocument/didChange", Params: mustJSON(ch)})
	if d := s.getDocument(uri); d == nil || d.text != "b\n" {
		t.Fatalf("doc not changed")
	}
	// didClose
	s.handleDidClose(Request{JSONRPC: "2.0", Method: "textDocument/didClose", Params: mustJSON(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}})})
	if s.getDocument(uri) != nil {
		t.Fatalf("doc not closed")
	}
}

func TestClientShowDocument_WritesRequest(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	uri := "file:///x.go"
	sel := Range{Start: Position{Line: 1}, End: Position{Line: 2}}
	out.Reset()
	s.clientShowDocument(uri, &sel)
	req := captureRequest(t, &out)
	if req.Method != "window/showDocument" {
		t.Fatalf("got %s", req.Method)
	}
}

func TestHandleExecuteCommand_ShowDocument(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	uri := "file:///x.go"
	r := Range{Start: Position{Line: 0}, End: Position{Line: 0}}
	args := []any{uri, r}
	params := ExecuteCommandParams{Command: "hexai.showDocument", Arguments: args}
	s.handleExecuteCommand(Request{JSONRPC: "2.0", ID: json.RawMessage("11"), Method: "workspace/executeCommand", Params: mustJSON(params)})
	req := captureRequest(t, &out)
	if req.Method != "window/showDocument" {
		t.Fatalf("expected showDocument after executeCommand, got %s", req.Method)
	}
}

func TestDeferShowDocument_WritesLater(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	uri := "file:///x.go"
	out.Reset()
	s.deferShowDocument(uri, Range{Start: Position{Line: 0}, End: Position{Line: 0}})
	// wait >120ms per implementation
	time.Sleep(160 * time.Millisecond)
	req := captureRequest(t, &out)
	if req.Method != "window/showDocument" {
		t.Fatalf("expected showDocument, got %s", req.Method)
	}
}
