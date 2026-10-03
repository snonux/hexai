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

// TestApplyChatEdits_SkipsEditedPromptLine is a regression test for the
// stale-captured-position bug in applyChatEdits. The chat response is produced
// asynchronously, so a didChange may edit the prompt line during the LLM
// round-trip. Stale trigger coordinates would then delete user content, and
// re-parsing the live line would attach the old answer to the edited question
// (which is asked separately). Here the user inserted "XX" before the prompt,
// so applyChatEdits must emit no edit at all.
func TestApplyChatEdits_SkipsEditedPromptLine(t *testing.T) {
	s := newTestServer()
	var out bytes.Buffer
	s.out = &out
	uri := "file:///chat.go"
	s.setDocument(uri, "XXhello?>\n")
	out.Reset()

	if s.chatSvc().applyChatEdits(uri, 0, "hello?>", "> reply") || out.Len() != 0 {
		t.Fatalf("expected no edit for an edited prompt line, got %q", out.String())
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

	s.chatSvc().applyChatEdits(uri, 0, "hello?>", "> reply")

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

	s.chatSvc().applyChatEdits(uri, 0, "hello?>", "> reply")

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

	s.chatSvc().applyChatEdits(uri, 0, "/reload>", "> reply")

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
