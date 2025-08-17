// Summary: OpenAI client implementation for chat completions with optional streaming and detailed logging.
// Not yet reviewed by a human
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

	"hexai/internal/logging"
)

// openAIClient implements Client against OpenAI's Chat Completions API.
type openAIClient struct {
	httpClient   *http.Client
	apiKey       string
	baseURL      string
	defaultModel string
    chatLogger   logging.ChatLogger
}

// newOpenAI constructs an OpenAI client using explicit configuration values.
// The apiKey may be empty; calls will fail until a valid key is supplied.
func newOpenAI(baseURL, model, apiKey string) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4.1"
	}
    return openAIClient{
        httpClient:   &http.Client{Timeout: 30 * time.Second},
        apiKey:       apiKey,
        baseURL:      baseURL,
        defaultModel: model,
        chatLogger:   logging.NewChatLogger("openai"),
    }
}

type oaChatRequest struct {
	Model       string      `json:"model"`
	Messages    []oaMessage `json:"messages"`
	Temperature *float64    `json:"temperature,omitempty"`
	MaxTokens   *int        `json:"max_tokens,omitempty"`
	Stop        []string    `json:"stop,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
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
	logMessages := make([]struct {
		Role    string
		Content string
	}, len(messages))
	for i, m := range messages {
		logMessages[i] = struct {
			Role    string
			Content string
		}{Role: m.Role, Content: m.Content}
	}
	c.chatLogger.LogStart(false, o.Model, o.Temperature, o.MaxTokens, o.Stop, logMessages)

	req := oaChatRequest{Model: o.Model}
	req.Messages = make([]oaMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = oaMessage{Role: m.Role, Content: m.Content}
	}
	if o.Temperature != 0 {
		req.Temperature = &o.Temperature
	}
	if o.MaxTokens > 0 {
		req.MaxTokens = &o.MaxTokens
	}
	if len(o.Stop) > 0 {
		req.Stop = o.Stop
	}

	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return "", err
	}
	endpoint := c.baseURL + "/chat/completions"
	logging.Logf("llm/openai ", "POST %s", endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.logf("new request error: %v", err)
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logging.Logf("llm/openai ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr oaChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil && apiErr.Error.Message != "" {
			logging.Logf("llm/openai ", "%sapi error status=%d type=%s msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), logging.AnsiBase)
			return "", fmt.Errorf("openai error: %s (status %d)", apiErr.Error.Message, resp.StatusCode)
		}
		logging.Logf("llm/openai ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
		return "", fmt.Errorf("openai http error: status %d", resp.StatusCode)
	}
	var out oaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logging.Logf("llm/openai ", "%sdecode error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	if len(out.Choices) == 0 {
		logging.Logf("llm/openai ", "%sno choices returned duration=%s%s", logging.AnsiRed, time.Since(start), logging.AnsiBase)
		return "", errors.New("openai: no choices returned")
	}
	content := out.Choices[0].Message.Content
	// Received context (green)
	logging.Logf("llm/openai ", "success choice=0 finish=%s size=%d preview=%s%s%s duration=%s", out.Choices[0].FinishReason, len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

func (c openAIClient) logf(format string, args ...any) { logging.Logf("llm/openai ", format, args...) }

// Provider metadata
func (c openAIClient) Name() string         { return "openai" }
func (c openAIClient) DefaultModel() string { return c.defaultModel }

// Streaming support (optional)
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
	logMessages := make([]struct {
		Role    string
		Content string
	}, len(messages))
	for i, m := range messages {
		logMessages[i] = struct {
			Role    string
			Content string
		}{Role: m.Role, Content: m.Content}
	}
	c.chatLogger.LogStart(true, o.Model, o.Temperature, o.MaxTokens, o.Stop, logMessages)

	req := oaChatRequest{Model: o.Model, Stream: true}
	req.Messages = make([]oaMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = oaMessage{Role: m.Role, Content: m.Content}
	}
	if o.Temperature != 0 {
		req.Temperature = &o.Temperature
	}
	if o.MaxTokens > 0 {
		req.MaxTokens = &o.MaxTokens
	}
	if len(o.Stop) > 0 {
		req.Stop = o.Stop
	}

	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return err
	}
	endpoint := c.baseURL + "/chat/completions"
	logging.Logf("llm/openai ", "POST %s (stream)", endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.logf("new request error: %v", err)
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	// Streaming uses SSE-style data lines
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logging.Logf("llm/openai ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// try to decode body to surface message
		var apiErr oaChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil && apiErr.Error.Message != "" {
			logging.Logf("llm/openai ", "%sapi error status=%d type=%s msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), logging.AnsiBase)
			return fmt.Errorf("openai error: %s (status %d)", apiErr.Error.Message, resp.StatusCode)
		}
		logging.Logf("llm/openai ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
		return fmt.Errorf("openai http error: status %d", resp.StatusCode)
	}

	// Parse SSE: lines starting with "data: " containing JSON or [DONE]
	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer for long lines
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
			continue // skip malformed lines
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			logging.Logf("llm/openai ", "%sstream error: %s%s", logging.AnsiRed, chunk.Error.Message, logging.AnsiBase)
			return fmt.Errorf("openai stream error: %s", chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				onDelta(ch.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logging.Logf("llm/openai ", "%sstream read error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	logging.Logf("llm/openai ", "stream end duration=%s", time.Since(start))
	return nil
}
