package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"
	"time"
)

func frame(t *testing.T, msg any) string {
	t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), b)
}

// TestRun_AppliesDidChangeInOrder sends many full-sync didChange notifications
// back to back. They used to be handled in separate goroutines, so an older
// change could land last and leave the server with stale text.
func TestRun_AppliesDidChangeInOrder(t *testing.T) {
	uri := "file:///order.go"
	var in strings.Builder
	in.WriteString(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "text": ""}},
	}))
	for i := 1; i <= 200; i++ {
		in.WriteString(frame(t, map[string]any{
			"jsonrpc": "2.0", "method": "textDocument/didChange",
			"params": map[string]any{
				"textDocument":   map[string]any{"uri": uri, "version": i},
				"contentChanges": []map[string]any{{"text": fmt.Sprintf("v%d", i)}},
			},
		}))
	}
	s := NewServer(strings.NewReader(in.String()), io.Discard, log.New(io.Discard, "", 0), ServerOptions{})
	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if d := s.getDocument(uri); d == nil || d.text != "v200" {
		t.Fatalf("expected the last change to win, got %+v", d)
	}
}

// TestRun_ReturnsOnExitWithoutEOF checks that "exit" stops the server even
// when the client keeps stdin open.
func TestRun_ReturnsOnExitWithoutEOF(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	s := NewServer(pr, io.Discard, log.New(io.Discard, "", 0), ServerOptions{})
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background()) }()
	if _, err := io.WriteString(pw, frame(t, map[string]any{"jsonrpc": "2.0", "method": "exit"})); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not exit")
	}
}

func TestHandledInline(t *testing.T) {
	for _, m := range []string{"textDocument/didOpen", "textDocument/didChange", "textDocument/didClose", "exit"} {
		if !handledInline(m) {
			t.Fatalf("%s should be handled inline", m)
		}
	}
	if handledInline("textDocument/completion") {
		t.Fatalf("requests should run concurrently")
	}
}

func TestBeginCompletion_SupersedesOlderRequest(t *testing.T) {
	s := newTestServer()
	cs := s.completionSvc()
	ctx1, done1 := cs.beginCompletion("file:///a.go")
	ctxOther, doneOther := cs.beginCompletion("file:///b.go")
	ctx2, done2 := cs.beginCompletion("file:///a.go")
	if ctx1.Err() == nil {
		t.Fatalf("older request for the same document should be cancelled")
	}
	if ctxOther.Err() != nil || ctx2.Err() != nil {
		t.Fatalf("other requests must stay alive")
	}
	done1(false) // finishing the superseded request must not drop the newer one
	if cs.active["file:///a.go"] == nil {
		t.Fatalf("newer request should still be registered")
	}
	done2(false)
	doneOther(true)
	if ctx2.Err() == nil {
		t.Fatalf("done should release the context")
	}
	if ctxOther.Err() != nil {
		t.Fatalf("background completion should keep its context for now")
	}
	if len(cs.active) != 0 {
		t.Fatalf("expected no active requests, got %d", len(cs.active))
	}
}

func TestNotifyLLMFailure(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	s.notifyLLMFailure("chat", context.Canceled)
	if out.Len() != 0 {
		t.Fatalf("cancellation should stay silent")
	}
	s.notifyLLMFailure("document code", nil)
	msgs := serverMessages(t, &out)
	if len(msgs) != 1 || msgs[0].Method != "window/showMessage" || !strings.Contains(string(msgs[0].Params), "document code returned no result") {
		t.Fatalf("unexpected messages %+v", msgs)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.serverCtx = ctx
	cancel()
	out.Reset()
	s.notifyLLMFailure("chat", fmt.Errorf("boom"))
	if out.Len() != 0 {
		t.Fatalf("no notifications after shutdown")
	}
}

func TestFitToSelection(t *testing.T) {
	cases := []struct{ sel, out, want string }{
		{"\t\tx := 1\n", "x := 2", "\t\tx := 2\n"},
		{"x := 1", "x := 2", "x := 2"},
		{"\tfoo()\n\tbar()\n", "\tfoo()\n\tbaz()\n", "\tfoo()\n\tbaz()\n"},
	}
	for _, c := range cases {
		if got := fitToSelection(c.sel, c.out); got != c.want {
			t.Fatalf("fitToSelection(%q, %q) = %q, want %q", c.sel, c.out, got, c.want)
		}
	}
}

func TestCodeActionLabel(t *testing.T) {
	if got := codeActionLabel(CodeAction{Title: "Hexai: rewrite selection"}); got != "rewrite selection" {
		t.Fatalf("got %q", got)
	}
	if got := codeActionLabel(CodeAction{}); got != "code action" {
		t.Fatalf("got %q", got)
	}
}

func TestCompleteCodeAction_KeepsLineBreakAndIndent(t *testing.T) {
	s := newTestServer()
	s.out = io.Discard
	s.llmClient = fakeLLM{resp: "```go\ny := 2\n```"}
	uri := "file:///x.go"
	s.setDocument(uri, "func f() {\n\tx := 1\n}\n")
	rng := Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 2, Character: 0}}
	ca, ok := s.completeCodeAction(CodeAction{Title: "Hexai: rewrite selection"}, uri, rng, "sys", "user", time.Second)
	if !ok || ca.Edit == nil {
		t.Fatalf("expected an edit")
	}
	if got := ca.Edit.Changes[uri][0].NewText; got != "\ty := 2\n" {
		t.Fatalf("got %q", got)
	}
}
