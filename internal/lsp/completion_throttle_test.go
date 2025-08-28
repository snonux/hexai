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

// Note: The server no longer exposes a busy guard; completion requests are
// handled sequentially and the LSP can request again if needed. This test used
// to assert a busy path; it now asserts that a normal trigger proceeds and
// calls the LLM without reporting busy.
func TestTryLLMCompletion_NoBusyPath_CurrentBehavior(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_"} }
    fake := &countingLLM{}
    s.llmClient = fake
    p := CompletionParams{ Position: Position{ Line: 0, Character: 4 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok, busy := s.tryLLMCompletion(p, "", "foo.", "", "", "", false, "")
    if !ok {
        t.Fatalf("expected ok=true for a normal triggered completion")
    }
    if busy {
        t.Fatalf("did not expect busy=true in current behavior")
    }
    if len(items) == 0 {
        t.Fatalf("expected some completion items when triggered")
    }
    if fake.calls == 0 {
        t.Fatalf("expected LLM Chat to be called")
    }
}

func TestTryLLMCompletion_MinPrefixSkipsEarly(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_"} }
    fake := &countingLLM{}
    s.llmClient = fake
    // No trigger character -> skip regardless of prefix
    p := CompletionParams{ Position: Position{ Line: 0, Character: 1 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok, _ := s.tryLLMCompletion(p, "", "a", "", "", "", false, "")
    if !ok {
        t.Fatalf("expected ok=true when skipped by min-prefix heuristic")
    }
    if len(items) != 0 {
        t.Fatalf("expected zero items when not triggered")
    }
    if fake.calls != 0 {
        t.Fatalf("LLM Chat should not be called when not triggered; calls=%d", fake.calls)
    }
}

func TestTryLLMCompletion_RequiresTriggerChar(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_", " "} }
    fake := &countingLLM{}
    s.llmClient = fake
    // With trigger character '.' directly before cursor -> allowed
    items, ok, _ := s.tryLLMCompletion(CompletionParams{ Position: Position{ Line: 0, Character: 1 }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }, "", ".", "", "", "", false, "")
    if !ok || len(items) == 0 || fake.calls == 0 { t.Fatalf("expected allowed with '.' trigger") }
    // Without trigger -> skipped
    fake.calls = 0
    items, ok, _ = s.tryLLMCompletion(CompletionParams{ Position: Position{ Line: 0, Character: 1 }, TextDocument: TextDocumentIdentifier{URI: "file://y.go"} }, "", "a", "", "", "", false, "")
    if !ok || len(items) != 0 || fake.calls != 0 { t.Fatalf("expected skip without trigger; ok=%v len=%d calls=%d", ok, len(items), fake.calls) }
}

func TestTryLLMCompletion_AllowsSpaceTrigger(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_", " "} }
    fake := &countingLLM{}
    s.llmClient = fake
    line := "type Matrix "
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    items, ok, _ := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if !ok || len(items) == 0 || fake.calls == 0 {
        t.Fatalf("expected allowed with space trigger; ok=%v len=%d calls=%d", ok, len(items), fake.calls)
    }
}
