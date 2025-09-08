package logging

import (
	"bytes"
	"log"
	"testing"
)

func TestPreviewAndLogfAndChatLogger(t *testing.T) {
	var buf bytes.Buffer
	Bind(log.New(&buf, "", 0))
	SetLogPreviewLimit(3)
	if got := PreviewForLog("abcdef"); got != "abc…" {
		t.Fatalf("preview wrong: %q", got)
	}
	Logf("unit ", "hello %s", "x")
	cl := NewChatLogger("p")
	cl.LogStart(true, "m", 0.5, 100, []string{"stop"}, []struct{ Role, Content string }{{"user", "hello"}})
	out := buf.String()
	if out == "" || !bytes.Contains([]byte(out), []byte("start")) {
		t.Fatalf("expected logged content, got %q", out)
	}
}
