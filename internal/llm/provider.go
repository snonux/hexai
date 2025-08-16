package llm

import (
    "context"
    "errors"
    "os"
    "strings"
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
    // Name returns the provider's short name (e.g., "openai", "ollama").
    Name() string
    // DefaultModel returns the configured default model name.
    DefaultModel() string
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
// Selection order:
// 1) HEXAI_LLM_PROVIDER=openai|ollama
// 2) If OPENAI_API_KEY is set -> OpenAI
// 3) If any OLLAMA_* vars are set -> Ollama
func NewDefault() (Client, error) {
    // Explicit provider selection
    if p := strings.ToLower(strings.TrimSpace(os.Getenv("HEXAI_LLM_PROVIDER"))); p != "" {
        switch p {
        case "openai":
            apiKey := os.Getenv("OPENAI_API_KEY")
            if apiKey == "" {
                return nil, errors.New("OPENAI_API_KEY is not set")
            }
            return newOpenAIFromEnv(apiKey), nil
        case "ollama":
            return newOllamaFromEnv(), nil
        default:
            return nil, errors.New("unknown HEXAI_LLM_PROVIDER: " + p)
        }
    }

    // Auto-detect
    if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
        return newOpenAIFromEnv(apiKey), nil
    }
    if os.Getenv("OLLAMA_BASE_URL") != "" || os.Getenv("OLLAMA_HOST") != "" || os.Getenv("OLLAMA_MODEL") != "" {
        return newOllamaFromEnv(), nil
    }
    return nil, errors.New("no LLM provider configured (set OPENAI_API_KEY or HEXAI_LLM_PROVIDER/OLLAMA_*)")
}
