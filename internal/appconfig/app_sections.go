package appconfig

import "slices"

// CoreConfig contains core runtime and interaction settings.
// It is embedded in App; JSON tags ensure marshalling works correctly.
type CoreConfig struct {
	MaxTokens          int    `json:"max_tokens"`
	ContextMode        string `json:"context_mode"`
	ContextWindowLines int    `json:"context_window_lines"`
	MaxContextTokens   int    `json:"max_context_tokens"`
	LogPreviewLimit    int    `json:"log_preview_limit"`
	RequestTimeout     int    `json:"request_timeout"`
	// Single knob for LSP requests; if set, overrides hardcoded temps in LSP.
	CodingTemperature *float64 `json:"coding_temperature"`
	// Minimum identifier characters required for manual (TriggerKind=1) invoke
	// to proceed without structural triggers. 0 means always allow.
	ManualInvokeMinPrefix int `json:"manual_invoke_min_prefix"`
	// Completion debounce in milliseconds. When > 0, the server waits until
	// there has been no text change for at least this duration before sending
	// an LLM completion request.
	CompletionDebounceMs int `json:"completion_debounce_ms"`
	// Completion throttle in milliseconds. When > 0, caps the minimum spacing
	// between LLM requests (both chat and code-completer paths).
	CompletionThrottleMs int `json:"completion_throttle_ms"`
	// CompletionWaitAll controls whether to wait for all configured completion
	// backends before returning results. When true (default), waits for all
	// backends. When false, returns the first result immediately.
	CompletionWaitAll *bool    `json:"completion_wait_all"`
	TriggerCharacters []string `json:"trigger_characters"`
	Provider          string   `json:"provider"`
	// Inline prompt trigger characters (default: >!text> and >>!text>)
	InlineOpen  string `json:"inline_open"`
	InlineClose string `json:"inline_close"`
	// In-editor chat triggers (default: suffix ">" after one of [?, !, :, ;])
	ChatSuffix   string   `json:"chat_suffix"`
	ChatPrefixes []string `json:"chat_prefixes"`
}

// ProviderConfig contains provider endpoints/models and per-surface model overrides.
// It is embedded in App; JSON tags ensure marshalling works correctly.
type ProviderConfig struct {
	// Provider-specific options
	OpenAIBaseURL string `json:"openai_base_url"`
	OpenAIModel   string `json:"openai_model"`
	// Default temperature for OpenAI requests (nil means use provider default)
	OpenAITemperature *float64 `json:"openai_temperature"`
	OpenRouterBaseURL string   `json:"openrouter_base_url"`
	OpenRouterModel   string   `json:"openrouter_model"`
	// Default temperature for OpenRouter requests (nil means use provider default)
	OpenRouterTemperature *float64 `json:"openrouter_temperature"`
	OllamaBaseURL         string   `json:"ollama_base_url"`
	OllamaModel           string   `json:"ollama_model"`
	// Default temperature for Ollama requests (nil means use provider default)
	OllamaTemperature *float64 `json:"ollama_temperature"`
	AnthropicBaseURL  string   `json:"anthropic_base_url"`
	AnthropicModel    string   `json:"anthropic_model"`
	// Default temperature for Anthropic requests (nil means use provider default)
	AnthropicTemperature *float64 `json:"anthropic_temperature"`
	// Per-surface provider/model configurations (ordered; first entry is primary)
	CompletionConfigs []SurfaceConfig `json:"-"`
	CodeActionConfigs []SurfaceConfig `json:"-"`
	ChatConfigs       []SurfaceConfig `json:"-"`
	CLIConfigs        []SurfaceConfig `json:"-"`
}

// PromptConfig contains all prompt templates and custom action prompts.
// It is embedded in App; fields use json:"-" since prompts are not exposed via JSON.
type PromptConfig struct {
	// Prompt templates (configured only via file; no env overrides)
	// Completion
	PromptCompletionSystemGeneral string `json:"-"`
	PromptCompletionSystemParams  string `json:"-"`
	PromptCompletionSystemInline  string `json:"-"`
	PromptCompletionUserGeneral   string `json:"-"`
	PromptCompletionUserParams    string `json:"-"`
	PromptCompletionExtraHeader   string `json:"-"`
	// Provider-native code-completer
	PromptNativeCompletion string `json:"-"`
	// In-editor chat
	PromptChatSystem string `json:"-"`
	// Code actions
	PromptCodeActionRewriteSystem     string `json:"-"`
	PromptCodeActionDiagnosticsSystem string `json:"-"`
	PromptCodeActionDocumentSystem    string `json:"-"`
	PromptCodeActionRewriteUser       string `json:"-"`
	PromptCodeActionDiagnosticsUser   string `json:"-"`
	PromptCodeActionDocumentUser      string `json:"-"`
	PromptCodeActionGoTestSystem      string `json:"-"`
	PromptCodeActionGoTestUser        string `json:"-"`
	PromptCodeActionSimplifySystem    string `json:"-"`
	PromptCodeActionSimplifyUser      string `json:"-"`
	PromptCodeActionFixTyposSystem    string `json:"-"`
	PromptCodeActionFixTyposUser      string `json:"-"`
	// CLI
	PromptCLIDefaultSystem string `json:"-"`
	PromptCLIExplainSystem string `json:"-"`
	// Custom code actions and tmux integration
	CustomActions        []CustomAction `json:"-"`
	TmuxCustomMenuHotkey string         `json:"-"`
}

