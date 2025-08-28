package lsp

import (
	"context"
	"errors"
	"testing"

	"hexai/internal/llm"
)

// fakeCodeLLM implements both llm.Client and llm.CodeCompleter.
type fakeCodeLLM struct {
	codeCalls int
	chatCalls int
	result    string
	codeErr   error
}

func (f *fakeCodeLLM) CodeCompletion(_ context.Context, _ string, _ string, n int, _ string, _ float64) ([]string, error) {
	f.codeCalls++
	if f.codeErr != nil {
		return nil, f.codeErr
	}
	if n <= 0 {
		n = 1
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = f.result
	}
	return out, nil
}

func (f *fakeCodeLLM) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	f.chatCalls++
	return "chat", nil
}
func (f *fakeCodeLLM) Name() string         { return "fake" }
func (f *fakeCodeLLM) DefaultModel() string { return "m" }

func TestTryLLMCompletion_PrefersCodeCompleterOverChat(t *testing.T) {
	s := &Server{maxTokens: 32, triggerChars: []string{"."}, compCache: make(map[string]string)}
	fake := &fakeCodeLLM{result: "DoThing()"}
	s.llmClient = fake
	line := "obj."
	p := CompletionParams{Position: Position{Line: 0, Character: len(line)}, TextDocument: TextDocumentIdentifier{URI: "file://x.go"}}
	items, ok := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
	if !ok || len(items) == 0 {
		t.Fatalf("expected completion items via CodeCompleter path")
	}
	if fake.codeCalls == 0 {
		t.Fatalf("expected CodeCompletion to be called")
	}
	if fake.chatCalls != 0 {
		t.Fatalf("did not expect Chat fallback when CodeCompletion succeeds")
	}
}

func TestTryLLMCompletion_FallsBackToChatOnCodeCompleterError(t *testing.T) {
	s := &Server{maxTokens: 32, triggerChars: []string{"."}, compCache: make(map[string]string)}
	fake := &fakeCodeLLM{result: "DoThing()", codeErr: errors.New("boom")}
	s.llmClient = fake
	line := "obj."
	p := CompletionParams{Position: Position{Line: 0, Character: len(line)}, TextDocument: TextDocumentIdentifier{URI: "file://y.go"}}
	items, ok := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
	if !ok {
		t.Fatalf("expected ok=true even on fallback path")
	}
	if len(items) == 0 {
		t.Fatalf("expected some items from Chat fallback")
	}
	if fake.codeCalls == 0 {
		t.Fatalf("expected CodeCompletion to be attempted first")
	}
	if fake.chatCalls == 0 {
		t.Fatalf("expected Chat fallback to be called when CodeCompletion errors")
	}
}
