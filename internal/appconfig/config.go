// Summary: Application configuration model and loader; reads ~/.config/hexai/config.json and merges defaults.
package appconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
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
    CodingTemperature  *float64 `json:"coding_temperature"`

	TriggerCharacters []string `json:"trigger_characters"`
	Provider          string   `json:"provider"`

    // Provider-specific options
    OpenAIBaseURL  string `json:"openai_base_url"`
    OpenAIModel    string `json:"openai_model"`
    // Default temperature for OpenAI requests (nil means use provider default)
    OpenAITemperature *float64 `json:"openai_temperature"`
    OllamaBaseURL  string `json:"ollama_base_url"`
    OllamaModel    string `json:"ollama_model"`
    // Default temperature for Ollama requests (nil means use provider default)
    OllamaTemperature *float64 `json:"ollama_temperature"`
    CopilotBaseURL string `json:"copilot_base_url"`
    CopilotModel   string `json:"copilot_model"`
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
		return cfg
	}

	fileCfg, err := loadFromFile(configPath, logger)
	if err != nil {
		return cfg
	}

    cfg.mergeWith(fileCfg)
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
    if other.MaxTokens > 0 {
        a.MaxTokens = other.MaxTokens
    }
    if strings.TrimSpace(other.ContextMode) != "" {
        a.ContextMode = other.ContextMode
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
    if strings.TrimSpace(other.Provider) != "" {
        a.Provider = other.Provider
    }
    if strings.TrimSpace(other.OpenAIBaseURL) != "" {
        a.OpenAIBaseURL = other.OpenAIBaseURL
    }
    if strings.TrimSpace(other.OpenAIModel) != "" {
        a.OpenAIModel = other.OpenAIModel
    }
    if other.OpenAITemperature != nil { // allow explicit 0.0
        a.OpenAITemperature = other.OpenAITemperature
    }
    if strings.TrimSpace(other.OllamaBaseURL) != "" {
        a.OllamaBaseURL = other.OllamaBaseURL
    }
    if strings.TrimSpace(other.OllamaModel) != "" {
        a.OllamaModel = other.OllamaModel
    }
    if other.OllamaTemperature != nil { // allow explicit 0.0
        a.OllamaTemperature = other.OllamaTemperature
    }
    if strings.TrimSpace(other.CopilotBaseURL) != "" {
        a.CopilotBaseURL = other.CopilotBaseURL
    }
    if strings.TrimSpace(other.CopilotModel) != "" {
        a.CopilotModel = other.CopilotModel
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
