package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/snonux/hexai/internal"
	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/hexaicli"
)

type configLoader func(string) appconfig.App

type cliRunner func(context.Context, []string, io.Reader, io.Writer, io.Writer) error

type appRunner struct {
	loadConfig configLoader
	runCLI     cliRunner
}

type parsedAppArgs struct {
	args          []string
	finalPath     string
	tpsSimulation string
	selection     []int
	showVersion   bool
}

func newAppRunner() appRunner {
	return appRunner{
		loadConfig: loadAppConfig,
		runCLI:     hexaicli.Run,
	}
}

func (r appRunner) run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	runner := normalizeAppRunner(r)
	configPath, remaining := splitConfigPath(args)
	cfg := runner.loadConfig(configPath)
	parsed, err := parseAppArgs(cfg, configPath, remaining, stderr)
	if err != nil {
		return 2
	}
	if parsed.showVersion {
		fmt.Fprintln(stdout, internal.Version)
		return 0
	}
	ctx := buildCLIContext(parsed)
	if err := runner.runCLI(ctx, parsed.args, stdin, stdout, stderr); err != nil {
		return 1
	}
	return 0
}

func normalizeAppRunner(r appRunner) appRunner {
	if r.loadConfig == nil {
		r.loadConfig = loadAppConfig
	}
	if r.runCLI == nil {
		r.runCLI = hexaicli.Run
	}
	return r
}

func loadAppConfig(configPath string) appconfig.App {
	logger := log.New(io.Discard, "", 0)
	// Config is loaded before the signal-aware CLI context exists, so use a
	// background context here; the load is a quick local file read.
	return appconfig.LoadWithOptions(context.Background(), logger, appconfig.LoadOptions{ConfigPath: configPath})
}

func parseAppArgs(cfg appconfig.App, configPath string, args []string, stderr io.Writer) (parsedAppArgs, error) {
	cliEntries := cliEntriesFromConfig(cfg)
	fs := flag.NewFlagSet("hexai", flag.ContinueOnError)
	fs.SetOutput(stderr)
	defaultPath := appconfig.DefaultConfigPath()
	configFlag := fs.String("config", configPath, fmt.Sprintf("path to config file (default: %s)", defaultPath))
	tpsSimulation := fs.String("tps-simulation", "", "simulate stdout at a token-per-second rate; accepts '12' or '10-20'")
	showVersion := fs.Bool("version", false, "print version and exit")
	selectedFlags := bindProviderFlags(fs, cfg, cliEntries)
	if err := fs.Parse(args); err != nil {
		return parsedAppArgs{}, err
	}
	return parsedAppArgs{
		args:          fs.Args(),
		finalPath:     normalizeConfigPath(*configFlag, configPath),
		tpsSimulation: strings.TrimSpace(*tpsSimulation),
		selection:     selectionFromFlags(selectedFlags),
		showVersion:   *showVersion,
	}, nil
}

func cliEntriesFromConfig(cfg appconfig.App) []appconfig.SurfaceConfig {
	if len(cfg.CLIConfigs) > 0 {
		return cfg.CLIConfigs
	}
	return []appconfig.SurfaceConfig{{Provider: cfg.Provider}}
}

func bindProviderFlags(fs *flag.FlagSet, cfg appconfig.App, cliEntries []appconfig.SurfaceConfig) []bool {
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
	return selectedFlags
}

func normalizeConfigPath(flagValue, fallback string) string {
	finalPath := strings.TrimSpace(flagValue)
	if finalPath != "" {
		return finalPath
	}
	return fallback
}

func selectionFromFlags(selectedFlags []bool) []int {
	var selection []int
	for i, sel := range selectedFlags {
		if sel {
			selection = append(selection, i)
		}
	}
	return selection
}

func buildCLIContext(parsed parsedAppArgs) context.Context {
	ctx := context.Background()
	if parsed.finalPath != "" {
		ctx = hexaicli.WithCLIConfigPath(ctx, parsed.finalPath)
	}
	if parsed.tpsSimulation != "" {
		ctx = hexaicli.WithCLITPSSimulation(ctx, parsed.tpsSimulation)
	}
	if len(parsed.selection) > 0 {
		ctx = hexaicli.WithCLISelection(ctx, parsed.selection)
	}
	return ctx
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
