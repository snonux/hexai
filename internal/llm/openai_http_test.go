package llm

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
    "strings"
    "time"
)

func TestOpenAI_Chat_Success(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/chat/completions" { t.Fatalf("unexpected path: %s", r.URL.Path) }
        _ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"index":0, "message": map[string]string{"role":"assistant","content":"OK"}}}})
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    out, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"hi"}})
    if err != nil || out != "OK" { t.Fatalf("openai chat: %v %q", err, out) }
}

func TestOpenAI_Chat_MissingKey(t *testing.T) {
    c := newOpenAI("http://x", "g", "", f64p(0.2)).(openAIClient)
    if _, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"hi"}}); err == nil { t.Fatalf("expected error for missing key") }
}

func TestOpenAI_ChatStream_SSE(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Return SSE-like stream
        w.Header().Set("Content-Type", "text/event-stream")
        io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
        io.WriteString(w, "data: [DONE]\n")
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    var got string
    err := c.ChatStream(context.Background(), []Message{{Role:"user", Content:"hi"}}, func(s string){ got += s })
    if err != nil || got != "Hi" { t.Fatalf("chat stream: %v %q", err, got) }
}

func TestHandleOpenAINon2xx_NoErrorBody(t *testing.T) {
    resp := &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("{}"))}
    if err := handleOpenAINon2xx(resp, time.Now()); err == nil { t.Fatalf("expected http error") }
}
