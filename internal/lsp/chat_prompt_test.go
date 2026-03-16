package lsp

import (
	"bytes"
	"testing"
)

func TestDetectAndHandleChat_UsesConfiguredSystemPrompt(t *testing.T) {
	s := newTestServer()
	cap := &captureLLM{}
	s.llmClient = cap
	cfg := s.cfg
	cfg.PromptChatSystem = "CHAT-SYS"
	s.cfg = cfg
	uri := "file:///chat.txt"
	// Avoid nil writer in applyChatEdits
	var out bytes.Buffer
	s.out = &out
	// Line that should trigger chat: ends with '>' and previous char in prefixes
	s.setDocument(uri, "help?>\n")
	s.detectAndHandleChat(uri)
	// Wait for the background chat goroutine to finish.
	s.inflight.Wait()
	if len(cap.msgs) == 0 {
		t.Fatalf("expected Chat to be called")
	}
	if cap.msgs[0].Role != "system" || cap.msgs[0].Content != "CHAT-SYS" {
		t.Fatalf("unexpected system msg: %+v", cap.msgs[0])
	}
	// Last should be user with prompt without trailing '>'
	last := cap.msgs[len(cap.msgs)-1]
	if last.Role != "user" || last.Content != "help?" {
		t.Fatalf("unexpected last user msg: %+v", last)
	}
}
