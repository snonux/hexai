package llmutils

import (
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

// CanonicalProvider normalizes provider names and defaults to openai.
func CanonicalProvider(name string) string {
	provider := strings.ToLower(strings.TrimSpace(name))
	if provider == "" {
		return "openai"
	}
	return provider
}

// DefaultModelForProvider returns the configured default model for a provider.
func DefaultModelForProvider(cfg appconfig.App, provider string) string {
	switch CanonicalProvider(provider) {
	case "openrouter":
		return strings.TrimSpace(cfg.OpenRouterModel)
	case "ollama":
		return strings.TrimSpace(cfg.OllamaModel)
	case "anthropic":
		return strings.TrimSpace(cfg.AnthropicModel)
	default:
		return strings.TrimSpace(cfg.OpenAIModel)
	}
}

// ConfigForProvider returns cfg adjusted for the selected provider/model.
func ConfigForProvider(cfg appconfig.App, provider, modelOverride string) appconfig.App {
	derived := cfg
	if strings.TrimSpace(provider) == "" {
		provider = cfg.Provider
	}
	normalized := CanonicalProvider(provider)
	derived.Provider = normalized
	model := strings.TrimSpace(modelOverride)
	if model == "" {
		return derived
	}
	switch normalized {
	case "openrouter":
		derived.OpenRouterModel = model
	case "ollama":
		derived.OllamaModel = model
	case "anthropic":
		derived.AnthropicModel = model
	default:
		derived.OpenAIModel = model
	}
	return derived
}

// NewClientFromAppForProvider builds a client for a specific provider/model.
func NewClientFromAppForProvider(cfg appconfig.App, provider, modelOverride string) (llm.Client, error) {
	return NewClientFromApp(ConfigForProvider(cfg, provider, modelOverride))
}

// NewClientFromApp builds an llm.Client using app config and environment keys.
func NewClientFromApp(cfg appconfig.App) (llm.Client, error) {
	llmCfg := llm.Config{
		Provider:              cfg.Provider,
		RequestTimeout:        cfg.RequestTimeout,
		OpenAIBaseURL:         cfg.OpenAIBaseURL,
		OpenAIModel:           cfg.OpenAIModel,
		OpenAITemperature:     cfg.OpenAITemperature,
		OpenRouterBaseURL:     cfg.OpenRouterBaseURL,
		OpenRouterModel:       cfg.OpenRouterModel,
		OpenRouterTemperature: cfg.OpenRouterTemperature,
		OllamaBaseURL:         cfg.OllamaBaseURL,
		OllamaModel:           cfg.OllamaModel,
		OllamaTemperature:     cfg.OllamaTemperature,
		AnthropicBaseURL:      cfg.AnthropicBaseURL,
		AnthropicModel:        cfg.AnthropicModel,
		AnthropicTemperature:  cfg.AnthropicTemperature,
	}
	oaKey := os.Getenv("HEXAI_OPENAI_API_KEY")
	if strings.TrimSpace(oaKey) == "" {
		oaKey = os.Getenv("OPENAI_API_KEY")
	}
	orKey := os.Getenv("HEXAI_OPENROUTER_API_KEY")
	if strings.TrimSpace(orKey) == "" {
		orKey = os.Getenv("OPENROUTER_API_KEY")
	}
	anKey := os.Getenv("HEXAI_ANTHROPIC_API_KEY")
	if strings.TrimSpace(anKey) == "" {
		anKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return llm.NewFromConfig(llmCfg, oaKey, orKey, anKey)
}
