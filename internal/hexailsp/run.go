// Summary: Hexai LSP runner; configures logging, loads config, builds the LLM client,
// and constructs/runs the LSP server (with injectable factory for tests).
package hexailsp

import (
	"io"
	"log"
	"os"
	"strings"

	"hexai/internal/appconfig"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"hexai/internal/lsp"
)

// ServerRunner is the minimal interface satisfied by lsp.Server.
type ServerRunner interface{ Run() error }

// ServerFactory creates a ServerRunner. Default uses lsp.NewServer.
type ServerFactory func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner

// Run configures logging, loads config, builds the LLM client and runs the LSP server.
// It is thin and delegates to RunWithFactory for testability.
func Run(logPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	logger := log.New(stderr, "hexai-lsp ", log.LstdFlags|log.Lmsgprefix)
	if strings.TrimSpace(logPath) != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Fatalf("failed to open log file: %v", err)
		}
		defer f.Close()
		logger.SetOutput(f)
	}
	logging.Bind(logger)
	cfg := appconfig.Load(logger)
	return RunWithFactory(logPath, stdin, stdout, logger, cfg, nil, nil)
}

// RunWithFactory is the testable entrypoint. When client is nil, it is built from cfg+env.
// When factory is nil, lsp.NewServer is used.
func RunWithFactory(logPath string, stdin io.Reader, stdout io.Writer, logger *log.Logger, cfg appconfig.App, client llm.Client, factory ServerFactory) error {
	normalizeLoggingConfig(&cfg)
	client = buildClientIfNil(cfg, client)
	factory = ensureFactory(factory)

	opts := makeServerOptions(cfg, strings.TrimSpace(logPath) != "", client)
	server := factory(stdin, stdout, logger, opts)
	if err := server.Run(); err != nil {
		logger.Fatalf("server error: %v", err)
	}
	return nil
}

// --- helpers to keep RunWithFactory small ---

func normalizeLoggingConfig(cfg *appconfig.App) {
	cfg.ContextMode = strings.ToLower(strings.TrimSpace(cfg.ContextMode))
	if cfg.LogPreviewLimit >= 0 {
		logging.SetLogPreviewLimit(cfg.LogPreviewLimit)
	}
}

func buildClientIfNil(cfg appconfig.App, client llm.Client) llm.Client {
	if client != nil {
		return client
	}
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
    // Prefer HEXAI_OPENAI_API_KEY; fall back to OPENAI_API_KEY
    oaKey := os.Getenv("HEXAI_OPENAI_API_KEY")
    if strings.TrimSpace(oaKey) == "" {
        oaKey = os.Getenv("OPENAI_API_KEY")
    }
    // Prefer HEXAI_COPILOT_API_KEY; fall back to COPILOT_API_KEY
    cpKey := os.Getenv("HEXAI_COPILOT_API_KEY")
    if strings.TrimSpace(cpKey) == "" {
        cpKey = os.Getenv("COPILOT_API_KEY")
    }
	if c, err := llm.NewFromConfig(llmCfg, oaKey, cpKey); err != nil {
		logging.Logf("lsp ", "llm disabled: %v", err)
		return nil
	} else {
		logging.Logf("lsp ", "llm enabled provider=%s model=%s", c.Name(), c.DefaultModel())
		return c
	}
}

func ensureFactory(factory ServerFactory) ServerFactory {
	if factory != nil {
		return factory
	}
	return func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		return lsp.NewServer(r, w, logger, opts)
	}
}

func makeServerOptions(cfg appconfig.App, logContext bool, client llm.Client) lsp.ServerOptions {
    return lsp.ServerOptions{
        LogContext:        logContext,
        MaxTokens:         cfg.MaxTokens,
        ContextMode:       cfg.ContextMode,
        WindowLines:       cfg.ContextWindowLines,
        MaxContextTokens:  cfg.MaxContextTokens,
        CodingTemperature: cfg.CodingTemperature,
        Client:            client,
        TriggerCharacters: cfg.TriggerCharacters,
    }
}
