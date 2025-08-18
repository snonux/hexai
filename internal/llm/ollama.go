// Summary: Ollama client against a local server; supports chat responses and streaming via /api/chat.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hexai/internal/logging"
)

// ollamaClient implements Client against a local Ollama server.
type ollamaClient struct {
	httpClient         *http.Client
	baseURL            string
	defaultModel       string
	chatLogger         logging.ChatLogger
	defaultTemperature *float64
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

// Constructor (kept among the first functions by convention)
func newOllama(baseURL, model string, defaultTemp *float64) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:11434"
	}
	if strings.TrimSpace(model) == "" {
		model = "qwen3-coder:30b-a3b-q4_K_M`"
	}
	return ollamaClient{
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		baseURL:            strings.TrimRight(baseURL, "/"),
		defaultModel:       model,
		chatLogger:         logging.NewChatLogger("ollama"),
		defaultTemperature: defaultTemp,
	}
}

// TODO: This function is too long and should be refactored for readability and maintainability.
func (c ollamaClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}

	start := time.Now()
	c.logStart(false, o, messages)
	req := buildOllamaRequest(o, messages, c.defaultTemperature, false)
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	endpoint := c.baseURL + "/api/chat"
	logging.Logf("llm/ollama ", "POST %s", endpoint)
	resp, err := c.doJSON(ctx, endpoint, body)
	if err != nil {
		logging.Logf("llm/ollama ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer resp.Body.Close()
	if err := handleOllamaNon2xx(resp, start); err != nil {
		return "", err
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
func (c ollamaClient) Name() string         { return "ollama" }
func (c ollamaClient) DefaultModel() string { return c.defaultModel }

// Streaming support (optional)
func (c ollamaClient) ChatStream(ctx context.Context, messages []Message, onDelta func(string), opts ...RequestOption) error {
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}

	start := time.Now()
	c.logStart(true, o, messages)
	req := buildOllamaRequest(o, messages, c.defaultTemperature, true)
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	endpoint := c.baseURL + "/api/chat"
	logging.Logf("llm/ollama ", "POST %s (stream)", endpoint)
	resp, err := c.doJSON(ctx, endpoint, body)
	if err != nil {
		logging.Logf("llm/ollama ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	defer resp.Body.Close()
	if err := handleOllamaNon2xx(resp, start); err != nil {
		return err
	}

	dec := json.NewDecoder(resp.Body)
	for {
		var ev ollamaChatResponse
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			logging.Logf("llm/ollama ", "%sdecode stream error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
			return err
		}
		if strings.TrimSpace(ev.Error) != "" {
			logging.Logf("llm/ollama ", "%sstream event error: %s%s", logging.AnsiRed, ev.Error, logging.AnsiBase)
			return fmt.Errorf("ollama stream error: %s", ev.Error)
		}
		if s := ev.Message.Content; strings.TrimSpace(s) != "" {
			onDelta(s)
		}
		if ev.Done {
			break
		}
	}
	logging.Logf("llm/ollama ", "stream end duration=%s", time.Since(start))
	return nil
}

// helpers to keep methods small
func (c ollamaClient) logStart(stream bool, o Options, messages []Message) {
	logMessages := make([]struct{ Role, Content string }, len(messages))
	for i, m := range messages {
		logMessages[i] = struct{ Role, Content string }{m.Role, m.Content}
	}
	c.chatLogger.LogStart(stream, o.Model, o.Temperature, o.MaxTokens, o.Stop, logMessages)
}

func buildOllamaRequest(o Options, messages []Message, defaultTemp *float64, stream bool) ollamaChatRequest {
	req := ollamaChatRequest{Model: o.Model, Stream: stream}
	req.Messages = make([]oaMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = oaMessage{Role: m.Role, Content: m.Content}
	}
	optsMap := map[string]any{}
	if o.Temperature != 0 {
		optsMap["temperature"] = o.Temperature
	} else if defaultTemp != nil {
		optsMap["temperature"] = *defaultTemp
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
	return req
}

func (c ollamaClient) doJSON(ctx context.Context, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

func handleOllamaNon2xx(resp *http.Response, start time.Time) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var apiErr ollamaChatResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	if strings.TrimSpace(apiErr.Error) != "" {
		logging.Logf("llm/ollama ", "%sapi error status=%d msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error, time.Since(start), logging.AnsiBase)
		return fmt.Errorf("ollama error: %s (status %d)", apiErr.Error, resp.StatusCode)
	}
	logging.Logf("llm/ollama ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
	return fmt.Errorf("ollama http error: status %d", resp.StatusCode)
}
