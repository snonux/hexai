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

// tmuxActionError formats an error with the hexai-tmux-action prefix for stderr output.
type tmuxActionError struct {
	inner error
}

func (e tmuxActionError) Error() string {
	return logging.AnsiBase + "hexai-tmux-action: " + e.inner.Error() + logging.AnsiReset
}

func (e tmuxActionError) Unwrap() error {
	return e.inner
}

// logTmuxActionError logs an error to stderr with the hexai-tmux-action prefix.
func logTmuxActionError(stderr io.Writer, err error) error {
	_, _ = fmt.Fprintf(stderr, "%v\n", tmuxActionError{err})
	return err
}

// requireInput validates that input selection is not empty.
func requireInput(sel string) error {
	if strings.TrimSpace(sel) == "" {
		return fmt.Errorf("no input provided on stdin")
	}
	return nil
}

// logAndReturnError logs an error to stderr and returns it (hexaiaction pattern).
func logAndReturnError(stderr io.Writer, err error) error {
	return logTmuxActionError(stderr, err)
}

type configPathKey struct{}

type actionChoice struct {
	kind   ActionKind
	custom *appconfig.CustomAction
}

type actionChooser func(cfg appconfig.App) (actionChoice, error)

type actionClient interface {
	chatDoer
	Name() string
}

type actionClientFactory func(cfg appconfig.App) (actionClient, error)

type actionConfigLoader func(context.Context, *log.Logger) appconfig.App

type actionStatusSink interface {
	SetLLMStart(provider, model string) error
}

// Runner executes action requests with injectable dependencies for testability.
type Runner struct {
	chooseAction actionChooser
	newClient    actionClientFactory
	loadConfig   actionConfigLoader
	statusSink   actionStatusSink
}

// NewRunner builds a Runner with production dependencies.
func NewRunner() *Runner {
	return &Runner{
		chooseAction: chooseActionFromConfig,
		newClient:    defaultActionClientFactory,
		loadConfig:   loadActionConfig,
		statusSink:   tmuxActionStatusSink{},
	}
}

func chooseActionFromConfig(cfg appconfig.App) (actionChoice, error) {
	if len(cfg.CustomActions) == 0 {
		kind, err := RunTUI()
		return actionChoice{kind: kind}, err
	}
	kind, custom, err := RunTUIWithCustom(cfg.CustomActions, cfg.TmuxCustomMenuHotkey)
	return actionChoice{kind: kind, custom: custom}, err
}

func defaultActionClientFactory(cfg appconfig.App) (actionClient, error) {
	return llmutils.NewClientFromApp(cfg)
}

type tmuxActionStatusSink struct{}

func (tmuxActionStatusSink) SetLLMStart(provider, model string) error {
	return tmux.SetStatus(tmux.FormatLLMStartStatus(provider, model))
}

func loadActionConfig(ctx context.Context, logger *log.Logger) appconfig.App {
	return appconfig.LoadWithOptions(logger, appconfig.LoadOptions{ConfigPath: configPathFromContext(ctx)})
}

type actionPlan struct {
	fallback string
	run      func(context.Context) (string, error)
}

