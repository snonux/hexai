package hexaiaction

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llmutils"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/tmux"
)

// Run executes the hexai-tmux-action command flow.
// seams for testability
var (
	chooseActionFn   = RunTUI
	newClientFromApp = llmutils.NewClientFromApp
)

// selectedCustom carries the chosen custom action (if any) from the TUI submenu
// to the executor. Cleared after use.
var selectedCustom *appconfig.CustomAction

func Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	logger := log.New(stderr, "hexai-tmux-action ", log.LstdFlags|log.Lmsgprefix)
	cfg := appconfig.Load(logger)
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	// Enable custom action submenu with configurable hotkey
	if len(cfg.CustomActions) > 0 {
		chooseActionFn = func() (ActionKind, error) { return RunTUIWithCustom(cfg.CustomActions, cfg.TmuxCustomMenuHotkey) }
	}
	cli, err := newClientFromApp(cfg)
	if err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	_ = tmux.SetStatus(tmux.FormatLLMStartStatus(cli.Name(), cli.DefaultModel()))
	var client chatDoer = cli
	parts, err := ParseInput(stdin)
	if err != nil {
		fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: failed to read input"+logging.AnsiReset)
		return err
	}
	if strings.TrimSpace(parts.Selection) == "" {
		return fmt.Errorf("hexai-tmux-action: no input provided on stdin")
	}
	kind, err := chooseActionFn()
	if err != nil {
		return err
	}
	out, err := executeAction(ctx, kind, parts, cfg, client, stderr)
	if err != nil {
		return err
	}
	io.WriteString(stdout, out)
	return nil
}

func executeAction(ctx context.Context, kind ActionKind, parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (string, error) {
	switch kind {
	case ActionSkip:
		return parts.Selection, nil
	case ActionRewrite:
		instr, cleaned := ExtractInstruction(parts.Selection)
		if strings.TrimSpace(instr) == "" {
			fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: no inline instruction found; echoing input"+logging.AnsiReset)
			return parts.Selection, nil
		}
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runRewrite(cctx, cfg, client, instr, cleaned)
	case ActionDiagnostics:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runDiagnostics(cctx, cfg, client, parts.Diagnostics, parts.Selection)
	case ActionDocument:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runDocument(cctx, cfg, client, parts.Selection)
	case ActionGoTest:
		cctx, cancel := timeout8s(ctx)
		defer cancel()
		return runGoTest(cctx, cfg, client, parts.Selection)
	case ActionSimplify:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runSimplify(cctx, cfg, client, parts.Selection)
	case ActionCustom:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		if selectedCustom != nil {
			// Run configured custom action
			out, err := runCustom(cctx, cfg, client, *selectedCustom, parts)
			selectedCustom = nil // clear after use
			return out, err
		}
		// No selected custom; treat as no-op
		return parts.Selection, nil
	case ActionCustomPrompt:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		// Open editor for free-form instruction
		prompt, err := editor.OpenTempAndEdit(nil)
		if err != nil || strings.TrimSpace(prompt) == "" {
			fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: custom prompt canceled or empty; echoing input"+logging.AnsiReset)
			return parts.Selection, nil
		}
		return runRewrite(cctx, cfg, client, prompt, parts.Selection)
	default:
		return parts.Selection, nil
	}
}

// client construction is shared via internal/llmutils
