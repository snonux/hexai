package appconfig

import (
    "encoding/json"
    "log"
    "os"
    "path/filepath"
    "strings"
)

// App holds user-configurable settings read from ~/.config/hexai/config.json.
type App struct {
    MaxTokens          int      `json:"max_tokens"`
    ContextMode        string   `json:"context_mode"`
    ContextWindowLines int      `json:"context_window_lines"`
    MaxContextTokens   int      `json:"max_context_tokens"`
    LogPreviewLimit    int      `json:"log_preview_limit"`
    NoDiskIO           bool     `json:"no_disk_io"`
    TriggerCharacters  []string `json:"trigger_characters"`
    Provider           string   `json:"provider"`
    // Provider-specific options
    OpenAIBaseURL  string `json:"openai_base_url"`
    OpenAIModel    string `json:"openai_model"`
    OllamaBaseURL  string `json:"ollama_base_url"`
    OllamaModel    string `json:"ollama_model"`
    CopilotBaseURL string `json:"copilot_base_url"`
    CopilotModel   string `json:"copilot_model"`
}

// Load reads configuration from ~/.config/hexai/config.json and merges with defaults.
func Load(logger *log.Logger) App {
    cfg := App{
        MaxTokens:          4000,
        ContextMode:        "always-full",
        ContextWindowLines: 120,
        MaxContextTokens:   4000,
        LogPreviewLimit:    100,
        NoDiskIO:           true,
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return cfg
    }
    path := filepath.Join(home, ".config", "hexai", "config.json")
    f, err := os.Open(path)
    if err != nil {
        return cfg
    }
    defer f.Close()
    dec := json.NewDecoder(f)
    var fileCfg App
    if err := dec.Decode(&fileCfg); err != nil {
        if logger != nil {
            logger.Printf("invalid config file %s: %v", path, err)
        }
        return cfg
    }
    // Merge: file overrides defaults when provided
    if fileCfg.MaxTokens > 0 {
        cfg.MaxTokens = fileCfg.MaxTokens
    }
    if strings.TrimSpace(fileCfg.ContextMode) != "" {
        cfg.ContextMode = fileCfg.ContextMode
    }
    if fileCfg.ContextWindowLines > 0 {
        cfg.ContextWindowLines = fileCfg.ContextWindowLines
    }
    if fileCfg.MaxContextTokens > 0 {
        cfg.MaxContextTokens = fileCfg.MaxContextTokens
    }
    if fileCfg.LogPreviewLimit >= 0 {
        cfg.LogPreviewLimit = fileCfg.LogPreviewLimit
    }
    cfg.NoDiskIO = fileCfg.NoDiskIO
    if len(fileCfg.TriggerCharacters) > 0 {
        cfg.TriggerCharacters = append([]string{}, fileCfg.TriggerCharacters...)
    }
    if strings.TrimSpace(fileCfg.Provider) != "" {
        cfg.Provider = fileCfg.Provider
    }
    // Provider-specific options
    if strings.TrimSpace(fileCfg.OpenAIBaseURL) != "" {
        cfg.OpenAIBaseURL = fileCfg.OpenAIBaseURL
    }
    if strings.TrimSpace(fileCfg.OpenAIModel) != "" {
        cfg.OpenAIModel = fileCfg.OpenAIModel
    }
    if strings.TrimSpace(fileCfg.OllamaBaseURL) != "" {
        cfg.OllamaBaseURL = fileCfg.OllamaBaseURL
    }
    if strings.TrimSpace(fileCfg.OllamaModel) != "" {
        cfg.OllamaModel = fileCfg.OllamaModel
    }
    if strings.TrimSpace(fileCfg.CopilotBaseURL) != "" {
        cfg.CopilotBaseURL = fileCfg.CopilotBaseURL
    }
    if strings.TrimSpace(fileCfg.CopilotModel) != "" {
        cfg.CopilotModel = fileCfg.CopilotModel
    }
    return cfg
}

