// Package main is the Hexai MCP server entrypoint; parses flags and delegates to internal/hexaimcp.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexaimcp"
)

// buildOverrides constructs MCPOverrides from parsed CLI options.
func buildOverrides(opts mcpOptions) hexaimcp.MCPOverrides {
	return hexaimcp.MCPOverrides{
		PromptsDir:       opts.promptsDir,
		SlashCommandSync: opts.slashCommandSync,
		SlashCommandDir:  opts.slashCommandDir,
	}
}

// deprecationWarning is the notice runMain emits on every startup so users
// see this binary is experimental. Kept as a constant (not printf'd) so
// tests can assert on its contents directly.
const deprecationWarning = `
⚠️  DEPRECATION NOTICE ⚠️

hexai-mcp-server is currently EXPERIMENTAL and NOT ACTIVELY MAINTAINED.

The author does not have a real use case for this MCP server at this time.
Prompts are now managed through slash commands and meta-commands in the
main hexai agent system, making this MCP server's prompt management
functionality redundant.

This code is kept in the repository for potential future enhancements
(possibly adding more useful functionality than prompt management), but
no guarantees are made about its stability or continued support.

Use at your own risk.

────────────────────────────────────────────────────────────────────────
`

// mcpOptions holds the parsed command-line flags for the MCP server.
type mcpOptions struct {
	logPath          string
	configPath       string
	promptsDir       string
	slashCommandSync bool
	slashCommandDir  string
	syncAll          bool
	showVersion      bool
}

type mcpDeps struct {
	runMCP      func(context.Context, string, string, hexaimcp.MCPOverrides, io.Reader, io.Writer, io.Writer) error
	runBackfill func(context.Context, string, string, hexaimcp.MCPOverrides) error
}

func defaultMCPDeps() mcpDeps {
	return mcpDeps{
		runMCP:      hexaimcp.Run,
		runBackfill: hexaimcp.RunBackfill,
	}
}

func main() { os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// runMain prints the deprecation warning, parses flags, and delegates to
// run. It returns the process exit code: 2 for flag-parse errors (matching
// stdlib `flag.ExitOnError`), 1 for state-dir or run failures, 0 on success.
// Pulling this out of main keeps it testable without touching package-level
// flag state.
func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runMainWithDeps(args, stdin, stdout, stderr, defaultMCPDeps())
}

func runMainWithDeps(args []string, stdin io.Reader, stdout, stderr io.Writer, deps mcpDeps) int {
	fmt.Fprint(stderr, deprecationWarning)

	defaultLog, err := defaultLogPath()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fs := flag.NewFlagSet("hexai-mcp-server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	logPath := fs.String("log", defaultLog, "path to log file (optional)")
	configPath := fs.String("config", "", "path to config file (optional)")
	promptsDir := fs.String("prompts-dir", "", "path to prompts directory (optional)")
	slashCommandSync := fs.Bool("slashcommand-sync", false, "enable slash command sync")
	slashCommandDir := fs.String("slashcommand-dir", "", "directory for slash command files")
	syncAll := fs.Bool("sync-all", false, "backfill all existing prompts and exit")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := mcpOptions{
		logPath:          *logPath,
		configPath:       *configPath,
		promptsDir:       *promptsDir,
		slashCommandSync: *slashCommandSync,
		slashCommandDir:  *slashCommandDir,
		syncAll:          *syncAll,
		showVersion:      *showVersion,
	}
	// Cancel the run on interrupt/terminate so the serve loop exits cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runWithDeps(ctx, opts, stdin, stdout, stderr, deps); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// run executes the MCP server logic with the given options and I/O streams.
// CLI flag values are passed via MCPOverrides instead of environment variables.
// ctx is threaded into the server/backfill so they stop on shutdown signals.
func run(ctx context.Context, opts mcpOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	return runWithDeps(ctx, opts, stdin, stdout, stderr, defaultMCPDeps())
}

func runWithDeps(ctx context.Context, opts mcpOptions, stdin io.Reader, stdout, stderr io.Writer, deps mcpDeps) error {
	if opts.showVersion {
		fmt.Fprintln(stdout, internal.Version)
		return nil
	}

	overrides := buildOverrides(opts)

	// Handle backfill operation
	if opts.syncAll {
		return deps.runBackfill(ctx, opts.logPath, opts.configPath, overrides)
	}

	return deps.runMCP(ctx, opts.logPath, opts.configPath, overrides, stdin, stdout, stderr)
}

// defaultLogPath returns the default MCP log file path in the state directory.
// Returns an error if the state directory cannot be created.
func defaultLogPath() (string, error) {
	stateDir, err := appconfig.StateDir()
	if err != nil {
		return "", fmt.Errorf("cannot create state directory: %w", err)
	}
	return filepath.Join(stateDir, "hexai-mcp-server.log"), nil
}
