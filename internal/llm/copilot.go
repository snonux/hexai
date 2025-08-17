// Summary: GitHub Copilot client implementation for chat completions using the Copilot API.
// Not yet reviewed by a human
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

// copilotClient implements Client against GitHub Copilot's Chat Completions API.
type copilotClient struct {
    httpClient   *http.Client
    apiKey       string
    baseURL      string
    defaultModel string
    chatLogger   logging.ChatLogger
}

type copilotChatRequest struct {
    Model       string           `json:"model"`
    Messages    []copilotMessage `json:"messages"`
    Temperature *float64         `json:"temperature,omitempty"`
    MaxTokens   *int             `json:"max_tokens,omitempty"`
    Stop        []string         `json:"stop,omitempty"`
}

type copilotMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type copilotChatResponse struct {
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

// Constructor (kept among the first functions by convention)
func newCopilot(baseURL, model, apiKey string) Client {
    if strings.TrimSpace(baseURL) == "" {
        baseURL = "https://api.githubcopilot.com"
    }
    if strings.TrimSpace(model) == "" {
        model = "gpt-4.1"
    }
    return copilotClient{
        httpClient:   &http.Client{Timeout: 30 * time.Second},
        apiKey:       apiKey,
        baseURL:      strings.TrimRight(baseURL, "/"),
        defaultModel: model,
        chatLogger:   logging.NewChatLogger("copilot"),
    }
}

func (c copilotClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nilStringErr("missing Copilot API key")
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

	req := copilotChatRequest{Model: o.Model}
	req.Messages = make([]copilotMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = copilotMessage{Role: m.Role, Content: m.Content}
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
		logging.Logf("llm/copilot ", "marshal error: %v", err)
		return "", err
	}

	endpoint := c.baseURL + "/chat/completions"
	logging.Logf("llm/copilot ", "POST %s", endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		logging.Logf("llm/copilot ", "new request error: %v", err)
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logging.Logf("llm/copilot ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr copilotChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil && strings.TrimSpace(apiErr.Error.Message) != "" {
			logging.Logf("llm/copilot ", "%sapi error status=%d type=%s msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), logging.AnsiBase)
			return "", fmt.Errorf("copilot error: %s (status %d)", apiErr.Error.Message, resp.StatusCode)
		}
		logging.Logf("llm/copilot ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
		return "", fmt.Errorf("copilot http error: status %d", resp.StatusCode)
	}

	var out copilotChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logging.Logf("llm/copilot ", "%sdecode error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	if len(out.Choices) == 0 {
		logging.Logf("llm/copilot ", "%sno choices returned duration=%s%s", logging.AnsiRed, time.Since(start), logging.AnsiBase)
		return "", errors.New("copilot: no choices returned")
	}
	content := out.Choices[0].Message.Content
	logging.Logf("llm/copilot ", "success choice=0 finish=%s size=%d preview=%s%s%s duration=%s", out.Choices[0].FinishReason, len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

// Provider metadata
func (c copilotClient) Name() string         { return "copilot" }
func (c copilotClient) DefaultModel() string { return c.defaultModel }
