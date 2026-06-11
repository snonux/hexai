package lsp

import (
	"io"
	"testing"
)

func TestStripTrailingTrigger(t *testing.T) {
	s := newTestServer()
	if got := s.chatSvc().stripTrailingTrigger("what?"); got != "what" {
		t.Fatalf("should remove trailing ?")
	}
	if got := s.chatSvc().stripTrailingTrigger("what?>"); got != "what?" {
		t.Fatalf("should drop trailing > when preceded by ?")
	}
	if got := s.chatSvc().stripTrailingTrigger("ok!>"); got != "ok!" {
		t.Fatalf("should drop > after !")
	}
	if got := s.chatSvc().stripTrailingTrigger("note:>"); got != "note:" {
		t.Fatalf("should drop > after :")
	}
	if got := s.chatSvc().stripTrailingTrigger("go;>"); got != "go;" {
		t.Fatalf("should drop > after ;")
	}
}

func TestBuildChatHistory_OrderAndLimit(t *testing.T) {
	s := newTestServer()
	uri := "file:///chat.txt"
	// Conversation: q1, > a1, blank, q2, > a2 lines, then current prompt
	doc := "q1\n> a1\n\nq2\n> a2\n\n"
	s.setDocument(uri, doc)
	msgs := s.chatSvc().buildChatHistory(uri, 5, "q3")
	// Expect: user q1, assistant a1, user q2, assistant a2, user q3
	if len(msgs) != 5 || msgs[0].Role != "user" || msgs[1].Role != "assistant" || msgs[2].Role != "user" || msgs[3].Role != "assistant" || msgs[4].Role != "user" {
		t.Fatalf("unexpected roles: %+v", msgs)
	}
	if msgs[0].Content != "q1" || msgs[1].Content != "a1" || msgs[2].Content != "q2" || msgs[3].Content != "a2" || msgs[4].Content != "q3" {
		t.Fatalf("unexpected contents: %+v", msgs)
	}
}

// TestChatEdits_StaleLineIndexAfterShrink is a regression test for the LSP
// server panic that occurred when an async chat response was applied after a
// concurrent didChange shrank the document. The chat prompt was detected at a
// line index that is now past the end of the (smaller) document, so the stale
// lineIdx exceeds len(d.lines). Before the fix, both applyChatEdits and
// buildChatHistory indexed d.lines without an upper-bound check and panicked
// with index-out-of-range. The fix makes applyChatEdits skip the stale edit and
// buildChatHistory clamp its starting index, so neither must panic.
func TestChatEdits_StaleLineIndexAfterShrink(t *testing.T) {
	s := newTestServer()
	// io.Discard avoids a nil-writer panic on the (valid-index) clientApplyEdit
	// path; the stale-index path bails out before reaching it.
	s.out = io.Discard
	uri := "file:///shrink.txt"
	// Originally the prompt sat on line 8, but the document has since shrunk to
	// just three lines. lineIdx=8 is now well past len(d.lines).
	s.setDocument(uri, "line0\nline1\nline2\n")
	staleLineIdx := 8

	// applyChatEdits must not panic on a stale (out-of-range) line index.
	s.chatSvc().applyChatEdits(uri, staleLineIdx, 12, 1, "> reply")

	// buildChatHistory must not panic and must still return the current prompt.
	msgs := s.chatSvc().buildChatHistory(uri, staleLineIdx, "current?")
	if len(msgs) == 0 || msgs[len(msgs)-1].Content != "current?" {
		t.Fatalf("expected history to end with current prompt, got: %+v", msgs)
	}

	// Boundary case: lineIdx exactly equals len(d.lines) (one past the last line).
	s.chatSvc().applyChatEdits(uri, 3, 4, 1, "> reply")
	if msgs := s.chatSvc().buildChatHistory(uri, 3, "q"); len(msgs) == 0 {
		t.Fatalf("expected non-empty history at boundary index")
	}

	// Negative index must also be handled gracefully.
	s.chatSvc().applyChatEdits(uri, -1, 0, 1, "> reply")
}
