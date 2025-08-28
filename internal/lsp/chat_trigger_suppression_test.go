package lsp

import "testing"

// Ensure completion is suppressed when a chat trigger is at EOL (?>,!>,:>,;>)
func TestCompletionSuppressedOnChatTriggerEOL(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_"}, compCache: make(map[string]string) }
    s.llmClient = &countingLLM{}
    tests := []string{"What now?>", "Explain!>", "Refactor:>", "note ;>"}
    for i, line := range tests {
        p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://chat-suppr.go"} }
        items, ok := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
        if !ok { t.Fatalf("case %d: expected ok=true", i) }
        if len(items) != 0 { t.Fatalf("case %d: expected no completion items for EOL chat trigger", i) }
    }
}

