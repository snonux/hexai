package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/llm"
)

// fakeLLMInline returns a canned suggestion to help validate inline prompt handling.
type fakeLLMInline struct{}

func (fakeLLMInline) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "Здравей свят", nil
}
func (fakeLLMInline) Name() string         { return "fake" }
func (fakeLLMInline) DefaultModel() string { return "inline" }

func TestHandleCompletionInlinePromptDoubleArrow(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{})
	initServerDefaults(s)
	s.llmClient = fakeLLMInline{}
	uri := "file:///inline.go"
	line := "hello world >>!translate this into bulgarian>"
	s.setDocument(uri, line)
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 0, Character: len(line)}}
	ctx := struct {
		TriggerKind      int    `json:"triggerKind"`
		TriggerCharacter string `json:"triggerCharacter"`
	}{TriggerKind: 1}
	bctx, _ := json.Marshal(ctx)
	p.Context = json.RawMessage(bctx)

	s.handleCompletion(Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "textDocument/completion", Params: mustJSON(p)})
	resp := captureResponse(t, &out)
	var list CompletionList
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatalf("decode completion list: %v", err)
	}
	if len(list.Items) == 0 {
		t.Fatalf("expected completion items")
	}
	item := list.Items[0]
	if got := strings.TrimSpace(item.Label); got == "" {
		t.Fatalf("expected label for inline completion")
	}
	if len(item.AdditionalTextEdits) == 0 {
		t.Fatalf("expected removal edits for inline prompt")
	}
	found := false
	for _, edit := range item.AdditionalTextEdits {
		if edit.Range.Start.Line == 0 && edit.Range.End.Line == 0 && edit.Range.Start.Character == 0 && edit.Range.End.Character == len(line) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("inline prompt removal edit missing: %+v", item.AdditionalTextEdits)
	}
}
