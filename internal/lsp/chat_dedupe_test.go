package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	// Typing after the closing marker and elsewhere must not ask again.
	for _, text := range []string{
		strings.Replace(base, "one>", "one> x", 1),
		strings.Replace(base, "one>", "one> xy", 1) + "// typing",
	} {
		s.setDocument(uri, text)
		s.chatSvc().detectAndHandleChat(uri)
	}
	close(g.release)
	s.inflight.Wait()
	if got := g.calls.Load(); got != 1 {
		t.Fatalf("expected one inline request, got %d", got)
	}
	edits := chatEditsFromOutput(t, &out, uri)
	if len(edits) != 1 {
		t.Fatalf("expected one edit replacing the tag, got %+v", edits)
	}
	want := Range{Start: Position{Line: 3, Character: 1}, End: Position{Line: 3, Character: 13}}
	if edits[0].Range != want || edits[0].NewText != "fmt.Println(1)" {
		t.Fatalf("unexpected edit %+v", edits[0])
	}
}

func TestApplyInlineCompletion_FollowsShiftedLine(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///main.go"
	// The prompt was on line 1; a line was inserted above meanwhile.
	s.setDocument(uri, "a\nb\nv := >!x> // keep")
	if !s.chatSvc().applyInlineCompletion(uri, 1, ">!x>", "y") {
		t.Fatalf("expected edit")
	}
	edits := chatEditsFromOutput(t, &out, uri)
	want := Range{Start: Position{Line: 2, Character: 5}, End: Position{Line: 2, Character: 9}}
	if len(edits) != 1 || edits[0].Range != want {
		t.Fatalf("expected the tag on line 2 to be replaced, got %+v", edits)
	}
	out.Reset()
	if s.chatSvc().applyInlineCompletion(uri, 1, ">!gone>", "y") || out.Len() != 0 {
		t.Fatalf("expected no edit when the prompt is gone")
	}
}

