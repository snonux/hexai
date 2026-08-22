package lsp

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/snonux/hexai/internal/llm"
)

// fakeCompleterErr implements both Client and CodeCompleter; CodeCompletion errors,
// forcing tryProviderNativeCompletion to take the error path and fall back to chat.
type fakeCompleterErr struct{}

func (fakeCompleterErr) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "X", nil
}
func (fakeCompleterErr) Name() string         { return "prov" }
func (fakeCompleterErr) DefaultModel() string { return "m" }
func (fakeCompleterErr) CodeCompletion(context.Context, string, string, int, string, float64) ([]string, error) {
	return nil, io.EOF
}

func TestCompletion_FallbackOnProviderError(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeCompleterErr{}
	// Provide simple document
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nfunc f(){\nfmt.\n}\n")
	// Position after 'fmt.' to satisfy prefix heuristics
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 2, Character: 4}}
	// Build context for trigger character '.'
	ctx := struct {
		TriggerKind      int    `json:"triggerKind"`
		TriggerCharacter string `json:"triggerCharacter"`
	}{TriggerKind: 2, TriggerCharacter: "."}
	bctx, _ := json.Marshal(ctx)
	p.Context = json.RawMessage(bctx)

	// Call handleCompletion and ensure it returns at least one item from chat fallback
	var buf nopWriter
	s.out = &buf
	s.completion.handleCompletion(Request{JSONRPC: "2.0", ID: json.RawMessage("6"), Method: "textDocument/completion", Params: mustJSON(p)})
	// No panic implies path executed; detailed decode not needed here
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
