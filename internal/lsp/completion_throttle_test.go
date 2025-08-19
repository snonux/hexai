package lsp

import (
    "bytes"
    "context"
    "log"
    "testing"

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

func TestTryLLMCompletion_BusySkipsConcurrent(t *testing.T) {
    s := &Server{ maxTokens: 32 }
    fake := &countingLLM{}
    s.llmClient = fake
    // Simulate another LLM request in flight
    s.llmBusy = true
    p := CompletionParams{ Position: Position{ Line: 0, Character: 3 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok := s.tryLLMCompletion(p, "", "foo", "", "", "", false, "")
    if !ok {
        t.Fatalf("expected ok=true when busy guard skips")
    }
    if len(items) != 0 {
        t.Fatalf("expected zero items when busy, got %d", len(items))
    }
    if fake.calls != 0 {
        t.Fatalf("LLM Chat should not be called when busy; calls=%d", fake.calls)
    }
}

func TestTryLLMCompletion_MinPrefixSkipsEarly(t *testing.T) {
    s := &Server{ maxTokens: 32 }
    fake := &countingLLM{}
    s.llmClient = fake
    // Zero identifier characters before cursor
    p := CompletionParams{ Position: Position{ Line: 0, Character: 0 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok := s.tryLLMCompletion(p, "", "", "", "", "", false, "")
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

func TestTryLLMCompletion_AllowsAfterTrailingSpace(t *testing.T) {
    s := &Server{ maxTokens: 32 }
    fake := &countingLLM{}
    s.llmClient = fake
    line := "type Matrix "
    // Cursor after trailing space
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if !ok || len(items) == 0 || fake.calls == 0 {
        t.Fatalf("expected completion allowed after trailing space; ok=%v len=%d calls=%d", ok, len(items), fake.calls)
    }
}
