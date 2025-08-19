// Summary: Application configuration model and loader; reads ~/.config/hexai/config.json and merges defaults.
package appconfig

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "slices"
    "strconv"
    "strings"
)

// App holds user-configurable settings read from ~/.config/hexai/config.json.
type App struct {
	MaxTokens          int    `json:"max_tokens"`
	ContextMode        string `json:"context_mode"`
	ContextWindowLines int    `json:"context_window_lines"`
	MaxContextTokens   int    `json:"max_context_tokens"`
	LogPreviewLimit    int    `json:"log_preview_limit"`
	// Single knob for LSP requests; if set, overrides hardcoded temps in LSP.
	CodingTemperature *float64 `json:"coding_temperature"`

	TriggerCharacters []string `json:"trigger_characters"`
	Provider          string   `json:"provider"`

	// Provider-specific options
	OpenAIBaseURL string `json:"openai_base_url"`
	OpenAIModel   string `json:"openai_model"`
	// Default temperature for OpenAI requests (nil means use provider default)
	OpenAITemperature *float64 `json:"openai_temperature"`
	OllamaBaseURL     string   `json:"ollama_base_url"`
	OllamaModel       string   `json:"ollama_model"`
	// Default temperature for Ollama requests (nil means use provider default)
	OllamaTemperature *float64 `json:"ollama_temperature"`
	CopilotBaseURL    string   `json:"copilot_base_url"`
	CopilotModel      string   `json:"copilot_model"`
	// Default temperature for Copilot requests (nil means use provider default)
	CopilotTemperature *float64 `json:"copilot_temperature"`
}

// Constructor: defaults for App (kept first among functions)
func newDefaultConfig() App {
	// Coding-friendly default temperature across providers
	// Users can override per provider in config.json (including 0.0).
	t := 0.2
	return App{
		MaxTokens:          4000,
		ContextMode:        "always-full",
		ContextWindowLines: 120,
		MaxContextTokens:   4000,
		LogPreviewLimit:    100,
		CodingTemperature:  &t,
		OpenAITemperature:  &t,
		OllamaTemperature:  &t,
		CopilotTemperature: &t,
	}
}

// Load reads configuration from a file and merges with defaults.
// It respects the XDG Base Directory Specification.
func Load(logger *log.Logger) App {
    cfg := newDefaultConfig()
    if logger == nil {
        return cfg // Return defaults if no logger is provided (e.g. in tests)
    }

    configPath, err := getConfigPath()
    if err != nil {
        logger.Printf("%v", err)
        // Even if config path cannot be resolved, still allow env overrides below.
    } else {
        if fileCfg, err := loadFromFile(configPath, logger); err == nil && fileCfg != nil {
            cfg.mergeWith(fileCfg)
        }
        // When the config file is missing or invalid, we keep defaults and still
        // apply any environment overrides below.
    }

    // Environment overrides (take precedence over file)
    if envCfg := loadFromEnv(logger); envCfg != nil {
        cfg.mergeWith(envCfg)
    }
    return cfg
}

// Private helpers
func loadFromFile(path string, logger *log.Logger) (*App, error) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) && logger != nil {
			logger.Printf("cannot open config file %s: %v", path, err)
		}
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var fileCfg App
	if err := dec.Decode(&fileCfg); err != nil {
		if logger != nil {
			logger.Printf("invalid config file %s: %v", path, err)
		}
		return nil, err
	}
	return &fileCfg, nil
}

func (a *App) mergeWith(other *App) {
    a.mergeBasics(other)
    a.mergeProviderFields(other)
}

// mergeBasics merges general (non-provider) fields.
func (a *App) mergeBasics(other *App) {
	if other.MaxTokens > 0 {
		a.MaxTokens = other.MaxTokens
	}
	if s := strings.TrimSpace(other.ContextMode); s != "" {
		a.ContextMode = s
	}
	if other.ContextWindowLines > 0 {
		a.ContextWindowLines = other.ContextWindowLines
	}
	if other.MaxContextTokens > 0 {
		a.MaxContextTokens = other.MaxContextTokens
	}
	if other.LogPreviewLimit >= 0 {
		a.LogPreviewLimit = other.LogPreviewLimit
	}
	if other.CodingTemperature != nil { // allow explicit 0.0
		a.CodingTemperature = other.CodingTemperature
	}
	if len(other.TriggerCharacters) > 0 {
		a.TriggerCharacters = slices.Clone(other.TriggerCharacters)
	}
	if s := strings.TrimSpace(other.Provider); s != "" {
		a.Provider = s
	}
}