// FeatureConfig contains non-LLM feature toggles/integration settings.
// It is embedded in App; fields use json:"-" since features are not exposed via JSON.
type FeatureConfig struct {
	// Stats
	StatsWindowMinutes int `json:"-"`
	// Ignore: gitignore-aware file filtering for LSP
	IgnoreGitignore     *bool    `json:"-"`
	IgnoreExtraPatterns []string `json:"-"`
	IgnoreLSPNotify     *bool    `json:"-"`
	// TmuxEdit: popup editor settings for hexai-tmux-edit
	TmuxEditPopupWidth   string             `json:"-"`
	TmuxEditPopupHeight  string             `json:"-"`
	TmuxEditDefaultAgent string             `json:"-"`
	TmuxEditAgents       []TmuxEditAgentCfg `json:"-"`
	// MCP: Model Context Protocol server settings
	MCPPromptsDir       string `json:"-"` // Directory for prompt storage
	MCPSlashCommandSync bool   `json:"-"` // Enable slash command sync
	MCPSlashCommandDir  string `json:"-"` // Directory for slash command files
}

// AppSections is the focused split of App into subsystem-specific config groups.
type AppSections struct {
	Core      CoreConfig
	Providers ProviderConfig
	Prompts   PromptConfig
	Features  FeatureConfig
}

// Sections returns the app configuration split into focused sub-configs.
func (a *App) Sections() AppSections {
	return AppSections{
		Core:      a.CoreSection(),
		Providers: a.ProviderSection(),
		Prompts:   a.PromptSection(),
		Features:  a.FeatureSection(),
	}
}

// ApplySections applies focused sub-config groups back onto App.
func (a *App) ApplySections(sections AppSections) {
	a.ApplyCoreSection(sections.Core)
	a.ApplyProviderSection(sections.Providers)
	a.ApplyPromptSection(sections.Prompts)
	a.ApplyFeatureSection(sections.Features)
}

// CoreSection returns a deep copy of the core runtime and interaction settings.
// Slices are cloned to prevent callers from mutating the original.
func (a *App) CoreSection() CoreConfig {
	c := a.CoreConfig
	c.TriggerCharacters = slices.Clone(a.TriggerCharacters)
	c.ChatPrefixes = slices.Clone(a.ChatPrefixes)
	return c
}

// ApplyCoreSection applies core runtime and interaction settings.
// Slices are cloned to prevent the caller's copy from being shared.
func (a *App) ApplyCoreSection(core CoreConfig) {
	a.CoreConfig = core
	a.TriggerCharacters = slices.Clone(core.TriggerCharacters)
	a.ChatPrefixes = slices.Clone(core.ChatPrefixes)
}

// ProviderSection returns a deep copy of provider endpoint/model settings.
// Surface config slices are cloned to prevent callers from mutating the original.
func (a *App) ProviderSection() ProviderConfig {
	p := a.ProviderConfig
	p.CompletionConfigs = cloneSurfaceConfigs(a.CompletionConfigs)
	p.CodeActionConfigs = cloneSurfaceConfigs(a.CodeActionConfigs)
	p.ChatConfigs = cloneSurfaceConfigs(a.ChatConfigs)
	p.CLIConfigs = cloneSurfaceConfigs(a.CLIConfigs)
	return p
}

// ApplyProviderSection applies provider endpoint/model settings and surface overrides.
// Surface config slices are cloned to prevent the caller's copy from being shared.
func (a *App) ApplyProviderSection(providers ProviderConfig) {
	a.ProviderConfig = providers
	a.CompletionConfigs = cloneSurfaceConfigs(providers.CompletionConfigs)
	a.CodeActionConfigs = cloneSurfaceConfigs(providers.CodeActionConfigs)
	a.ChatConfigs = cloneSurfaceConfigs(providers.ChatConfigs)
	a.CLIConfigs = cloneSurfaceConfigs(providers.CLIConfigs)
}

// PromptSection returns a deep copy of prompt templates and custom action settings.
// The CustomActions slice is cloned to prevent callers from mutating the original.
func (a *App) PromptSection() PromptConfig {
	p := a.PromptConfig
	p.CustomActions = append([]CustomAction{}, a.CustomActions...)
	return p
}

// ApplyPromptSection applies prompt templates and custom action prompt settings.
// The CustomActions slice is cloned to prevent the caller's copy from being shared.
func (a *App) ApplyPromptSection(prompts PromptConfig) {
	a.PromptConfig = prompts
	a.CustomActions = append([]CustomAction{}, prompts.CustomActions...)
}

// FeatureSection returns a deep copy of non-LLM feature toggles and integrations.
// Slices are cloned to prevent callers from mutating the original.
func (a *App) FeatureSection() FeatureConfig {
	f := a.FeatureConfig
	f.IgnoreExtraPatterns = slices.Clone(a.IgnoreExtraPatterns)
	f.TmuxEditAgents = append([]TmuxEditAgentCfg{}, a.TmuxEditAgents...)
	return f
}

// ApplyFeatureSection applies non-LLM feature toggles and integrations.
// Slices are cloned to prevent the caller's copy from being shared.
func (a *App) ApplyFeatureSection(features FeatureConfig) {
	a.FeatureConfig = features
	a.IgnoreExtraPatterns = slices.Clone(features.IgnoreExtraPatterns)
	a.TmuxEditAgents = append([]TmuxEditAgentCfg{}, features.TmuxEditAgents...)
}
