// Summary: Hexai LSP runner; configures logging, loads config, builds the LLM client,
// and constructs/runs the LSP server (with injectable factory for tests).
package hexailsp

import (
	"io"
	"log"
	"os"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/lsp"
	"codeberg.org/snonux/hexai/internal/runtimeconfig"
	"codeberg.org/snonux/hexai/internal/stats"
)

// ServerRunner is the minimal interface satisfied by lsp.Server.
type ServerRunner interface{ Run() error }

// ServerFactory creates a ServerRunner. Default uses lsp.NewServer.
type ServerFactory func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner

// Run configures logging, loads config, builds the LLM client and runs the LSP server.
// It is thin and delegates to RunWithFactory for testability.

func Run(logPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return RunWithConfig(logPath, "", stdin, stdout, stderr)
}

func RunWithConfig(logPath string, configPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	logger := log.New(stderr, "hexai-lsp ", log.LstdFlags|log.Lmsgprefix)
	if strings.TrimSpace(logPath) != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logger.Fatalf("failed to open log file: %v", err)
		}
		defer f.Close()
		logger.SetOutput(f)
	}
	logging.Bind(logger)
	loadOpts := appconfig.LoadOptions{ConfigPath: configPath}
	cfg := appconfig.LoadWithOptions(logger, loadOpts)
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("invalid config: %v", err)
	}
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	return RunWithFactory(logPath, configPath, stdin, stdout, logger, cfg, nil, nil)
}

// RunWithFactory is the testable entrypoint. When client is nil, it is built from cfg+env.
// When factory is nil, lsp.NewServer is used.
func RunWithFactory(logPath string, configPath string, stdin io.Reader, stdout io.Writer, logger *log.Logger, cfg appconfig.App, client llm.Client, factory ServerFactory) error {
	normalizeLoggingConfig(&cfg)
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("invalid config: %v", err)
	}
	client = buildClientIfNil(cfg, client)
	factory = ensureFactory(factory)

	store := runtimeconfig.New(cfg)
	logContext := strings.TrimSpace(logPath) != ""
	loadOpts := appconfig.LoadOptions{ConfigPath: strings.TrimSpace(configPath)}
	opts := makeServerOptions(cfg, logContext, client, loadOpts)
	opts.ConfigLoadOptions = loadOpts
	opts.ConfigStore = store
	server := factory(stdin, stdout, logger, opts)
	if configurable, ok := server.(interface{ ApplyOptions(lsp.ServerOptions) }); ok {
		store.Subscribe(func(oldCfg, newCfg appconfig.App) {
			updated := newCfg
			normalizeLoggingConfig(&updated)
			if updated.StatsWindowMinutes > 0 {
				stats.SetWindow(time.Duration(updated.StatsWindowMinutes) * time.Minute)
			}
			if newClient := buildClientIfNil(updated, nil); newClient != nil {
				client = newClient
			}
			opts := makeServerOptions(updated, logContext, client, loadOpts)
			opts.ConfigStore = store
			configurable.ApplyOptions(opts)
		})
	}
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
	}
	// Prefer HEXAI_OPENAI_API_KEY; fall back to OPENAI_API_KEY
	oaKey := os.Getenv("HEXAI_OPENAI_API_KEY")
	if strings.TrimSpace(oaKey) == "" {
		oaKey = os.Getenv("OPENAI_API_KEY")
	}
	// Prefer HEXAI_OPENROUTER_API_KEY; fall back to OPENROUTER_API_KEY
	orKey := os.Getenv("HEXAI_OPENROUTER_API_KEY")
	if strings.TrimSpace(orKey) == "" {
		orKey = os.Getenv("OPENROUTER_API_KEY")
	}
	// Prefer HEXAI_COPILOT_API_KEY; fall back to COPILOT_API_KEY
	cpKey := os.Getenv("HEXAI_COPILOT_API_KEY")
	if strings.TrimSpace(cpKey) == "" {
		cpKey = os.Getenv("COPILOT_API_KEY")
	}
	if c, err := llm.NewFromConfig(llmCfg, oaKey, orKey, cpKey); err != nil {
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

func makeServerOptions(cfg appconfig.App, logContext bool, client llm.Client, loadOpts appconfig.LoadOptions) lsp.ServerOptions {
	// Map custom actions from appconfig to lsp type
	var customs []lsp.CustomAction
	if len(cfg.CustomActions) > 0 {
		customs = make([]lsp.CustomAction, 0, len(cfg.CustomActions))
		for _, ca := range cfg.CustomActions {
			customs = append(customs, lsp.CustomAction{
				ID:          ca.ID,
				Title:       ca.Title,
				Kind:        ca.Kind,
				Scope:       ca.Scope,
				Instruction: ca.Instruction,
				System:      ca.System,
				User:        ca.User,
			})
		}
	}
	return lsp.ServerOptions{
		ConfigLoadOptions:     loadOpts,
		LogContext:            logContext,
		ConfigStore:           nil,
		Config:                &cfg,
		MaxTokens:             cfg.MaxTokens,
		ContextMode:           cfg.ContextMode,
		WindowLines:           cfg.ContextWindowLines,
		MaxContextTokens:      cfg.MaxContextTokens,
		CodingTemperature:     cfg.CodingTemperature,
		Client:                client,
		TriggerCharacters:     cfg.TriggerCharacters,
		ManualInvokeMinPrefix: cfg.ManualInvokeMinPrefix,
		CompletionDebounceMs:  cfg.CompletionDebounceMs,
		CompletionThrottleMs:  cfg.CompletionThrottleMs,
		InlineOpen:            cfg.InlineOpen,
		InlineClose:           cfg.InlineClose,
		ChatSuffix:            cfg.ChatSuffix,
		ChatPrefixes:          cfg.ChatPrefixes,

		// Prompts
		PromptCompSysGeneral:    cfg.PromptCompletionSystemGeneral,
		PromptCompSysParams:     cfg.PromptCompletionSystemParams,
		PromptCompSysInline:     cfg.PromptCompletionSystemInline,
		PromptCompUserGeneral:   cfg.PromptCompletionUserGeneral,
		PromptCompUserParams:    cfg.PromptCompletionUserParams,
		PromptCompExtraHeader:   cfg.PromptCompletionExtraHeader,
		PromptNativeCompletion:  cfg.PromptNativeCompletion,
		PromptChatSystem:        cfg.PromptChatSystem,
		PromptRewriteSystem:     cfg.PromptCodeActionRewriteSystem,
		PromptDiagnosticsSystem: cfg.PromptCodeActionDiagnosticsSystem,
		PromptDocumentSystem:    cfg.PromptCodeActionDocumentSystem,
		PromptRewriteUser:       cfg.PromptCodeActionRewriteUser,
		PromptDiagnosticsUser:   cfg.PromptCodeActionDiagnosticsUser,
		PromptDocumentUser:      cfg.PromptCodeActionDocumentUser,
		PromptGoTestSystem:      cfg.PromptCodeActionGoTestSystem,
		PromptGoTestUser:        cfg.PromptCodeActionGoTestUser,
		PromptSimplifySystem:    cfg.PromptCodeActionSimplifySystem,
		PromptSimplifyUser:      cfg.PromptCodeActionSimplifyUser,
		CustomActions:           customs,
	}
}
