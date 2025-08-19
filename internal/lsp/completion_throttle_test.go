package lsp

import (
    "bytes"
    "context"
    "log"
    "testing"
    "time"

    "hexai/internal/llm"
)

// countingLLM counts Chat calls; minimal implementation for tests.
type countingLLM struct{ calls int }

func (f *countingLLM) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
    f.calls++
    return "x := 1", nil
}
func (f *countingLLM) Name() string         { return "fake" }
func (f *countingLLM) DefaultModel() string { return "m" }

func TestDefaultTriggerChars_DoesNotIncludeSemicolonOrQuestion(t *testing.T) {
    var buf bytes.Buffer
    logger := log.New(&buf, "", 0)
    s := NewServer(bytes.NewBuffer(nil), &buf, logger, ServerOptions{})
    has := func(ch string) bool {
        for _, c := range s.triggerChars {
            if c == ch { return true }
        }
        return false
    }
    if has(";") || has("?") {
        t.Fatalf("default trigger chars should not include ';' or '?' got=%v", s.triggerChars)
    }
}

func TestTryLLMCompletion_ThrottleSkipsRapidCalls(t *testing.T) {
    // Build server with long min interval and set last completion to now
    s := &Server{ maxTokens: 32 }
    s.minCompletionInterval = time.Hour
    s.lastLLMCompletion = time.Now()
    fake := &countingLLM{}
    s.llmClient = fake
    // Position with adequate prefix to avoid prefix heuristic from skipping
    p := CompletionParams{ Position: Position{ Line: 0, Character: 3 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok := s.tryLLMCompletion(p, "", "foo", "", "", "", false, "")
    if !ok {
        t.Fatalf("expected ok=true even when throttled")
    }
    if len(items) != 0 {
        t.Fatalf("expected zero items when throttled, got %d", len(items))
    }
    if fake.calls != 0 {
        t.Fatalf("LLM Chat should not be called when throttled; calls=%d", fake.calls)
    }
}

func TestTryLLMCompletion_MinPrefixSkipsEarly(t *testing.T) {
    s := &Server{ maxTokens: 32 }
    fake := &countingLLM{}
    s.llmClient = fake
    // Only 1 identifier character before cursor
    p := CompletionParams{ Position: Position{ Line: 0, Character: 1 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok := s.tryLLMCompletion(p, "", "a", "", "", "", false, "")
    if !ok {
        t.Fatalf("expected ok=true when skipped by min-prefix heuristic")
    }
    if len(items) != 0 {
        t.Fatalf("expected zero items when min-prefix not satisfied")
    }
    if fake.calls != 0 {
        t.Fatalf("LLM Chat should not be called when min-prefix not met; calls=%d", fake.calls)
    }
}
