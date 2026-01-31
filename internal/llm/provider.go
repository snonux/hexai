// Summary: LLM provider interfaces, request options, configuration, and factory to build a client from config.
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

// Streamer is an optional interface that providers may implement to support
// token-by-token streaming responses. Callers can type-assert to Streamer and
// fall back to Client.Chat when not implemented.
type Streamer interface {
	// ChatStream sends chat messages and invokes onDelta with incremental text
	// chunks as they are produced by the model. Implementations should call
	// onDelta with empty strings sparingly (prefer only non-empty chunks).
	ChatStream(ctx context.Context, messages []Message, onDelta func(string), opts ...RequestOption) error
}

// CodeCompleter is an optional interface for providers that support a
// prompt/suffix code-completion API (e.g., Copilot Codex endpoint). Clients
// can type-assert to this and prefer it over chat when available.
type CodeCompleter interface {
	// CodeCompletion requests up to n suggestions given a left-hand prompt and
	// right-hand suffix around the cursor. Language is advisory and may be
	// ignored. Temperature applies when provider supports it.
	CodeCompletion(ctx context.Context, prompt string, suffix string, n int, language string, temperature float64) ([]string, error)
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
	Provider       string
	RequestTimeout int // seconds; 0 means use default (30s)
	// OpenAI options
	OpenAIBaseURL     string
	OpenAIModel       string
	OpenAITemperature *float64
	// OpenRouter options
	OpenRouterBaseURL     string
	OpenRouterModel       string
	OpenRouterTemperature *float64
	// Ollama options
	OllamaBaseURL     string
	OllamaModel       string
	OllamaTemperature *float64
	// Copilot options
	CopilotBaseURL     string
	CopilotModel       string
	CopilotTemperature *float64
	// Anthropic options
	AnthropicBaseURL     string
	AnthropicModel       string
	AnthropicTemperature *float64
}

// NewFromConfig creates an LLM client using only the supplied configuration.
// The OpenAI API key is supplied separately and may be read from the environment
// by the caller; other environment-based configuration is not used.
func NewFromConfig(cfg Config, openAIAPIKey, openRouterAPIKey, copilotAPIKey, anthropicAPIKey string) (Client, error) {
	p := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if p == "" {
		p = "openai"
	}
	switch p {
	case "openai":
		if strings.TrimSpace(openAIAPIKey) == "" {
			return nil, errors.New("missing OPENAI_API_KEY for provider openai")
		}
		// Default temperature selection:
		// - When model is gpt-5*, prefer 1.0 by default (more exploratory).
		// - Otherwise, prefer 0.2 by default (coding friendly).
		// The app-wide defaults currently set provider temps to 0.2.
		// If the user hasn't explicitly overridden and the model is gpt-5*,
		// upgrade 0.2 → 1.0 to satisfy the requested default for gpt-5.
		model := strings.ToLower(strings.TrimSpace(cfg.OpenAIModel))
		if strings.HasPrefix(model, "gpt-5") {
			if cfg.OpenAITemperature == nil {
				v := 1.0
				cfg.OpenAITemperature = &v
			} else if *cfg.OpenAITemperature == 0.2 {
				v := 1.0
				cfg.OpenAITemperature = &v
			}
		} else if cfg.OpenAITemperature == nil {
			v := 0.2
			cfg.OpenAITemperature = &v
		}
		return newOpenAIWithTimeout(cfg.OpenAIBaseURL, cfg.OpenAIModel, openAIAPIKey, cfg.OpenAITemperature, cfg.RequestTimeout), nil
	case "openrouter":
		if strings.TrimSpace(openRouterAPIKey) == "" {
			return nil, errors.New("missing OPENROUTER_API_KEY for provider openrouter")
		}
		if cfg.OpenRouterTemperature == nil {
			t := 0.2
			cfg.OpenRouterTemperature = &t
		}
		return newOpenRouterWithTimeout(cfg.OpenRouterBaseURL, cfg.OpenRouterModel, openRouterAPIKey, cfg.OpenRouterTemperature, cfg.RequestTimeout), nil
	case "ollama":
		if cfg.OllamaTemperature == nil {
			t := 0.2
			cfg.OllamaTemperature = &t
		}
		return newOllamaWithTimeout(cfg.OllamaBaseURL, cfg.OllamaModel, cfg.OllamaTemperature, cfg.RequestTimeout), nil
	case "copilot":
		if strings.TrimSpace(copilotAPIKey) == "" {
			return nil, errors.New("missing COPILOT_API_KEY for provider copilot")
		}
		if cfg.CopilotTemperature == nil {
			t := 0.2
			cfg.CopilotTemperature = &t
		}
		return newCopilotWithTimeout(cfg.CopilotBaseURL, cfg.CopilotModel, copilotAPIKey, cfg.CopilotTemperature, cfg.RequestTimeout), nil
	case "anthropic":
		if strings.TrimSpace(anthropicAPIKey) == "" {
			return nil, errors.New("missing ANTHROPIC_API_KEY for provider anthropic")
		}
		if cfg.AnthropicTemperature == nil {
			t := 0.2
			cfg.AnthropicTemperature = &t
		}
		return newAnthropicWithTimeout(cfg.AnthropicBaseURL, cfg.AnthropicModel, anthropicAPIKey, cfg.AnthropicTemperature, cfg.RequestTimeout), nil
	default:
		return nil, errors.New("unknown LLM provider: " + p)
	}
}
