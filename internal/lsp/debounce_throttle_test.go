package lsp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
)

// timeLLM records the time when Chat is invoked.
type timeLLM struct{ t time.Time }

func (t *timeLLM) Chat(ctx context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	t.t = time.Now()
	return "ok", nil
}
func (t *timeLLM) Name() string         { return "fake" }
func (t *timeLLM) DefaultModel() string { return "m" }

func TestCompletionDebounce_WaitsUntilQuiet(t *testing.T) {
	s := newTestServer()
	s.completion.compCache = make(map[string]string)
	cfg := s.cfg
	cfg.TriggerCharacters = []string{".", ":", "/", "_"}
	cfg.MaxTokens = 32
	cfg.CompletionDebounceMs = 30
	s.cfg = cfg
	s.markActivity() // simulate recent input

	f := &timeLLM{}
	s.llmClient = f

	line := "func f(i int) "
	p := CompletionParams{Position: Position{Line: 0, Character: len(line)}, TextDocument: TextDocumentIdentifier{URI: "file://debounce.go"}}
	p.Context = json.RawMessage([]byte(`{"triggerKind":1}`))

	start := time.Now()
	_, ok, _ := s.completion.tryLLMCompletion(p, "", line, "", "", "", false, "")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if f.t.IsZero() {
		t.Fatalf("expected LLM to be called")
	}
	if f.t.Sub(start) < 25*time.Millisecond { // allow minor timing noise
		t.Fatalf("expected debounce delay, got %s", f.t.Sub(start))
	}
}

func TestCompletionThrottle_SerializesCalls(t *testing.T) {
	s := newTestServer()
	s.completion.compCache = make(map[string]string)
	cfg := s.cfg
	cfg.TriggerCharacters = []string{".", ":", "/", "_"}
	cfg.MaxTokens = 32
	cfg.CompletionThrottleMs = 25
	s.cfg = cfg

	// first call uses timeLLM to record time
	f1 := &timeLLM{}
	s.llmClient = f1
	line := "func f(i int) "
	p := CompletionParams{Position: Position{Line: 0, Character: len(line)}, TextDocument: TextDocumentIdentifier{URI: "file://throttle.go"}}
	p.Context = json.RawMessage([]byte(`{"triggerKind":1}`))
	start := time.Now()
	if _, ok, _ := s.completion.tryLLMCompletion(p, "", line, "", "", "", false, ""); !ok {
		t.Fatalf("first call expected ok=true")
	}
	if f1.t.IsZero() {
		t.Fatalf("expected first call time recorded")
	}

	// second call immediately after; should be delayed by ~interval.
	// Clear cache to ensure we actually call the LLM again.
	s.completion.compCache = make(map[string]string)
	f2 := &timeLLM{}
	s.llmClient = f2
	if _, ok, _ := s.completion.tryLLMCompletion(p, "", line, "", "", "", false, ""); !ok {
		t.Fatalf("second call expected ok=true")
	}
	if f2.t.IsZero() {
		t.Fatalf("expected second call time recorded")
	}
	interval := time.Duration(cfg.CompletionThrottleMs) * time.Millisecond
	if f2.t.Sub(start) < interval {
		t.Fatalf("expected throttle spacing >= %s, got %s", interval, f2.t.Sub(start))
	}
}
