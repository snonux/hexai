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
		Provider:           cfg.Provider,
		OpenAIBaseURL:      cfg.OpenAIBaseURL,
		OpenAIModel:        cfg.OpenAIModel,
		OpenAITemperature:  cfg.OpenAITemperature,
		OllamaBaseURL:      cfg.OllamaBaseURL,
		OllamaModel:        cfg.OllamaModel,
		OllamaTemperature:  cfg.OllamaTemperature,
		CopilotBaseURL:     cfg.CopilotBaseURL,
		CopilotModel:       cfg.CopilotModel,
		CopilotTemperature: cfg.CopilotTemperature,
	}
	oaKey := os.Getenv("HEXAI_OPENAI_API_KEY")
	if strings.TrimSpace(oaKey) == "" {
		oaKey = os.Getenv("OPENAI_API_KEY")
	}
	cpKey := os.Getenv("HEXAI_COPILOT_API_KEY")
	if strings.TrimSpace(cpKey) == "" {
		cpKey = os.Getenv("COPILOT_API_KEY")
	}
	return llm.NewFromConfig(llmCfg, oaKey, cpKey)
}
