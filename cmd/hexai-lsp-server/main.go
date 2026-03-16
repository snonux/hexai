// Summary: Hexai LSP entrypoint; parses flags and delegates to internal/hexailsp.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexailsp"
)

func main() {
	defaultLog := defaultLogPath()
	logPath := flag.String("log", defaultLog, "path to log file (optional)")
	defaultCfg := appconfig.DefaultConfigPath()
	configPath := flag.String("config", "", fmt.Sprintf("path to config file (default: %s)", defaultCfg))
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		log.Println(internal.Version)
		return
	}

	path := strings.TrimSpace(*configPath)
	if err := hexailsp.RunWithConfig(*logPath, path, os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// defaultLogPath returns the default LSP log file path in the state directory.
// Falls back to the system temp directory if the state directory is unavailable.
func defaultLogPath() string {
	stateDir, err := appconfig.StateDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "hexai-lsp-server.log")
	}
	return filepath.Join(stateDir, "hexai-lsp-server.log")
}
