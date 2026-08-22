package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
)

func TestShouldSuppressForChatTriggerEOL_CustomConfig(t *testing.T) {
	s := newTestServer()
	// Customize: only ")#" at EOL suppresses
	cfg := s.cfg
	cfg.ChatSuffix = "#"
	cfg.ChatPrefixes = []string{")"}
	s.cfg = cfg

	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line: 0, Character: 6}}
	if !s.completion.shouldSuppressForChatTriggerEOL("ok)#", p) {
		t.Fatalf("expected suppression for custom prefix+suffix at EOL")
	}
	if s.completion.shouldSuppressForChatTriggerEOL("ok]#", p) {
		t.Fatalf("did not expect suppression for non-matching prefix")
	}
}

func TestNewServer_AssignsTriggerGlobals_AndParsingUsesThem(t *testing.T) {
	var out bytes.Buffer
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{InlineOpen: "<", InlineClose: ">", ChatSuffix: ")", ChatPrefixes: []string{":"}}}
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{Config: &cfg})
	openStr, _, openChar, closeChar := s.inlineMarkers()
	if openChar != '<' || closeChar != '>' {
		t.Fatalf("inline markers not applied: %q %q", string(openChar), string(closeChar))
	}
	_, prefixes, suffixChar := s.chatConfig()
	if suffixChar != ')' || len(prefixes) == 0 || prefixes[0] != ":" {
		t.Fatalf("chat markers not applied: suffix=%q prefixes=%v", string(suffixChar), prefixes)
	}
	if txt, l, r, ok := findStrictInlineTag("x<do>y", openStr, openChar, closeChar); !ok || txt != "do" || l != 1 || r != 5 {
		t.Fatalf("findStrictInlineTag failed: ok=%v txt=%q l=%d r=%d", ok, txt, l, r)
	}
	if got := s.chatSvc().stripTrailingTrigger("note:)"); got != "note:" {
		t.Fatalf("stripTrailingTrigger failed: %q", got)
	}
}

func TestIsTriggerEvent_BareDoubleOpenBlocksEvenWithContextTriggerChar(t *testing.T) {
	s := newTestServer()
	cfg := s.cfg
	cfg.InlineOpen = ">"
	cfg.TriggerCharacters = []string{"."}
	s.cfg = cfg
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
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{ChatSuffix: "#", ChatPrefixes: []string{")"}}}
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{Config: &cfg})
	s.llmClient = fakeLLM{resp: "Hello\nmulti-line reply"}
	uri := "file:///chat2.go"
	s.setDocument(uri, "ok)#\n\n")
	out.Reset()
	s.chatSvc().detectAndHandleChat(uri)
	// Wait for the background chat goroutine to finish writing.
	s.inflight.Wait()
	if out.Len() == 0 {
		t.Fatalf("no output written for custom chat config")
	}
}
