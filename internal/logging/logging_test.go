package logging

import (
    "bytes"
    "log"
    "strings"
    "testing"
)

func TestLogf_WithBindAndPreview(t *testing.T) {
    var buf bytes.Buffer
    l := log.New(&buf, "", 0)
    Bind(l)

    SetLogPreviewLimit(5)
    if got := PreviewForLog("abcdef"); got != "abcde…" {
        t.Fatalf("preview truncation failed: %q", got)
    }
    if got := PreviewForLog("abcd"); got != "abcd" {
        t.Fatalf("preview (no trunc) failed: %q", got)
    }
    SetLogPreviewLimit(0)
    if got := PreviewForLog("abcdef"); got != "abcdef" {
        t.Fatalf("preview unlimited failed: %q", got)
    }

    Logf("mod ", "hello %s", "world")
    out := buf.String()
    if !strings.Contains(out, "hello world") || !strings.Contains(out, AnsiBase) || !strings.Contains(out, AnsiReset) {
        t.Fatalf("log output missing parts: %q", out)
    }
}

func TestChatLogger_LogStart(t *testing.T) {
    var buf bytes.Buffer
    Bind(log.New(&buf, "", 0))
    SetLogPreviewLimit(3)
    cl := NewChatLogger("prov")
    msgs := []struct{ Role, Content string }{{"user", "abcdef"}, {"assistant", "xyz"}}
    cl.LogStart(true, "m", 0.2, 128, []string{"END"}, msgs)
    out := buf.String()
    if !strings.Contains(out, "stream start model=m") || !strings.Contains(out, "messages=2") {
        t.Fatalf("missing header log: %q", out)
    }
    if !strings.Contains(out, "msg[0] role=user") || !strings.Contains(out, "preview=") {
        t.Fatalf("missing message logs: %q", out)
    }
}