// CodeActionHandler builds a plan for an action and resolves it.
type CodeActionHandler interface {
	Build(parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (actionPlan, bool)
	Resolve(ctx context.Context, plan actionPlan) (string, error)
}

type codeActionHandler struct {
	build func(parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (actionPlan, bool)
}

func (h codeActionHandler) Build(parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (actionPlan, bool) {
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
	return NewRunner().Run(ctx, stdin, stdout, stderr)
}

func (r *Runner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	chooser := chooseActionFromConfig
	newClient := defaultActionClientFactory
	loadConfig := loadActionConfig
	statusSink := actionStatusSink(tmuxActionStatusSink{})
	if r != nil {
		if r.chooseAction != nil {
			chooser = r.chooseAction
		}
		if r.newClient != nil {
			newClient = r.newClient
		}
		if r.loadConfig != nil {
			loadConfig = r.loadConfig
		}
		if r.statusSink != nil {
			statusSink = r.statusSink
		}
	}

	logger := log.New(stderr, "hexai-tmux-action ", log.LstdFlags|log.Lmsgprefix)
	cfg := loadConfig(ctx, logger)
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	if err := cfg.Validate(); err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	if len(cfg.CodeActionConfigs) > 0 {
		if provider := strings.TrimSpace(cfg.CodeActionConfigs[0].Provider); provider != "" {
			cfg.Provider = provider
		}
	}
	cli, err := newClient(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	primaryModel := strings.TrimSpace(reqOptsFrom(&cfg).model)
	if primaryModel == "" {
		primaryModel = cli.DefaultModel()
	}
	if statusSink != nil {
		_ = statusSink.SetLLMStart(cli.Name(), primaryModel)
	}
	var client chatDoer = cli
	parts, err := ParseInput(stdin)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: failed to read input"+logging.AnsiReset)
		return err
	}
	if strings.TrimSpace(parts.Selection) == "" {
		return fmt.Errorf("hexai-tmux-action: no input provided on stdin")
	}
	choice, err := chooser(cfg)
	if err != nil {
		return err
	}
	out, err := executeAction(ctx, choice.kind, parts, &cfg, client, stderr, choice.custom)
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

func executeAction(ctx context.Context, kind ActionKind, parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer, selectedCustom *appconfig.CustomAction) (string, error) {
	if kind == ActionCustom {
		return handleCustomAction(ctx, parts, cfg, client, selectedCustom)
	}
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
		ActionCustomPrompt: codeActionHandler{build: buildCustomPromptPlan},
	}
}

func buildSkipPlan(parts InputParts, _ actionConfig, _ chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{fallback: parts.Selection}, true
}

func buildRewritePlan(parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleRewriteAction(ctx, parts, cfg, client, stderr)
		},
	}, true
}

func buildDiagnosticsPlan(parts InputParts, cfg actionConfig, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleDiagnosticsAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildDocumentPlan(parts InputParts, cfg actionConfig, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleDocumentAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildGoTestPlan(parts InputParts, cfg actionConfig, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleGoTestAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildSimplifyPlan(parts InputParts, cfg actionConfig, client chatDoer, _ io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleSimplifyAction(ctx, parts, cfg, client)
		},
	}, true
}

func buildCustomPromptPlan(parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (actionPlan, bool) {
	return actionPlan{
		fallback: parts.Selection,
		run: func(ctx context.Context) (string, error) {
			return handleCustomPromptAction(ctx, parts, cfg, client, stderr)
		},
	}, true
}

func handleRewriteAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (string, error) {
	instr, cleaned := ExtractInstruction(parts.Selection)
	if strings.TrimSpace(instr) == "" {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: no inline instruction found; echoing input"+logging.AnsiReset)
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runRewrite(cctx, cfg, client, instr, cleaned)
	})
}

func handleDiagnosticsAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runDiagnostics(cctx, cfg, client, parts.Diagnostics, parts.Selection)
	})
}

func handleDocumentAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runDocument(cctx, cfg, client, parts.Selection)
	})
}

func handleGoTestAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout18s, func(cctx context.Context) (string, error) {
		return runGoTest(cctx, cfg, client, parts.Selection)
	})
}

func handleSimplifyAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runSimplify(cctx, cfg, client, parts.Selection)
	})
}

func handleCustomAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer, selectedCustom *appconfig.CustomAction) (string, error) {
	if selectedCustom == nil {
		return parts.Selection, nil
	}
	custom := *selectedCustom
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runCustom(cctx, cfg, client, custom, parts)
	})
}

func handleCustomPromptAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (string, error) {
	prompt, err := editor.OpenTempAndEdit(nil)
	if err != nil || strings.TrimSpace(prompt) == "" {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: custom prompt canceled or empty; echoing input"+logging.AnsiReset)
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runRewrite(cctx, cfg, client, prompt, parts.Selection)
	})
}

func runWithTimeout(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), fn func(context.Context) (string, error)) (string, error) {
	innerCtx, cancel := timeout(ctx)
	defer cancel()
	return fn(innerCtx)
}

// client construction is shared via internal/llmutils
