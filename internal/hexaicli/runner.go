package hexaicli

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/tmux"
)

type cliConfigLoader func(context.Context, *log.Logger) appconfig.App

type cliEditorOpener func([]byte) (string, error)

type cliClientFactory func(appconfig.App) (llm.Client, error)

type cliStatusSink interface {
	SetLLMStart(provider, model string) error
	SetGlobal(snapshot stats.Snapshot, provider, model string, scopeRPM float64, scopeReq int64) error
}

// Runner executes the CLI with injectable configuration, editor, client, and status dependencies.
type Runner struct {
	loadConfig cliConfigLoader
	openEditor cliEditorOpener
	newClient  cliClientFactory
	statusSink cliStatusSink
}

type tmuxCLIStatusSink struct{}

func (tmuxCLIStatusSink) SetLLMStart(provider, model string) error {
	return tmux.SetStatus(tmux.FormatLLMStartStatus(provider, model))
}

func (tmuxCLIStatusSink) SetGlobal(snapshot stats.Snapshot, provider, model string, scopeRPM float64, scopeReq int64) error {
	return tmux.SetStatus(tmux.FormatGlobalStatusColored(
		snapshot.Global.Reqs,
		snapshot.RPM,
		snapshot.Global.Sent,
		snapshot.Global.Recv,
		provider,
		model,
		scopeRPM,
		scopeReq,
		snapshot.Window,
	))
}

// NewRunner builds a CLI runner with production dependencies.
func NewRunner() *Runner {
	return &Runner{
		loadConfig: loadConfigFromContext,
		openEditor: editor.OpenTempAndEdit,
		newClient:  newClientFromApp,
		statusSink: tmuxCLIStatusSink{},
	}
}

func (r *Runner) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	runner := normalizeRunner(r)
	if len(args) > 0 {
		sub := args[0]
		if sub == "config" {
			if len(args) > 1 {
				err := fmt.Errorf(`hexai %s: unexpected arguments (use only %q)`, sub, sub)
				_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"%v"+logging.AnsiReset+"\n", err)
				return err
			}
			cfgPath := strings.TrimSpace(configPathFromContext(ctx))
			if cfgPath == "" {
				p, pathErr := appconfig.ConfigPath()
				if pathErr != nil {
					err := fmt.Errorf("hexai %s: %w", sub, pathErr)
					_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"%v"+logging.AnsiReset+"\n", err)
					return err
				}
				cfgPath = p
			}
			if err := editor.OpenFile(cfgPath); err != nil {
				_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai %s: %v"+logging.AnsiReset+"\n", sub, err)
				return err
			}
			return nil
		}
	}
	if spec, ok, err := tpsSimulationFromContext(ctx); err != nil {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+err.Error()+logging.AnsiReset)
		return err
	} else if ok {
		input, inputErr := readSimulationInput(stdin, args)
		if inputErr != nil {
			_, _ = fmt.Fprintln(stderr, logging.AnsiBase+inputErr.Error()+logging.AnsiReset)
			return inputErr
		}
		return runTPSSimulation(ctx, spec, input, stdout)
	}

	logger := log.New(io.Discard, "", 0)
	cfg := runner.loadConfig(ctx, logger)
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	jobs, err := buildCLIJobs(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	if selected := selectionFromContext(ctx); len(selected) > 0 {
		jobs, err = filterJobsBySelection(jobs, selected)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai: %v"+logging.AnsiReset+"\n", err)
			return err
		}
	}
	if len(jobs) == 0 {
		return fmt.Errorf("hexai: no CLI providers configured")
	}

	input, rerr := readInput(stdin, args)
	if rerr != nil && len(args) == 0 {
		if prompt, eerr := runner.openEditor(nil); eerr == nil && strings.TrimSpace(prompt) != "" {
			args = []string{prompt}
			input, rerr = readInput(stdin, args)
		}
	}
	if rerr != nil {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+rerr.Error()+logging.AnsiReset)
		return rerr
	}
	msgs := buildMessagesFromConfig(cfg, input)
	if err := runCLIJobs(ctx, jobs, msgs, input, stdout, stderr, runner.newClient, runner.statusSink); err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	return nil
}

func (r *Runner) RunWithClient(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, client llm.Client) error {
	runner := normalizeRunner(r)
	input, err := readInput(stdin, args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+err.Error()+logging.AnsiReset)
		return err
	}
	req := requestArgs{model: strings.TrimSpace(client.DefaultModel())}
	printProviderInfo(stderr, client, req.model)
	msgs := buildMessages(input)
	if err := runChatWithStatus(runner.statusSink, ctx, client, req, msgs, input, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	return nil
}

func normalizeRunner(r *Runner) Runner {
	if r == nil {
		return *NewRunner()
	}
	runner := *r
	if runner.loadConfig == nil {
		runner.loadConfig = loadConfigFromContext
	}
	if runner.openEditor == nil {
		runner.openEditor = editor.OpenTempAndEdit
	}
	if runner.newClient == nil {
		runner.newClient = newClientFromApp
	}
	if runner.statusSink == nil {
		runner.statusSink = tmuxCLIStatusSink{}
	}
	return runner
}

func loadConfigFromContext(ctx context.Context, logger *log.Logger) appconfig.App {
	return appconfig.LoadWithOptions(logger, appconfig.LoadOptions{ConfigPath: configPathFromContext(ctx)})
}
