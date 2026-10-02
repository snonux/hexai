package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/snonux/hexai/internal/llm"
)

// gatedLLM blocks every Chat call until release is closed and counts calls,
// mimicking a slow model while the user keeps typing.
type gatedLLM struct {
	calls   atomic.Int32
	release chan struct{}
	resp    string
}

func (g *gatedLLM) Chat(ctx context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	g.calls.Add(1)
	select {
	case <-g.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return g.resp, nil
}
func (g *gatedLLM) Name() string         { return "fake" }
func (g *gatedLLM) DefaultModel() string { return "m" }

// serverMessages decodes every JSON-RPC message the server wrote to buf.
func serverMessages(t *testing.T, buf *bytes.Buffer) []Request {
	t.Helper()
	var out []Request
	raw := buf.String()
	for raw != "" {
		idx := strings.Index(raw, "\r\n\r\n")
		if idx < 0 {
			break
		}
		var n int
		if _, err := fmtSscanContentLength(raw[:idx], &n); err != nil {
			t.Fatalf("bad header %q: %v", raw[:idx], err)
		}
		body := raw[idx+4 : idx+4+n]
		var req Request
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("decode %q: %v", body, err)
		}
		out = append(out, req)
		raw = raw[idx+4+n:]
	}
	return out
}

func methodsOf(msgs []Request) []string {
	var m []string
	for _, r := range msgs {
		m = append(m, r.Method)
	}
	return m
}

func TestDetectAndHandleChat_OneRequestWhileTyping(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	g := &gatedLLM{release: make(chan struct{}), resp: "Paris"}
	s.llmClient = g
	uri := "file:///notes.txt"
	s.setDocument(uri, "capital of France?>")
	s.chatSvc().detectAndHandleChat(uri)
	// The user keeps typing on a new line while the model is still answering.
	for _, text := range []string{"capital of France?>\n", "capital of France?>\nmo", "capital of France?>\nmore"} {
		s.setDocument(uri, text)
		s.chatSvc().detectAndHandleChat(uri)
	}
	close(g.release)
	s.inflight.Wait()
	if got := g.calls.Load(); got != 1 {
		t.Fatalf("expected exactly one LLM request, got %d", got)
	}
	msgs := serverMessages(t, &out)
	if len(msgs) != 1 || msgs[0].Method != "workspace/applyEdit" {
		t.Fatalf("expected a single applyEdit, got %v", methodsOf(msgs))
	}
	// Until the editor applied the answer, further changes must not ask again.
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()
	if got := g.calls.Load(); got != 1 {
		t.Fatalf("expected no new request before the answer lands, got %d", got)
	}
	// Once the answer is in, the same question can be asked again later.
	s.setDocument(uri, "capital of France?\n\n> Paris\n")
	s.chatSvc().detectAndHandleChat(uri)
	s.setDocument(uri, "capital of France?\n\n> Paris\n\ncapital of France?>")
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()
	if got := g.calls.Load(); got != 2 {
		t.Fatalf("expected a second request for a new prompt, got %d", got)
	}
}

func TestDetectAndHandleChat_InlinePromptRunsOnce(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	g := &gatedLLM{release: make(chan struct{}), resp: "fmt.Println(1)"}
	s.llmClient = g
	uri := "file:///main.go"
	base := "package main\n\nfunc main() {\n\t>!print one>\n}\n"
	s.setDocument(uri, base)
	s.chatSvc().detectAndHandleChat(uri)
	s.setDocument(uri, base+"// typing")
	s.chatSvc().detectAndHandleChat(uri)
	close(g.release)
	s.inflight.Wait()
	if got := g.calls.Load(); got != 1 {
		t.Fatalf("expected one inline request, got %d", got)
	}
	edits := chatEditsFromOutput(t, &out, uri)
	if len(edits) != 2 {
		t.Fatalf("expected marker removal + insert, got %+v", edits)
	}
	if edits[0].Range.Start != (Position{Line: 3, Character: 1}) || edits[0].Range.End != (Position{Line: 3, Character: 13}) {
		t.Fatalf("unexpected marker removal range %+v", edits[0].Range)
	}
	if edits[1].NewText != "fmt.Println(1)" || edits[1].Range.Start != (Position{Line: 3, Character: 13}) {
		t.Fatalf("unexpected insert %+v", edits[1])
	}
}

