// Package main is the Hexai CLI entrypoint; parses flags and delegates to internal/hexaicli.
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
	configPath, remaining := splitConfigPath(os.Args[1:])
	logger := log.New(io.Discard, "", 0)
	cfg := appconfig.LoadWithOptions(logger, appconfig.LoadOptions{ConfigPath: configPath})
	cliEntries := cfg.CLIConfigs
	if len(cliEntries) == 0 {
		cliEntries = []appconfig.SurfaceConfig{{Provider: cfg.Provider}}
	}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	defaultPath := appconfig.DefaultConfigPath()
	configFlag := fs.String("config", configPath, fmt.Sprintf("path to config file (default: %s)", defaultPath))
	tpsSimulation := fs.String("tps-simulation", "", "simulate stdout at a token-per-second rate; accepts '12' or '10-20'")
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
	_ = fs.Parse(remaining)
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
	finalPath := strings.TrimSpace(*configFlag)
	if finalPath == "" {
		finalPath = configPath
	}
	ctx := context.Background()
	if finalPath != "" {
		ctx = hexaicli.WithCLIConfigPath(ctx, finalPath)
	}
	if strings.TrimSpace(*tpsSimulation) != "" {
		ctx = hexaicli.WithCLITPSSimulation(ctx, *tpsSimulation)
	}
	if len(selection) > 0 {
		ctx = hexaicli.WithCLISelection(ctx, selection)
	}
	if err := hexaicli.Run(ctx, fs.Args(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func splitConfigPath(args []string) (string, []string) {
	var path string
	rest := make([]string, 0, len(args))
	skip := false
	for i := 0; i < len(args); i++ {
		if skip {
			skip = false
			continue
		}
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-config":
			if i+1 < len(args) {
				path = args[i+1]
				skip = true
			}
		case strings.HasPrefix(arg, "--config="):
			path = arg[len("--config="):]
		case strings.HasPrefix(arg, "-config="):
			path = arg[len("-config="):]
		default:
			rest = append(rest, arg)
		}
	}
	return strings.TrimSpace(path), rest
}

func pickDefaultModel(cfg appconfig.App, provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return strings.TrimSpace(cfg.OllamaModel)
	case "anthropic":
		return strings.TrimSpace(cfg.AnthropicModel)
	default:
		return strings.TrimSpace(cfg.OpenAIModel)
	}
}
