package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			t.Fatalf("expected /messages endpoint, got %s", r.URL.Path)
		}
		// Check headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("expected anthropic-version header")
		}
		// Verify request body
		var req anthropicChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "claude-3-5-sonnet-20241022" {
			t.Fatalf("expected model claude-3-5-sonnet-20241022, got %s", req.Model)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Fatalf("expected user role, got %s", req.Messages[0].Role)
		}
		if req.Messages[0].Content != "Hello" {
			t.Fatalf("expected content 'Hello', got '%s'", req.Messages[0].Content)
		}
		// Send response
		resp := anthropicChatResponse{
			ID:         "msg-123",
			Type:       "message",
			StopReason: "end_turn",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hi there!"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newAnthropic(srv.URL, "claude-3-5-sonnet-20241022", "test-key", nil).(anthropicClient)
	response, err := c.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if response != "Hi there!" {
		t.Fatalf("expected 'Hi there!', got '%s'", response)
	}
}

func TestAnthropicChat_NoAPIKey(t *testing.T) {
	c := newAnthropic("https://api.anthropic.com/v1", "claude-3-5-sonnet-20241022", "", nil)
	_, err := c.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatalf("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "missing Anthropic API key") {
		t.Fatalf("expected 'missing Anthropic API key', got '%s'", err.Error())
	}
}

func TestAnthropicChat_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := anthropicChatResponse{
			Error: &struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}{
				Type:    "authentication_error",
				Message: "Invalid API key",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newAnthropic(srv.URL, "claude-3-5-sonnet-20241022", "invalid-key", nil).(anthropicClient)
	_, err := c.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatalf("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Fatalf("expected 'Invalid API key' in error, got '%s'", err.Error())
	}
}

func TestAnthropicChat_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicChatResponse{
			ID:         "msg-123",
			Type:       "message",
			StopReason: "end_turn",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newAnthropic(srv.URL, "claude-3-5-sonnet-20241022", "test-key", nil).(anthropicClient)
	_, err := c.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatalf("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "no content returned") {
		t.Fatalf("expected 'no content returned', got '%s'", err.Error())
	}
}

func TestAnthropicChat_WithTemperature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Temperature == nil || *req.Temperature != 0.5 {
			t.Fatalf("expected temperature 0.5, got %v", req.Temperature)
		}
		resp := anthropicChatResponse{
			ID:         "msg-123",
			Type:       "message",
			StopReason: "end_turn",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Response"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newAnthropic(srv.URL, "claude-3-5-sonnet-20241022", "test-key", nil).(anthropicClient)
	_, err := c.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	}, WithTemperature(0.5))
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}

func TestAnthropicStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Send streaming response
		streamEvents := []string{
			`data: {"type":"message_start","message":{"id":"msg-123","type":"message"}}`,
			`data: {"type":"content_block_start","content_block":{"type":"text"}}`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" "}}`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"world"}}`,
			`data: {"type":"message_stop"}`,
		}
		for _, event := range streamEvents {
			io.WriteString(w, event+"\n")
		}
	}))
	defer srv.Close()

	c := newAnthropic(srv.URL, "claude-3-5-sonnet-20241022", "test-key", nil)
	streamer, ok := c.(Streamer)
	if !ok {
		t.Fatalf("Anthropic client does not implement Streamer interface")
	}
	var chunks []string
	err := streamer.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "Say hello"},
	}, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello" || chunks[1] != " " || chunks[2] != "world" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestAnthropicStream_NoAPIKey(t *testing.T) {
	c := newAnthropic("https://api.anthropic.com/v1", "claude-3-5-sonnet-20241022", "", nil)
	streamer, ok := c.(Streamer)
	if !ok {
		t.Fatalf("Anthropic client does not implement Streamer interface")
	}
	err := streamer.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	}, func(chunk string) {})
	if err == nil {
		t.Fatalf("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "missing Anthropic API key") {
		t.Fatalf("expected 'missing Anthropic API key', got '%s'", err.Error())
	}
}

func TestAnthropicClient_Name(t *testing.T) {
	c := newAnthropic("https://api.anthropic.com/v1", "claude-3-5-sonnet-20241022", "test-key", nil)
	if c.Name() != "anthropic" {
		t.Fatalf("expected 'anthropic', got '%s'", c.Name())
	}
}

func TestAnthropicClient_DefaultModel(t *testing.T) {
	model := "claude-3-opus-20250219"
	c := newAnthropic("https://api.anthropic.com/v1", model, "test-key", nil).(anthropicClient)
	if c.DefaultModel() != model {
		t.Fatalf("expected '%s', got '%s'", model, c.DefaultModel())
	}
}

func TestAnthropicClient_DefaultBaseURL(t *testing.T) {
	c := newAnthropic("", "claude-3-5-sonnet-20241022", "test-key", nil).(anthropicClient)
	if c.baseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("expected default base URL, got '%s'", c.baseURL)
	}
}

func TestAnthropicClient_DefaultModel_Empty(t *testing.T) {
	c := newAnthropic("https://api.anthropic.com/v1", "", "test-key", nil).(anthropicClient)
	if c.defaultModel != "claude-3-5-sonnet-20241022" {
		t.Fatalf("expected default model, got '%s'", c.defaultModel)
	}
}
