package appconfig

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ProjectConfigFilename is the name of the per-project config file placed at a git repo root.
const ProjectConfigFilename = ".hexaiconfig.toml"

// Load reads configuration from a file and merges with defaults.
// It respects the XDG Base Directory Specification.
func Load(logger *log.Logger) App { return LoadWithOptions(logger, LoadOptions{}) }

// LoadWithOptions reads configuration and applies the requested loading options.
func LoadWithOptions(logger *log.Logger, opts LoadOptions) App {
	cfg := newDefaultConfig()
	if logger == nil {
		return cfg // Return defaults if no logger is provided (e.g. in tests)
	}

	// Step 1: Load global config file
	configPath := strings.TrimSpace(opts.ConfigPath)
	if configPath != "" {
		if fileCfg, err := loadFromFile(configPath, logger); err == nil && fileCfg != nil {
			cfg.mergeWith(fileCfg)
		} else if err != nil {
			logger.Printf("cannot open config file %s: %v", configPath, err)
		}
	} else {
		path, err := getConfigPath()
		if err != nil {
			logger.Printf("%v", err)
		} else if fileCfg, err := loadFromFile(path, logger); err == nil && fileCfg != nil {
			cfg.mergeWith(fileCfg)
		}
	}

	// Step 2: Load per-project config (.hexaiconfig.toml at git repo root).
	// Project config overrides global config but is itself overridden by env vars.
	loadProjectConfig(logger, opts, &cfg)

	// Step 3: Environment overrides (always take precedence over all config files)
	if !opts.IgnoreEnv {
		if envCfg := loadFromEnv(logger); envCfg != nil {
			cfg.mergeWith(envCfg)
		}
	}
	return cfg
}

// ConfigPath returns the default config file path
// ($XDG_CONFIG_HOME/hexai/config.toml or ~/.config/hexai/config.toml).
func ConfigPath() (string, error) {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "hexai", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "hexai", "config.toml"), nil
}

// StateDir returns the XDG state directory for hexai (~/.local/hexai/state by default).
// Creates the directory if it doesn't exist. This is used for persistent state data
// like logs and history that should survive reboots.
func StateDir() (string, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot find user home directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "hexai")
	}

	stateDir := filepath.Join(stateHome, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create state directory: %w", err)
	}
	return stateDir, nil
}

// ProjectConfigPath returns the path to the per-project config file if a git repository
// root is detected from the current working directory. Returns empty string otherwise.
func ProjectConfigPath() string {
	root := FindGitRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, ProjectConfigFilename)
}

// FindGitRoot walks up from the current working directory to find the nearest
// .git directory or file (worktrees use a .git file), returning its parent
// path or "" if none is found.
func FindGitRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil &&
			(info.IsDir() || info.Mode().IsRegular()) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root
		}
		dir = parent
	}
}

func getConfigPath() (string, error) {
	return ConfigPath()
}

// loadProjectConfig attempts to load .hexaiconfig.toml from the project root and
// merges it into cfg. Uses opts.ProjectRoot if set, otherwise auto-detects via FindGitRoot().
func loadProjectConfig(logger *log.Logger, opts LoadOptions, cfg *App) {
	projectRoot := strings.TrimSpace(opts.ProjectRoot)
	if projectRoot == "" {
		projectRoot = FindGitRoot()
	}
	if projectRoot == "" {
		return
	}
	projectCfgPath := filepath.Join(projectRoot, ProjectConfigFilename)
	if projCfg, err := loadFromFile(projectCfgPath, logger); err == nil && projCfg != nil {
		cfg.mergeWith(projCfg)
	}
}

