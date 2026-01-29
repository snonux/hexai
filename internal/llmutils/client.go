package llmutils

import (
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

// NewClientFromApp builds an llm.Client using app config and environment keys.
func NewClientFromApp(cfg appconfig.App) (llm.Client, error) {
	llmCfg := llm.Config{
		Provider:              cfg.Provider,
		OpenAIBaseURL:         cfg.OpenAIBaseURL,
		OpenAIModel:           cfg.OpenAIModel,
		OpenAITemperature:     cfg.OpenAITemperature,
		OpenRouterBaseURL:     cfg.OpenRouterBaseURL,
		OpenRouterModel:       cfg.OpenRouterModel,
		OpenRouterTemperature: cfg.OpenRouterTemperature,
		OllamaBaseURL:         cfg.OllamaBaseURL,
		OllamaModel:           cfg.OllamaModel,
		OllamaTemperature:     cfg.OllamaTemperature,
		CopilotBaseURL:        cfg.CopilotBaseURL,
		CopilotModel:          cfg.CopilotModel,
		CopilotTemperature:    cfg.CopilotTemperature,
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
	cpKey := os.Getenv("HEXAI_COPILOT_API_KEY")
	if strings.TrimSpace(cpKey) == "" {
		cpKey = os.Getenv("COPILOT_API_KEY")
	}
	anKey := os.Getenv("HEXAI_ANTHROPIC_API_KEY")
	if strings.TrimSpace(anKey) == "" {
		anKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return llm.NewFromConfig(llmCfg, oaKey, orKey, cpKey, anKey)
}
