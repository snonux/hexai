package llm

import "testing"

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

