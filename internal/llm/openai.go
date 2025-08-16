package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
)

// openAIClient implements Client against OpenAI's Chat Completions API.
type openAIClient struct {
	httpClient   *http.Client
	apiKey       string
	baseURL      string
	defaultModel string
	logger       *log.Logger
}

const (
    ansiBgBlack = "\x1b[40m"
    ansiGrey    = "\x1b[90m"
    ansiCyan    = "\x1b[36m"
    ansiGreen   = "\x1b[32m"
    ansiRed     = "\x1b[31m"
    ansiReset   = "\x1b[0m"
)

// Base style: black background + grey foreground
const ansiNormal = ansiBgBlack + ansiGrey

func newOpenAIFromEnv(apiKey string, logger *log.Logger) Client {
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &openAIClient{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		apiKey:       apiKey,
		baseURL:      base,
		defaultModel: model,
		logger:       logger,
	}
}

type oaChatRequest struct {
	Model       string      `json:"model"`
	Messages    []oaMessage `json:"messages"`
	Temperature *float64    `json:"temperature,omitempty"`
	MaxTokens   *int        `json:"max_tokens,omitempty"`
	Stop        []string    `json:"stop,omitempty"`
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

func (c *openAIClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
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
	c.logf("chat start model=%s temp=%.2f max_tokens=%d stop=%d messages=%d", o.Model, o.Temperature, o.MaxTokens, len(o.Stop), len(messages))
    for i, m := range messages {
        // Sending context (cyan) — log full content with real newlines
        c.logf("msg[%d] role=%s size=%d preview=%s%s%s", i, m.Role, len(m.Content), ansiCyan, m.Content, ansiNormal)
    }
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
	c.logf("POST %s", endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.logf("new request error: %v", err)
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        c.logf("%shttp error after %s: %v%s", ansiRed, time.Since(start), err, ansiNormal)
        return "", err
    }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr oaChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
        if apiErr.Error != nil && apiErr.Error.Message != "" {
            c.logf("%sapi error status=%d type=%s msg=%s duration=%s%s", ansiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), ansiNormal)
            return "", fmt.Errorf("openai error: %s (status %d)", apiErr.Error.Message, resp.StatusCode)
        }
        c.logf("%shttp non-2xx status=%d duration=%s%s", ansiRed, resp.StatusCode, time.Since(start), ansiNormal)
        return "", fmt.Errorf("openai http error: status %d", resp.StatusCode)
    }
	var out oaChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        c.logf("%sdecode error after %s: %v%s", ansiRed, time.Since(start), err, ansiNormal)
        return "", err
    }
    if len(out.Choices) == 0 {
        c.logf("%sno choices returned duration=%s%s", ansiRed, time.Since(start), ansiNormal)
        return "", errors.New("openai: no choices returned")
    }
    content := out.Choices[0].Message.Content
    // Received context (green) — log full content with real newlines
    c.logf("success choice=0 finish=%s size=%d preview=%s%s%s duration=%s", out.Choices[0].FinishReason, len(content), ansiGreen, content, ansiNormal, time.Since(start))
    return content, nil
}

// small helper to keep return type consistent
func nilStringErr(msg string) (string, error) { return "", errors.New(msg) }

func (c *openAIClient) logf(format string, args ...any) {
    if c.logger == nil {
        return
    }
    msg := fmt.Sprintf(format, args...)
    // Wrap each message with black background + grey base color.
    c.logger.Print(ansiNormal + "llm/openai " + msg + ansiReset)
}

func trimPreview(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
