package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"testing"
	"time"
)

func TestShouldSuppressForChatTriggerEOL_CustomConfig(t *testing.T) {
	s := newTestServer()
	// Customize: only ")#" at EOL suppresses
	s.chatSuffix = "#"
	s.chatPrefixes = []string{")"}

	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line: 0, Character: 6}}
	if !s.shouldSuppressForChatTriggerEOL("ok)#", p) {
		t.Fatalf("expected suppression for custom prefix+suffix at EOL")
	}
	if s.shouldSuppressForChatTriggerEOL("ok]#", p) {
		t.Fatalf("did not expect suppression for non-matching prefix")
	}
}

func TestNewServer_AssignsTriggerGlobals_AndParsingUsesThem(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{
		InlineOpen: "<", InlineClose: ">", ChatSuffix: ")", ChatPrefixes: []string{":"},
	})
	_ = s // ensure server constructed applies globals
	if s.inlineOpenChar != '<' || s.inlineCloseChar != '>' {
		t.Fatalf("inline markers not applied: %q %q", string(s.inlineOpenChar), string(s.inlineCloseChar))
	}
	if s.chatSuffixChar != ')' || len(s.chatPrefixes) == 0 || s.chatPrefixes[0] != ":" {
		t.Fatalf("chat markers not applied: suffix=%q prefixes=%v", string(s.chatSuffixChar), s.chatPrefixes)
	}
	if txt, l, r, ok := findStrictInlineTag("x<do>y", s.inlineOpenChar, s.inlineCloseChar); !ok || txt != "do" || l != 1 || r != 5 {
		t.Fatalf("findStrictInlineTag failed: ok=%v txt=%q l=%d r=%d", ok, txt, l, r)
	}
	if got := s.stripTrailingTrigger("note:)"); got != "note:" {
		t.Fatalf("stripTrailingTrigger failed: %q", got)
	}
}

func TestIsTriggerEvent_BareDoubleOpenBlocksEvenWithContextTriggerChar(t *testing.T) {
	s := newTestServer()
	s.inlineOpen = ">" // ensure bare ">>" check is active
	s.triggerChars = []string{"."}
	// LSP context indicates TriggerCharacter '.' but current line is bare ">>"
	ctx := struct {
		TriggerKind      int    `json:"triggerKind"`
		TriggerCharacter string `json:"triggerCharacter"`
	}{TriggerKind: 2, TriggerCharacter: "."}
	raw, _ := json.Marshal(ctx)
	p := CompletionParams{Position: Position{Line: 0, Character: 2}, Context: json.RawMessage(raw)}
	if s.isTriggerEvent(p, ">>") {
		t.Fatalf("bare double-open should block trigger event even with context trigger char")
	}
}

func TestDetectAndHandleChat_CustomConfig_InsertsReply(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{ChatSuffix: "#", ChatPrefixes: []string{")"}})
	s.llmClient = fakeLLM{resp: "Hello\nmulti-line reply"}
	uri := "file:///chat2.go"
	s.setDocument(uri, "ok)#\n\n")
	out.Reset()
	s.detectAndHandleChat(uri)
	// Give time for applyEdit request
	for i := 0; i < 20 && out.Len() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if out.Len() == 0 {
		t.Fatalf("no output written for custom chat config")
	}
}
