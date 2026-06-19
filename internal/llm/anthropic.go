// Anthropic client implementation using Messages API with optional streaming support.

package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm/policy"
	"codeberg.org/snonux/hexai/internal/logging"
)

// anthropicClient implements Client against Anthropic's Messages API.
type anthropicClient struct {
	httpClient         *http.Client
	apiKey             string
	baseURL            string
	defaultModel       string
	chatLogger         logging.ChatLogger
	defaultTemperature *float64
}

// Ensure anthropicClient implements Client and Streamer. All of its methods use
// value receivers, so the assertions use a zero-value struct (not a pointer).
var (
	_ Client   = anthropicClient{}
	_ Streamer = anthropicClient{}
)

type anthropicChatRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream,omitempty"`
	System      string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicChatResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Streaming event types
type anthropicStreamStart struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Role  string `json:"role"`
		Model string `json:"model"`
	} `json:"message"`
}

type anthropicStreamDelta struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

type anthropicStreamError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Ensure anthropicClient implements Client and Streamer.
var (
	_ Client   = anthropicClient{}
	_ Streamer = anthropicClient{}
)

func anthropicProviderFactory(cfg Config, keys ProviderKeys) (Client, error) {
	if strings.TrimSpace(keys.AnthropicAPIKey) == "" {
		return nil, missingAPIKeyError("anthropic", "ANTHROPIC_API_KEY", "HEXAI_ANTHROPIC_API_KEY")
	}
	return newAnthropicWithTimeout(
		cfg.AnthropicBaseURL,
		cfg.AnthropicModel,
		keys.AnthropicAPIKey,
		withDefaultTemperature(cfg.AnthropicTemperature, 0.2),
		cfg.RequestTimeout,
	), nil
}

// Constructor
// newAnthropic constructs an Anthropic client using explicit configuration values.
// The apiKey may be empty; calls will fail until a valid key is supplied.
// Following the Go idiom "return concrete types, accept interfaces", this
// returns anthropicClient directly; the provider registry wraps it back into a
// Client at registration time.
func newAnthropic(baseURL, model, apiKey string, defaultTemp *float64) anthropicClient {
	return newAnthropicWithTimeout(baseURL, model, apiKey, defaultTemp, 0)
}

func newAnthropicWithTimeout(baseURL, model, apiKey string, defaultTemp *float64, timeoutSec int) anthropicClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "claude-3-5-sonnet-20240620"
	}
	if timeoutSec <= 0 {
		// Fall back to the shared default chat timeout from the policy package.
		timeoutSec = policy.DefaultRequestTimeoutSeconds
	}
	return anthropicClient{
		httpClient:         &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		apiKey:             apiKey,
		baseURL:            baseURL,
		defaultModel:       model,
		chatLogger:         logging.NewChatLogger("anthropic"),
		defaultTemperature: defaultTemp,
	}
}

// Chat sends a request to Anthropic and returns the response.
func (c anthropicClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	if c.apiKey == "" {
		return "", missingAPIKeyError("anthropic", "ANTHROPIC_API_KEY", "HEXAI_ANTHROPIC_API_KEY")
	}
	o := c.resolveOptions(opts)
	start := time.Now()
	c.logStart(false, o, messages)

	resp, err := c.sendRequest(ctx, o, messages, false, start)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Logf("llm/anthropic", "failed to close response body: %v", err)
		}
	}()

	if err := handleAnthropicNon2xx(resp, start); err != nil {
		return "", err
	}
	out, err := decodeAnthropicChat(resp, start)
	if err != nil {
		return "", err
	}
	return c.extractContent(out, start)
}

// Name returns the provider's short name.
func (c anthropicClient) Name() string { return "anthropic" }

// DefaultModel returns the configured default model name.
func (c anthropicClient) DefaultModel() string { return c.defaultModel }

// ChatStream sends a streaming request and invokes onDelta for each text chunk.
func (c anthropicClient) ChatStream(ctx context.Context, messages []Message, onDelta func(string), opts ...RequestOption) error {
	if c.apiKey == "" {
		return missingAPIKeyError("anthropic", "ANTHROPIC_API_KEY", "HEXAI_ANTHROPIC_API_KEY")
	}
	o := c.resolveOptions(opts)
	start := time.Now()
	c.logStart(true, o, messages)

	resp, err := c.sendRequest(ctx, o, messages, true, start)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Logf("llm/anthropic", "failed to close response body: %v", err)
		}
	}()

	if err := handleAnthropicNon2xx(resp, start); err != nil {
		return err
	}
	if err := parseAnthropicStream(resp, start, onDelta); err != nil {
		return err
	}
	logging.Logf("llm/anthropic ", "stream end duration=%s", time.Since(start))
	return nil
}

