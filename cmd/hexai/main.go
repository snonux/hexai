// Summary: Hexai CLI entrypoint; parses flags and delegates to internal/hexaicli.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexaicli"
)

func main() {
	logger := log.New(io.Discard, "", 0)
	cfg := appconfig.Load(logger)
	cliEntries := cfg.CLIConfigs
	if len(cliEntries) == 0 {
		cliEntries = []appconfig.SurfaceConfig{{Provider: cfg.Provider}}
	}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	showVersion := fs.Bool("version", false, "print version and exit")
	selectedFlags := make([]bool, len(cliEntries))
	for i, entry := range cliEntries {
		name := strconv.Itoa(i)
		provider := strings.TrimSpace(entry.Provider)
		if provider == "" {
			provider = cfg.Provider
		}
		model := strings.TrimSpace(entry.Model)
		if model == "" {
			model = pickDefaultModel(cfg, provider)
		}
		desc := fmt.Sprintf("use only provider #%d (%s:%s)", i, provider, model)
		fs.BoolVar(&selectedFlags[i], name, false, desc)
	}
	_ = fs.Parse(os.Args[1:])
	if *showVersion {
		fmt.Fprintln(os.Stdout, internal.Version)
		return
	}
	var selection []int
	for i, sel := range selectedFlags {
		if sel {
			selection = append(selection, i)
		}
	}
	ctx := context.Background()
	if len(selection) > 0 {
		ctx = hexaicli.WithCLISelection(ctx, selection)
	}
	if err := hexaicli.Run(ctx, fs.Args(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func pickDefaultModel(cfg appconfig.App, provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return strings.TrimSpace(cfg.OllamaModel)
	case "copilot":
		return strings.TrimSpace(cfg.CopilotModel)
	default:
		return strings.TrimSpace(cfg.OpenAIModel)
	}
}
