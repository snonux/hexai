package llm

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
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

// bytesReader wraps a byte slice with an io.ReadCloser without importing extra.
type bytesReader []byte
func (b bytesReader) Read(p []byte) (int, error) { n := copy(p, b); return n, io.EOF }
func (b bytesReader) Close() error { return nil }