// loadFromFile reads a TOML config file, validates it, and returns the parsed App.
// Returns (nil, err) on I/O or parse errors; returns (nil, nil) when the file does not exist.
func loadFromFile(path string, logger *log.Logger) (*App, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) && logger != nil {
			logger.Printf("cannot open TOML config file %s: %v", path, err)
		}
		return nil, err
	}

	tables, raw, err := decodeTOML(b, path, logger)
	if err != nil {
		return nil, err
	}
	if err := rejectLegacyKeys(raw); err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Printf("loaded configuration from %s (TOML)", path)
	}

	tab := tables.toApp()
	applyRawIntOverrides(raw, &tab)
	if m := parseSurfaceModels(raw, logger); m != nil {
		tab.mergeSurfaceModels(m)
	}
	return &tab, nil
}

// decodeTOML parses raw TOML bytes into both the typed fileConfig and a raw map
// for validation and defensive integer handling.
func decodeTOML(b []byte, path string, logger *log.Logger) (*fileConfig, map[string]any, error) {
	var tables fileConfig
	errTables := toml.NewDecoder(strings.NewReader(string(b))).Decode(&tables)
	var raw map[string]any
	errRaw := toml.Unmarshal(b, &raw)
	if errTables != nil {
		if logger != nil {
			logger.Printf("invalid TOML config file %s: %v", path, errTables)
		}
		return nil, nil, errTables
	}
	if errRaw != nil {
		if logger != nil {
			logger.Printf("invalid TOML config file %s: %v", path, errRaw)
		}
		return nil, nil, errRaw
	}
	return &tables, raw, nil
}

// rejectLegacyKeys returns an error if the raw map contains flat keys from the
// old unsectioned config format. Only sectioned table keys are allowed.
func rejectLegacyKeys(raw map[string]any) error {
	legacy := map[string]struct{}{
		"max_tokens": {}, "context_mode": {}, "context_window_lines": {}, "max_context_tokens": {},
		"log_preview_limit": {}, "completion_debounce_ms": {}, "completion_throttle_ms": {},
		"manual_invoke_min_prefix": {}, "trigger_characters": {}, "inline_open": {}, "inline_close": {},
		"chat_suffix": {}, "chat_prefixes": {}, "coding_temperature": {}, "provider": {},
		"openai_model": {}, "openai_base_url": {}, "openai_temperature": {},
		"ollama_model": {}, "ollama_base_url": {}, "ollama_temperature": {},
	}
	knownTables := map[string]struct{}{
		"general": {}, "logging": {}, "completion": {}, "triggers": {}, "inline": {},
		"chat": {}, "provider": {}, "models": {}, "openai": {}, "ollama": {}, "prompts": {},
	}
	for k := range raw {
		if _, isTable := knownTables[k]; isTable {
			continue
		}
		if _, isLegacy := legacy[k]; isLegacy {
			return fmt.Errorf("unsupported flat key '%s' in config; use sectioned tables (see config.toml.example)", k)
		}
	}
	return nil
}

// applyRawIntOverrides defensively re-applies integer values from the raw TOML map
// that the typed decoder may have silently zeroed (e.g. int vs float mismatch).
func applyRawIntOverrides(raw map[string]any, tab *App) {
	if t, ok := raw["completion"].(map[string]any); ok {
		applyRawInt(&tab.ManualInvokeMinPrefix, t, "manual_invoke_min_prefix")
	}
	if t, ok := raw["logging"].(map[string]any); ok {
		applyRawInt(&tab.LogPreviewLimit, t, "log_preview_limit")
	}
}

// applyRawInt sets *dst from table[key] when the value is a numeric type.
func applyRawInt(dst *int, table map[string]any, key string) {
	v, present := table[key]
	if !present {
		return
	}
	switch vv := v.(type) {
	case int64:
		*dst = int(vv)
	case int:
		*dst = vv
	case float64:
		*dst = int(vv)
	}
}

func (fc *fileConfig) toApp() App {
	out := App{}
	applyCoreSections(fc, &out)
	applyProviderSections(fc, &out)
	applyPromptSections(fc, &out)
	applyFeatureSections(fc, &out)
	return out
}