func TestApplyInlineCompletion_DoubleOpenReplacesLine(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///main.go"
	s.setDocument(uri, "\t>>!loop>")
	tag, ok := s.findInlineTag("\t>>!loop>")
	if !ok || !tag.wholeLine {
		t.Fatalf("expected a line-replacing tag, got %+v %v", tag, ok)
	}
	if !s.chatSvc().applyInlineCompletion(uri, 0, tag.text, "\tfor {}") {
		t.Fatalf("expected edit")
	}
	edits := chatEditsFromOutput(t, &out, uri)
	if edits[0].Range.Start.Character != 0 || edits[0].Range.End.Character != 9 {
		t.Fatalf("expected the whole line to be replaced, got %+v", edits[0].Range)
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
	if got := findPromptLine(lines, 2, "nope"); got != -1 {
		t.Fatalf("expected -1 for an edited prompt line, got %d", got)
	}
	if got := findPromptLine([]string{"x", "q?> more"}, 0, "q?>"); got != 1 {
		t.Fatalf("expected the line that still starts with the prompt, got %d", got)
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

func TestCompletion_SkipsInlinePromptInFlight(t *testing.T) {
	s := newTestServer()
	s.out = io.Discard
	c := &countingLLM{}
	s.llmClient = c
	uri := "file:///main.go"
	line := "\t>!print one>"
	s.setDocument(uri, line)
	s.chatSvc().tryBeginPrompt(inlineKey(uri, ">!print one>"), 1)
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 0, Character: len(line)}}
	p.Context = map[string]int{"triggerKind": 1}
	list := s.completionSvc().completeWithLLM(p, "", line, "", "", "")
	if len(list.Items) != 0 || c.calls != 0 {
		t.Fatalf("expected an empty result without LLM call, got %+v calls=%d", list, c.calls)
	}
}

func TestApplyChatEdits_TextTypedAfterTrigger(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.txt"
	s.setDocument(uri, "intro\nwhy?> and more")
	if !s.chatSvc().applyChatEdits(uri, 0, "why?>", "> because") {
		t.Fatalf("expected the answer to be inserted")
	}
	edits := chatEditsFromOutput(t, &out, uri)
	if edits[0].Range.Start != (Position{Line: 1, Character: 4}) || edits[0].Range.End != (Position{Line: 1, Character: 5}) {
		t.Fatalf("expected the original trigger to be removed, got %+v", edits[0].Range)
	}
	if edits[1].Range.Start != (Position{Line: 1, Character: 14}) {
		t.Fatalf("expected the answer after the line, got %+v", edits[1].Range)
	}
}

// TestApplyChatEdits_EditedQuestionDropsAnswer: when the question was rewritten
// while its answer was on the way, the old answer must not be attached to the
// new question (which is asked separately).
func TestApplyChatEdits_EditedQuestionDropsAnswer(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.txt"
	s.setDocument(uri, "how do I use rust?>")
	if s.chatSvc().applyChatEdits(uri, 0, "why is go fast?>", "> go answer") {
		t.Fatalf("expected the stale answer to be dropped")
	}
	if out.Len() != 0 {
		t.Fatalf("expected no edit, got %s", out.String())
	}
}

// TestPrunePrompts_IdenticalCopyAskedAfterFirstAnswered: identical prompt
// lines share a key; once one copy is answered the other must be released.
func TestPrunePrompts_IdenticalCopyAskedAfterFirstAnswered(t *testing.T) {
	s := newTestServer()
	c := s.chatSvc()
	uri := "file:///chat.txt"
	key := promptKey(uri, "why?>")
	before := []string{"why?>", "", "why?>"}
	if !c.tryBeginPrompt(key, c.promptCounts(uri, before)[key]) {
		t.Fatalf("expected the first request to start")
	}
	c.finishPrompt(key)
	c.prunePrompts(uri, c.promptCounts(uri, before))
	if !c.promptPending(key) {
		t.Fatalf("unchanged prompts must stay reserved")
	}
	c.prunePrompts(uri, c.promptCounts(uri, []string{"why?", "", "> because", "", "why?>"}))
	if c.promptPending(key) {
		t.Fatalf("the remaining copy must be released once the first was answered")
	}
}

// TestPrunePrompts_CopyAddedWhileInFlight: a copy pasted while the request is
// running raises the count, so answering the original releases the copy.
func TestPrunePrompts_CopyAddedWhileInFlight(t *testing.T) {
	s := newTestServer()
	c := s.chatSvc()
	uri := "file:///chat.txt"
	key := promptKey(uri, "why?>")
	c.tryBeginPrompt(key, 1)
	c.prunePrompts(uri, c.promptCounts(uri, []string{"why?>", "why?>"}))
	c.finishPrompt(key)
	c.prunePrompts(uri, c.promptCounts(uri, []string{"why?>", "why?>"}))
	if !c.promptPending(key) {
		t.Fatalf("unchanged prompts must stay reserved")
	}
	c.prunePrompts(uri, c.promptCounts(uri, []string{"why?", "> because", "why?>"}))
	if c.promptPending(key) {
		t.Fatalf("expected the copy to be released")
	}
}

// TestPrunePrompts_FailedPromptStaysReserved: a failed (finished) prompt is
// not retried while its line is unchanged, but is after it was edited.
func TestPrunePrompts_FailedPromptStaysReserved(t *testing.T) {
	s := newTestServer()
	c := s.chatSvc()
	uri := "file:///chat.txt"
	key := promptKey(uri, "why?>")
	c.tryBeginPrompt(key, 1)
	c.finishPrompt(key)
	c.prunePrompts(uri, c.promptCounts(uri, []string{"why?>", "typing elsewhere"}))
	if c.tryBeginPrompt(key, 1) {
		t.Fatalf("a failed prompt must not be retried on every keystroke")
	}
	c.prunePrompts(uri, c.promptCounts(uri, []string{"why?"}))
	if !c.tryBeginPrompt(key, 1) {
		t.Fatalf("expected the prompt to be askable again after editing")
	}
}
