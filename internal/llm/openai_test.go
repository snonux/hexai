package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/logging"
)

func TestOpenAIChatSuccess(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected auth header, got %q", got)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}]}`)),
			Header:     make(http.Header),
		}, nil
	})

	client := openAIClient{
		httpClient:   &http.Client{Transport: transport},
		apiKey:       "test-key",
		baseURL:      "https://example.com",
		defaultModel: "gpt-test",
		chatLogger:   logging.NewChatLogger("openai"),
	}

	out, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if out != "hi there" {
		t.Fatalf("unexpected chat output: %q", out)
	}
}

func TestOpenAIChatStreamDeliversChunks(t *testing.T) {
	client := openAIClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n" +
				"data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n" +
				"data: [DONE]\n"
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})},
		apiKey:       "test-key",
		baseURL:      "https://example.com",
		defaultModel: "gpt-test",
		chatLogger:   logging.NewChatLogger("openai"),
	}

	var received string
	err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hello"}}, func(chunk string) {
		received += chunk
	})
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
	}
	if received != "Hello" {
		t.Fatalf("expected streamed content, got %q", received)
	}
}

func TestOpenAIChatHandlesNon2xx(t *testing.T) {
	client := openAIClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("denied")), Header: make(http.Header)}, nil
		})},
		apiKey:       "test-key",
		baseURL:      "https://example.com",
		defaultModel: "gpt-test",
		chatLogger:   logging.NewChatLogger("openai"),
	}

	if _, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}); err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
