// Summary: Hexai MCP server entrypoint; parses flags and delegates to internal/hexaimcp.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexaimcp"
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

func main() {
	printDeprecationWarning()

	defaultLog := defaultLogPath()
	logPath := flag.String("log", defaultLog, "path to log file (optional)")
	configPath := flag.String("config", "", "path to config file (optional)")
	promptsDir := flag.String("prompts-dir", "", "path to prompts directory (optional)")
	slashCommandSync := flag.Bool("slashcommand-sync", false, "enable slash command sync")
	slashCommandDir := flag.String("slashcommand-dir", "", "directory for slash command files")
	syncAll := flag.Bool("sync-all", false, "backfill all existing prompts and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(internal.Version)
		return
	}

	// If prompts-dir is specified, set environment variable for RunWithFactory
	if *promptsDir != "" {
		os.Setenv("HEXAI_MCP_PROMPTS_DIR", *promptsDir)
	}
	if *slashCommandSync {
		os.Setenv("HEXAI_MCP_SLASHCOMMAND_SYNC", "true")
	}
	if *slashCommandDir != "" {
		os.Setenv("HEXAI_MCP_SLASHCOMMAND_DIR", *slashCommandDir)
	}

	// Handle backfill operation
	if *syncAll {
		if err := hexaimcp.RunBackfill(*logPath, *configPath); err != nil {
			log.Fatalf("backfill error: %v", err)
		}
		return
	}

	if err := hexaimcp.Run(*logPath, *configPath, os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// defaultLogPath returns the default MCP log file path in the state directory.
// Panics if state directory cannot be created.
func defaultLogPath() string {
	stateDir, err := appconfig.StateDir()
	if err != nil {
		panic(fmt.Sprintf("cannot create state directory: %v", err))
	}
	return fmt.Sprintf("%s/hexai-mcp-server.log", stateDir)
}