func applyCoreSections(fc *fileConfig, out *App) {
	applyGeneralSection(fc, out)
	applyLoggingSection(fc, out)
	applyCompletionSection(fc, out)
	applyTriggerSection(fc, out)
	applyInlineSection(fc, out)
	applyChatSection(fc, out)
	applyProviderNameSection(fc, out)
	applyIgnoreSection(fc, out)
}

func applyProviderSections(fc *fileConfig, out *App) {
	applyOpenAISection(fc, out)
	applyOpenRouterSection(fc, out)
	applyOllamaSection(fc, out)
	applyAnthropicSection(fc, out)
}

func applyPromptSections(fc *fileConfig, out *App) {
	applyPromptCompletion(fc, out)
	applyPromptChat(fc, out)
	applyPromptCodeAction(fc, out)
	applyPromptCLI(fc, out)
	applyPromptProviderNative(fc, out)
}

func applyFeatureSections(fc *fileConfig, out *App) {
	applyTmuxSection(fc, out)
	applyStatsSection(fc, out)
	fc.applyTmuxEdit(out)
	applyMCPSection(fc, out)
	applyTmuxActionSection(fc, out)
}

// applyTmuxActionSection converts [[tmux_action.menu]] entries into App fields.
func applyTmuxActionSection(fc *fileConfig, out *App) {
	if len(fc.TmuxAction.Menu) == 0 {
		return
	}
	entries := make([]TmuxActionMenuEntry, 0, len(fc.TmuxAction.Menu))
	for _, e := range fc.TmuxAction.Menu {
		entries = append(entries, TmuxActionMenuEntry{
			Kind:     strings.TrimSpace(e.Kind),
			CustomID: strings.TrimSpace(e.CustomID),
			Title:    strings.TrimSpace(e.Title),
			Hotkey:   strings.TrimSpace(e.Hotkey),
		})
	}
	out.TmuxActionMenu = entries
}

func applyGeneralSection(fc *fileConfig, out *App) {
	if (fc.General == sectionGeneral{}) && fc.General.CodingTemperature == nil {
		return
	}
	tmp := App{CoreConfig: CoreConfig{
		MaxTokens:          fc.General.MaxTokens,
		ContextMode:        fc.General.ContextMode,
		ContextWindowLines: fc.General.ContextWindowLines,
		MaxContextTokens:   fc.General.MaxContextTokens,
		CodingTemperature:  fc.General.CodingTemperature,
		RequestTimeout:     fc.General.RequestTimeout,
	}}
	out.mergeBasics(&tmp)
}

func applyLoggingSection(fc *fileConfig, out *App) {
	if fc.Logging == (sectionLogging{}) {
		return
	}
	out.mergeBasics(&App{CoreConfig: CoreConfig{LogPreviewLimit: fc.Logging.LogPreviewLimit}})
}

func applyCompletionSection(fc *fileConfig, out *App) {
	if fc.Completion.CompletionDebounceMs == 0 &&
		fc.Completion.CompletionThrottleMs == 0 &&
		fc.Completion.ManualInvokeMinPrefix == 0 &&
		fc.Completion.CompletionWaitAll == nil {
		return
	}
	tmp := App{CoreConfig: CoreConfig{
		CompletionDebounceMs:  fc.Completion.CompletionDebounceMs,
		CompletionThrottleMs:  fc.Completion.CompletionThrottleMs,
		ManualInvokeMinPrefix: fc.Completion.ManualInvokeMinPrefix,
		CompletionWaitAll:     fc.Completion.CompletionWaitAll,
	}}
	out.mergeBasics(&tmp)
}

func applyTriggerSection(fc *fileConfig, out *App) {
	if len(fc.Triggers.TriggerCharacters) == 0 {
		return
	}
	out.mergeBasics(&App{CoreConfig: CoreConfig{TriggerCharacters: fc.Triggers.TriggerCharacters}})
}

