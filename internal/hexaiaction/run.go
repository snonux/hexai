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

type configPathKey struct{}

// selectedCustom carries the chosen custom action (if any) from the TUI submenu
// to the executor. Cleared after use.
var selectedCustom *appconfig.CustomAction

type actionPlan struct {
	fallback string
	run      func(context.Context) (string, error)
}

// CodeActionHandler builds a plan for an action and resolves it.
type CodeActionHandler interface {
	Build(parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (actionPlan, bool)
	Resolve(ctx context.Context, plan actionPlan) (string, error)
}

type codeActionHandler struct {
	build func(parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (actionPlan, bool)
}

func (h codeActionHandler) Build(parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (actionPlan, bool) {
	if h.build == nil {
		return actionPlan{}, false
	}
	return h.build(parts, cfg, client, stderr)
}

func (h codeActionHandler) Resolve(ctx context.Context, plan actionPlan) (string, error) {
	if plan.run == nil {
		return plan.fallback, nil
	}
	return plan.run(ctx)
}

func Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	logger := log.New(stderr, "hexai-tmux-action ", log.LstdFlags|log.Lmsgprefix)
	cfg := appconfig.LoadWithOptions(logger, appconfig.LoadOptions{ConfigPath: configPathFromContext(ctx)})
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	if err := cfg.Validate(); err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	// Enable custom action submenu with configurable hotkey
	if len(cfg.CustomActions) > 0 {
		chooseActionFn = func() (ActionKind, error) { return RunTUIWithCustom(cfg.CustomActions, cfg.TmuxCustomMenuHotkey) }
	}
	if len(cfg.CodeActionConfigs) > 0 {
		if provider := strings.TrimSpace(cfg.CodeActionConfigs[0].Provider); provider != "" {
			cfg.Provider = provider
		}
	}
	cli, err := newClientFromApp(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	primaryModel := strings.TrimSpace(reqOptsFrom(cfg).model)
	if primaryModel == "" {
		primaryModel = cli.DefaultModel()
	}
	_ = tmux.SetStatus(tmux.FormatLLMStartStatus(cli.Name(), primaryModel))
	var client chatDoer = cli
	parts, err := ParseInput(stdin)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: failed to read input"+logging.AnsiReset)
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
	_, _ = io.WriteString(stdout, out)
	return nil
}

// WithConfigPath attaches a config path override to the context for Run/RunCommand.
func WithConfigPath(ctx context.Context, path string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, configPathKey{}, strings.TrimSpace(path))
}

func configPathFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(configPathKey{}).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func executeAction(ctx context.Context, kind ActionKind, parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (string, error) {
	handler, ok := codeActionHandlers()[kind]
	if !ok {
		return parts.Selection, nil
	}
	plan, ok := handler.Build(parts, cfg, client, stderr)
	if !ok {
		return parts.Selection, nil
	}
	return handler.Resolve(ctx, plan)
}

func codeActionHandlers() map[ActionKind]CodeActionHandler {
	return map[ActionKind]CodeActionHandler{
		ActionSkip:         codeActionHandler{build: buildSkipPlan},
		ActionRewrite:      codeActionHandler{build: buildRewritePlan},
		ActionDiagnostics:  codeActionHandler{build: buildDiagnosticsPlan},
		ActionDocument:     codeActionHandler{build: buildDocumentPlan},
		ActionGoTest:       codeActionHandler{build: buildGoTestPlan},
		ActionSimplify:     codeActionHandler{build: buildSimplifyPlan},
		ActionCustom:       codeActionHandler{build: buildCustomPlan},
		ActionCustomPrompt: codeActionHandler{build: buildCustomPromptPlan},
	}
}

func buildSkipPlan(parts InputParts, _ appconfig.App, _ chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{fallback: parts.Selection}, true
}

func buildRewritePlan(parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleRewriteAction(ctx, parts, cfg, client, stderr)
		},
	}, true
}

func buildDiagnosticsPlan(parts InputParts, cfg appconfig.App, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleDiagnosticsAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildDocumentPlan(parts InputParts, cfg appconfig.App, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleDocumentAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildGoTestPlan(parts InputParts, cfg appconfig.App, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleGoTestAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildSimplifyPlan(parts InputParts, cfg appconfig.App, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleSimplifyAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildCustomPlan(parts InputParts, cfg appconfig.App, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleCustomAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildCustomPromptPlan(parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleCustomPromptAction(ctx, parts, cfg, client, stderr)
		},
	}, true
}

func handleRewriteAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (string, error) {
	instr, cleaned := ExtractInstruction(parts.Selection)
	if strings.TrimSpace(instr) == "" {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: no inline instruction found; echoing input"+logging.AnsiReset)
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout10s, func(cctx context.Context) (string, error) {
		return runRewrite(cctx, cfg, client, instr, cleaned)
	})
}

func handleDiagnosticsAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout10s, func(cctx context.Context) (string, error) {
		return runDiagnostics(cctx, cfg, client, parts.Diagnostics, parts.Selection)
	})
}

func handleDocumentAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout10s, func(cctx context.Context) (string, error) {
		return runDocument(cctx, cfg, client, parts.Selection)
	})
}

func handleGoTestAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout8s, func(cctx context.Context) (string, error) {
		return runGoTest(cctx, cfg, client, parts.Selection)
	})
}

func handleSimplifyAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout10s, func(cctx context.Context) (string, error) {
		return runSimplify(cctx, cfg, client, parts.Selection)
	})
}

func handleCustomAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer) (string, error) {
	if selectedCustom == nil {
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout10s, func(cctx context.Context) (string, error) {
		out, err := runCustom(cctx, cfg, client, *selectedCustom, parts)
		selectedCustom = nil
		return out, err
	})
}

func handleCustomPromptAction(ctx context.Context, parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (string, error) {
	prompt, err := editor.OpenTempAndEdit(nil)
	if err != nil || strings.TrimSpace(prompt) == "" {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: custom prompt canceled or empty; echoing input"+logging.AnsiReset)
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout10s, func(cctx context.Context) (string, error) {
		return runRewrite(cctx, cfg, client, prompt, parts.Selection)
	})
}

func runWithTimeout(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), fn func(context.Context) (string, error)) (string, error) {
	innerCtx, cancel := timeout(ctx)
	defer cancel()
	return fn(innerCtx)
}

// client construction is shared via internal/llmutils