// mergeProviderFields merges per-provider configuration.
func (a *App) mergeProviderFields(other *App) {
	if s := strings.TrimSpace(other.OpenAIBaseURL); s != "" {
		a.OpenAIBaseURL = s
	}
	if s := strings.TrimSpace(other.OpenAIModel); s != "" {
		a.OpenAIModel = s
	}
	if other.OpenAITemperature != nil { // allow explicit 0.0
		a.OpenAITemperature = other.OpenAITemperature
	}
	if s := strings.TrimSpace(other.OllamaBaseURL); s != "" {
		a.OllamaBaseURL = s
	}
	if s := strings.TrimSpace(other.OllamaModel); s != "" {
		a.OllamaModel = s
	}
	if other.OllamaTemperature != nil { // allow explicit 0.0
		a.OllamaTemperature = other.OllamaTemperature
	}
	if s := strings.TrimSpace(other.CopilotBaseURL); s != "" {
		a.CopilotBaseURL = s
	}
	if s := strings.TrimSpace(other.CopilotModel); s != "" {
		a.CopilotModel = s
	}
	if other.CopilotTemperature != nil { // allow explicit 0.0
		a.CopilotTemperature = other.CopilotTemperature
	}
}

func getConfigPath() (string, error) {
	var configPath string
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		configPath = filepath.Join(xdgConfigHome, "hexai", "config.json")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot find user home directory: %v", err)
		}
		configPath = filepath.Join(home, ".config", "hexai", "config.json")
	}
    return configPath, nil
}

// --- Environment overrides ---

// loadFromEnv constructs an App containing only fields set via HEXAI_* env vars.
// These values should take precedence over file config when merged.
func loadFromEnv(logger *log.Logger) *App {
    var out App
    var any bool

    // helpers
    getenv := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
    parseInt := func(k string) (int, bool) {
        v := getenv(k)
        if v == "" { return 0, false }
        n, err := strconv.Atoi(v)
        if err != nil {
            if logger != nil { logger.Printf("invalid %s: %v", k, err) }
            return 0, false
        }
        return n, true
    }
    parseFloatPtr := func(k string) (*float64, bool) {
        v := getenv(k)
        if v == "" { return nil, false }
        f, err := strconv.ParseFloat(v, 64)
        if err != nil {
            if logger != nil { logger.Printf("invalid %s: %v", k, err) }
            return nil, false
        }
        return &f, true
    }

    if n, ok := parseInt("HEXAI_MAX_TOKENS"); ok {
        out.MaxTokens = n; any = true
    }
    if s := getenv("HEXAI_CONTEXT_MODE"); s != "" {
        out.ContextMode = s; any = true
    }
    if n, ok := parseInt("HEXAI_CONTEXT_WINDOW_LINES"); ok {
        out.ContextWindowLines = n; any = true
    }
    if n, ok := parseInt("HEXAI_MAX_CONTEXT_TOKENS"); ok {
        out.MaxContextTokens = n; any = true
    }
    if n, ok := parseInt("HEXAI_LOG_PREVIEW_LIMIT"); ok {
        out.LogPreviewLimit = n; any = true
    }
    if f, ok := parseFloatPtr("HEXAI_CODING_TEMPERATURE"); ok {
        out.CodingTemperature = f; any = true
    }
    if s := getenv("HEXAI_TRIGGER_CHARACTERS"); s != "" {
        parts := strings.Split(s, ",")
        out.TriggerCharacters = nil
        for _, p := range parts {
            if t := strings.TrimSpace(p); t != "" {
                out.TriggerCharacters = append(out.TriggerCharacters, t)
            }
        }
        any = true
    }
    if s := getenv("HEXAI_PROVIDER"); s != "" {
        out.Provider = s; any = true
    }

    // Provider-specific
    if s := getenv("HEXAI_OPENAI_BASE_URL"); s != "" { out.OpenAIBaseURL = s; any = true }
    if s := getenv("HEXAI_OPENAI_MODEL"); s != "" { out.OpenAIModel = s; any = true }
    if f, ok := parseFloatPtr("HEXAI_OPENAI_TEMPERATURE"); ok { out.OpenAITemperature = f; any = true }

    if s := getenv("HEXAI_OLLAMA_BASE_URL"); s != "" { out.OllamaBaseURL = s; any = true }
    if s := getenv("HEXAI_OLLAMA_MODEL"); s != "" { out.OllamaModel = s; any = true }
    if f, ok := parseFloatPtr("HEXAI_OLLAMA_TEMPERATURE"); ok { out.OllamaTemperature = f; any = true }

    if s := getenv("HEXAI_COPILOT_BASE_URL"); s != "" { out.CopilotBaseURL = s; any = true }
    if s := getenv("HEXAI_COPILOT_MODEL"); s != "" { out.CopilotModel = s; any = true }
    if f, ok := parseFloatPtr("HEXAI_COPILOT_TEMPERATURE"); ok { out.CopilotTemperature = f; any = true }

    if !any {
        return nil
    }
    return &out
}