func applyInlineSection(fc *fileConfig, out *App) {
	if fc.Inline == (sectionInline{}) {
		return
	}
	out.mergeBasics(&App{CoreConfig: CoreConfig{InlineOpen: fc.Inline.InlineOpen, InlineClose: fc.Inline.InlineClose}})
}

func applyChatSection(fc *fileConfig, out *App) {
	if strings.TrimSpace(fc.Chat.ChatSuffix) == "" && len(fc.Chat.ChatPrefixes) == 0 {
		return
	}
	out.mergeBasics(&App{CoreConfig: CoreConfig{ChatSuffix: fc.Chat.ChatSuffix, ChatPrefixes: fc.Chat.ChatPrefixes}})
}

func applyProviderNameSection(fc *fileConfig, out *App) {
	if strings.TrimSpace(fc.Provider.Name) == "" {
		return
	}
	out.mergeBasics(&App{CoreConfig: CoreConfig{Provider: fc.Provider.Name}})
}

func applyIgnoreSection(fc *fileConfig, out *App) {
	if fc.Ignore.Gitignore == nil && len(fc.Ignore.ExtraPatterns) == 0 && fc.Ignore.LSPNotifyIgnored == nil {
		return
	}
	tmp := App{FeatureConfig: FeatureConfig{
		IgnoreGitignore:     fc.Ignore.Gitignore,
		IgnoreExtraPatterns: fc.Ignore.ExtraPatterns,
		IgnoreLSPNotify:     fc.Ignore.LSPNotifyIgnored,
	}}
	out.mergeBasics(&tmp)
}

func applyOpenAISection(fc *fileConfig, out *App) {
	if fc.OpenAI.isZero() && fc.OpenAI.Temperature == nil {
		return
	}
	tmp := App{ProviderConfig: ProviderConfig{
		OpenAIBaseURL:     fc.OpenAI.BaseURL,
		OpenAIModel:       fc.OpenAI.resolvedModel(),
		OpenAITemperature: fc.OpenAI.Temperature,
	}}
	out.mergeProviderFields(&tmp)
}

func applyOpenRouterSection(fc *fileConfig, out *App) {
	if fc.OpenRouter == (sectionOpenRouter{}) && fc.OpenRouter.Temperature == nil {
		return
	}
	tmp := App{ProviderConfig: ProviderConfig{
		OpenRouterBaseURL:     fc.OpenRouter.BaseURL,
		OpenRouterModel:       fc.OpenRouter.Model,
		OpenRouterTemperature: fc.OpenRouter.Temperature,
	}}
	out.mergeProviderFields(&tmp)
}

func applyOllamaSection(fc *fileConfig, out *App) {
	if fc.Ollama == (sectionOllama{}) && fc.Ollama.Temperature == nil {
		return
	}
	tmp := App{ProviderConfig: ProviderConfig{
		OllamaBaseURL:     fc.Ollama.BaseURL,
		OllamaModel:       fc.Ollama.Model,
		OllamaTemperature: fc.Ollama.Temperature,
	}}
	out.mergeProviderFields(&tmp)
}

func applyAnthropicSection(fc *fileConfig, out *App) {
	if fc.Anthropic == (sectionAnthropic{}) && fc.Anthropic.Temperature == nil {
		return
	}
	tmp := App{ProviderConfig: ProviderConfig{
		AnthropicBaseURL:     fc.Anthropic.BaseURL,
		AnthropicModel:       fc.Anthropic.Model,
		AnthropicTemperature: fc.Anthropic.Temperature,
	}}
	out.mergeProviderFields(&tmp)
}

