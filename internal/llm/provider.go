// Summary: LLM provider interfaces, request options, configuration, and factory to build a client from config.
package llm

import (
	"context"
	"errors"
	"strings"
	"sync"
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
	// Anthropic options
	AnthropicBaseURL     string
	AnthropicModel       string
	AnthropicTemperature *float64
}

// ProviderKeys contains API credentials used by provider factories.
type ProviderKeys struct {
	OpenAIAPIKey     string
	OpenRouterAPIKey string
	AnthropicAPIKey  string
}

// ProviderFactory builds an LLM client for a named provider.
type ProviderFactory func(cfg Config, keys ProviderKeys) (Client, error)

var (
	providerRegistryMu sync.RWMutex
	providerRegistry   = map[string]ProviderFactory{}
)

// RegisterProvider registers a provider factory by normalized name.
func RegisterProvider(name string, factory ProviderFactory) {
	normalized := normalizeProvider(name)
	if normalized == "" {
		panic("llm: provider name cannot be empty")
	}
	if factory == nil {
		panic("llm: provider factory cannot be nil")
	}
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	if _, exists := providerRegistry[normalized]; exists {
		panic("llm: provider already registered: " + normalized)
	}
	providerRegistry[normalized] = factory
}

// NewFromConfig creates an LLM client using only the supplied configuration.
// The OpenAI API key is supplied separately and may be read from the environment
// by the caller; other environment-based configuration is not used.
func NewFromConfig(cfg Config, openAIAPIKey, openRouterAPIKey, anthropicAPIKey string) (Client, error) {
	provider := normalizeProvider(cfg.Provider)
	if provider == "" {
		provider = "openai"
	}

	factory, ok := lookupProviderFactory(provider)
	if !ok {
		return nil, errors.New("unknown LLM provider: " + provider)
	}

	return factory(cfg, ProviderKeys{
		OpenAIAPIKey:     openAIAPIKey,
		OpenRouterAPIKey: openRouterAPIKey,
		AnthropicAPIKey:  anthropicAPIKey,
	})
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func lookupProviderFactory(provider string) (ProviderFactory, bool) {
	providerRegistryMu.RLock()
	defer providerRegistryMu.RUnlock()
	factory, ok := providerRegistry[provider]
	return factory, ok
}

func withDefaultTemperature(configured *float64, fallback float64) *float64 {
	if configured != nil {
		return configured
	}
	v := fallback
	return &v
}
