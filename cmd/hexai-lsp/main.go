package main

import (
    "flag"
    "log"
    "os"
    "strings"

    "hexai/internal"
    "hexai/internal/appconfig"
    "hexai/internal/llm"
    "hexai/internal/logging"
    "hexai/internal/lsp"
)

func main() {
    logPath := flag.String("log", "/tmp/hexai-lsp.log", "path to log file (optional)")
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
    cfg := appconfig.Load(logger)

    // Normalize and apply logging config
    cfg.ContextMode = strings.ToLower(strings.TrimSpace(cfg.ContextMode))
    if cfg.LogPreviewLimit >= 0 {
        logging.SetLogPreviewLimit(cfg.LogPreviewLimit)
    }

    // Build LLM client from config
    var client llm.Client
    {
        llmCfg := llm.Config{
            Provider:       cfg.Provider,
            OpenAIBaseURL:  cfg.OpenAIBaseURL,
            OpenAIModel:    cfg.OpenAIModel,
            OllamaBaseURL:  cfg.OllamaBaseURL,
            OllamaModel:    cfg.OllamaModel,
            CopilotBaseURL: cfg.CopilotBaseURL,
            CopilotModel:   cfg.CopilotModel,
        }
        oaKey := os.Getenv("OPENAI_API_KEY")
        cpKey := os.Getenv("COPILOT_API_KEY")
        if c, err := llm.NewFromConfig(llmCfg, oaKey, cpKey); err != nil {
            logging.Logf("lsp ", "llm disabled: %v", err)
        } else {
            client = c
            logging.Logf("lsp ", "llm enabled provider=%s model=%s", c.Name(), c.DefaultModel())
        }
    }

    server := lsp.NewServer(os.Stdin, os.Stdout, logger, lsp.ServerOptions{
        LogContext:        *logPath != "",
        MaxTokens:         cfg.MaxTokens,
        ContextMode:       cfg.ContextMode,
        WindowLines:       cfg.ContextWindowLines,
        MaxContextTokens:  cfg.MaxContextTokens,
        NoDiskIO:          cfg.NoDiskIO,
        Client:            client,
        TriggerCharacters: cfg.TriggerCharacters,
    })
    if err := server.Run(); err != nil {
        logger.Fatalf("server error: %v", err)
    }
}