func applyPromptCompletion(fc *fileConfig, out *App) {
	if fc.Prompts.Completion == (sectionPromptsCompletion{}) {
		return
	}
	setIfNotBlank(&out.PromptCompletionSystemGeneral, fc.Prompts.Completion.SystemGeneral)
	setIfNotBlank(&out.PromptCompletionSystemParams, fc.Prompts.Completion.SystemParams)
	setIfNotBlank(&out.PromptCompletionSystemInline, fc.Prompts.Completion.SystemInline)
	setIfNotBlank(&out.PromptCompletionUserGeneral, fc.Prompts.Completion.UserGeneral)
	setIfNotBlank(&out.PromptCompletionUserParams, fc.Prompts.Completion.UserParams)
	setIfNotBlank(&out.PromptCompletionExtraHeader, fc.Prompts.Completion.ExtraHeader)
}

func applyPromptChat(fc *fileConfig, out *App) {
	setIfNotBlank(&out.PromptChatSystem, fc.Prompts.Chat.System)
}

func applyPromptCodeAction(fc *fileConfig, out *App) {
	ca := fc.Prompts.CodeAction
	if strings.TrimSpace(ca.RewriteSystem) == "" &&
		strings.TrimSpace(ca.DiagnosticsSystem) == "" &&
		strings.TrimSpace(ca.DocumentSystem) == "" &&
		strings.TrimSpace(ca.RewriteUser) == "" &&
		strings.TrimSpace(ca.DiagnosticsUser) == "" &&
		strings.TrimSpace(ca.DocumentUser) == "" &&
		strings.TrimSpace(ca.GoTestSystem) == "" &&
		strings.TrimSpace(ca.GoTestUser) == "" &&
		strings.TrimSpace(ca.SimplifySystem) == "" &&
		strings.TrimSpace(ca.SimplifyUser) == "" &&
		strings.TrimSpace(ca.FixTyposSystem) == "" &&
		strings.TrimSpace(ca.FixTyposUser) == "" &&
		len(ca.Custom) == 0 {
		return
	}
	setIfNotBlank(&out.PromptCodeActionRewriteSystem, ca.RewriteSystem)
	setIfNotBlank(&out.PromptCodeActionDiagnosticsSystem, ca.DiagnosticsSystem)
	setIfNotBlank(&out.PromptCodeActionDocumentSystem, ca.DocumentSystem)
	setIfNotBlank(&out.PromptCodeActionRewriteUser, ca.RewriteUser)
	setIfNotBlank(&out.PromptCodeActionDiagnosticsUser, ca.DiagnosticsUser)
	setIfNotBlank(&out.PromptCodeActionDocumentUser, ca.DocumentUser)
	setIfNotBlank(&out.PromptCodeActionGoTestSystem, ca.GoTestSystem)
	setIfNotBlank(&out.PromptCodeActionGoTestUser, ca.GoTestUser)
	setIfNotBlank(&out.PromptCodeActionSimplifySystem, ca.SimplifySystem)
	setIfNotBlank(&out.PromptCodeActionSimplifyUser, ca.SimplifyUser)
	setIfNotBlank(&out.PromptCodeActionFixTyposSystem, ca.FixTyposSystem)
	setIfNotBlank(&out.PromptCodeActionFixTyposUser, ca.FixTyposUser)
	if len(ca.Custom) > 0 {
		out.CustomActions = append(out.CustomActions, toCustomActions(ca.Custom)...)
	}
}

func applyPromptCLI(fc *fileConfig, out *App) {
	if fc.Prompts.CLI == (sectionPromptsCLI{}) {
		return
	}
	setIfNotBlank(&out.PromptCLIDefaultSystem, fc.Prompts.CLI.DefaultSystem)
	setIfNotBlank(&out.PromptCLIExplainSystem, fc.Prompts.CLI.ExplainSystem)
}

func applyPromptProviderNative(fc *fileConfig, out *App) {
	setIfNotBlank(&out.PromptNativeCompletion, fc.Prompts.ProviderNative.Completion)
}

func applyTmuxSection(fc *fileConfig, out *App) {
	if fc.Tmux == (sectionTmux{}) {
		return
	}
	out.TmuxCustomMenuHotkey = strings.TrimSpace(fc.Tmux.CustomMenuHotkey)
}

