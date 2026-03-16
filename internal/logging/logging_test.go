package logging

import (
	"bytes"
	"log"
	"sync"
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

// TestPreviewForLog_ShortString verifies no truncation when string fits within the limit.
func TestPreviewForLog_ShortString(t *testing.T) {
	SetLogPreviewLimit(10)
	if got := PreviewForLog("abc"); got != "abc" {
		t.Fatalf("expected no truncation, got %q", got)
	}
}

// TestPreviewForLog_ExactLimit verifies no truncation when string length equals the limit.
func TestPreviewForLog_ExactLimit(t *testing.T) {
	SetLogPreviewLimit(3)
	if got := PreviewForLog("abc"); got != "abc" {
		t.Fatalf("expected no truncation for exact limit, got %q", got)
	}
}

// TestPreviewForLog_ZeroLimit verifies unlimited preview when limit is 0.
func TestPreviewForLog_ZeroLimit(t *testing.T) {
	SetLogPreviewLimit(0)
	long := "this is a long string that should not be truncated"
	if got := PreviewForLog(long); got != long {
		t.Fatalf("expected full string with limit=0, got %q", got)
	}
}

// TestPreviewForLog_NegativeLimit verifies negative limit behaves like unlimited.
func TestPreviewForLog_NegativeLimit(t *testing.T) {
	SetLogPreviewLimit(-5)
	s := "should not truncate"
	if got := PreviewForLog(s); got != s {
		t.Fatalf("expected full string with negative limit, got %q", got)
	}
}

// TestPreviewForLog_EmptyString verifies empty input returns empty output.
func TestPreviewForLog_EmptyString(t *testing.T) {
	SetLogPreviewLimit(10)
	if got := PreviewForLog(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

// TestLogf_NilLogger verifies Logf does not panic when no logger is bound.
func TestLogf_NilLogger(t *testing.T) {
	// Store a nil logger to test the nil-check path
	std.Store(nil)
	// Should not panic
	Logf("test ", "should be a no-op %s", "value")
}

// TestConcurrentAccess verifies atomic operations prevent data races
// under concurrent usage of Bind, Logf, SetLogPreviewLimit, and PreviewForLog.
func TestConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	Bind(log.New(&buf, "", 0))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			SetLogPreviewLimit(n + 1)
			_ = PreviewForLog("concurrent access test string")
			Logf("goroutine ", "n=%d", n)
		}(i)
	}
	wg.Wait()
}
