package llm

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"codeberg.org/snonux/hexai/internal/logging"
)

// small helper to keep return type consistent
func nilStringErr(msg string) (string, error) { return "", errors.New(msg) }

func doJSONRequest(ctx context.Context, httpClient *http.Client, url string, body []byte, headers map[string]string, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return httpClient.Do(req)
}

func logStartMessages(chatLogger logging.ChatLogger, stream bool, o Options, messages []Message) {
	logMessages := make([]struct{ Role, Content string }, len(messages))
	for i, m := range messages {
		logMessages[i] = struct{ Role, Content string }{m.Role, m.Content}
	}
	chatLogger.LogStart(stream, o.Model, o.Temperature, o.MaxTokens, o.Stop, logMessages)
}
