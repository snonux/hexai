// OpenRouter client implementation leveraging OpenAI-compatible helpers with provider-specific headers.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/logging"
)

type openRouterClient struct {
	httpClient         *http.Client
	apiKey             string
	baseURL            string
	defaultModel       string
	chatLogger         logging.ChatLogger
	defaultTemperature *float64
}

func init() {
	RegisterProvider("openrouter", openRouterProviderFactory)
}

func openRouterProviderFactory(cfg Config, keys ProviderKeys) (Client, error) {
	if strings.TrimSpace(keys.OpenRouterAPIKey) == "" {
		return nil, errors.New("missing OPENROUTER_API_KEY for provider openrouter")
	}
	return newOpenRouterWithTimeout(
		cfg.OpenRouterBaseURL,
		cfg.OpenRouterModel,
		keys.OpenRouterAPIKey,
		withDefaultTemperature(cfg.OpenRouterTemperature, 0.2),
		cfg.RequestTimeout,
	), nil
}

func newOpenRouter(baseURL, model, apiKey string, defaultTemp *float64) Client {
	return newOpenRouterWithTimeout(baseURL, model, apiKey, defaultTemp, 0)
}

func newOpenRouterWithTimeout(baseURL, model, apiKey string, defaultTemp *float64, timeoutSec int) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "openrouter/auto"
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return openRouterClient{
		httpClient:         &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		apiKey:             apiKey,
		baseURL:            baseURL,
		defaultModel:       model,
		chatLogger:         logging.NewChatLogger("openrouter"),
		defaultTemperature: defaultTemp,
	}
}

func (c openRouterClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nilStringErr("missing OpenRouter API key")
	}
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if strings.TrimSpace(o.Model) == "" {
		o.Model = c.defaultModel
	}
	start := time.Now()
	c.logStart(false, o, messages)
	req := buildOAChatRequest(o, messages, c.defaultTemperature, false, "llm/openrouter ")
	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return "", err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	logging.Logf("llm/openrouter ", "POST %s", endpoint)
	resp, err := c.doJSON(ctx, endpoint, body)
	if err != nil {
		logging.Logf("llm/openrouter ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Logf("llm/openrouter", "failed to close response body: %v", err)
		}
	}()
	if err := handleOpenAINon2xx(resp, start, "llm/openrouter ", "openrouter"); err != nil {
		return "", err
	}
	out, err := decodeOpenAIChat(resp, start, "llm/openrouter ")
	if err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		logging.Logf("llm/openrouter ", "%sno choices returned duration=%s%s", logging.AnsiRed, time.Since(start), logging.AnsiBase)
		return "", errors.New("openrouter: no choices returned")
	}
	content := out.Choices[0].Message.Content
	logging.Logf("llm/openrouter ", "success choice=0 finish=%s size=%d preview=%s%s%s duration=%s", out.Choices[0].FinishReason, len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

func (c openRouterClient) Name() string         { return "openrouter" }
func (c openRouterClient) DefaultModel() string { return c.defaultModel }

func (c openRouterClient) ChatStream(ctx context.Context, messages []Message, onDelta func(string), opts ...RequestOption) error {
	if strings.TrimSpace(c.apiKey) == "" {
		return errors.New("missing OpenRouter API key")
	}
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if strings.TrimSpace(o.Model) == "" {
		o.Model = c.defaultModel
	}
	start := time.Now()
	c.logStart(true, o, messages)
	req := buildOAChatRequest(o, messages, c.defaultTemperature, true, "llm/openrouter ")
	body, err := json.Marshal(req)
	if err != nil {
		c.logf("marshal error: %v", err)
		return err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	logging.Logf("llm/openrouter ", "POST %s (stream)", endpoint)
	resp, err := c.doJSONWithAccept(ctx, endpoint, body, "text/event-stream")
	if err != nil {
		logging.Logf("llm/openrouter ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Logf("llm/openrouter", "failed to close response body: %v", err)
		}
	}()
	if err := handleOpenAINon2xx(resp, start, "llm/openrouter ", "openrouter"); err != nil {
		return err
	}
	if err := parseOpenAIStream(resp, start, onDelta, "llm/openrouter ", "openrouter"); err != nil {
		return err
	}
	logging.Logf("llm/openrouter ", "stream end duration=%s", time.Since(start))
	return nil
}

func (c openRouterClient) logf(format string, args ...any) {
	logging.Logf("llm/openrouter ", format, args...)
}

func (c openRouterClient) logStart(stream bool, o Options, messages []Message) {
	logStartMessages(c.chatLogger, stream, o, messages)
}

func (c openRouterClient) doJSON(ctx context.Context, url string, body []byte) (*http.Response, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
		"HTTP-Referer":  "https://github.com/snonux/hexai",
		"X-Title":       "Hexai",
	}
	return c.doJSONWithHeaders(ctx, url, body, headers, "")
}

func (c openRouterClient) doJSONWithAccept(ctx context.Context, url string, body []byte, accept string) (*http.Response, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
		"HTTP-Referer":  "https://github.com/snonux/hexai",
		"X-Title":       "Hexai",
	}
	return c.doJSONWithHeaders(ctx, url, body, headers, accept)
}

func (c openRouterClient) doJSONWithHeaders(ctx context.Context, url string, body []byte, headers map[string]string, accept string) (*http.Response, error) {
	return doJSONRequest(ctx, c.httpClient, url, body, headers, accept)
}
