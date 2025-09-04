package llm

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
    "encoding/base64"
)

type rtFunc2 func(*http.Request) (*http.Response, error)
func (f rtFunc2) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCopilot_EnsureSession_AndChat_Success(t *testing.T) {
    // Mock chat endpoint
    chatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/chat/completions" { t.Fatalf("unexpected path: %s", r.URL.Path) }
        _ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"index":0, "message": map[string]string{"role":"assistant","content":"OK"}}}})
    }))
    defer chatSrv.Close()
    c := newCopilot(chatSrv.URL, "gpt-4o-mini", "APIKEY", f64p(0.1)).(copilotClient)
    // Intercept token endpoint to return a session token
    tr := rtFunc2(func(r *http.Request) (*http.Response, error) {
        if r.URL.Host == "api.github.com" && r.URL.Path == "/copilot_internal/v2/token" {
            rw := httptest.NewRecorder()
            _ = json.NewEncoder(rw).Encode(map[string]string{"token":"tok"})
            res := rw.Result()
            res.StatusCode = 200
            return res, nil
        }
        // Fallback to default transport for chatSrv
        return http.DefaultTransport.RoundTrip(r)
    })
    c.httpClient = &http.Client{Transport: tr, Timeout: 5 * time.Second}
    out, err := c.Chat(context.Background(), []Message{{Role:"user", Content:"hi"}})
    if err != nil || out != "OK" { t.Fatalf("copilot chat failed: %v %q", err, out) }
}

func TestCopilot_HandleNon2xx(t *testing.T) {
    b, _ := json.Marshal(map[string]any{"error": map[string]any{"message":"bad","type":"invalid"}})
    resp := &http.Response{StatusCode: 400, Body: io.NopCloser(bytesReader(b))}
    if err := handleCopilotNon2xx(resp, time.Now()); err == nil { t.Fatalf("expected error") }
}

func TestCopilot_CodeCompletion_Success(t *testing.T) {
    c := newCopilot("https://api.githubcopilot.com", "gpt-4o-mini", "API", f64p(0.1)).(copilotClient)
    tr := rtFunc2(func(r *http.Request) (*http.Response, error) {
        // Token endpoint
        if r.URL.Host == "api.github.com" && r.URL.Path == "/copilot_internal/v2/token" {
            rw := httptest.NewRecorder()
            _ = json.NewEncoder(rw).Encode(map[string]string{"token":"tok"})
            res := rw.Result(); res.StatusCode = 200; return res, nil
        }
        // Codex completion endpoint
        if r.URL.Host == "copilot-proxy.githubusercontent.com" && strings.HasSuffix(r.URL.Path, "/v1/engines/copilot-codex/completions") {
            rw := httptest.NewRecorder()
            // two choices for index 0 and 1
            rw.WriteString("data: {\"choices\":[{\"index\":0,\"text\":\"A\"}]}\n")
            rw.WriteString("data: {\"choices\":[{\"index\":1,\"text\":\"B\"}]}\n")
            res := rw.Result(); res.StatusCode = 200; return res, nil
        }
        return http.DefaultTransport.RoundTrip(r)
    })
    c.httpClient = &http.Client{Transport: tr, Timeout: 5 * time.Second}
    out, err := c.CodeCompletion(context.Background(), "p", "s", 2, "go", 0.1)
    if err != nil || len(out) != 2 || out[0] != "A" || out[1] != "B" {
        t.Fatalf("codex: %v %#v", err, out)
    }
}

func TestCopilot_Chat_MultiChoice_And_ErrorBody(t *testing.T) {
    // Chat multi-choice: return two choices; client returns first content
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(map[string]any{
            "choices": []map[string]any{
                {"index": 0, "finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": "FIRST"}},
                {"index": 1, "finish_reason": "length", "message": map[string]string{"role": "assistant", "content": "SECOND"}},
            },
        })
    }))
    defer srv.Close()
    c := newCopilot(srv.URL, "gpt-4o-mini", "KEY", f64p(0.1)).(copilotClient)
    // Token success
    tr := rtFunc2(func(r *http.Request) (*http.Response, error) {
        if r.URL.Host == "api.github.com" && r.URL.Path == "/copilot_internal/v2/token" {
            rw := httptest.NewRecorder(); _ = json.NewEncoder(rw).Encode(map[string]string{"token":"tok"}); res := rw.Result(); res.StatusCode = 200; return res, nil
        }
        return http.DefaultTransport.RoundTrip(r)
    })
    c.httpClient = &http.Client{Transport: tr, Timeout: 5 * time.Second}
    out, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
    if err != nil || out != "FIRST" { t.Fatalf("copilot multi-choice: %v %q", err, out) }

    // Non-2xx with error body
    srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(403)
        _ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message":"denied","type":"forbidden"}})
    }))
    defer srv2.Close()
    c2 := newCopilot(srv2.URL, "gpt-4o-mini", "KEY", f64p(0.1)).(copilotClient)
    c2.httpClient = &http.Client{Transport: tr, Timeout: 5 * time.Second}
    if _, err := c2.Chat(context.Background(), []Message{{Role:"user", Content:"hi"}}); err == nil {
        t.Fatalf("expected error for copilot non-2xx with error body")
    }
}

func TestParseJWTExp_AndParseInt64(t *testing.T) {
    // Valid base64 payload
    payload := `{"exp": 1700000000}`
    b := base64.RawURLEncoding.EncodeToString([]byte(payload))
    tok := "x." + b + ".y"
    if tm := parseJWTExp(tok); tm.IsZero() { t.Fatalf("expected non-zero time") }
    if n, err := parseInt64("123"); err != nil || n != 123 { t.Fatalf("parseInt64: %v %d", err, n) }
}

// bytesReader wraps a byte slice with an io.ReadCloser without importing extra.
type bytesReader []byte
func (b bytesReader) Read(p []byte) (int, error) { n := copy(p, b); return n, io.EOF }
func (b bytesReader) Close() error { return nil }
