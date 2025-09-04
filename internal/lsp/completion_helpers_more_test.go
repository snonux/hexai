package lsp

import (
    "encoding/json"
    "testing"
)

func TestExtractTriggerInfo_ParseManualInvoke(t *testing.T) {
    // Compose a CompletionParams with a raw JSON context
    ctx := struct{ TriggerKind int `json:"triggerKind"`; TriggerCharacter string `json:"triggerCharacter"` }{TriggerKind: 1, TriggerCharacter: "."}
    raw, _ := json.Marshal(ctx)
    p := CompletionParams{Context: json.RawMessage(raw)}
    kind, ch := extractTriggerInfo(p)
    if kind != 1 || ch != "." { t.Fatalf("unexpected trigger info: %d %q", kind, ch) }
    if !parseManualInvoke(json.RawMessage(raw)) { t.Fatalf("expected manual invoke true") }
}

func TestShouldSuppressForChatTriggerEOL(t *testing.T) {
    s := newTestServer()
    p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line:0, Character:10}}
    line := "say hi;>"
    if !s.shouldSuppressForChatTriggerEOL(line, p) { t.Fatalf("expected suppression when ;> at EOL") }
    if s.shouldSuppressForChatTriggerEOL("plain>", p) { t.Fatalf("should not suppress for plain >") }
}

func TestPrefixHeuristicAllows(t *testing.T) {
    s := newTestServer()
    // inline prompt allows zero prefix
    if !s.prefixHeuristicAllows(true, "", CompletionParams{Position: Position{Line:0, Character:0}}, false) { t.Fatalf("inline prompt should allow") }
    // structural triggers like '.' allow without prefix
    if !s.prefixHeuristicAllows(false, "fmt.", CompletionParams{Position: Position{Line:0, Character:4}}, false) { t.Fatalf("dot trigger should allow") }
    // otherwise need at least minimal prefix (default min=1)
    if s.prefixHeuristicAllows(false, " ", CompletionParams{Position: Position{Line:0, Character:0}}, false) { t.Fatalf("should not allow with no prefix") }
}

