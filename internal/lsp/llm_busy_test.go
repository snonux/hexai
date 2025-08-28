package lsp

import (
    "encoding/json"
    "testing"
)

// Ensure a visible busy item is returned when a prior LLM request is in flight.
func TestLLMBusy_YieldsBusyCompletionItem(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{"."}, compCache: make(map[string]string) }
    s.llmClient = &countingLLM{}
    // Mark busy
    s.setLLMBusy(true)
    t.Cleanup(func(){ s.setLLMBusy(false) })
    line := "obj."
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://busy.go"} }
    // Simulate manual invoke to bypass min-prefix
    p.Context = json.RawMessage([]byte(`{"triggerKind":1}`))
    items, ok := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if !ok { t.Fatalf("expected ok=true") }
    if len(items) != 1 { t.Fatalf("expected one busy item, got %d", len(items)) }
    if items[0].InsertText != "" { t.Fatalf("busy item should not insert text") }
    if items[0].Label == "" { t.Fatalf("busy item should have a label") }
}

