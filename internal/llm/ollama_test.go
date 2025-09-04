package llm

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestBuildOllamaRequest_OptionsAndStream(t *testing.T) {
    o := Options{Model: "codemodel", Temperature: 0, MaxTokens: 256, Stop: []string{"STOP"}}
    msgs := []Message{{Role: "user", Content: "hello"}}
    req := buildOllamaRequest(o, msgs, f64p(0.2), false)
    if req.Model != "codemodel" || req.Stream { t.Fatalf("model/stream mismatch: %+v", req) }
    if req.Options == nil { t.Fatalf("expected options map") }
    if req.Options.(map[string]any)["temperature"].(float64) != 0.2 { t.Fatalf("default temp not applied") }
    if req.Options.(map[string]any)["num_predict"].(int) != 256 { t.Fatalf("num_predict not applied") }
    if req.Options.(map[string]any)["stop"].([]string)[0] != "STOP" { t.Fatalf("stop not applied") }

    req2 := buildOllamaRequest(o, msgs, f64p(0.2), true)
    if !req2.Stream { t.Fatalf("expected stream=true") }
}

func TestBuildOllamaRequest_TempOverride(t *testing.T) {
    o := Options{Model: "m", Temperature: 0.9}
    msgs := []Message{{Role: "user", Content: "hi"}}
    req := buildOllamaRequest(o, msgs, f64p(0.2), false)
    m := req.Options.(map[string]any)
    if m["temperature"].(float64) != 0.9 { t.Fatalf("explicit temp should override default") }
}

func TestOllama_NameAndModel(t *testing.T) {
    c := newOllama("http://x", "model-x", nil).(ollamaClient)
    if c.Name() != "ollama" { t.Fatalf("name: %q", c.Name()) }
    if c.DefaultModel() != "model-x" { t.Fatalf("default model: %q", c.DefaultModel()) }
}

func TestOllamaChat_Success(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost || r.URL.Path != "/api/chat" { t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path) }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role":"assistant","content":"Hello"}, "done": true})
    }))
    defer ts.Close()
    c := newOllama(ts.URL, "m", f64p(0.1)).(ollamaClient)
    c.httpClient = ts.Client()
    out, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
    if err != nil { t.Fatalf("unexpected err: %v", err) }
    if out != "Hello" { t.Fatalf("got %q", out) }
}

func TestOllamaChat_EmptyContent(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role":"assistant","content":""}, "done": true})
    }))
    defer ts.Close()
    c := newOllama(ts.URL, "m", nil).(ollamaClient)
    c.httpClient = ts.Client()
    if _, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"x"}}); err == nil {
        t.Fatalf("expected error for empty content")
    }
}

func TestOllamaChat_Non2xx(t *testing.T) {
    // API error string
    ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(400)
        _ = json.NewEncoder(w).Encode(map[string]any{"error":"bad"})
    }))
    defer ts1.Close()
    c1 := newOllama(ts1.URL, "m", nil).(ollamaClient)
    c1.httpClient = ts1.Client()
    if _, err := c1.Chat(context.Background(), []Message{{Role:"user", Content:"x"}}); err == nil {
        t.Fatalf("expected error for 400 with api body")
    }
    // Plain HTTP error without api message
    ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(500)
        _, _ = w.Write([]byte("{}"))
    }))
    defer ts2.Close()
    c2 := newOllama(ts2.URL, "m", nil).(ollamaClient)
    c2.httpClient = ts2.Client()
    if _, err := c2.Chat(context.Background(), []Message{{Role:"user", Content:"x"}}); err == nil {
        t.Fatalf("expected error for 500")
    }
}

type rtFunc func(*http.Request) (*http.Response, error)
func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOllamaChat_HTTPError(t *testing.T) {
    c := newOllama("http://127.0.0.1:0", "m", nil).(ollamaClient)
    c.httpClient = &http.Client{Transport: rtFunc(func(*http.Request)(*http.Response,error){ return nil, fmt.Errorf("boom") })}
    if _, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"x"}}); err == nil {
        t.Fatalf("expected http error path")
    }
}

func TestOllamaChat_DecodeError(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("{bad json}"))
    }))
    defer ts.Close()
    c := newOllama(ts.URL, "m", nil).(ollamaClient)
    c.httpClient = ts.Client()
    if _, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"x"}}); err == nil {
        t.Fatalf("expected decode error")
    }
}

func TestHandleOllamaNon2xx_OK(t *testing.T) {
    resp := &http.Response{StatusCode: 200, Body: ioNopCloser(strings.NewReader(""))}
    if err := handleOllamaNon2xx(resp, time.Now()); err != nil { t.Fatalf("unexpected: %v", err) }
}

func TestOllamaChatStream_Success(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        // two JSON objects back-to-back
        _, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Hi"},"done":false}`))
        _, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"!"},"done":true}`))
    }))
    defer ts.Close()
    c := newOllama(ts.URL, "m", nil).(ollamaClient)
    c.httpClient = ts.Client()
    var got strings.Builder
    if err := c.ChatStream(context.Background(), []Message{{Role:"user", Content:"x"}}, func(s string){ got.WriteString(s) }); err != nil {
        t.Fatalf("unexpected: %v", err)
    }
    if got.String() != "Hi!" { t.Fatalf("got %q", got.String()) }
}

func TestOllamaChatStream_ErrorEvent(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(map[string]any{"error":"oops"})
    }))
    defer ts.Close()
    c := newOllama(ts.URL, "m", nil).(ollamaClient)
    c.httpClient = ts.Client()
    if err := c.ChatStream(context.Background(), []Message{{Role:"user", Content:"x"}}, func(string){}); err == nil {
        t.Fatalf("expected stream error")
    }
}

func TestOllamaChatStream_DecodeError(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("{not json}"))
    }))
    defer ts.Close()
    c := newOllama(ts.URL, "m", nil).(ollamaClient)
    c.httpClient = ts.Client()
    if err := c.ChatStream(context.Background(), []Message{{Role:"user", Content:"x"}}, func(string){}); err == nil {
        t.Fatalf("expected decode error")
    }
}

// small helper to construct an io.ReadCloser without importing extra packages
type readCloser struct{ *strings.Reader }
func (readCloser) Close() error { return nil }
func ioNopCloser(r *strings.Reader) *readCloser { return &readCloser{r} }
