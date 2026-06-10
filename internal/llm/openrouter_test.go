package llm

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/logging"
)

func TestOpenRouter_Chat_SendsHeadersAndBody(t *testing.T) {
	if os.Getenv("HEXAI_TEST_SKIP_NET") == "1" {
		t.Skip("skip network-bound tests in restricted environments")
	}
	var capturedHeaders http.Header
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		capturedBody = append([]byte(nil), body...)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "ack"}},
			},
		})
	}))
	defer srv.Close()

	c := newOpenRouter(srv.URL, "anthropic/claude-test", "KEY", f64p(0.2))
	c.httpClient = srv.Client()
	out, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "ping"}})
	if err != nil {
		t.Fatalf("chat returned error: %v", err)
	}
	if out != "ack" {
		t.Fatalf("unexpected response: %q", out)
	}
	if capturedHeaders.Get("Authorization") != "Bearer KEY" {
		t.Fatalf("missing auth header: %#v", capturedHeaders)
	}
	if capturedHeaders.Get("HTTP-Referer") != "https://github.com/snonux/hexai" {
		t.Fatalf("missing referer header: %#v", capturedHeaders)
	}
	if capturedHeaders.Get("X-Title") != "Hexai" {
		t.Fatalf("missing title header: %#v", capturedHeaders)
	}

	var req oaChatRequest
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Model != "anthropic/claude-test" {
		t.Fatalf("unexpected model: %q", req.Model)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "ping" {
		t.Fatalf("unexpected messages: %#v", req.Messages)
	}
}

func TestOpenRouter_ChatStream_SendsHeaders(t *testing.T) {
	if os.Getenv("HEXAI_TEST_SKIP_NET") == "1" {
		t.Skip("skip network-bound tests in restricted environments")
	}
	var acceptHeader string
	var referer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptHeader = r.Header.Get("Accept")
		referer = r.Header.Get("HTTP-Referer")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n")
	}))
	defer srv.Close()

	c := newOpenRouter(srv.URL, "anthropic/claude-test", "KEY", f64p(0.2))
	c.httpClient = srv.Client()
	var got string
	err := c.ChatStream(context.Background(), []Message{{Role: "user", Content: "ping"}}, func(s string) { got += s })
	if err != nil {
		t.Fatalf("chat stream error: %v", err)
	}
	if got != "hi" {
		t.Fatalf("expected stream output 'hi', got %q", got)
	}
	if acceptHeader != "text/event-stream" {
		t.Fatalf("unexpected Accept header: %q", acceptHeader)
	}
	if referer != "https://github.com/snonux/hexai" {
		t.Fatalf("missing referer header in stream: %q", referer)
	}
}

func TestOpenRouter_Chat_MissingKey(t *testing.T) {
	c := newOpenRouter("http://example", "anthropic/claude-test", "", f64p(0.2))
	if _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "ping"}}); err == nil {
		t.Fatalf("expected error for missing api key")
	} else if !strings.Contains(err.Error(), "OPENROUTER_API_KEY") || !strings.Contains(err.Error(), "HEXAI_OPENROUTER_API_KEY") {
		t.Fatalf("expected actionable API key hint, got %q", err.Error())
	}
}

func TestOpenRouter_DefaultsAndMetadata(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	logging.Bind(logger)
	c := newOpenRouter("", "", "KEY", nil)
	if c.baseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("default baseURL mismatch: %s", c.baseURL)
	}
	if c.defaultModel != "openrouter/auto" {
		t.Fatalf("default model mismatch: %s", c.defaultModel)
	}
	if name := c.Name(); name != "openrouter" {
		t.Fatalf("Name() = %s", name)
	}
	if model := c.DefaultModel(); model != "openrouter/auto" {
		t.Fatalf("DefaultModel() = %s", model)
	}
	c.logf("smoke")
}
