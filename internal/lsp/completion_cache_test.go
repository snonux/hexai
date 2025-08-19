package lsp

import (
    "bytes"
    "log"
    "strings"
    "testing"

    "hexai/internal/logging"
)

func TestCompletionCache_IgnoresWhitespaceBeforeCursor(t *testing.T) {
    var buf bytes.Buffer
    logger := log.New(&buf, "", 0)
    s := NewServer(bytes.NewBuffer(nil), &buf, logger, ServerOptions{})
    logging.Bind(logger)
    s.triggerChars = []string{" ", "."}
    fake := &countingLLM{}
    s.llmClient = fake

    // First request with trailing spaces before cursor
    line := "foo   "
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok, _ := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if !ok || len(items) == 0 || fake.calls != 1 {
        t.Fatalf("expected first call to invoke LLM; ok=%v len=%d calls=%d", ok, len(items), fake.calls)
    }

    // Same logical context but with a different amount of trailing whitespace
    line2 := "foo             "
    p2 := CompletionParams{ Position: Position{ Line: 0, Character: len(line2) }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items2, ok2, _ := s.tryLLMCompletion(p2, "", line2, "", "", "", false, "")
    if !ok2 || len(items2) == 0 {
        t.Fatalf("expected cache hit to still return items")
    }
    if fake.calls != 1 {
        t.Fatalf("expected cache hit to avoid LLM call; calls=%d", fake.calls)
    }
    if !strings.Contains(buf.String(), "completion cache hit") {
        t.Fatalf("expected log to contain cache hit message, got: %s", buf.String())
    }
}
