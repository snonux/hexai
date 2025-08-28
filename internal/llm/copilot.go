// Summary: GitHub Copilot client for chat and Codex-style code completion.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"encoding/base64"
	appver "hexai/internal"
	"hexai/internal/logging"
)

// copilotClient implements Client against GitHub Copilot's Chat Completions API.
type copilotClient struct {
	httpClient         *http.Client
	apiKey             string
	baseURL            string
	defaultModel       string
	chatLogger         logging.ChatLogger
	defaultTemperature *float64

	// cached Copilot session token retrieved from GitHub API using apiKey
	sessionToken string
	tokenExpiry  time.Time
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
func newCopilot(baseURL, model, apiKey string, defaultTemp *float64) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.githubcopilot.com"
	}
	if strings.TrimSpace(model) == "" {
		// GitHub Models (Copilot API) commonly supports gpt-4o/gpt-4o-mini.
		// Default to a broadly available, cost-effective option.
		model = "gpt-4o-mini"
	}
	return copilotClient{
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		apiKey:             apiKey,
		baseURL:            strings.TrimRight(baseURL, "/"),
		defaultModel:       model,
		chatLogger:         logging.NewChatLogger("copilot"),
		defaultTemperature: defaultTemp,
	}
}

func (c copilotClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nilStringErr("missing Copilot API key")
	}
	// Ensure we have a fresh session token
	if err := c.ensureSession(ctx); err != nil {
		return "", err
	}
	o := Options{Model: c.defaultModel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Model == "" {
		o.Model = c.defaultModel
	}
	start := time.Now()
	logMessages := make([]struct{ Role, Content string }, len(messages))
	for i, m := range messages {
		logMessages[i] = struct{ Role, Content string }{m.Role, m.Content}
	}
	c.chatLogger.LogStart(false, o.Model, o.Temperature, o.MaxTokens, o.Stop, logMessages)

	req := buildCopilotChatRequest(o, messages, c.defaultTemperature)
	body, err := json.Marshal(req)
	if err != nil {
		logging.Logf("llm/copilot ", "marshal error: %v", err)
		return "", err
	}

	endpoint := c.baseURL + "/chat/completions"
	logging.Logf("llm/copilot ", "POST %s", endpoint)
	resp, err := c.postJSON(ctx, endpoint, body, c.headersChat())
	if err != nil {
		logging.Logf("llm/copilot ", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer resp.Body.Close()
	if err := handleCopilotNon2xx(resp, start); err != nil {
		return "", err
	}
	out, err := decodeCopilotChat(resp, start)
	if err != nil {
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

// helpers
func buildCopilotChatRequest(o Options, messages []Message, defaultTemp *float64) copilotChatRequest {
	req := copilotChatRequest{Model: o.Model}
	req.Messages = make([]copilotMessage, len(messages))
	for i, m := range messages {
		req.Messages[i] = copilotMessage{Role: m.Role, Content: m.Content}
	}
	if o.Temperature != 0 {
		req.Temperature = &o.Temperature
	} else if defaultTemp != nil {
		t := *defaultTemp
		req.Temperature = &t
	}
	if o.MaxTokens > 0 {
		req.MaxTokens = &o.MaxTokens
	}
	if len(o.Stop) > 0 {
		req.Stop = o.Stop
	}
	return req
}

func (c copilotClient) postJSON(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil { return nil, err }
    for k, v := range headers { req.Header.Set(k, v) }
    return c.httpClient.Do(req)
}

func handleCopilotNon2xx(resp *http.Response, start time.Time) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var apiErr copilotChatResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Error != nil && strings.TrimSpace(apiErr.Error.Message) != "" {
		logging.Logf("llm/copilot ", "%sapi error status=%d type=%s msg=%s duration=%s%s", logging.AnsiRed, resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message, time.Since(start), logging.AnsiBase)
		return fmt.Errorf("copilot error: %s (status %d)", apiErr.Error.Message, resp.StatusCode)
	}
	logging.Logf("llm/copilot ", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
	return fmt.Errorf("copilot http error: status %d", resp.StatusCode)
}

func decodeCopilotChat(resp *http.Response, start time.Time) (copilotChatResponse, error) {
	var out copilotChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logging.Logf("llm/copilot ", "%sdecode error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return copilotChatResponse{}, err
	}
	return out, nil
}

// --- Copilot session token management ---

type ghCopilotTokenResp struct {
    Token string `json:"token"`
}

func (c *copilotClient) ensureSession(ctx context.Context) error {
    // If token valid for >60s, reuse
    if c.sessionToken != "" && time.Now().Add(60*time.Second).Before(c.tokenExpiry) {
        return nil
    }
    if strings.TrimSpace(c.apiKey) == "" {
        return errors.New("missing Copilot API key")
    }
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/copilot_internal/v2/token", nil)
    if err != nil { return err }
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("User-Agent", "hexai/"+appver.Version)
    resp, err := c.httpClient.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("copilot token http error: %d", resp.StatusCode)
    }
    var out ghCopilotTokenResp
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return err }
    if strings.TrimSpace(out.Token) == "" { return errors.New("empty copilot session token") }
    // Parse JWT exp
    exp := parseJWTExp(out.Token)
    if exp.IsZero() { exp = time.Now().Add(10 * time.Minute) }
    c.sessionToken = out.Token
    c.tokenExpiry = exp
    return nil
}

var jwtExpRe = regexp.MustCompile(`"exp"\s*:\s*([0-9]+)`) // fallback if we can't base64 decode

func parseJWTExp(token string) time.Time {
    parts := strings.Split(token, ".")
    if len(parts) < 2 { return time.Time{} }
    b, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        if m := jwtExpRe.FindStringSubmatch(token); len(m) == 2 {
            if n, err2 := parseInt64(m[1]); err2 == nil { return time.Unix(n, 0) }
        }
        return time.Time{}
    }
    var payload struct{ Exp int64 `json:"exp"` }
    _ = json.Unmarshal(b, &payload)
    if payload.Exp == 0 { return time.Time{} }
    return time.Unix(payload.Exp, 0)
}

func parseInt64(s string) (int64, error) { var n int64; _, err := fmt.Sscan(s, &n); return n, err }

// --- Copilot headers ---

func (c *copilotClient) headersChat() map[string]string {
    _ = c.ensureSession(context.Background())
    h := map[string]string{
        "Content-Type": "application/json; charset=utf-8",
        "Accept":       "application/json",
        "Authorization": "Bearer " + c.sessionToken,
        "User-Agent":  "GitHubCopilotChat/0.8.0",
        "Editor-Plugin-Version": "copilot-chat/0.8.0",
        "Editor-Version":        "vscode/1.85.1",
        "Openai-Intent":         "conversation-panel",
        "Openai-Organization":   "github-copilot",
        "VScode-MachineId":      randHex(64),
        "VScode-SessionId":      randHex(8) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(12),
        "X-Request-Id":          randHex(8) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(12),
    }
    return h
}

func (c *copilotClient) headersGhost() map[string]string {
    _ = c.ensureSession(context.Background())
    h := map[string]string{
        "Content-Type": "application/json; charset=utf-8",
        "Accept":       "*/*",
        "Authorization": "Bearer " + c.sessionToken,
        "User-Agent":  "GithubCopilot/1.155.0",
        "Editor-Plugin-Version": "copilot/1.155.0",
        "Editor-Version":        "vscode/1.85.1",
        "Openai-Intent":         "copilot-ghost",
        "Openai-Organization":   "github-copilot",
        "VScode-MachineId":      randHex(64),
        "VScode-SessionId":      randHex(8) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(12),
        "X-Request-Id":          randHex(8) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(4) + "-" + randHex(12),
    }
    return h
}

func randHex(n int) string {
    const hex = "0123456789abcdef"
    b := make([]byte, n)
    for i := range b {
        b[i] = hex[int(time.Now().UnixNano()+int64(i))%len(hex)]
    }
    return string(b)
}

// --- Codex-style code completion ---

// CodeCompletion implements CodeCompleter; returns up to n suggestions.
func (c copilotClient) CodeCompletion(ctx context.Context, prompt string, suffix string, n int, language string, temperature float64) ([]string, error) {
    if strings.TrimSpace(c.apiKey) == "" { return nil, errors.New("missing Copilot API key") }
    if err := c.ensureSession(ctx); err != nil { return nil, err }
    if n <= 0 { n = 1 }
    maxTokens := 500
    body := map[string]any{
        "extra": map[string]any{
            "language": language,
            "next_indent": 0,
            "prompt_tokens": 500,
            "suffix_tokens": 400,
            "trim_by_indentation": true,
        },
        "max_tokens": maxTokens,
        "n": n,
        "nwo": "hexai",
        "prompt": prompt,
        "stop": []string{"\n\n"},
        "stream": true,
        "suffix": suffix,
        "temperature": temperature,
        "top_p": 1,
    }
    buf, _ := json.Marshal(body)
    url := "https://copilot-proxy.githubusercontent.com/v1/engines/copilot-codex/completions"
    resp, err := c.postJSON(ctx, url, buf, c.headersGhost())
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("copilot codex http error: %d", resp.StatusCode)
    }
    // Read all and parse lines that start with "data: " accumulating by index
    raw, _ := io.ReadAll(resp.Body)
    byIndex := make(map[int]string)
    lines := strings.Split(string(raw), "\n")
    for _, ln := range lines {
        if !strings.HasPrefix(ln, "data: ") { continue }
        var evt struct{ Choices []struct{ Index int `json:"index"`; Text string `json:"text"` } `json:"choices"` }
        if err := json.Unmarshal([]byte(strings.TrimPrefix(ln, "data: ")), &evt); err != nil { continue }
        for _, ch := range evt.Choices { byIndex[ch.Index] += ch.Text }
    }
    out := make([]string, 0, len(byIndex))
    for i := 0; i < n; i++ {
        if s, ok := byIndex[i]; ok && strings.TrimSpace(s) != "" { out = append(out, s) }
    }
    return out, nil
}

// newLineDataReader wraps a streaming body and exposes a JSON decoder that
// decodes successive objects from lines prefixed by "data: ".
// (no streaming decoder needed; we parse whole body lines)
