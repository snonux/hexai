package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOpenAI_ChatStream_SSE_MalformedChunk(t *testing.T) {
	if os.Getenv("HEXAI_TEST_SKIP_NET") == "1" {
		t.Skip("skip network-bound tests in restricted environments")
	}
	// Malformed JSON chunk should be skipped; no onDelta calls; no error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {not json}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n")
	}))
	defer srv.Close()
	c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2))
	c.httpClient = srv.Client()
	var got string
	if err := c.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(s string) { got += s }); err != nil {
		t.Fatalf("unexpected error for malformed chunk: %v", err)
	}
	if got != "" {
		t.Fatalf("expected no deltas for malformed chunk, got %q", got)
	}
}
