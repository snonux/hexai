// Summary: Hexai LSP runner; configures logging, loads config, builds the LLM client,
// and constructs/runs the LSP server (with injectable factory for tests).
package hexailsp

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/ignore"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
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
	logger := log.New(stderr, "hexai-lsp-server ", log.LstdFlags|log.Lmsgprefix)
	if strings.TrimSpace(logPath) != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				logger.Printf("failed to close log file: %v", err)
			}
		}()
		logger.SetOutput(f)
	}
	logging.Bind(logger)
	loadOpts := appconfig.LoadOptions{ConfigPath: configPath}
	cfg := appconfig.LoadWithOptions(logger, loadOpts)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
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
		return fmt.Errorf("invalid config: %w", err)
	}
	client = buildClientIfNil(cfg, client)
	factory = ensureFactory(factory)

	// Create gitignore-aware file checker for LSP filtering
	gitRoot := appconfig.FindGitRoot()
	useGI := cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore
	ignoreChecker := ignore.New(gitRoot, useGI, cfg.IgnoreExtraPatterns)

	store := runtimeconfig.New(cfg)
	logContext := strings.TrimSpace(logPath) != ""
	loadOpts := appconfig.LoadOptions{ConfigPath: strings.TrimSpace(configPath)}
	opts := makeServerOptions(cfg, logContext, client, loadOpts, ignoreChecker)
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
			// Update ignore checker patterns on config hot-reload
			useGI := updated.IgnoreGitignore == nil || *updated.IgnoreGitignore
			ignoreChecker.Update(useGI, updated.IgnoreExtraPatterns)
			opts := makeServerOptions(updated, logContext, client, loadOpts, ignoreChecker)
			opts.ConfigStore = store
			configurable.ApplyOptions(opts)
		})
	}
	if err := server.Run(); err != nil {
		return fmt.Errorf("server error: %w", err)
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
	c, err := llmutils.NewClientFromApp(cfg)
	if err != nil {
		logging.Logf("lsp ", "llm disabled: %v", err)
		return nil
	}
	logging.Logf("lsp ", "llm enabled provider=%s model=%s", c.Name(), c.DefaultModel())
	return c
}

func ensureFactory(factory ServerFactory) ServerFactory {
	if factory != nil {
		return factory
	}
	return func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		return lsp.NewServer(r, w, logger, opts)
	}
}

func makeServerOptions(cfg appconfig.App, logContext bool, client llm.Client, loadOpts appconfig.LoadOptions, ignoreChecker *ignore.Checker) lsp.ServerOptions {
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
		CompletionWaitAll:     cfg.CompletionWaitAll,
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
		IgnoreChecker:           ignoreChecker,
	}
}
