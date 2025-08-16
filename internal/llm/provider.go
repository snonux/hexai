package llm

import (
    "context"
    "errors"
    "log"
    "os"
)

// Message represents a chat-style prompt message.
type Message struct {
	Role    string
	Content string
}

// Client is a minimal LLM provider interface.
// Future providers (Ollama, etc.) should implement this.
type Client interface {
    // Chat sends chat messages and returns the assistant text.
    Chat(ctx context.Context, messages []Message, opts ...RequestOption) (string, error)
}

// Options for a request. Providers may ignore unsupported fields.
type Options struct {
	Model       string
	Temperature float64
	MaxTokens   int
	Stop        []string
}

// RequestOption mutates Options.
type RequestOption func(*Options)

func WithModel(model string) RequestOption    { return func(o *Options) { o.Model = model } }
func WithTemperature(t float64) RequestOption { return func(o *Options) { o.Temperature = t } }
func WithMaxTokens(n int) RequestOption       { return func(o *Options) { o.MaxTokens = n } }
func WithStop(stop ...string) RequestOption {
	return func(o *Options) { o.Stop = append([]string{}, stop...) }
}

// NewDefault returns the default provider using environment configuration.
// Currently this is the OpenAI provider using OPENAI_API_KEY.
func NewDefault(logger *log.Logger) (Client, error) {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        return nil, errors.New("OPENAI_API_KEY is not set")
    }
    return newOpenAIFromEnv(apiKey, logger), nil
}

// Logging configuration for previews
var logPreviewLimit int // 0 means unlimited

// SetLogPreviewLimit sets the maximum number of characters to log for
// request/response previews. Set to 0 for unlimited.
func SetLogPreviewLimit(n int) { logPreviewLimit = n }

func previewForLog(s string) string {
    if logPreviewLimit > 0 {
        return trimPreview(s, logPreviewLimit)
    }
    return s
}
