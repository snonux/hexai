package lsp

import (
    "context"
    "testing"

    "codeberg.org/snonux/hexai/internal/llm"
)

type fakeCompleterOk struct{}

func (fakeCompleterOk) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) { return "", nil }
func (fakeCompleterOk) Name() string         { return "prov" }
func (fakeCompleterOk) DefaultModel() string { return "m" }
func (fakeCompleterOk) CodeCompletion(context.Context, string, string, int, string, float64) ([]string, error) {
    return []string{"SUGG"}, nil
}

func TestProviderNativeCompletion_Success(t *testing.T) {
    s := newTestServer()
    s.llmClient = fakeCompleterOk{}
    // current line with dot trigger; position after dot
    current := "fmt."
    p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x.go"}, Position: Position{Line: 0, Character: len(current)}}
    items, ok := s.tryProviderNativeCompletion(current, p, "", "", "func f(){}", "doc", false, "", false)
    if !ok || len(items) == 0 {
        t.Fatalf("expected provider-native items")
    }
    if items[0].Label == "" || items[0].TextEdit == nil {
        t.Fatalf("unexpected completion item: %+v", items[0])
    }
}

