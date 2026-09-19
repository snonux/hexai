package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type failoverTestClient struct {
	name  string
	model string
	chat  string
	err   error
	calls int
}

type failoverStreamClient struct {
	failoverTestClient
	deltas []string
}

func (c *failoverStreamClient) ChatStream(_ context.Context, _ []Message, onDelta func(string), _ ...RequestOption) error {
	for _, delta := range c.deltas {
		onDelta(delta)
	}
	return c.err
}

type failoverCodeClient struct {
	failoverTestClient
	suggestions []string
}

func (c *failoverCodeClient) CodeCompletion(context.Context, string, string, int, string, float64) ([]string, error) {
	return c.suggestions, c.err
}

func (c *failoverTestClient) Chat(context.Context, []Message, ...RequestOption) (string, error) {
	c.calls++
	return c.chat, c.err
}

func (c *failoverTestClient) Name() string         { return c.name }
func (c *failoverTestClient) DefaultModel() string { return c.model }

func TestChatFallsBackAfterRetryableHTTPError(t *testing.T) {
	primary := &failoverTestClient{name: "ollama-cloud", model: "cloud", err: &HTTPError{Provider: "ollama", Status: 429, Message: "quota exceeded"}}
	fallback := &failoverTestClient{name: "ollama-local", model: "local", chat: "answer"}
	result, err := Chat(context.Background(), []Target{{Name: primary.name, Model: primary.model, Client: primary}, {Name: fallback.name, Model: fallback.model, Client: fallback}}, nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if result.Text != "answer" || result.Target.Name != "ollama-local" || primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("unexpected result: %+v primary=%d fallback=%d", result, primary.calls, fallback.calls)
	}
}

func TestChatDoesNotFallBackForInvalidRequest(t *testing.T) {
	primary := &failoverTestClient{name: "primary", err: &HTTPError{Provider: "ollama", Status: 400, Message: "invalid model"}}
	fallback := &failoverTestClient{name: "fallback", chat: "unexpected"}
	_, err := Chat(context.Background(), []Target{{Name: "primary", Client: primary}, {Name: "fallback", Client: fallback}}, nil)
	if err == nil || fallback.calls != 0 {
		t.Fatalf("expected primary error without fallback, err=%v fallback calls=%d", err, fallback.calls)
	}
}

func TestChatDoesNotFallBackAfterCancellation(t *testing.T) {
	primary := &failoverTestClient{name: "primary", err: context.Canceled}
	fallback := &failoverTestClient{name: "fallback", chat: "unexpected"}
	_, err := Chat(context.Background(), []Target{{Name: "primary", Client: primary}, {Name: "fallback", Client: fallback}}, nil)
	if !errors.Is(err, context.Canceled) || fallback.calls != 0 {
		t.Fatalf("expected cancellation without fallback, err=%v fallback calls=%d", err, fallback.calls)
	}
}

func TestStreamFallsBackBeforeFirstOutput(t *testing.T) {
	primary := &failoverTestClient{name: "primary", err: &HTTPError{Provider: "ollama", Status: 503}}
	fallback := &failoverTestClient{name: "fallback", chat: "answer"}
	var output string
	target, err := Stream(context.Background(), []Target{{Name: "primary", Client: primary}, {Name: "fallback", Client: fallback}}, nil, func(delta string) { output += delta })
	if err != nil || target.Name != "fallback" || output != "answer" {
		t.Fatalf("unexpected stream result target=%+v output=%q err=%v", target, output, err)
	}
}

func TestStreamDoesNotAppendFallbackAfterOutput(t *testing.T) {
	primary := &failoverStreamClient{failoverTestClient: failoverTestClient{name: "primary", err: &HTTPError{Provider: "ollama", Status: 503}}, deltas: []string{"partial"}}
	fallback := &failoverTestClient{name: "fallback", chat: "unexpected"}
	var output string
	target, err := Stream(context.Background(), []Target{{Name: "primary", Client: primary}, {Name: "fallback", Client: fallback}}, nil, func(delta string) { output += delta })
	if target.Name != "primary" || output != "partial" || err == nil || fallback.calls != 0 {
		t.Fatalf("unexpected post-output stream result target=%+v output=%q err=%v fallback=%d", target, output, err, fallback.calls)
	}
}

func TestCodeCompletionFallsBackToNativeTarget(t *testing.T) {
	primary := &failoverCodeClient{failoverTestClient: failoverTestClient{name: "primary", err: &HTTPError{Provider: "openai", Status: 500}}}
	fallback := &failoverCodeClient{failoverTestClient: failoverTestClient{name: "fallback"}, suggestions: []string{"x"}}
	got, target, err := CodeCompletion(context.Background(), []Target{{Name: "primary", Client: primary}, {Name: "fallback", Client: fallback}}, "left", "right", 1, "go", 0.2)
	if err != nil || target.Name != "fallback" || len(got) != 1 || got[0] != "x" {
		t.Fatalf("unexpected code completion result: got=%v target=%+v err=%v", got, target, err)
	}
}

func TestChatCombinesTwoRetryableFailures(t *testing.T) {
	primary := &failoverTestClient{name: "primary", err: &HTTPError{Provider: "ollama", Status: 503}}
	fallback := &failoverTestClient{name: "fallback", err: &HTTPError{Provider: "ollama", Status: 429, Message: "quota"}}
	_, err := Chat(context.Background(), []Target{{Name: "primary", Client: primary}, {Name: "fallback", Client: fallback}}, nil)
	if err == nil || !strings.Contains(err.Error(), "primary") || !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("combined error = %v", err)
	}
}
