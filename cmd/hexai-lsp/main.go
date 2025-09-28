// Summary: Hexai LSP entrypoint; parses flags and delegates to internal/hexailsp.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexailsp"
)

func main() {
	logPath := flag.String("log", "/tmp/hexai-lsp.log", "path to log file (optional)")
	defaultCfg := defaultConfigPath()
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

func defaultConfigPath() string {
	path, err := appconfig.ConfigPath()
	if err != nil {
		return "$XDG_CONFIG_HOME/hexai/config.toml"
	}
	return path
}
