package lsp

import (
	"context"
	"hexai/internal/llm"
)

// countingLLM counts Chat calls; minimal implementation for tests that need
// to assert whether the chat-based completion path ran.
type countingLLM struct{ calls int }

func (f *countingLLM) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	f.calls++
	return "x := 1", nil
}
func (f *countingLLM) Name() string         { return "fake" }
func (f *countingLLM) DefaultModel() string { return "m" }
