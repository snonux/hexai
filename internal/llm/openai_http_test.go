package llm

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
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

