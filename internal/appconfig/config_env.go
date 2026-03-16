package appconfig

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// --- Environment overrides ---

// loadFromEnv constructs an App containing only fields set via HEXAI_* env vars.
// These values should take precedence over file config when merged.
func loadFromEnv(logger *log.Logger) *App {
	var out App
	any := applyCoreEnv(&out, logger)
	any = applyProviderEnv(&out, logger) || any
	any = applySurfaceEnv(&out, logger) || any
	any = applyIgnoreEnv(&out) || any
	any = applyMCPEnv(&out) || any
	if !any {
		return nil
	}
	return &out
}

func applyCoreEnv(out *App, logger *log.Logger) bool {
	any := false
	any = applyEnvInt(&out.MaxTokens, "HEXAI_MAX_TOKENS", logger) || any
	any = applyEnvString(&out.ContextMode, "HEXAI_CONTEXT_MODE") || any
	any = applyEnvInt(&out.ContextWindowLines, "HEXAI_CONTEXT_WINDOW_LINES", logger) || any
	any = applyEnvInt(&out.MaxContextTokens, "HEXAI_MAX_CONTEXT_TOKENS", logger) || any
	any = applyEnvInt(&out.LogPreviewLimit, "HEXAI_LOG_PREVIEW_LIMIT", logger) || any
	any = applyEnvInt(&out.RequestTimeout, "HEXAI_REQUEST_TIMEOUT", logger) || any
	any = applyEnvInt(&out.ManualInvokeMinPrefix, "HEXAI_MANUAL_INVOKE_MIN_PREFIX", logger) || any
	any = applyEnvInt(&out.CompletionDebounceMs, "HEXAI_COMPLETION_DEBOUNCE_MS", logger) || any
	any = applyEnvInt(&out.CompletionThrottleMs, "HEXAI_COMPLETION_THROTTLE_MS", logger) || any
	any = applyEnvFloat(&out.CodingTemperature, "HEXAI_CODING_TEMPERATURE", logger) || any
	any = applyEnvCSV(&out.TriggerCharacters, "HEXAI_TRIGGER_CHARACTERS") || any
	any = applyEnvString(&out.InlineOpen, "HEXAI_INLINE_OPEN") || any
	any = applyEnvString(&out.InlineClose, "HEXAI_INLINE_CLOSE") || any
	any = applyEnvString(&out.ChatSuffix, "HEXAI_CHAT_SUFFIX") || any
	any = applyEnvCSV(&out.ChatPrefixes, "HEXAI_CHAT_PREFIXES") || any
	any = applyEnvString(&out.Provider, "HEXAI_PROVIDER") || any
	return any
}

func applyProviderEnv(out *App, logger *log.Logger) bool {
	picker := newModelPicker(out.Provider)
	any := false
	any = applyEnvString(&out.OpenAIBaseURL, "HEXAI_OPENAI_BASE_URL") || any
	if model, ok := picker.pick("openai", getenvTrim("HEXAI_OPENAI_MODEL")); ok {
		out.OpenAIModel = model
		any = true
	}
	any = applyEnvFloat(&out.OpenAITemperature, "HEXAI_OPENAI_TEMPERATURE", logger) || any

	any = applyEnvString(&out.OpenRouterBaseURL, "HEXAI_OPENROUTER_BASE_URL") || any
	if model, ok := picker.pick("openrouter", getenvTrim("HEXAI_OPENROUTER_MODEL")); ok {
		out.OpenRouterModel = model
		any = true
	}
	any = applyEnvFloat(&out.OpenRouterTemperature, "HEXAI_OPENROUTER_TEMPERATURE", logger) || any

	any = applyEnvString(&out.OllamaBaseURL, "HEXAI_OLLAMA_BASE_URL") || any
	if model, ok := picker.pick("ollama", getenvTrim("HEXAI_OLLAMA_MODEL")); ok {
		out.OllamaModel = model
		any = true
	}
	any = applyEnvFloat(&out.OllamaTemperature, "HEXAI_OLLAMA_TEMPERATURE", logger) || any

	any = applyEnvString(&out.AnthropicBaseURL, "HEXAI_ANTHROPIC_BASE_URL") || any
	if model, ok := picker.pick("anthropic", getenvTrim("HEXAI_ANTHROPIC_MODEL")); ok {
		out.AnthropicModel = model
		any = true
	}
	any = applyEnvFloat(&out.AnthropicTemperature, "HEXAI_ANTHROPIC_TEMPERATURE", logger) || any
	return any
}

