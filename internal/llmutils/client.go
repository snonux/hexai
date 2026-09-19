package llmutils

import (
	"os"
	"strings"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
)

// CanonicalProvider normalizes provider names and defaults to ollama (Ollama
// Cloud at https://ollama.com when paired with the default base URL).
func CanonicalProvider(name string) string {
	provider := strings.ToLower(strings.TrimSpace(name))
	if provider == "" {
		return "ollama"
	}
	return provider
}

// ProviderProfileFor resolves a named profile. Built-in provider names are
// implicit profiles backed by the legacy provider sections.
func ProviderProfileFor(cfg appconfig.App, name string) (appconfig.ProviderProfile, bool) {
	key := strings.TrimSpace(name)
	if key == "" {
		key = cfg.Provider
	}
	if profile, ok := cfg.ProviderProfiles[key]; ok {
		return profile, true
	}
	typeName := CanonicalProvider(key)
	profile := appconfig.ProviderProfile{Type: typeName}
	switch typeName {
	case "ollama":
		profile.BaseURL, profile.Model, profile.Temperature = cfg.OllamaBaseURL, cfg.OllamaModel, cfg.OllamaTemperature
	case "openrouter":
		profile.BaseURL, profile.Model, profile.Temperature = cfg.OpenRouterBaseURL, cfg.OpenRouterModel, cfg.OpenRouterTemperature
	case "anthropic":
		profile.BaseURL, profile.Model, profile.Temperature = cfg.AnthropicBaseURL, cfg.AnthropicModel, cfg.AnthropicTemperature
	case "openai":
		profile.BaseURL, profile.Model, profile.Temperature = cfg.OpenAIBaseURL, cfg.OpenAIModel, cfg.OpenAITemperature
	default:
		return appconfig.ProviderProfile{}, false
	}
	return profile, true
}

// DefaultModelForProvider returns the configured default model for a provider.
func DefaultModelForProvider(cfg appconfig.App, provider string) string {
	if profile, ok := ProviderProfileFor(cfg, provider); ok && strings.TrimSpace(profile.Model) != "" {
		return strings.TrimSpace(profile.Model)
	}
	switch CanonicalProvider(provider) {
	case "openrouter":
		if model := strings.TrimSpace(cfg.OpenRouterModel); model != "" {
			return model
		}
		return "openrouter/auto"
	case "ollama":
		if model := strings.TrimSpace(cfg.OllamaModel); model != "" {
			return model
		}
		return appconfig.DefaultOllamaModel
	case "anthropic":
		if model := strings.TrimSpace(cfg.AnthropicModel); model != "" {
			return model
		}
		return "claude-3-5-sonnet-20240620"
	default:
		if model := strings.TrimSpace(cfg.OpenAIModel); model != "" {
			return model
		}
		return "gpt-4.1"
	}
}

// ConfigForProvider returns cfg adjusted for the selected provider/model.
func ConfigForProvider(cfg appconfig.App, provider, modelOverride string) appconfig.App {
	derived := cfg
	if strings.TrimSpace(provider) == "" {
		provider = cfg.Provider
	}
	profile, ok := ProviderProfileFor(cfg, provider)
	normalized := CanonicalProvider(provider)
	if ok {
		normalized = CanonicalProvider(profile.Type)
		if strings.TrimSpace(profile.BaseURL) != "" {
			setProviderBaseURL(&derived, normalized, profile.BaseURL)
		}
		if strings.TrimSpace(profile.Model) != "" {
			setProviderModel(&derived, normalized, profile.Model)
		}
		if profile.Temperature != nil {
			setProviderTemperature(&derived, normalized, profile.Temperature)
		}
	}
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

func setProviderBaseURL(cfg *appconfig.App, provider, value string) {
	switch provider {
	case "ollama":
		cfg.OllamaBaseURL = value
	case "openrouter":
		cfg.OpenRouterBaseURL = value
	case "anthropic":
		cfg.AnthropicBaseURL = value
	case "openai":
		cfg.OpenAIBaseURL = value
	}
}

func setProviderModel(cfg *appconfig.App, provider, value string) {
	switch provider {
	case "ollama":
		cfg.OllamaModel = value
	case "openrouter":
		cfg.OpenRouterModel = value
	case "anthropic":
		cfg.AnthropicModel = value
	case "openai":
		cfg.OpenAIModel = value
	}
}

func setProviderTemperature(cfg *appconfig.App, provider string, value *float64) {
	switch provider {
	case "ollama":
		cfg.OllamaTemperature = value
	case "openrouter":
		cfg.OpenRouterTemperature = value
	case "anthropic":
		cfg.AnthropicTemperature = value
	case "openai":
		cfg.OpenAITemperature = value
	}
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
	// Ollama API key is optional: only needed for Ollama Cloud (ollama.ai).
	// A local Ollama server keeps working when this is empty.
	olKey := os.Getenv("HEXAI_OLLAMA_API_KEY")
	if strings.TrimSpace(olKey) == "" {
		olKey = os.Getenv("OLLAMA_API_KEY")
	}
	return llm.NewFromConfig(llmCfg, oaKey, orKey, anKey, olKey)
}
