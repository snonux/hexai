package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"testing"
)

func TestHandleShutdown_Replies(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	initServerDefaults(s)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("12"), Method: "shutdown"}
	out.Reset()
	s.handleShutdown(req)
	resp := captureResponse(t, &out)
	if string(resp.ID) != "12" || resp.Error != nil {
		t.Fatalf("unexpected shutdown response: %+v", resp)
	}
}
