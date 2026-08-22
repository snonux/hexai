package lsp

import (
	"context"

	"github.com/snonux/hexai/internal/llm"
)

// captureLLM captures messages sent to Chat for assertions.
type captureLLM struct{ msgs []llm.Message }

func (c *captureLLM) Chat(_ context.Context, m []llm.Message, _ ...llm.RequestOption) (string, error) {
	c.msgs = append([]llm.Message{}, m...)
	return "OK", nil
}
func (*captureLLM) Name() string         { return "cap" }
func (*captureLLM) DefaultModel() string { return "m" }
