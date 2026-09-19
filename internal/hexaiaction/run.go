package hexaiaction

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/editor"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/llmutils"
	"github.com/snonux/hexai/internal/logging"
	"github.com/snonux/hexai/internal/stats"
	"github.com/snonux/hexai/internal/tmux"
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
		return fmt.Errorf("no input provided on stdin; pipe the selected text or pane contents into hexai-tmux-action")
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

type actionEditorOpener func(context.Context, []byte) (string, error)

type actionStatusSink interface {
	SetLLMStart(provider, model string) error
}

// Runner executes action requests with injectable dependencies for testability.
type Runner struct {
	chooseAction actionChooser
	newClient    actionClientFactory
	loadConfig   actionConfigLoader
	openEditor   actionEditorOpener
	statusSink   actionStatusSink
}

type actionRunDeps struct {
	chooser    actionChooser
	newClient  actionClientFactory
	loadConfig actionConfigLoader
	openEditor actionEditorOpener
	statusSink actionStatusSink
}

// NewRunner builds a Runner with production dependencies.
func NewRunner() *Runner {
	return &Runner{
		chooseAction: chooseActionFromConfig,
		newClient:    defaultActionClientFactory,
		loadConfig:   loadActionConfig,
		openEditor:   editor.OpenTempAndEdit,
		statusSink:   tmuxActionStatusSink{},
	}
}

func chooseActionFromConfig(cfg appconfig.App) (actionChoice, error) {
	// Config-driven menu takes priority when defined.
	if len(cfg.TmuxActionMenu) > 0 {
		kind, custom, err := RunTUIFromConfig(cfg.TmuxActionMenu, cfg.CustomActions)
		return actionChoice{kind: kind, custom: custom}, err
	}
	// Default path: built-in menu, with optional custom-actions submenu.
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
	return appconfig.LoadWithOptions(ctx, logger, appconfig.LoadOptions{ConfigPath: configPathFromContext(ctx)})
}

type actionEditorKey struct{}

func withActionEditor(ctx context.Context, open actionEditorOpener) context.Context {
	if open == nil {
		open = editor.OpenTempAndEdit
	}
	return context.WithValue(ctx, actionEditorKey{}, open)
}

func actionEditorFromContext(ctx context.Context) actionEditorOpener {
	if open, ok := ctx.Value(actionEditorKey{}).(actionEditorOpener); ok && open != nil {
		return open
	}
	return editor.OpenTempAndEdit
}

func Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	return NewRunner().Run(ctx, stdin, stdout, stderr)
}

func (r *Runner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	deps := r.resolveDeps()
	logger := log.New(stderr, "hexai-tmux-action ", log.LstdFlags|log.Lmsgprefix)
	cfg, err := prepareRunConfig(ctx, stderr, logger, deps.loadConfig)
	if err != nil {
		return err
	}
	client, err := prepareActionClient(stderr, cfg, deps.newClient, deps.statusSink)
	if err != nil {
		return err
	}
	parts, err := ParseInput(stdin)
	if err != nil {
		return fmt.Errorf("hexai-tmux-action: failed to read action input from stdin (pipe the selected text or pane contents into hexai-tmux-action): %w", err)
	}
	if err := requireInput(parts.Selection); err != nil {
		return fmt.Errorf("hexai-tmux-action: %w", err)
	}
	choice, err := deps.chooser(cfg)
	if err != nil {
		return err
	}
	setActionStartStatus(deps.statusSink, client, cfg)
	out, err := executeAction(withActionEditor(ctx, deps.openEditor), choice.kind, parts, &cfg, client, stderr, choice.custom)
	if err != nil {
		return err
	}
	_, _ = io.WriteString(stdout, out)
	return nil
}

func (r *Runner) resolveDeps() actionRunDeps {
	deps := actionRunDeps{
		chooser:    chooseActionFromConfig,
		newClient:  defaultActionClientFactory,
		loadConfig: loadActionConfig,
		openEditor: actionEditorOpener(editor.OpenTempAndEdit),
		statusSink: actionStatusSink(tmuxActionStatusSink{}),
	}
	if r == nil {
		return deps
	}
	return r.applyOverrides(deps)
}

