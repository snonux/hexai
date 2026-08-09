package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// chatEditsFromOutput decodes the first workspace/applyEdit request in buf and
// returns its TextEdits for uri. It mirrors the framing used by captureRequest
// but is local to the chat-edit tests so they stay self-contained.
func chatEditsFromOutput(t *testing.T, buf *bytes.Buffer, uri string) []TextEdit {
	t.Helper()
	raw := buf.String()
	off := 0
	for off < len(raw) {
		rest := raw[off:]
		idx := strings.Index(rest, "\r\n\r\n")
		if idx < 0 {
			break
		}
		body := rest[idx+4:]
		hdr := rest[:idx]
		clen := 0
		for _, line := range strings.Split(hdr, "\r\n") {
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				var n int
				_, _ = fmt.Sscanf(line, "Content-Length: %d", &n)
				clen = n
				break
			}
		}
		if clen <= 0 || clen > len(body) {
			clen = len(body)
		}
		piece := body[:clen]
		var req Request
		_ = json.Unmarshal([]byte(piece), &req)
		if req.Method == "workspace/applyEdit" {
			var params ApplyWorkspaceEditParams
			if err := json.Unmarshal(req.Params, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			edits, ok := params.Edit.Changes[uri]
			if !ok {
				t.Fatalf("no edits for %s in applyEdit payload", uri)
			}
			return edits
		}
		off += idx + 4 + clen
	}
	t.Fatalf("no workspace/applyEdit request in output: %q", raw)
	return nil
}

// TestApplyChatEdits_RecomputesTriggerAfterLineEdit is a regression test for the
// stale-captured-position bug in applyChatEdits. The chat response is produced
// asynchronously: handleChatPrompt captures the trigger position
// (match.lastNonSpace / match.removeCount) from the ORIGINAL line, then a
// goroutine calls applyChatEdits after the LLM round-trip. If a didChange
// shifted characters before the trigger during the round-trip, the stale
// coordinates pointed at user content and the delete edit removed the wrong
// characters, corrupting the trigger line.
//
// The fix recomputes the trigger coordinates from the live line. This test
// simulates the round-trip by mutating the line (inserting "XX" before the
// trigger, shifting the suffix '>' two columns right) and then calling
// applyChatEdits directly. It asserts the delete range covers the live '>' at
// the shifted position, not the stale original one.
func TestApplyChatEdits_RecomputesTriggerAfterLineEdit(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.go"
	// Original trigger line would have been "hello?>" with the '>' suffix at
	// character index 6. During the async round-trip the user inserted "XX" at
	// the start, shifting the trigger to character 8.
	s.setDocument(uri, "XXhello?>\n")
	out.Reset()

	s.chatSvc().applyChatEdits(uri, 0, "> reply")

	edits := chatEditsFromOutput(t, &out, uri)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (delete+insert), got %d: %+v", len(edits), edits)
	}
	del := edits[0]
	// The delete must target the live '>' at character 8, not the stale 6.
	if got := del.Range.Start; got.Line != 0 || got.Character != 8 {
		t.Fatalf("delete start should be char 8 (live trigger), got %+v", got)
	}
	if got := del.Range.End; got.Line != 0 || got.Character != 9 {
		t.Fatalf("delete end should be char 9 (live trigger), got %+v", got)
	}
	// Insert goes at end of the live line (length 9 incl. trailing newline-stripped
	// content; d.lines stores lines without the trailing newline).
	if got := edits[1].Range.Start; got.Line != 0 || got.Character != 9 {
		t.Fatalf("insert start should be char 9 (end of live line), got %+v", got)
	}
}

// TestApplyChatEdits_SkipsWhenTriggerGone asserts that when the trigger
// punctuation was removed/changed on the line during the async round-trip,
// applyChatEdits emits no edit rather than deleting user content at the stale
// position.
func TestApplyChatEdits_SkipsWhenTriggerGone(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.go"
	// The user replaced the trigger '>' with a space during the round-trip, so
	// the line no longer ends with the trigger punctuation.
	s.setDocument(uri, "hello? \n")
	out.Reset()

	s.chatSvc().applyChatEdits(uri, 0, "> reply")

	if out.Len() != 0 {
		edits := chatEditsFromOutput(t, &out, uri)
		t.Fatalf("expected no edit when trigger is gone, got %+v", edits)
	}
}

// TestApplyChatEdits_SkipsWhenTriggerPrefixInvalidated covers the case where the
// suffix '>' is still present but the user edited the preceding character to
// one that is not a configured trigger prefix. parseChatPromptLine rejects this
// via hasTriggerPrefix, so no edit must be emitted — protecting the now-mismatched
// user content from a stale delete. This is the core corruption scenario the
// fix exists to prevent.
func TestApplyChatEdits_SkipsWhenTriggerPrefixInvalidated(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.go"
	// Original was "hello?>"; the user changed the '?' to 'x' (not a trigger
	// prefix), leaving the suffix '>' but an invalid trigger pair.
	s.setDocument(uri, "hellox>\n")
	out.Reset()

	s.chatSvc().applyChatEdits(uri, 0, "> reply")

	if out.Len() != 0 {
		edits := chatEditsFromOutput(t, &out, uri)
		t.Fatalf("expected no edit when trigger prefix invalidated, got %+v", edits)
	}
}

// TestApplyChatEdits_SlashCommandDeleteRange locks in that the synchronous
// slash-command path (handleChatPrompt -> chatCommandResponse -> applyChatEdits)
// still deletes exactly the trailing '>' from the live line. Slash prompts do
// not require a trigger prefix (the '/' short-circuits hasTriggerPrefix), so
// re-parsing "/reload>" must succeed and target the suffix at the live position.
func TestApplyChatEdits_SlashCommandDeleteRange(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.go"
	s.setDocument(uri, "/reload>\n")
	out.Reset()

	s.chatSvc().applyChatEdits(uri, 0, "> reply")

	edits := chatEditsFromOutput(t, &out, uri)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (delete+insert), got %d: %+v", len(edits), edits)
	}
	del := edits[0]
	// "/reload>" has the suffix '>' at character 7; the delete must remove only it.
	if got := del.Range.Start; got.Line != 0 || got.Character != 7 {
		t.Fatalf("slash delete start should be char 7, got %+v", got)
	}
	if got := del.Range.End; got.Line != 0 || got.Character != 8 {
		t.Fatalf("slash delete end should be char 8, got %+v", got)
	}
}
