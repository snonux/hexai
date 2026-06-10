package lsp

import (
	"bytes"
	"strings"
	"testing"
)

// Ensure in-editor chat respects general.context_mode by adding window/full-file context.
func TestChat_RespectsContextModeWindow(t *testing.T) {
	s := newTestServer()
	// Configure window mode with small window
	s.cfg.ContextMode = "window"
	s.cfg.ContextWindowLines = 2
	s.cfg.MaxContextTokens = 2000
	cap := &captureLLM{}
	s.llmClient = cap
	var out bytes.Buffer
	s.out = &out

	uri := "file:///ctx.go"
	// Build a small file where the last line triggers chat
	src := "package main\nline2 context\nwhat?>\n"
	s.setDocument(uri, src)

	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()
	if len(cap.msgs) == 0 {
		t.Fatalf("expected Chat to be called")
	}
	// Expect first system, then an extra context user message, then history ending with prompt
	if cap.msgs[0].Role != "system" {
		t.Fatalf("first message should be system, got %q", cap.msgs[0].Role)
	}
	if len(cap.msgs) < 3 {
		t.Fatalf("expected at least 3 messages (system, extra, user prompt), got %d", len(cap.msgs))
	}
	extra := cap.msgs[1]
	if extra.Role != "user" || !strings.HasPrefix(extra.Content, "Additional context:\n") {
		t.Fatalf("second message should be user extra context, got role=%q content=%q", extra.Role, extra.Content)
	}
	if !strings.Contains(extra.Content, "line2 context") {
		t.Fatalf("extra context should include window text; got %q", extra.Content)
	}
	last := cap.msgs[len(cap.msgs)-1]
	if last.Role != "user" || last.Content != "what?" {
		t.Fatalf("last message should be current prompt user, got %+v", last)
	}
}

func TestChat_ContextModeMinimal_NoExtra(t *testing.T) {
	s := newTestServer()
	s.cfg.ContextMode = "minimal"
	s.cfg.MaxContextTokens = 2000
	cap := &captureLLM{}
	s.llmClient = cap
	var out bytes.Buffer
	s.out = &out

	uri := "file:///ctx2.go"
	s.setDocument(uri, "package main\nhelp?>\n")
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()

	if len(cap.msgs) != 2 {
		t.Fatalf("expected exactly 2 messages (system + user prompt), got %d", len(cap.msgs))
	}
	if cap.msgs[0].Role != "system" || cap.msgs[1].Role != "user" || cap.msgs[1].Content != "help?" {
		t.Fatalf("unexpected messages: %+v", cap.msgs)
	}
}

func TestChat_ContextModeAlwaysFull_AddsExtra(t *testing.T) {
	s := newTestServer()
	s.cfg.ContextMode = "always-full"
	s.cfg.MaxContextTokens = 2000
	cap := &captureLLM{}
	s.llmClient = cap
	var out bytes.Buffer
	s.out = &out

	uri := "file:///ctx3.go"
	s.setDocument(uri, "package main\nline2\nhelp?>\n")
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()

	if len(cap.msgs) < 3 {
		t.Fatalf("expected >=3 messages (system, extra, user prompt), got %d", len(cap.msgs))
	}
	if cap.msgs[1].Role != "user" || !strings.HasPrefix(cap.msgs[1].Content, "Additional context:\n") {
		t.Fatalf("second message should be user extra context, got role=%q content=%q", cap.msgs[1].Role, cap.msgs[1].Content)
	}
	if !strings.Contains(cap.msgs[1].Content, "package main") {
		t.Fatalf("extra context should include full file, got %q", cap.msgs[1].Content)
	}
	if last := cap.msgs[len(cap.msgs)-1]; last.Role != "user" || last.Content != "help?" {
		t.Fatalf("last message should be the current prompt, got %+v", last)
	}
}

func TestChat_ContextModeFileOnNewFunc_NoExtraWithoutSignature(t *testing.T) {
	s := newTestServer()
	s.cfg.ContextMode = "file-on-new-func"
	s.cfg.MaxContextTokens = 2000
	cap := &captureLLM{}
	s.llmClient = cap
	var out bytes.Buffer
	s.out = &out

	uri := "file:///ctx4.go"
	s.setDocument(uri, "package main\nhelp?>\n")
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()

	if len(cap.msgs) != 2 {
		t.Fatalf("expected exactly 2 messages (system + user prompt), got %d", len(cap.msgs))
	}
}

func TestChat_ContextModeFileOnNewFunc_WithSignature_AddsExtra(t *testing.T) {
	s := newTestServer()
	s.cfg.ContextMode = "file-on-new-func"
	s.cfg.MaxContextTokens = 2000
	cap := &captureLLM{}
	s.llmClient = cap
	var out bytes.Buffer
	s.out = &out

	uri := "file:///ctx5.go"
	// Signature without '{' yet; chat prompt appears before the body, so newFunc=true
	src := "package main\n\nfunc add(x int) int\nhelp?>\n"
	s.setDocument(uri, src)
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()

	if len(cap.msgs) < 3 {
		t.Fatalf("expected >=3 messages (system, extra, user prompt), got %d", len(cap.msgs))
	}
	if cap.msgs[1].Role != "user" || !strings.HasPrefix(cap.msgs[1].Content, "Additional context:\n") {
		t.Fatalf("second message should be user extra context, got role=%q content=%q", cap.msgs[1].Role, cap.msgs[1].Content)
	}
	if !strings.Contains(cap.msgs[1].Content, "func add(x int) int") {
		t.Fatalf("extra context should include full file or signature, got %q", cap.msgs[1].Content)
	}
}