func (r *Runner) applyOverrides(deps actionRunDeps) actionRunDeps {
	if r.chooseAction != nil {
		deps.chooser = r.chooseAction
	}
	if r.newClient != nil {
		deps.newClient = r.newClient
	}
	if r.loadConfig != nil {
		deps.loadConfig = r.loadConfig
	}
	if r.openEditor != nil {
		deps.openEditor = r.openEditor
	}
	if r.statusSink != nil {
		deps.statusSink = r.statusSink
	}
	return deps
}

func prepareRunConfig(ctx context.Context, stderr io.Writer, logger *log.Logger, loadConfig actionConfigLoader) (appconfig.App, error) {
	cfg := loadConfig(ctx, logger)
	if cfg.StatsWindowMinutes > 0 {
		stats.SetWindow(time.Duration(cfg.StatsWindowMinutes) * time.Minute)
	}
	if err := cfg.Validate(); err != nil {
		_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: %v"+logging.AnsiReset+"\n", err)
		return cfg, err
	}
	return cfg, nil
}

func prepareActionClient(stderr io.Writer, cfg appconfig.App, newClient actionClientFactory, statusSink actionStatusSink) (chatDoer, error) {
	entries := cfg.CodeActionConfigs
	if len(entries) == 0 {
		entries = []appconfig.SurfaceConfig{{Provider: cfg.Provider}}
	}
	targets := make([]llm.Target, 0, 2)
	var failures []string
	primary := entries[0]
	if strings.TrimSpace(primary.Provider) == "" {
		primary.Provider = cfg.Provider
	}
	if strings.TrimSpace(primary.Model) == "" {
		primary.Model = strings.TrimSpace(reqOptsFrom(&cfg).model)
	}
	entries = []appconfig.SurfaceConfig{primary}
	if strings.TrimSpace(primary.FallbackProvider) != "" {
		entries = append(entries, appconfig.SurfaceConfig{Provider: primary.FallbackProvider, Model: primary.FallbackModel, Temperature: primary.Temperature})
	}
	for _, entry := range entries {
		provider := strings.TrimSpace(entry.Provider)
		derived := llmutils.ConfigForProvider(cfg, provider, entry.Model)
		client, err := newClient(derived)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: LLM disabled: %v"+logging.AnsiReset+"\n", err)
			failures = append(failures, fmt.Sprintf("%s: %v", provider, err))
			continue
		}
		req := reqOptsFromEntry(&cfg, entry, derived)
		name := strings.TrimSpace(client.Name())
		if name == "" {
			name = provider
		}
		targets = append(targets, llm.Target{Name: name, Model: req.model, Client: client, Options: req.options})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("llm: no code-action targets available: %s", strings.Join(failures, "; "))
	}
	return &resolvedActionClient{targets: targets, active: targets[0]}, nil
}

func setActionStartStatus(statusSink actionStatusSink, client chatDoer, cfg appconfig.App) {
	if statusSink == nil || client == nil {
		return
	}
	model := strings.TrimSpace(reqOptsFrom(&cfg).model)
	if model == "" {
		model = client.DefaultModel()
	}
	_ = statusSink.SetLLMStart(providerOf(client), model)
}

type resolvedActionClient struct {
	targets []llm.Target
	active  llm.Target
}

func (c *resolvedActionClient) Chat(ctx context.Context, msgs []llm.Message, _ ...llm.RequestOption) (string, error) {
	result, err := llm.Chat(ctx, c.targets, msgs)
	if err != nil {
		return "", err
	}
	c.active = result.Target
	return result.Text, nil
}

func (c *resolvedActionClient) Name() string { return c.active.Name }

func (c *resolvedActionClient) DefaultModel() string { return c.active.Model }

func (c *resolvedActionClient) ResponderModel() string { return c.active.Model }

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
	handler, ok := lookupActionHandler(kind)
	if !ok {
		return parts.Selection, nil
	}
	return handler.Execute(ctx, actionRequest{
		parts:          parts,
		cfg:            cfg,
		client:         client,
		stderr:         stderr,
		selectedCustom: selectedCustom,
	})
}

func runWithTimeout(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), fn func(context.Context) (string, error)) (string, error) {
	innerCtx, cancel := timeout(ctx)
	defer cancel()
	return fn(innerCtx)
}

// client construction is shared via internal/llmutils
