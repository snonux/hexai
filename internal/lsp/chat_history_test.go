package lsp

import "testing"

func TestStripTrailingTrigger(t *testing.T) {
	if got := stripTrailingTrigger("what?"); got != "what" {
		t.Fatalf("should remove trailing ?")
	}
	if got := stripTrailingTrigger("what?>"); got != "what?" {
		t.Fatalf("should drop trailing > when preceded by ?")
	}
	if got := stripTrailingTrigger("ok!>"); got != "ok!" {
		t.Fatalf("should drop > after !")
	}
	if got := stripTrailingTrigger("note:>"); got != "note:" {
		t.Fatalf("should drop > after :")
	}
	if got := stripTrailingTrigger("go;>"); got != "go;" {
		t.Fatalf("should drop > after ;")
	}
}

func TestBuildChatHistory_OrderAndLimit(t *testing.T) {
	s := newTestServer()
	uri := "file:///chat.txt"
	// Conversation: q1, > a1, blank, q2, > a2 lines, then current prompt
	doc := "q1\n> a1\n\nq2\n> a2\n\n"
	s.setDocument(uri, doc)
	msgs := s.buildChatHistory(uri, 5, "q3")
	// Expect: user q1, assistant a1, user q2, assistant a2, user q3
	if len(msgs) != 5 || msgs[0].Role != "user" || msgs[1].Role != "assistant" || msgs[2].Role != "user" || msgs[3].Role != "assistant" || msgs[4].Role != "user" {
		t.Fatalf("unexpected roles: %+v", msgs)
	}
	if msgs[0].Content != "q1" || msgs[1].Content != "a1" || msgs[2].Content != "q2" || msgs[3].Content != "a2" || msgs[4].Content != "q3" {
		t.Fatalf("unexpected contents: %+v", msgs)
	}
}
