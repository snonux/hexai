// Package hexaimcp is the MCP server orchestrator; loads config, sets up store, and runs server.
package hexaimcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/mcp"
	"codeberg.org/snonux/hexai/internal/promptstore"
	"codeberg.org/snonux/hexai/internal/slashcommands"
)

// MCPOverrides holds CLI flag values that override config settings.
// These are passed explicitly from the CLI entrypoint instead of using
// environment variables, avoiding the code smell of os.Setenv in production code.
type MCPOverrides struct {
	PromptsDir       string
	SlashCommandSync bool
	SlashCommandDir  string
}

// ServerRunner interface allows dependency injection for testing.
// Run takes a context so the serve loop is cancelled when the process shuts
// down (e.g. on SIGINT/SIGTERM from the top-level caller).
type ServerRunner interface {
	Run(ctx context.Context) error
}

// ServerFactory creates a server instance (testable).
type ServerFactory func(
	r io.Reader,
	w io.Writer,
	logger *log.Logger,
	store promptstore.PromptStore,
	syncer mcp.SlashCommandSyncer,
) ServerRunner

// defaultServerFactory is the production server factory.
func defaultServerFactory(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
	return mcp.NewServer(r, w, logger, store, syncer)
}

// Run starts the MCP server with the given configuration and overrides.
// This is the main entry point called from cmd/hexai-mcp-server/main.go.
// ctx is threaded through config loading and the serve loop so the run is
// cancellable from the process entry point.
func Run(ctx context.Context, logPath, configPath string, overrides MCPOverrides, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return RunWithFactory(ctx, logPath, configPath, overrides, stdin, stdout, stderr, defaultServerFactory)
}

// RunWithFactory allows test injection of server factory.
// Overrides are applied to the loaded config before use, allowing CLI flags
// to take precedence over config file and environment variable settings.
func RunWithFactory(
	ctx context.Context,
	logPath string,
	configPath string,
	overrides MCPOverrides,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	factory ServerFactory,
) error {
	loadOpts := appconfig.LoadOptions{ConfigPath: configPath}
	return runWithFactoryLoadOptions(ctx, logPath, loadOpts, overrides, stdin, stdout, stderr, factory)
}

func runWithFactoryLoadOptions(
	ctx context.Context,
	logPath string,
	loadOpts appconfig.LoadOptions,
	overrides MCPOverrides,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	factory ServerFactory,
) error {
	// Setup logger
	logger, err := setupLogger(logPath)
	if err != nil {
		return fmt.Errorf("cannot setup logger: %w", err)
	}
	defer func() {
		if f, ok := logger.Writer().(*os.File); ok && f != os.Stderr {
			f.Close()
		}
	}()

	logger.Printf("hexai-mcp-server starting")
	logger.Printf("WARNING: hexai-mcp-server is DEPRECATED and experimental - not actively maintained")

	// Load configuration and apply CLI overrides
	cfg := loadConfigWithOptions(ctx, logger, loadOpts)
	applyOverrides(&cfg, overrides)

	return runServer(ctx, cfg, logger, stdin, stdout, factory)
}

// runServer creates the prompt store, syncer, and runs the MCP server.
func runServer(ctx context.Context, cfg appconfig.App, logger *log.Logger, stdin io.Reader, stdout io.Writer, factory ServerFactory) error {
	// Determine prompts directory from config (overrides already applied)
	promptsDir, err := getPromptsDir(cfg)
	if err != nil {
		return fmt.Errorf("cannot determine prompts directory: %w", err)
	}
	logger.Printf("using prompts directory: %s", promptsDir)

	// Create prompt store
	store, err := promptstore.NewJSONLStore(promptsDir)
	if err != nil {
		return fmt.Errorf("cannot create prompt store: %w", err)
	}

	// Create slash command syncer (optional)
	syncer, err := createSyncer(cfg, logger)
	if err != nil {
		return fmt.Errorf("cannot create syncer: %w", err)
	}

	// Create and run server
	server := factory(stdin, stdout, logger, store, syncer)
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	logger.Printf("hexai-mcp-server exiting")
	return nil
}

// setupLogger creates a logger that writes to the specified log file.
// If logPath is empty, logs to stderr.
func setupLogger(logPath string) (*log.Logger, error) {
	logPath = strings.TrimSpace(logPath)
	if logPath == "" {
		return log.New(os.Stderr, "mcp ", log.LstdFlags), nil
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create log directory: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot open log file: %w", err)
	}

	return log.New(f, "mcp ", log.LstdFlags), nil
}

