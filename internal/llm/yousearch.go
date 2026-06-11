// You.com Research API provider. Maps Chat() to a single research request using
// the last user message as the query. System messages are ignored — the Research
// API has its own reasoning pipeline. Sources are appended as a markdown section.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm/policy"
	"codeberg.org/snonux/hexai/internal/logging"
)

const youSearchResearchURL = "https://api.you.com/v1/research"

type youSearchClient struct {
	httpClient     *http.Client
	apiKey         string
	baseURL        string // research endpoint URL (overridable for tests)
	researchEffort string // lite|standard|deep|exhaustive
	chatLogger     logging.ChatLogger
}

var _ Client = youSearchClient{}

type youSearchRequest struct {
	Input          string `json:"input"`
	ResearchEffort string `json:"research_effort,omitempty"`
}

type youSearchSource struct {
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	Snippets []string `json:"snippets"`
}

type youSearchResponse struct {
	Output struct {
		Content     interface{}       `json:"content"`
		ContentType string            `json:"content_type"`
		Sources     []youSearchSource `json:"sources"`
	} `json:"output"`
}

func youSearchProviderFactory(cfg Config, keys ProviderKeys) (Client, error) {
	if strings.TrimSpace(keys.YouSearchAPIKey) == "" {
		return nil, missingAPIKeyError("yousearch", "HEXAI_YOUSEARCH_API_KEY", "YOU_API_KEY")
	}
	timeoutSec := cfg.RequestTimeout
	if timeoutSec <= 0 {
		// The Research API runs a long multi-step pipeline, so it uses the
		// larger research timeout from the policy package rather than the
		// default chat timeout.
		timeoutSec = policy.ResearchRequestTimeoutSeconds
	}
	return youSearchClient{
		httpClient:     &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		apiKey:         strings.TrimSpace(keys.YouSearchAPIKey),
		baseURL:        youSearchResearchURL,
		researchEffort: strings.TrimSpace(cfg.YouSearchResearchEffort),
		chatLogger:     logging.NewChatLogger("yousearch"),
	}, nil
}

func (c youSearchClient) endpoint() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return youSearchResearchURL
}

func (c youSearchClient) Name() string         { return "yousearch" }
func (c youSearchClient) DefaultModel() string { return c.effectiveEffort() }

func (c youSearchClient) effectiveEffort() string {
	if c.researchEffort != "" {
		return c.researchEffort
	}
	return "standard"
}

// Chat extracts the last user message and sends it as a research query.
func (c youSearchClient) Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error) {
	query := lastUserMessage(messages)
	if query == "" {
		return "", fmt.Errorf("yousearch: no user message found in conversation")
	}

	start := time.Now()
	logStartMessages(c.chatLogger, false, Options{Model: c.effectiveEffort()}, messages)

	payload, err := json.Marshal(youSearchRequest{
		Input:          query,
		ResearchEffort: c.effectiveEffort(),
	})
	if err != nil {
		return "", err
	}

	url := c.endpoint()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	logging.Logf("llm/yousearch", "POST %s effort=%s", url, c.effectiveEffort())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Logf("llm/yousearch", "%shttp error after %s: %v%s", logging.AnsiRed, time.Since(start), err, logging.AnsiBase)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logging.Logf("llm/yousearch", "failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logging.Logf("llm/yousearch", "%shttp non-2xx status=%d duration=%s%s", logging.AnsiRed, resp.StatusCode, time.Since(start), logging.AnsiBase)
		return "", fmt.Errorf("yousearch: API error status %d", resp.StatusCode)
	}

	var result youSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("yousearch: decoding response: %w", err)
	}

	content := formatYouSearchContent(result)
	if content == "" {
		return "", fmt.Errorf("yousearch: empty response")
	}

	logging.Logf("llm/yousearch", "success size=%d preview=%s%s%s duration=%s",
		len(content), logging.AnsiGreen, logging.PreviewForLog(content), logging.AnsiBase, time.Since(start))
	return content, nil
}

func formatYouSearchContent(result youSearchResponse) string {
	var sb strings.Builder

	switch v := result.Output.Content.(type) {
	case string:
		sb.WriteString(strings.TrimSpace(v))
	default:
		out, _ := json.MarshalIndent(v, "", "  ")
		sb.Write(out)
	}

	if len(result.Output.Sources) > 0 {
		sb.WriteString("\n\n**Sources:**\n")
		for i, s := range result.Output.Sources {
			title := s.Title
			if title == "" {
				title = s.URL
			}
			sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, title, s.URL))
		}
	}

	return sb.String()
}

func lastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.ToLower(messages[i].Role) == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}