func applyStatsSection(fc *fileConfig, out *App) {
	if fc.Stats.WindowMinutes > 0 {
		out.StatsWindowMinutes = fc.Stats.WindowMinutes
	}
}

func applyMCPSection(fc *fileConfig, out *App) {
	if strings.TrimSpace(fc.MCP.PromptsDir) != "" {
		out.MCPPromptsDir = strings.TrimSpace(fc.MCP.PromptsDir)
	}
	if fc.MCP.SlashCommandSync {
		out.MCPSlashCommandSync = fc.MCP.SlashCommandSync
	}
	if strings.TrimSpace(fc.MCP.SlashCommandDir) != "" {
		out.MCPSlashCommandDir = strings.TrimSpace(fc.MCP.SlashCommandDir)
	}
}

func toCustomActions(custom []sectionCustomAction) []CustomAction {
	out := make([]CustomAction, 0, len(custom))
	for _, ca := range custom {
		out = append(out, CustomAction{
			ID:          strings.TrimSpace(ca.ID),
			Title:       strings.TrimSpace(ca.Title),
			Kind:        strings.TrimSpace(ca.Kind),
			Scope:       strings.ToLower(strings.TrimSpace(ca.Scope)),
			Hotkey:      strings.TrimSpace(ca.Hotkey),
			Instruction: ca.Instruction,
			System:      ca.System,
			User:        ca.User,
		})
	}
	return out
}

func setIfNotBlank(dst *string, value string) {
	if strings.TrimSpace(value) != "" {
		*dst = value
	}
}

// applyTmuxEdit converts the [tmux_edit] section into App fields.
func (fc *fileConfig) applyTmuxEdit(out *App) {
	te := fc.TmuxEdit
	if strings.TrimSpace(te.PopupWidth) != "" {
		out.TmuxEditPopupWidth = strings.TrimSpace(te.PopupWidth)
	}
	if strings.TrimSpace(te.PopupHeight) != "" {
		out.TmuxEditPopupHeight = strings.TrimSpace(te.PopupHeight)
	}
	if strings.TrimSpace(te.DefaultAgent) != "" {
		out.TmuxEditDefaultAgent = strings.TrimSpace(te.DefaultAgent)
	}
	for _, a := range te.Agents {
		if strings.TrimSpace(a.Name) == "" {
			continue
		}
		out.TmuxEditAgents = append(out.TmuxEditAgents, TmuxEditAgentCfg{
			Name:           strings.TrimSpace(a.Name),
			DisplayName:    strings.TrimSpace(a.DisplayName),
			DetectPattern:  strings.TrimSpace(a.DetectPattern),
			SectionPattern: strings.TrimSpace(a.SectionPattern),
			PromptPattern:  strings.TrimSpace(a.PromptPattern),
			StripPatterns:  a.StripPatterns,
			ClearFirst:     a.ClearFirst,
			ClearKeys:      strings.TrimSpace(a.ClearKeys),
			NewlineKeys:    strings.TrimSpace(a.NewlineKeys),
			SubmitKeys:     strings.TrimSpace(a.SubmitKeys),
		})
	}
}

func parseSurfaceModels(raw map[string]any, logger *log.Logger) *App {
	modelsRaw, ok := raw["models"]
	if !ok {
		return nil
	}
	table, ok := modelsRaw.(map[string]any)
	if !ok {
		if logger != nil {
			logger.Printf("config: ignoring models section (expected table, got %T)", modelsRaw)
		}
		return nil
	}
	var out App
	appendEntries := func(dest *[]SurfaceConfig, key string, val any) bool {
		entries, ok := parseSurfaceEntries(val, key, logger)
		if !ok || len(entries) == 0 {
			return false
		}
		*dest = append(*dest, entries...)
		return true
	}
	found := appendEntries(&out.CompletionConfigs, "models.completion", table["completion"])
	if ok := appendEntries(&out.CodeActionConfigs, "models.code_action", table["code_action"]); ok {
		if len(out.CodeActionConfigs) > 1 {
			if logger != nil {
				logger.Printf("config: models.code_action supports a single entry; ignoring %d extra", len(out.CodeActionConfigs)-1)
			}
			out.CodeActionConfigs = out.CodeActionConfigs[:1]
		}
		found = true
	}
	found = appendEntries(&out.ChatConfigs, "models.chat", table["chat"]) || found
	found = appendEntries(&out.CLIConfigs, "models.cli", table["cli"]) || found
	if !found {
		return nil
	}
	return &out
}