// Private helpers

func (c anthropicClient) resolveOptions(opts []RequestOption) Options {
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}
	return o
}

func (c anthropicClient) sendRequest(ctx context.Context, o Options, messages []Message, stream bool, start time.Time) (*http.Response, error) {
	req := buildAnthropicChatRequest(o, messages, c.defaultModel, c.defaultTemperature, stream)
	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return nil, err
	}
	endpoint := c.baseURL + "/messages"
	mode := "POST"
	if stream {
		mode = "POST (stream)"
	}
	logging.Logf("llm/anthropic ", "%s %s", mode, endpoint)
	return c.doJSON(ctx, endpoint, body, map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	})
}

func (c anthropicClient) extractContent(out anthropicChatResponse, start time.Time) (string, error) {
	if len(out.Content) == 0 {
		logging.Logf("llm/anthropic ", "%sno content returned duration=%s%s", logging.AnsiRed, time.Since(start), logging.AnsiBase)
		return "", errors.New("anthropic: no content returned")
	}
	content := out.Content[0].Text
	logging.Logf("llm/anthropic ", "success stop_reason=%s size=%d preview=%s%s%s duration=%s", out.StopReason, len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

func (c anthropicClient) logf(format string, args ...any) {
	logging.Logf("llm/anthropic ", format, args...)
}

func (c anthropicClient) logStart(stream bool, o Options, messages []Message) {
	logStartMessages(c.chatLogger, stream, o, messages)
}

func buildAnthropicChatRequest(o Options, messages []Message, defaultModel string, defaultTemp *float64, stream bool) anthropicChatRequest {
	req := anthropicChatRequest{
		Model:     o.Model,
		Stream:    stream,
		MaxTokens: 4096, // Anthropic requires max_tokens
	}
	// Anthropic requires system messages in a top-level "system" field, not in messages array
	var systemParts []string
	var nonSystemMessages []Message
	for _, m := range messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
		} else {
			nonSystemMessages = append(nonSystemMessages, m)
		}
	}
	if len(systemParts) > 0 {
		req.System = strings.Join(systemParts, "\n\n")
	}
	req.Messages = make([]anthropicMessage, len(nonSystemMessages))
	for i, m := range nonSystemMessages {
		req.Messages[i] = anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	if o.Temperature != 0 {
		req.Temperature = &o.Temperature
	} else if defaultTemp != nil {
		t := *defaultTemp
		req.Temperature = &t
	}
	if o.MaxTokens > 0 {
		req.MaxTokens = o.MaxTokens
	}
	return req
}

func (c anthropicClient) doJSON(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	return doJSONRequest(ctx, c.httpClient, url, body, headers, "")
}

func handleAnthropicNon2xx(resp *http.Response, start time.Time) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var apiErr anthropicChatResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Error != nil && apiErr.Error.Message != "" {
		logging.Logf("llm/anthropic ", "%sapi error status=%d type=%s msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), logging.AnsiBase)
		return fmt.Errorf("anthropic error: %s (status %d)", apiErr.Error.Message, resp.StatusCode)
	}
	logging.Logf("llm/anthropic ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
	return fmt.Errorf("anthropic http error: status %d", resp.StatusCode)
}

func decodeAnthropicChat(resp *http.Response, start time.Time) (anthropicChatResponse, error) {
	var out anthropicChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logging.Logf("llm/anthropic ", "%sdecode error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return anthropicChatResponse{}, err
	}
	return out, nil
}

func parseAnthropicStream(resp *http.Response, start time.Time, onDelta func(string)) error {
	// Parse server-sent events: lines starting with "data: " containing JSON
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
		// Check for stream end event
		if strings.Contains(payload, "\"type\":\"message_stop\"") {
			break
		}
		// Try to parse as delta event
		var delta anthropicStreamDelta
		if err := json.Unmarshal([]byte(payload), &delta); err != nil {
			continue
		}
		if delta.Type == "content_block_delta" && delta.Delta.Type == "text_delta" && delta.Delta.Text != "" {
			onDelta(delta.Delta.Text)
		}
		// Check for errors in stream
		var errEvent anthropicStreamError
		if err := json.Unmarshal([]byte(payload), &errEvent); err == nil {
			if errEvent.Type == "error" && errEvent.Error.Message != "" {
				logging.Logf("llm/anthropic ", "%sstream error: %s%s", logging.AnsiRed, errEvent.Error.Message, logging.AnsiBase)
				return fmt.Errorf("anthropic stream error: %s", errEvent.Error.Message)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logging.Logf("llm/anthropic ", "%sstream read error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	return nil
}
