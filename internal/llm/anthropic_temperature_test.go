package llm

import (
	"encoding/json"
	"testing"
)

func TestAnthropicRequestTemperature(t *testing.T) {
	defaultTemp := 0.2
	tests := []struct {
		model       string
		temperature float64
		defaultTemp *float64
		want        any
	}{
		{"claude-haiku-4-5", 0, &defaultTemp, nil},
		{"claude-haiku-4-5-20251001", 0.7, &defaultTemp, nil},
		{"claude-3-5-haiku-20241022", 0.7, nil, nil},
		{"claude-3-haiku-20240307", 0, &defaultTemp, nil},
		{"Haiku", 0.7, &defaultTemp, nil},
		{"claude-sonnet-4-5", 0, &defaultTemp, 0.2},
		{"claude-sonnet-4-5", 0.7, &defaultTemp, 0.7},
		{"claude-opus-4-1", 0.7, nil, 0.7},
		{"claude-opus-4-1", 0, nil, nil},
	}
	for _, tt := range tests {
		for _, stream := range []bool{false, true} {
			t.Run(tt.model, func(t *testing.T) {
				client := newAnthropic("", "claude-sonnet-4-5", "test-key", tt.defaultTemp)
				opts := client.resolveOptions([]RequestOption{WithModel(tt.model), WithTemperature(tt.temperature)})
				req := buildAnthropicChatRequest(opts, nil, client.defaultModel, client.defaultTemperature, stream)
				body, err := json.Marshal(req)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Fatal(err)
				}
				got, present := fields["temperature"]
				if got != tt.want || present != (tt.want != nil) {
					t.Fatalf("stream=%t: temperature = %v (present=%t), want %v; body=%s", stream, got, present, tt.want, body)
				}
			})
		}
	}
}
