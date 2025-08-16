package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hexai/internal/logging"
)

// ollamaClient implements Client against a local Ollama server.
type ollamaClient struct {
	httpClient   *http.Client
	baseURL      string
	defaultModel string
}

func newOllama(baseURL, model string) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:11434"
	}
	if strings.TrimSpace(model) == "" {
		model = "qwen2.5-coder:latest"
	}
	return &ollamaClient{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: model,
	}
}

type ollamaChatRequest struct {
	Model    string      `json:"model"`
	Messages []oaMessage `json:"messages"`
	Stream   bool        `json:"stream"`
	Options  any         `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

func (c *ollamaClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}

	start := time.Now()
	logging.Logf("llm/ollama ", "chat start model=%s temp=%.2f max_tokens=%d stop=%d messages=%d", o.Model, o.Temperature, o.MaxTokens, len(o.Stop), len(messages))
	for i, m := range messages {
		logging.Logf("llm/ollama ", "msg[%d] role=%s size=%d preview=%s%s%s", i, m.Role, len(m.Content), logging.AnsiCyan, logging.PreviewForLog(m.Content), logging.AnsiBase)
	}

	req := ollamaChatRequest{Model: o.Model, Stream: false}
	req.Messages = make([]oaMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = oaMessage{Role: m.Role, Content: m.Content}
	}

	// Build options map only if any option is set
	optsMap := map[string]any{}
	if o.Temperature != 0 {
		optsMap["temperature"] = o.Temperature
	}
	if o.MaxTokens > 0 {
		optsMap["num_predict"] = o.MaxTokens
	}
	if len(o.Stop) > 0 {
		optsMap["stop"] = o.Stop
	}
	if len(optsMap) > 0 {
		req.Options = optsMap
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	endpoint := c.baseURL + "/api/chat"
	logging.Logf("llm/ollama ", "POST %s", endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logging.Logf("llm/ollama ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr ollamaChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if strings.TrimSpace(apiErr.Error) != "" {
			logging.Logf("llm/ollama ", "%sapi error status=%d msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error, time.Since(start), logging.AnsiBase)
			return "", fmt.Errorf("ollama error: %s (status %d)", apiErr.Error, resp.StatusCode)
		}
		logging.Logf("llm/ollama ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
		return "", fmt.Errorf("ollama http error: status %d", resp.StatusCode)
	}

	var out ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logging.Logf("llm/ollama ", "%sdecode error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	if strings.TrimSpace(out.Message.Content) == "" {
		logging.Logf("llm/ollama ", "%sempty content returned duration=%s%s", logging.AnsiRed, time.Since(start), logging.AnsiBase)
		return "", errors.New("ollama: empty content")
	}
	content := out.Message.Content
	logging.Logf("llm/ollama ", "success size=%d preview=%s%s%s duration=%s", len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

// Provider metadata
func (c *ollamaClient) Name() string         { return "ollama" }
func (c *ollamaClient) DefaultModel() string { return c.defaultModel }
