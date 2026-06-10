package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleCompletionRespectsDisableCommand(t *testing.T) {
	s := newTestServer()
	var buf bytes.Buffer
	s.out = &buf

	// Disable completions and trigger handler
	s.setCompletionsDisabled(true)

	params := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///test.go"}, Position: Position{Line: 0, Character: 0}}
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "textDocument/completion", Params: mustJSON(params)}

	s.completion.handleCompletion(req)

	payload := buf.String()
	parts := strings.SplitN(payload, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected response framing: %q", payload)
	}
	var resp Response
	if err := json.Unmarshal([]byte(parts[1]), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	switch v := resultMap["items"].(type) {
	case []any:
		if len(v) != 0 {
			t.Fatalf("expected no completion items when disabled, got %d", len(v))
		}
	case nil:
		// ok: encoder emitted null for slice
	default:
		t.Fatalf("expected items slice or null, got %T", v)
	}
}
