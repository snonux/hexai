package lsp

import (
	"encoding/json"
	"testing"
)

func TestLineHasInlinePrompt_BasicAndDoubleOpen(t *testing.T) {
	// Basic inline
	if !lineHasInlinePrompt("do >task> now", '>', '>') {
		t.Fatalf("expected inline prompt detection for >text>")
	}
	// Double-open variant should be recognized as inline prompt too
	if !lineHasInlinePrompt(">>replace>", '>', '>') {
		t.Fatalf("expected inline prompt detection for >>text>")
	}
}

func TestIsTriggerEvent_TriggerCharNotAllowed(t *testing.T) {
	s := newTestServer()
	s.triggerChars = []string{"."} // only dot allowed
	p := CompletionParams{Position: Position{Line: 0, Character: 3}}
	if s.isTriggerEvent(p, "ab:") { // ':' not in triggerChars
		t.Fatalf("expected false when TriggerCharacter not configured")
	}
}

func TestShouldSuppressForChatTriggerEOL_EmptySuffix_NoSuppression(t *testing.T) {
	s := newTestServer()
	s.chatSuffix = "" // disabled
	p := CompletionParams{Position: Position{Line: 0, Character: 5}}
	if s.shouldSuppressForChatTriggerEOL("What?>", p) {
		t.Fatalf("expected no suppression when chat suffix is empty")
	}
}

func TestIsTriggerEvent_TriggerCharacterMissing_ReturnsFalse(t *testing.T) {
	s := newTestServer()
	// Context says TriggerCharacter, but none provided
	ctx := struct {
		TriggerKind int `json:"triggerKind"`
	}{TriggerKind: 2}
	raw, _ := json.Marshal(ctx)
	p := CompletionParams{Position: Position{Line: 0, Character: 1}, Context: json.RawMessage(raw)}
	if s.isTriggerEvent(p, "a") {
		t.Fatalf("expected false when TriggerCharacter kind with empty char")
	}
}

func TestIsTriggerEvent_TriggerForIncomplete_FallsBackToChar(t *testing.T) {
	s := newTestServer()
	s.triggerChars = []string{"."}
	// TriggerKind=3 should consult fallback char check
	ctx := struct {
		TriggerKind int `json:"triggerKind"`
	}{TriggerKind: 3}
	raw, _ := json.Marshal(ctx)
	p := CompletionParams{Position: Position{Line: 0, Character: 2}, Context: json.RawMessage(raw)}
	if !s.isTriggerEvent(p, "x.") {
		t.Fatalf("expected true via fallback char for TriggerForIncomplete")
	}
}
