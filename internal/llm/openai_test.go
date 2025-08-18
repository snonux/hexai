package llm

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "strings"
    "testing"
    "time"
)

func f64p(v float64) *float64 { return &v }

func TestBuildOAChatRequest_TempFallbackAndFields(t *testing.T) {
    o := Options{Model: "m1", Temperature: 0, MaxTokens: 42, Stop: []string{"END"}}
    msgs := []Message{{Role: "user", Content: "hi"}}
    req := buildOAChatRequest(o, msgs, f64p(0.3), false)
    if req.Model != "m1" || req.Stream { t.Fatalf("model/stream mismatch: %+v", req) }
    if req.Temperature == nil || *req.Temperature != 0.3 { t.Fatalf("expected default temp 0.3, got %#v", req.Temperature) }
    if req.MaxTokens == nil || *req.MaxTokens != 42 { t.Fatalf("expected max tokens 42") }
    if len(req.Stop) != 1 || req.Stop[0] != "END" { t.Fatalf("stop not propagated: %#v", req.Stop) }
    if len(req.Messages) != 1 || req.Messages[0].Content != "hi" { t.Fatalf("messages not copied") }

    // stream on
    req2 := buildOAChatRequest(o, msgs, f64p(0.3), true)
    if !req2.Stream { t.Fatalf("expected stream=true") }
}

func TestHandleOpenAINon2xx_WithAPIError(t *testing.T) {
    api := oaChatResponse{Error: &struct{ Message string `json:"message"`; Type string `json:"type"`; Param any `json:"param"`; Code any `json:"code"` }{Message: "bad", Type: "invalid"}}
    b, _ := json.Marshal(api)
    resp := &http.Response{StatusCode: 400, Body: io.NopCloser(bytes.NewReader(b))}
    if err := handleOpenAINon2xx(resp, time.Now()); err == nil { t.Fatalf("expected error for non-2xx with body") }
}

func TestParseOpenAIStream_DeliversChunks(t *testing.T) {
    stream := "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n" +
        "data: [DONE]\n"
    resp := &http.Response{Body: io.NopCloser(strings.NewReader(stream))}
    var got strings.Builder
    if err := parseOpenAIStream(resp, time.Now(), func(s string){ got.WriteString(s) }); err != nil { t.Fatalf("unexpected error: %v", err) }
    if got.String() != "Hi" { t.Fatalf("got %q want %q", got.String(), "Hi") }
}
