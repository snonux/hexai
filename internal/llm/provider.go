package llm

import (
	"context"
	"errors"
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

// Config defines provider configuration read from the Hexai config file.
type Config struct {
    Provider string
    // OpenAI options
    OpenAIBaseURL string
    OpenAIModel   string
    // Ollama options
    OllamaBaseURL string
    OllamaModel   string
    // Copilot options
    CopilotBaseURL string
    CopilotModel   string
}

// NewFromConfig creates an LLM client using only the supplied configuration.
// The OpenAI API key is supplied separately and may be read from the environment
// by the caller; other environment-based configuration is not used.
func NewFromConfig(cfg Config, openAIAPIKey, copilotAPIKey string) (Client, error) {
    p := strings.ToLower(strings.TrimSpace(cfg.Provider))
    if p == "" {
        p = "openai"
    }
    switch p {
    case "openai":
        if strings.TrimSpace(openAIAPIKey) == "" {
            return nil, errors.New("missing OPENAI_API_KEY for provider openai")
        }
        return newOpenAI(cfg.OpenAIBaseURL, cfg.OpenAIModel, openAIAPIKey), nil
    case "ollama":
        return newOllama(cfg.OllamaBaseURL, cfg.OllamaModel), nil
    case "copilot":
        if strings.TrimSpace(copilotAPIKey) == "" {
            return nil, errors.New("missing COPILOT_API_KEY for provider copilot")
        }
        return newCopilot(cfg.CopilotBaseURL, cfg.CopilotModel, copilotAPIKey), nil
    default:
        return nil, errors.New("unknown LLM provider: " + p)
    }
}