func applySurfaceEnv(out *App, logger *log.Logger) bool {
	any := false
	if entries, ok := buildSurfaceEntryFromEnv("HEXAI_MODEL_COMPLETION", "HEXAI_TEMPERATURE_COMPLETION", "HEXAI_PROVIDER_COMPLETION", logger); ok {
		out.CompletionConfigs = entries
		any = true
	}
	if entries, ok := buildSurfaceEntryFromEnv("HEXAI_MODEL_CODE_ACTION", "HEXAI_TEMPERATURE_CODE_ACTION", "HEXAI_PROVIDER_CODE_ACTION", logger); ok {
		out.CodeActionConfigs = entries
		any = true
	}
	if entries, ok := buildSurfaceEntryFromEnv("HEXAI_MODEL_CHAT", "HEXAI_TEMPERATURE_CHAT", "HEXAI_PROVIDER_CHAT", logger); ok {
		out.ChatConfigs = entries
		any = true
	}
	if entries, ok := buildSurfaceEntryFromEnv("HEXAI_MODEL_CLI", "HEXAI_TEMPERATURE_CLI", "HEXAI_PROVIDER_CLI", logger); ok {
		out.CLIConfigs = entries
		any = true
	}
	return any
}

func applyIgnoreEnv(out *App) bool {
	any := false
	any = applyEnvBoolPtr(&out.IgnoreGitignore, "HEXAI_IGNORE_GITIGNORE") || any
	any = applyEnvCSV(&out.IgnoreExtraPatterns, "HEXAI_IGNORE_EXTRA_PATTERNS") || any
	any = applyEnvBoolPtr(&out.IgnoreLSPNotify, "HEXAI_IGNORE_LSP_NOTIFY") || any
	return any
}

func applyMCPEnv(out *App) bool {
	any := false
	any = applyEnvString(&out.MCPPromptsDir, "HEXAI_MCP_PROMPTS_DIR") || any
	any = applyEnvBool(&out.MCPSlashCommandSync, "HEXAI_MCP_SLASHCOMMAND_SYNC") || any
	any = applyEnvString(&out.MCPSlashCommandDir, "HEXAI_MCP_SLASHCOMMAND_DIR") || any
	return any
}

func buildSurfaceEntryFromEnv(modelKey, tempKey, providerKey string, logger *log.Logger) ([]SurfaceConfig, bool) {
	model := getenvTrim(modelKey)
	tempPtr, tempSet := parseEnvFloatPtr(tempKey, logger)
	provider := getenvTrim(providerKey)
	if model == "" && provider == "" && !tempSet {
		return nil, false
	}
	entry := SurfaceConfig{Provider: provider, Model: model}
	if tempSet {
		entry.Temperature = tempPtr
	}
	return []SurfaceConfig{entry}, true
}

func applyEnvString(target *string, key string) bool {
	value := getenvTrim(key)
	if value == "" {
		return false
	}
	*target = value
	return true
}

func applyEnvInt(target *int, key string, logger *log.Logger) bool {
	value, ok := parseEnvInt(key, logger)
	if !ok {
		return false
	}
	*target = value
	return true
}

func applyEnvFloat(target **float64, key string, logger *log.Logger) bool {
	value, ok := parseEnvFloatPtr(key, logger)
	if !ok {
		return false
	}
	*target = value
	return true
}

func applyEnvCSV(target *[]string, key string) bool {
	value := getenvTrim(key)
	if value == "" {
		return false
	}
	parts := strings.Split(value, ",")
	*target = nil
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			*target = append(*target, t)
		}
	}
	return true
}

func applyEnvBool(target *bool, key string) bool {
	value := getenvTrim(key)
	if value == "" {
		return false
	}
	*target = value == "true" || value == "1"
	return true
}

func applyEnvBoolPtr(target **bool, key string) bool {
	value := getenvTrim(key)
	if value == "" {
		return false
	}
	parsed := value == "true" || value == "1"
	*target = &parsed
	return true
}

func getenvTrim(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func parseEnvInt(key string, logger *log.Logger) (int, bool) {
	value := getenvTrim(key)
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		if logger != nil {
			logger.Printf("invalid %s: %v", key, err)
		}
		return 0, false
	}
	return n, true
}

func parseEnvFloatPtr(key string, logger *log.Logger) (*float64, bool) {
	value := getenvTrim(key)
	if value == "" {
		return nil, false
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		if logger != nil {
			logger.Printf("invalid %s: %v", key, err)
		}
		return nil, false
	}
	return &f, true
}

type modelPicker struct {
	providerLower string
	modelForce    string
	modelGeneric  string
	forceUsed     bool
	genericUsed   bool
}

func newModelPicker(provider string) *modelPicker {
	return &modelPicker{
		providerLower: strings.ToLower(strings.TrimSpace(provider)),
		modelForce:    getenvTrim("HEXAI_MODEL_FORCE"),
		modelGeneric:  getenvTrim("HEXAI_MODEL"),
	}
}

func (p *modelPicker) pick(providerName, specific string) (string, bool) {
	specific = strings.TrimSpace(specific)
	nameLower := strings.ToLower(strings.TrimSpace(providerName))
	if p.modelForce != "" {
		if p.providerLower == nameLower {
			p.forceUsed = true
			return p.modelForce, true
		}
		if p.providerLower == "" && !p.forceUsed {
			p.forceUsed = true
			return p.modelForce, true
		}
	}
	if specific != "" {
		return specific, true
	}
	if p.modelGeneric != "" {
		if p.providerLower == nameLower {
			return p.modelGeneric, true
		}
		if p.providerLower == "" && !p.genericUsed {
			p.genericUsed = true
			return p.modelGeneric, true
		}
	}
	return "", false
}
