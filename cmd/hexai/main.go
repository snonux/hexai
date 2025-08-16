package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"hexai/internal"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"hexai/internal/lsp"
)

func main() {
	logPath := flag.String("log", "/tmp/hexai.log", "path to log file (optional)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		log.Println(internal.Version)
		return
	}

	// Configure logging (path flag only)
	logger := log.New(os.Stderr, "hexai-lsp ", log.LstdFlags|log.Lmsgprefix)
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Fatalf("failed to open log file: %v", err)
		}
		defer f.Close()
		logger.SetOutput(f)
	}
	logging.Bind(logger)

	// Load config file
	cfg := loadConfig(logger)

	// Normalize and apply logging config
	cfg.ContextMode = strings.ToLower(strings.TrimSpace(cfg.ContextMode))
	if cfg.LogPreviewLimit >= 0 {
		logging.SetLogPreviewLimit(cfg.LogPreviewLimit)
	}

	// Build LLM client from config (only OPENAI_API_KEY may come from env)
	var client llm.Client
	{
		llmCfg := llm.Config{
			Provider:      cfg.Provider,
			OpenAIBaseURL: cfg.OpenAIBaseURL,
			OpenAIModel:   cfg.OpenAIModel,
			OllamaBaseURL: cfg.OllamaBaseURL,
			OllamaModel:   cfg.OllamaModel,
		}
		oaKey := os.Getenv("OPENAI_API_KEY")
		if c, err := llm.NewFromConfig(llmCfg, oaKey); err != nil {
			logging.Logf("lsp ", "llm disabled: %v", err)
		} else {
			client = c
			logging.Logf("lsp ", "llm enabled provider=%s model=%s", c.Name(), c.DefaultModel())
		}
	}

	server := lsp.NewServer(os.Stdin, os.Stdout, logger, *logPath != "", cfg.MaxTokens, cfg.ContextMode, cfg.ContextWindowLines, cfg.MaxContextTokens, cfg.NoDiskIO, client)
	if err := server.Run(); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}

// appConfig holds user-configurable settings.
type appConfig struct {
	MaxTokens          int    `json:"max_tokens"`
	ContextMode        string `json:"context_mode"`
	ContextWindowLines int    `json:"context_window_lines"`
	MaxContextTokens   int    `json:"max_context_tokens"`
	LogPreviewLimit    int    `json:"log_preview_limit"`
	NoDiskIO           bool   `json:"no_disk_io"`
	Provider           string `json:"provider"`
	// Provider-specific options
	OpenAIBaseURL string `json:"openai_base_url"`
	OpenAIModel   string `json:"openai_model"`
	OllamaBaseURL string `json:"ollama_base_url"`
	OllamaModel   string `json:"ollama_model"`
}

func loadConfig(logger *log.Logger) appConfig {
	// Defaults (mirror prior sensible values)
	cfg := appConfig{
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
	var fileCfg appConfig
	if err := dec.Decode(&fileCfg); err != nil {
		logger.Printf("invalid config file %s: %v", path, err)
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
	return cfg
}