func parseSurfaceEntries(raw any, path string, logger *log.Logger) ([]SurfaceConfig, bool) {
	switch v := raw.(type) {
	case nil:
		return nil, false
	case []any:
		var out []SurfaceConfig
		for i, entry := range v {
			cfg, ok := decodeModelEntry(entry, fmt.Sprintf("%s[%d]", path, i), logger)
			if !ok || cfg == nil {
				continue
			}
			out = append(out, *cfg)
		}
		return out, len(out) > 0
	default:
		if cfg, ok := decodeModelEntry(v, path, logger); ok && cfg != nil {
			return []SurfaceConfig{*cfg}, true
		}
		return nil, false
	}
}

// decodeModelEntryFromMap decodes a map[string]any entry into a SurfaceConfig.
// It validates that model, provider, and temperature fields have the correct types.
func decodeModelEntryFromMap(v map[string]any, path string, logger *log.Logger) (*SurfaceConfig, bool) {
	model := ""
	provider := ""
	if m, ok := v["model"]; ok {
		s, ok := m.(string)
		if !ok {
			if logger != nil {
				logger.Printf("config: %s.model must be a string", path)
			}
			return nil, false
		}
		model = strings.TrimSpace(s)
	}
	if pRaw, ok := v["provider"]; ok {
		ps, ok := pRaw.(string)
		if !ok {
			if logger != nil {
				logger.Printf("config: %s.provider must be a string", path)
			}
			return nil, false
		}
		provider = strings.TrimSpace(ps)
	}
	var tempPtr *float64
	if tRaw, ok := v["temperature"]; ok {
		parsed, ok := parseTemperatureValue(tRaw, path, logger)
		if !ok {
			return nil, false
		}
		tempPtr = parsed
	}
	if model == "" && tempPtr == nil && provider == "" {
		return nil, false
	}
	return &SurfaceConfig{Provider: provider, Model: model, Temperature: tempPtr}, true
}

// decodeModelEntry converts a raw TOML value (string or table) into a SurfaceConfig.
// A plain string is treated as a model name; a table may carry model, provider and temperature.
func decodeModelEntry(raw any, path string, logger *log.Logger) (*SurfaceConfig, bool) {
	if raw == nil {
		return nil, false
	}
	switch v := raw.(type) {
	case string:
		model := strings.TrimSpace(v)
		if model == "" {
			return nil, false
		}
		return &SurfaceConfig{Model: model}, true
	case map[string]any:
		return decodeModelEntryFromMap(v, path, logger)
	default:
		if logger != nil {
			logger.Printf("config: %s must be a string or table, got %T", path, raw)
		}
		return nil, false
	}
}

func parseTemperatureValue(raw any, path string, logger *log.Logger) (*float64, bool) {
	switch v := raw.(type) {
	case float64:
		return floatPtr(v), true
	case int64:
		return floatPtr(float64(v)), true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, true
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			if logger != nil {
				logger.Printf("config: %s.temperature invalid: %v", path, err)
			}
			return nil, false
		}
		return floatPtr(f), true
	default:
		if logger != nil {
			logger.Printf("config: %s.temperature must be numeric or string, got %T", path, raw)
		}
		return nil, false
	}
}

func floatPtr(v float64) *float64 {
	f := v
	return &f
}
