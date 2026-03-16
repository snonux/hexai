// Summary: Hexai MCP server entrypoint; parses flags and delegates to internal/hexaimcp.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// Seams for testing: override in tests to avoid launching real MCP server.
// Signatures match hexaimcp.Run and hexaimcp.RunBackfill respectively.
var (
	runMCP      = hexaimcp.Run
	runBackfill = hexaimcp.RunBackfill
)

// printDeprecationWarning outputs a deprecation notice to stderr explaining
// that hexai-mcp-server is experimental and not actively maintained.
func printDeprecationWarning() {
	warning := `
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
	fmt.Fprintln(os.Stderr, warning)
}

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

func main() {
	printDeprecationWarning()

	defaultLog, err := defaultLogPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	logPath := flag.String("log", defaultLog, "path to log file (optional)")
	configPath := flag.String("config", "", "path to config file (optional)")
	promptsDir := flag.String("prompts-dir", "", "path to prompts directory (optional)")
	slashCommandSync := flag.Bool("slashcommand-sync", false, "enable slash command sync")
	slashCommandDir := flag.String("slashcommand-dir", "", "directory for slash command files")
	syncAll := flag.Bool("sync-all", false, "backfill all existing prompts and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	opts := mcpOptions{
		logPath:          *logPath,
		configPath:       *configPath,
		promptsDir:       *promptsDir,
		slashCommandSync: *slashCommandSync,
		slashCommandDir:  *slashCommandDir,
		syncAll:          *syncAll,
		showVersion:      *showVersion,
	}
	if err := run(opts, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run executes the MCP server logic with the given options and I/O streams.
// CLI flag values are passed via MCPOverrides instead of environment variables.
func run(opts mcpOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	if opts.showVersion {
		fmt.Fprintln(stdout, internal.Version)
		return nil
	}

	overrides := buildOverrides(opts)

	// Handle backfill operation
	if opts.syncAll {
		return runBackfill(opts.logPath, opts.configPath, overrides)
	}

	return runMCP(opts.logPath, opts.configPath, overrides, stdin, stdout, stderr)
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
