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

func TestOpenAI_ChatStream_SSE_ErrorChunk(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        io.WriteString(w, "data: {\"error\":{\"message\":\"oops\"}}\n\n")
        io.WriteString(w, "data: [DONE]\n")
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    var got string
    if err := c.ChatStream(context.Background(), []Message{{Role:"user", Content:"hi"}}, func(s string){ got += s }); err == nil {
        t.Fatalf("expected error due to error chunk")
    }
}

func TestOpenAI_Chat_NoChoices_Error(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    if _, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"hi"}}); err == nil {
        t.Fatalf("expected error when choices empty")
    }
}

func TestOpenAI_ChatStream_SSE_EmptyDelta_NoError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        io.WriteString(w, "data: {\\\"choices\\\":[{\\\"delta\\\":{\\\"content\\\":\\\"\\\"}}]}\\n\\n")
        io.WriteString(w, "data: [DONE]\\n")
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    var got string
    if err := c.ChatStream(context.Background(), []Message{{Role:"user", Content:"hi"}}, func(s string){ got += s }); err != nil {
        t.Fatalf("unexpected error for empty delta: %v", err)
    }
    if got != "" { t.Fatalf("expected no output for empty delta, got %q", got) }
}

func TestOpenAI_Chat_DecodeError_StatusOK(t *testing.T) {
    // Return status 200 but invalid JSON body; Chat should return an error
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        io.WriteString(w, "{invalid")
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    if _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}); err == nil {
        t.Fatalf("expected decode error for invalid JSON body")
    }
}

func TestOpenAI_Chat_MultiChoiceAndErrorBody(t *testing.T) {
    // Multi-choice success: return two choices with different finish reasons
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(map[string]any{
            "choices": []map[string]any{
                {"index": 0, "finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": "FIRST"}},
                {"index": 1, "finish_reason": "length", "message": map[string]string{"role": "assistant", "content": "SECOND"}},
            },
        })
    }))
    defer srv.Close()
    c := newOpenAI(srv.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c.httpClient = srv.Client()
    out, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
    if err != nil || out != "FIRST" { t.Fatalf("openai multi-choice: %v %q", err, out) }

    // Error body case: non-2xx with error message
    srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(400)
        _ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad", "type": "invalid"}})
    }))
    defer srv2.Close()
    c2 := newOpenAI(srv2.URL, "g", "KEY", f64p(0.2)).(openAIClient)
    c2.httpClient = srv2.Client()
    if _, err := c2.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}); err == nil {
        t.Fatalf("expected error from non-2xx with error body")
    }
}
