// Summary: MCP server orchestrator; loads config, sets up store, and runs server.
package hexaimcp

import (
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

// ServerRunner interface allows dependency injection for testing.
type ServerRunner interface {
	Run() error
}

// ServerFactory creates a server instance (testable).
type ServerFactory func(
	r io.Reader,
	w io.Writer,
	logger *log.Logger,
	store promptstore.PromptStore,
	syncer *slashcommands.Syncer,
) ServerRunner

// defaultServerFactory is the production server factory.
func defaultServerFactory(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer *slashcommands.Syncer) ServerRunner {
	return mcp.NewServer(r, w, logger, store, syncer)
}

// Run starts the MCP server with the given configuration.
// This is the main entry point called from cmd/hexai-mcp-server/main.go.
func Run(logPath, configPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return RunWithFactory(logPath, configPath, stdin, stdout, stderr, defaultServerFactory)
}

// RunWithFactory allows test injection of server factory.
func RunWithFactory(
	logPath string,
	configPath string,
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

	// Load configuration
	cfg := loadConfig(logger, configPath)

	// Determine prompts directory
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
	if err := server.Run(); err != nil {
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
// Returns default config if loading fails.
func loadConfig(logger *log.Logger, configPath string) appconfig.App {
	opts := appconfig.LoadOptions{
		ConfigPath: configPath,
		IgnoreEnv:  false,
	}
	return appconfig.LoadWithOptions(logger, opts)
}

// getPromptsDir determines the prompts directory from config or environment.
// Precedence: CLI flag (via config) > env var > config file > default XDG location.
func getPromptsDir(cfg appconfig.App) (string, error) {
	// Check environment variable first
	if envDir := strings.TrimSpace(os.Getenv("HEXAI_MCP_PROMPTS_DIR")); envDir != "" {
		return expandPath(envDir)
	}

	// Check config file
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
	syncer, err := slashcommands.NewSyncer(cfg)
	if err != nil {
		return nil, err
	}

	if syncer != nil && cfg.MCPSlashCommandSync {
		logger.Printf("slash command sync enabled: %s", cfg.MCPSlashCommandDir)
	}

	return syncer, nil
}

// RunBackfill performs a one-time sync of all prompts and exits.
func RunBackfill(logPath, configPath string) error {
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

	cfg := loadConfig(logger, configPath)

	// Force enable sync for backfill
	if cfg.MCPSlashCommandDir == "" {
		return fmt.Errorf("commands directory not configured (use --slashcommand-dir)")
	}
	cfg.MCPSlashCommandSync = true

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