// loadConfig loads the hexai configuration.
// Returns default config if loading fails or ctx is already cancelled.
func loadConfig(ctx context.Context, logger *log.Logger, configPath string) appconfig.App {
	opts := appconfig.LoadOptions{
		ConfigPath: configPath,
		IgnoreEnv:  false,
	}
	return loadConfigWithOptions(ctx, logger, opts)
}

func loadConfigWithOptions(ctx context.Context, logger *log.Logger, opts appconfig.LoadOptions) appconfig.App {
	return appconfig.LoadWithOptions(ctx, logger, opts)
}

// applyOverrides applies CLI flag overrides to the loaded config.
// This replaces the previous approach of using os.Setenv to pass values.
func applyOverrides(cfg *appconfig.App, overrides MCPOverrides) {
	if overrides.PromptsDir != "" {
		cfg.MCPPromptsDir = overrides.PromptsDir
	}
	if overrides.SlashCommandSync {
		cfg.MCPSlashCommandSync = true
	}
	if overrides.SlashCommandDir != "" {
		cfg.MCPSlashCommandDir = overrides.SlashCommandDir
	}
}

// getPromptsDir determines the prompts directory from config.
// Precedence: CLI flag (via overrides applied to config) > env var (via
// applyMCPEnv in config loading) > config file > default XDG location.
// The env var HEXAI_MCP_PROMPTS_DIR is still supported through the config
// loading pipeline in appconfig, not read directly here.
func getPromptsDir(cfg appconfig.App) (string, error) {
	// Check config (which already includes env var and CLI overrides)
	if cfgDir := strings.TrimSpace(cfg.MCPPromptsDir); cfgDir != "" {
		return expandPath(cfgDir)
	}

	// Default: $XDG_DATA_HOME/prompts/ or ~/.local/hexai/data/prompts/
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot find user home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "hexai", "data")
	}

	return filepath.Join(dataDir, "prompts"), nil
}

// expandPath expands ~ to home directory and returns absolute path.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot find user home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	return filepath.Abs(path)
}

// createSyncer creates a slash command syncer from config.
// Returns nil syncer if sync is disabled.
func createSyncer(cfg appconfig.App, logger *log.Logger) (*slashcommands.Syncer, error) {
	syncer, err := slashcommands.NewSyncer(cfg.MCPSection())
	if err != nil {
		return nil, err
	}

	if syncer != nil && cfg.MCPSlashCommandSync {
		logger.Printf("slash command sync enabled: %s", cfg.MCPSlashCommandDir)
	}

	return syncer, nil
}

// RunBackfill performs a one-time sync of all prompts and exits.
// Overrides are applied to the loaded config before use. ctx lets a cancelled
// caller abort config loading before the (blocking) sync work begins.
func RunBackfill(ctx context.Context, logPath, configPath string, overrides MCPOverrides) error {
	logger, err := setupLogger(logPath)
	if err != nil {
		return fmt.Errorf("cannot setup logger: %w", err)
	}
	defer func() {
		if f, ok := logger.Writer().(*os.File); ok && f != os.Stderr {
			f.Close()
		}
	}()

	logger.Printf("hexai-mcp-server backfill starting")

	// Load configuration and apply CLI overrides
	cfg := loadConfig(ctx, logger, configPath)
	applyOverrides(&cfg, overrides)

	// Force enable sync for backfill
	if cfg.MCPSlashCommandDir == "" {
		return fmt.Errorf("commands directory not configured (use --slashcommand-dir)")
	}
	cfg.MCPSlashCommandSync = true

	return executeBackfill(cfg, logger)
}

// executeBackfill creates the syncer, store, and performs the backfill sync.
func executeBackfill(cfg appconfig.App, logger *log.Logger) error {
	syncer, err := createSyncer(cfg, logger)
	if err != nil {
		return fmt.Errorf("cannot create syncer: %w", err)
	}

	promptsDir, err := getPromptsDir(cfg)
	if err != nil {
		return fmt.Errorf("cannot determine prompts directory: %w", err)
	}

	store, err := promptstore.NewJSONLStore(promptsDir)
	if err != nil {
		return fmt.Errorf("cannot create prompt store: %w", err)
	}

	logger.Printf("starting backfill sync...")
	if err := syncer.SyncAll(store); err != nil {
		return fmt.Errorf("backfill failed: %w", err)
	}

	logger.Printf("backfill complete")
	return nil
}
