// Package hexailsp is the Hexai LSP runner; configures logging, loads config, builds the LLM client,
// and constructs/runs the LSP server (with injectable factory for tests).
package hexailsp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/ignore"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/lsp"
	"codeberg.org/snonux/hexai/internal/stats"
	tmx "codeberg.org/snonux/hexai/internal/tmux"
)

// ServerRunner is the minimal interface satisfied by lsp.Server.
// Run takes a context so the serve loop is cancelled when the process is
// shutting down (e.g. on SIGINT/SIGTERM from the top-level caller).
type ServerRunner interface {
	Run(ctx context.Context) error
}

// ConfigurableServerRunner supports runtime option updates.
type ConfigurableServerRunner interface {
	ServerRunner
	ApplyOptions(lsp.ServerOptions)
}

type tmuxStatusSink struct{}

func (tmuxStatusSink) SetLLMStart(provider, model string) error {
	return tmx.SetStatus(tmx.FormatLLMStartStatus(provider, model))
}

func (tmuxStatusSink) SetGlobal(gs lsp.GlobalStatus) error {
	status := tmx.FormatGlobalStatusColored(
		gs.Reqs, gs.RPM, gs.Sent, gs.Recv,
		gs.Provider, gs.Model, gs.ScopeRPM, gs.ScopeReqs, gs.Window,
	)
	return tmx.SetStatus(status)
}

// ServerFactory creates a ServerRunner. Default uses lsp.NewServer.
type ServerFactory func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner

// Run configures logging, loads config, builds the LLM client and runs the LSP server.
// It is thin and delegates to RunWithFactory for testability.

func Run(ctx context.Context, logPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return RunWithConfig(ctx, logPath, "", stdin, stdout, stderr)
}

// RunWithConfig is like Run but accepts an explicit config file path.
// ctx is threaded through config loading and the LSP serve loop so the whole
// run is cancellable from the process entry point.
func RunWithConfig(ctx context.Context, logPath string, configPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if err := llm.RegisterAllProviders(); err != nil {
		return fmt.Errorf("failed to register LLM providers: %w", err)
	}
	return runWithConfigDependencies(ctx, logPath, configPath, stdin, stdout, stderr, defaultRunDependencies())
}

func runWithConfigDependencies(ctx context.Context, logPath string, configPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer, deps runDependencies) error {
	deps = normalizeRunDependencies(deps)
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
	cfg := deps.loadConfig(ctx, logger, loadOpts)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	return runWithDependencies(ctx, logPath, configPath, stdin, stdout, logger, cfg, nil, nil, deps)
}

// RunWithFactory is the testable entrypoint. When client is nil, it is built from cfg+env.
// When factory is nil, lsp.NewServer is used.
func RunWithFactory(ctx context.Context, logPath string, configPath string, stdin io.Reader, stdout io.Writer, logger *log.Logger, cfg appconfig.App, client llm.Client, factory ServerFactory) error {
	return runWithDependencies(ctx, logPath, configPath, stdin, stdout, logger, cfg, client, factory, defaultRunDependencies())
}

func runWithDependencies(ctx context.Context, logPath string, configPath string, stdin io.Reader, stdout io.Writer, logger *log.Logger, cfg appconfig.App, client llm.Client, factory ServerFactory, deps runDependencies) error {
	deps = normalizeRunDependencies(deps)
	normalizeLoggingConfig(&cfg)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	client = deps.buildClient(cfg, client)
	factory = ensureFactory(factory)

	ignoreChecker := deps.newIgnoreChecker(cfg)
	store := deps.newConfigStore(cfg)
	logContext := strings.TrimSpace(logPath) != ""
	loadOpts := appconfig.LoadOptions{ConfigPath: strings.TrimSpace(configPath)}
	opts := makeServerOptions(cfg, logContext, client, loadOpts, ignoreChecker, deps.statusSink)
	opts.ConfigLoadOptions = loadOpts
	opts.ConfigStore = store
	server := factory(stdin, stdout, logger, opts)
	if configurable, ok := server.(ConfigurableServerRunner); ok {
		store.Subscribe(func(oldCfg, newCfg appconfig.App) {
			updated := newCfg
			normalizeLoggingConfig(&updated)
			if updated.StatsWindowMinutes > 0 {
				stats.SetWindow(time.Duration(updated.StatsWindowMinutes) * time.Minute)
			}
			if newClient := deps.buildClient(updated, nil); newClient != nil {
				client = newClient
			}
			// Update ignore checker patterns on config hot-reload
			useGI := updated.IgnoreGitignore == nil || *updated.IgnoreGitignore
			ignoreChecker.Update(useGI, updated.IgnoreExtraPatterns)
			opts := makeServerOptions(updated, logContext, client, loadOpts, ignoreChecker, deps.statusSink)
			opts.ConfigStore = store
			configurable.ApplyOptions(opts)
		})
	}
	if err := server.Run(ctx); err != nil {
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

func ensureFactory(factory ServerFactory) ServerFactory {
	if factory != nil {
		return factory
	}
	return func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		return lsp.NewServer(r, w, logger, opts)
	}
}

func makeServerOptions(cfg appconfig.App, logContext bool, client llm.Client, loadOpts appconfig.LoadOptions, ignoreChecker *ignore.Checker, statusSink lsp.StatusSink) lsp.ServerOptions {
	return lsp.ServerOptions{
		ConfigLoadOptions: loadOpts,
		LogContext:        logContext,
		ConfigStore:       nil,
		Config:            &cfg,
		Client:            client,
		IgnoreChecker:     ignoreChecker,
		StatusSink:        statusSink,
	}
}