func TestApplyInlineCompletion_FollowsShiftedLine(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///main.go"
	// The prompt was on line 1; a line was inserted above meanwhile.
	s.setDocument(uri, "a\nb\n>!x>")
	if !s.chatSvc().applyInlineCompletion(uri, 1, ">!x>", "y") {
		t.Fatalf("expected edit")
	}
	edits := chatEditsFromOutput(t, &out, uri)
	if len(edits) != 2 || edits[1].Range.Start.Line != 2 {
		t.Fatalf("expected edits on line 2, got %+v", edits)
	}
	out.Reset()
	if s.chatSvc().applyInlineCompletion(uri, 1, ">!gone>", "y") || out.Len() != 0 {
		t.Fatalf("expected no edit when the prompt line is gone")
	}
}

func TestChatFailureNotifiesOnceUntilLineEdited(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	s.llmClient = errLLM{}
	uri := "file:///notes.txt"
	s.setDocument(uri, "why?>")
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()
	msgs := serverMessages(t, &out)
	if len(msgs) != 1 || msgs[0].Method != "window/showMessage" {
		t.Fatalf("expected one showMessage, got %v", methodsOf(msgs))
	}
	if !strings.Contains(string(msgs[0].Params), "chat failed") {
		t.Fatalf("unexpected message %s", msgs[0].Params)
	}
	// Retyping the trigger asks again.
	s.setDocument(uri, "why?")
	s.chatSvc().detectAndHandleChat(uri)
	s.setDocument(uri, "why?>")
	s.chatSvc().detectAndHandleChat(uri)
	s.inflight.Wait()
	if n := len(serverMessages(t, &out)); n != 2 {
		t.Fatalf("expected a retry after editing the line, got %d messages", n)
	}
}

func TestParseChatPromptLine_RejectsRepliesAndComments(t *testing.T) {
	prefixes := []string{"?", "!", ":", ";"}
	if _, ok := parseChatPromptLine(`> Unknown command. Try /help?>`, ">", prefixes); ok {
		t.Fatalf("reply lines must not be prompts")
	}
	m, ok := parseChatPromptLine("// what is this?>", ">", prefixes)
	if !ok || m.prompt != "// what is this?" {
		t.Fatalf("comment question should be a prompt, got %+v %v", m, ok)
	}
	if _, ok := parseChatPromptLine("// no trigger>", ">", prefixes); ok {
		t.Fatalf("a comment is not a slash command")
	}
	if _, ok := parseChatPromptLine("/reload>", ">", prefixes); !ok {
		t.Fatalf("slash commands need no trigger prefix")
	}
}

func TestChatCommandResponse_IgnoresLineComments(t *testing.T) {
	s := newTestServer()
	if _, ok := s.chatSvc().chatCommandResponse("file:///x", 0, "// what is go?"); ok {
		t.Fatalf("a // comment question must go to the LLM, not the command handler")
	}
	if res, ok := s.chatSvc().chatCommandResponse("file:///x", 0, "/help?"); !ok || !strings.Contains(res.message, "slash commands") {
		t.Fatalf("expected help, got %+v %v", res, ok)
	}
}

func TestQuoteReply(t *testing.T) {
	got := quoteReply("line one\n\nline two\n")
	if want := "> line one\n>\n> line two"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindPromptLine(t *testing.T) {
	lines := []string{"x", "q?>", "y", "q?>"}
	if got := findPromptLine(lines, 2, "q?>"); got != 1 && got != 3 {
		t.Fatalf("expected nearest match, got %d", got)
	}
	if got := findPromptLine(lines, 0, "q?>"); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := findPromptLine(lines, 2, "nope"); got != 2 {
		t.Fatalf("expected fallback to lineIdx, got %d", got)
	}
	if got := findPromptLine(lines, 9, "nope"); got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}

func TestApplyChatEdits_UTF16Positions(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.txt"
	line := "Größe? 😀?>"
	s.setDocument(uri, line)
	if !s.chatSvc().applyChatEdits(uri, 0, line, "> ok") {
		t.Fatalf("expected edit")
	}
	edits := chatEditsFromOutput(t, &out, uri)
	// "Größe? 😀?" is 10 UTF-16 units (the emoji counts twice); '>' is at 10.
	if edits[0].Range.Start.Character != 10 || edits[0].Range.End.Character != 11 {
		t.Fatalf("unexpected delete range %+v", edits[0].Range)
	}
	if edits[1].Range.Start.Character != 11 {
		t.Fatalf("unexpected insert position %+v", edits[1].Range)
	}
}
