// Summary: OpenAI client implementation for chat completions with optional streaming and detailed logging.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/logging"
)

// openAIClient implements Client against OpenAI's Chat Completions API.
type openAIClient struct {
	httpClient         *http.Client
	apiKey             string
	baseURL            string
	defaultModel       string
	chatLogger         logging.ChatLogger
	defaultTemperature *float64
}

type oaChatRequest struct {
	Model               string      `json:"model"`
	Messages            []oaMessage `json:"messages"`
	Temperature         *float64    `json:"temperature,omitempty"`
	MaxTokens           *int        `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int        `json:"max_completion_tokens,omitempty"`
	Stop                []string    `json:"stop,omitempty"`
	Stream              bool        `json:"stream,omitempty"`
}

type oaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaChatResponse struct {
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   any    `json:"param"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

// Streaming response chunk type (SSE)
type oaStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   any    `json:"param"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

func init() {
	RegisterProvider("openai", openAIProviderFactory)
}

func openAIProviderFactory(cfg Config, keys ProviderKeys) (Client, error) {
	if strings.TrimSpace(keys.OpenAIAPIKey) == "" {
		return nil, errors.New("missing OPENAI_API_KEY for provider openai")
	}
	return newOpenAIWithTimeout(
		cfg.OpenAIBaseURL,
		cfg.OpenAIModel,
		keys.OpenAIAPIKey,
		resolveOpenAITemperature(cfg.OpenAIModel, cfg.OpenAITemperature),
		cfg.RequestTimeout,
	), nil
}

func resolveOpenAITemperature(model string, configured *float64) *float64 {
	isGPT5 := strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-5")
	if isGPT5 {
		if configured == nil || *configured == 0.2 {
			v := 1.0
			return &v
		}
		return configured
	}
	if configured != nil {
		return configured
	}
	v := 0.2
	return &v
}

// Constructor (kept among the first functions by convention)
// newOpenAI constructs an OpenAI client using explicit configuration values.
// The apiKey may be empty; calls will fail until a valid key is supplied.
func newOpenAI(baseURL, model, apiKey string, defaultTemp *float64) Client {
	return newOpenAIWithTimeout(baseURL, model, apiKey, defaultTemp, 0)
}

func newOpenAIWithTimeout(baseURL, model, apiKey string, defaultTemp *float64, timeoutSec int) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4.1"
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return openAIClient{
		httpClient:         &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		apiKey:             apiKey,
		baseURL:            baseURL,
		defaultModel:       model,
		chatLogger:         logging.NewChatLogger("openai"),
		defaultTemperature: defaultTemp,
	}
}

func (c openAIClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	if c.apiKey == "" {
		return nilStringErr("missing OpenAI API key")
	}
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}
	start := time.Now()
	c.logStart(false, o, messages)
	req := buildOAChatRequest(o, messages, c.defaultTemperature, false, "llm/openai ")
	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return "", err
	}
	endpoint := c.baseURL + "/chat/completions"
	logging.Logf("llm/openai ", "POST %s", endpoint)
	resp, err := c.doJSON(ctx, endpoint, body, map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	})
	if err != nil {
		logging.Logf("llm/openai ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Logf("llm/openai", "failed to close response body: %v", err)
		}
	}()
	if err := handleOpenAINon2xx(resp, start, "llm/openai ", "openai"); err != nil {
		return "", err
	}
	out, err := decodeOpenAIChat(resp, start, "llm/openai ")
	if err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		logging.Logf("llm/openai ", "%sno choices returned duration=%s%s", logging.AnsiRed, time.Since(start), logging.AnsiBase)
		return "", errors.New("openai: no choices returned")
	}
	content := out.Choices[0].Message.Content
	logging.Logf("llm/openai ", "success choice=0 finish=%s size=%d preview=%s%s%s duration=%s", out.Choices[0].FinishReason, len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

// Provider metadata
func (c openAIClient) Name() string         { return "openai" }
func (c openAIClient) DefaultModel() string { return c.defaultModel }

// Streaming support (optional)

func (c openAIClient) ChatStream(ctx context.Context, messages []Message, onDelta func(string), opts ...RequestOption) error {
	if c.apiKey == "" {
		return errors.New("missing OpenAI API key")
	}
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}
	start := time.Now()
	c.logStart(true, o, messages)
	req := buildOAChatRequest(o, messages, c.defaultTemperature, true, "llm/openai ")
	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return err
	}
	endpoint := c.baseURL + "/chat/completions"
	logging.Logf("llm/openai ", "POST %s (stream)", endpoint)
	resp, err := c.doJSONWithAccept(ctx, endpoint, body, map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}, "text/event-stream")
	if err != nil {
		logging.Logf("llm/openai ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Logf("llm/openai", "failed to close response body: %v", err)
		}
	}()
	if err := handleOpenAINon2xx(resp, start, "llm/openai ", "openai"); err != nil {
		return err
	}

	if err := parseOpenAIStream(resp, start, onDelta, "llm/openai ", "openai"); err != nil {
		return err
	}
	logging.Logf("llm/openai ", "stream end duration=%s", time.Since(start))
	return nil
}

// Private helpers
func (c openAIClient) logf(format string, args ...any) { logging.Logf("llm/openai ", format, args...) }

// helpers extracted to keep methods small
func (c openAIClient) logStart(stream bool, o Options, messages []Message) {
	logMessages := make([]struct{ Role, Content string }, len(messages))
	for i, m := range messages {
		logMessages[i] = struct{ Role, Content string }{m.Role, m.Content}
	}
	c.chatLogger.LogStart(stream, o.Model, o.Temperature, o.MaxTokens, o.Stop, logMessages)
}

func buildOAChatRequest(o Options, messages []Message, defaultTemp *float64, stream bool, logPrefix string) oaChatRequest {
	req := oaChatRequest{Model: o.Model, Stream: stream}
	req.Messages = make([]oaMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = oaMessage(m)
	}
	if o.Temperature != 0 {
		req.Temperature = &o.Temperature
	} else if defaultTemp != nil {
		t := *defaultTemp
		req.Temperature = &t
	}
	if o.MaxTokens > 0 {
		if requiresMaxCompletionTokens(o.Model) {
			req.MaxCompletionTokens = &o.MaxTokens
		} else {
			req.MaxTokens = &o.MaxTokens
		}
	}
	if len(o.Stop) > 0 {
		req.Stop = o.Stop
	}
	// Enforce gpt-5 temperature constraints: only default (1.0) is supported.
	if requiresMaxCompletionTokens(o.Model) {
		if req.Temperature == nil || *req.Temperature != 1.0 {
			t := 1.0
			req.Temperature = &t
			logging.Logf(logPrefix, "forcing temperature=1.0 for model=%s (gpt-5 constraint)", o.Model)
		}
	}
	return req
}

// requiresMaxCompletionTokens reports whether the given model prefers the
// new parameter name "max_completion_tokens" instead of "max_tokens". Newer
// models (e.g., gpt-5 family) expect this per OpenAI's API error guidance.
func requiresMaxCompletionTokens(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "gpt-5")
}

func (c openAIClient) doJSON(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.httpClient.Do(req)
}

func (c openAIClient) doJSONWithAccept(ctx context.Context, url string, body []byte, headers map[string]string, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.httpClient.Do(req)
}

func handleOpenAINon2xx(resp *http.Response, start time.Time, logPrefix, provider string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var apiErr oaChatResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Error != nil && apiErr.Error.Message != "" {
		logging.Logf(logPrefix, "%sapi error status=%d type=%s msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), logging.AnsiBase)
		return fmt.Errorf("%s error: %s (status %d)", provider, apiErr.Error.Message, resp.StatusCode)
	}
	logging.Logf(logPrefix, "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
	return fmt.Errorf("%s http error: status %d", provider, resp.StatusCode)
}

func decodeOpenAIChat(resp *http.Response, start time.Time, logPrefix string) (oaChatResponse, error) {
	var out oaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logging.Logf(logPrefix, "%sdecode error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return oaChatResponse{}, err
	}
	return out, nil
}

func parseOpenAIStream(resp *http.Response, start time.Time, onDelta func(string), logPrefix, provider string) error {
	// Parse SSE: lines starting with "data: " containing JSON or [DONE]
	scanner := bufio.NewScanner(resp.Body)
	const maxBuf = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxBuf)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(payload) == "[DONE]" {
			break
		}
		var chunk oaStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			logging.Logf(logPrefix, "%sstream error: %s%s", logging.AnsiRed, chunk.Error.Message, logging.AnsiBase)
			return fmt.Errorf("%s stream error: %s", provider, chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				onDelta(ch.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logging.Logf(logPrefix, "%sstream read error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	return nil
}
