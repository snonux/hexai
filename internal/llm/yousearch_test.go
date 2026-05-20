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

// unsetYouSearchEnv ensures no real YOU_API_KEY / HEXAI_YOUSEARCH_API_KEY in the
// environment can influence factory tests that should fail when no key is passed.
// We restore the previous values when the test ends.
func unsetYouSearchEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"YOU_API_KEY", "HEXAI_YOUSEARCH_API_KEY", "HEXAI_YOUSEARCH_RESEARCH_EFFORT"} {
		t.Setenv(k, "")
	}
}

func TestYouSearchFactory_MissingKey(t *testing.T) {
	unsetYouSearchEnv(t)
	if _, err := NewFromConfig(Config{Provider: "yousearch"}, "", "", "", "", ""); err == nil {
		t.Fatalf("expected error when YouSearch API key is missing")
	} else if !strings.Contains(err.Error(), "HEXAI_YOUSEARCH_API_KEY") || !strings.Contains(err.Error(), "YOU_API_KEY") {
		t.Fatalf("expected actionable API key hint, got %q", err.Error())
	}
}

func TestYouSearchFactory_Success(t *testing.T) {
	unsetYouSearchEnv(t)
	c, err := NewFromConfig(Config{Provider: "yousearch", YouSearchResearchEffort: "deep"}, "", "", "", "", "ys-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name() != "yousearch" {
		t.Fatalf("expected name yousearch, got %s", c.Name())
	}
	if c.DefaultModel() != "deep" {
		t.Fatalf("expected default model 'deep', got %s", c.DefaultModel())
	}
}

func TestYouSearchClient_DefaultModel_Empty(t *testing.T) {
	c := youSearchClient{}
	if got := c.DefaultModel(); got != "standard" {
		t.Fatalf("expected 'standard', got %s", got)
	}
}

func TestYouSearchChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-API-Key") != "ys-key" {
			t.Fatalf("expected X-API-Key header to be ys-key, got %q", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected application/json content type")
		}
		body, _ := io.ReadAll(r.Body)
		var req youSearchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Input != "what is foo?" {
			t.Fatalf("expected input 'what is foo?', got %q", req.Input)
		}
		if req.ResearchEffort != "lite" {
			t.Fatalf("expected research_effort lite, got %q", req.ResearchEffort)
		}
		resp := map[string]any{
			"output": map[string]any{
				"content":      "Foo is a placeholder.",
				"content_type": "text/markdown",
				"sources": []map[string]any{
					{"url": "https://example.com/a", "title": "A"},
					{"url": "https://example.com/b", "title": ""},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := youSearchClient{
		httpClient:     srv.Client(),
		apiKey:         "ys-key",
		baseURL:        srv.URL,
		researchEffort: "lite",
	}

	out, err := c.Chat(context.Background(), []Message{
		{Role: "system", Content: "ignored"},
		{Role: "user", Content: "what is foo?"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Foo is a placeholder.") {
		t.Fatalf("expected content in output, got %q", out)
	}
	if !strings.Contains(out, "**Sources:**") {
		t.Fatalf("expected sources section, got %q", out)
	}
	if !strings.Contains(out, "[A](https://example.com/a)") {
		t.Fatalf("expected first source linked, got %q", out)
	}
	if !strings.Contains(out, "[https://example.com/b](https://example.com/b)") {
		t.Fatalf("expected fallback title when title is empty, got %q", out)
	}
}

func TestYouSearchChat_NoUserMessage(t *testing.T) {
	c := youSearchClient{apiKey: "ys-key", baseURL: "http://unused.invalid"}
	_, err := c.Chat(context.Background(), []Message{{Role: "system", Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "no user message") {
		t.Fatalf("expected no-user-message error, got %v", err)
	}
}

func TestYouSearchChat_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()

	c := youSearchClient{httpClient: srv.Client(), apiKey: "ys-key", baseURL: srv.URL}
	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "q"}})
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("expected status 401 error, got %v", err)
	}
}

func TestYouSearchChat_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"content":"","sources":[]}}`))
	}))
	defer srv.Close()

	c := youSearchClient{httpClient: srv.Client(), apiKey: "ys-key", baseURL: srv.URL}
	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "q"}})
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("expected empty-response error, got %v", err)
	}
}

func TestLastUserMessage(t *testing.T) {
	got := lastUserMessage([]Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: " first  "},
		{Role: "assistant", Content: "a"},
		{Role: "USER", Content: " second "},
	})
	if got != "second" {
		t.Fatalf("expected 'second', got %q", got)
	}
	if got := lastUserMessage(nil); got != "" {
		t.Fatalf("expected empty for nil messages, got %q", got)
	}
	if got := lastUserMessage([]Message{{Role: "assistant", Content: "a"}}); got != "" {
		t.Fatalf("expected empty when no user messages, got %q", got)
	}
}

func TestFormatYouSearchContent_NonStringContent(t *testing.T) {
	var r youSearchResponse
	r.Output.Content = map[string]any{"k": "v"}
	got := formatYouSearchContent(r)
	if !strings.Contains(got, `"k"`) || !strings.Contains(got, `"v"`) {
		t.Fatalf("expected marshalled map in output, got %q", got)
	}
}
