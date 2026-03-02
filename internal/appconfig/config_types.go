// Summary: Application configuration model and defaults.
package appconfig

import "strings"

// SurfaceConfig describes a provider/model pairing (with optional temperature).
type SurfaceConfig struct {
	Provider    string
	Model       string
	Temperature *float64
}

// App holds user-configurable settings read from ~/.config/hexai/config.toml.
type App struct {
	MaxTokens          int    `json:"max_tokens" toml:"max_tokens"`
	ContextMode        string `json:"context_mode" toml:"context_mode"`
	ContextWindowLines int    `json:"context_window_lines" toml:"context_window_lines"`
	MaxContextTokens   int    `json:"max_context_tokens" toml:"max_context_tokens"`
	LogPreviewLimit    int    `json:"log_preview_limit" toml:"log_preview_limit"`
	RequestTimeout     int    `json:"request_timeout" toml:"request_timeout"`
	// Single knob for LSP requests; if set, overrides hardcoded temps in LSP.
	CodingTemperature *float64 `json:"coding_temperature" toml:"coding_temperature"`
	// Minimum identifier characters required for manual (TriggerKind=1) invoke
	// to proceed without structural triggers. 0 means always allow.
	ManualInvokeMinPrefix int `json:"manual_invoke_min_prefix" toml:"manual_invoke_min_prefix"`

	// Completion debounce in milliseconds. When > 0, the server waits until
	// there has been no text change for at least this duration before sending
	// an LLM completion request.
	CompletionDebounceMs int `json:"completion_debounce_ms" toml:"completion_debounce_ms"`
	// Completion throttle in milliseconds. When > 0, caps the minimum spacing
	// between LLM requests (both chat and code-completer paths).
	CompletionThrottleMs int `json:"completion_throttle_ms" toml:"completion_throttle_ms"`
	// CompletionWaitAll controls whether to wait for all configured completion
	// backends before returning results. When true (default), waits for all
	// backends. When false, returns the first result immediately.
	CompletionWaitAll *bool `json:"completion_wait_all" toml:"completion_wait_all"`

	TriggerCharacters []string `json:"trigger_characters" toml:"trigger_characters"`
	Provider          string   `json:"provider" toml:"provider"`

	// Inline prompt trigger characters (default: >!text> and >>!text>)
	InlineOpen  string `json:"inline_open" toml:"inline_open"`
	InlineClose string `json:"inline_close" toml:"inline_close"`
	// In-editor chat triggers (default: suffix ">" after one of [?, !, :, ;])
	ChatSuffix   string   `json:"chat_suffix" toml:"chat_suffix"`
	ChatPrefixes []string `json:"chat_prefixes" toml:"chat_prefixes"`

	// Provider-specific options
	OpenAIBaseURL string `json:"openai_base_url" toml:"openai_base_url"`
	OpenAIModel   string `json:"openai_model" toml:"openai_model"`
	// Default temperature for OpenAI requests (nil means use provider default)
	OpenAITemperature *float64 `json:"openai_temperature" toml:"openai_temperature"`
	OpenRouterBaseURL string   `json:"openrouter_base_url" toml:"openrouter_base_url"`
	OpenRouterModel   string   `json:"openrouter_model" toml:"openrouter_model"`
	// Default temperature for OpenRouter requests (nil means use provider default)
	OpenRouterTemperature *float64 `json:"openrouter_temperature" toml:"openrouter_temperature"`
	OllamaBaseURL         string   `json:"ollama_base_url" toml:"ollama_base_url"`
	OllamaModel           string   `json:"ollama_model" toml:"ollama_model"`
	// Default temperature for Ollama requests (nil means use provider default)
	OllamaTemperature *float64 `json:"ollama_temperature" toml:"ollama_temperature"`
	AnthropicBaseURL  string   `json:"anthropic_base_url" toml:"anthropic_base_url"`
	AnthropicModel    string   `json:"anthropic_model" toml:"anthropic_model"`
	// Default temperature for Anthropic requests (nil means use provider default)
	AnthropicTemperature *float64 `json:"anthropic_temperature" toml:"anthropic_temperature"`

	// Per-surface provider/model configurations (ordered; first entry is primary)
	CompletionConfigs []SurfaceConfig `json:"-" toml:"-"`
	CodeActionConfigs []SurfaceConfig `json:"-" toml:"-"`
	ChatConfigs       []SurfaceConfig `json:"-" toml:"-"`
	CLIConfigs        []SurfaceConfig `json:"-" toml:"-"`

	// Prompt templates (configured only via file; no env overrides)
	// Completion/chat/code action/CLI prompt strings. See config.toml.example for placeholders.
	// Completion
	PromptCompletionSystemGeneral string `json:"-" toml:"-"`
	PromptCompletionSystemParams  string `json:"-" toml:"-"`
	PromptCompletionSystemInline  string `json:"-" toml:"-"`
	PromptCompletionUserGeneral   string `json:"-" toml:"-"`
	PromptCompletionUserParams    string `json:"-" toml:"-"`
	PromptCompletionExtraHeader   string `json:"-" toml:"-"`
	// Provider-native code-completer
	PromptNativeCompletion string `json:"-" toml:"-"`
	// In-editor chat
	PromptChatSystem string `json:"-" toml:"-"`
	// Code actions
	PromptCodeActionRewriteSystem     string `json:"-" toml:"-"`
	PromptCodeActionDiagnosticsSystem string `json:"-" toml:"-"`
	PromptCodeActionDocumentSystem    string `json:"-" toml:"-"`
	PromptCodeActionRewriteUser       string `json:"-" toml:"-"`
	PromptCodeActionDiagnosticsUser   string `json:"-" toml:"-"`
	PromptCodeActionDocumentUser      string `json:"-" toml:"-"`
	PromptCodeActionGoTestSystem      string `json:"-" toml:"-"`
	PromptCodeActionGoTestUser        string `json:"-" toml:"-"`
	PromptCodeActionSimplifySystem    string `json:"-" toml:"-"`
	PromptCodeActionSimplifyUser      string `json:"-" toml:"-"`
	// CLI
	PromptCLIDefaultSystem string `json:"-" toml:"-"`
	PromptCLIExplainSystem string `json:"-" toml:"-"`

	// Custom code actions and tmux integration
	CustomActions        []CustomAction `json:"-" toml:"-"`
	TmuxCustomMenuHotkey string         `json:"-" toml:"-"`
	// Stats
	StatsWindowMinutes int `json:"-" toml:"-"`

	// Ignore: gitignore-aware file filtering for LSP
	IgnoreGitignore     *bool    `json:"-" toml:"-"`
	IgnoreExtraPatterns []string `json:"-" toml:"-"`
	IgnoreLSPNotify     *bool    `json:"-" toml:"-"`

	// TmuxEdit: popup editor settings for hexai-tmux-edit
	TmuxEditPopupWidth   string             `json:"-" toml:"-"`
	TmuxEditPopupHeight  string             `json:"-" toml:"-"`
	TmuxEditDefaultAgent string             `json:"-" toml:"-"`
	TmuxEditAgents       []TmuxEditAgentCfg `json:"-" toml:"-"`

	// MCP: Model Context Protocol server settings
	MCPPromptsDir       string `json:"-" toml:"-"` // Directory for prompt storage
	MCPSlashCommandSync bool   `json:"-" toml:"-"` // Enable slash command sync
	MCPSlashCommandDir  string `json:"-" toml:"-"` // Directory for slash command files
}

// CustomAction describes a user-defined code action.
type CustomAction struct {
	ID          string
	Title       string
	Kind        string // optional; default "refactor"
	Scope       string // "selection" (default) | "diagnostics"
	Hotkey      string // optional, used by tmux submenu
	Instruction string // optional; if set and User is empty, use global rewrite templates
	System      string // optional; used only when User is set
	User        string // optional; if set, render with available vars
}

// TmuxEditAgentCfg describes an AI agent's detection and interaction patterns
// for the tmux popup editor (hexai-tmux-edit).
type TmuxEditAgentCfg struct {
	Name           string
	DisplayName    string
	DetectPattern  string
	SectionPattern string
	PromptPattern  string
	StripPatterns  []string
	ClearFirst     *bool
	ClearKeys      string
	NewlineKeys    string
	SubmitKeys     string
}

// LoadOptions tune how configuration is loaded at runtime.
type LoadOptions struct {
	// IgnoreEnv skips applying environment overrides when true.
	IgnoreEnv bool
	// ConfigPath overrides the global config file path (e.g. via --config flag).
	ConfigPath string
	// ProjectRoot overrides the project root directory for locating .hexaiconfig.toml.
	// When empty, FindGitRoot() is used to auto-detect from the current working directory.
	ProjectRoot string
}

// Constructor: defaults for App (kept first among functions)
func newDefaultConfig() App {
	// Coding-friendly default temperature across providers
	// Users can override per provider in config.toml (including 0.0).
	t := 0.2
	return App{
		MaxTokens:             4000,
		ContextMode:           "always-full",
		ContextWindowLines:    120,
		MaxContextTokens:      4000,
		LogPreviewLimit:       100,
		RequestTimeout:        30,
		CodingTemperature:     &t,
		OpenAITemperature:     &t,
		OllamaTemperature:     &t,
		AnthropicTemperature:  &t,
		ManualInvokeMinPrefix: 0,
		CompletionDebounceMs:  800,
		CompletionThrottleMs:  0,
		// Inline/chat trigger defaults
		InlineOpen:   ">!",
		InlineClose:  ">",
		ChatSuffix:   ">",
		ChatPrefixes: []string{"?", "!", ":", ";"},

		// Default prompt templates (match current hard-coded strings)
		PromptCompletionSystemParams:  "You are a code completion engine for function signatures. Return only the parameter list contents (without parentheses), no braces, no prose. Prefer idiomatic names and types.",
		PromptCompletionUserParams:    "Cursor is inside the function parameter list. Suggest only the parameter list (no parentheses).\nFunction line: {{function}}\nCurrent line (cursor at {{char}}): {{current}}",
		PromptCompletionSystemGeneral: "You are a terse code completion engine. Return only the code to insert, no surrounding prose or backticks. Only continue from the cursor; never repeat characters already present to the left of the cursor on the current line (e.g., if 'name :=' is already typed, only return the right-hand side expression).",
		PromptCompletionUserGeneral:   "Provide the next likely code to insert at the cursor.\nFile: {{file}}\nFunction/context: {{function}}\nAbove line: {{above}}\nCurrent line (cursor at character {{char}}): {{current}}\nBelow line: {{below}}\nOnly return the completion snippet.",
		PromptCompletionSystemInline:  "You are a precise code completion/refactoring engine. Output only the code to insert with no prose, no comments, and no backticks. Return raw code only.",
		PromptCompletionExtraHeader:   "Additional context:\n{{context}}",

		PromptNativeCompletion: "// Path: {{path}}\n{{before}}",

		PromptChatSystem: "You are a helpful coding assistant. Answer concisely and clearly.",

		PromptCodeActionRewriteSystem:     "You are a precise code refactoring engine. Rewrite the given code strictly according to the instruction. Return only the updated code with no prose or backticks. Preserve formatting where reasonable.",
		PromptCodeActionDiagnosticsSystem: "You are a precise code fixer. Resolve the given diagnostics by editing only the selected code. Return only the corrected code with no prose or backticks. Keep behavior and style, and avoid unrelated changes.",
		PromptCodeActionDocumentSystem:    "You are a precise code documentation engine. Add idiomatic documentation comments to the given code. Preserve exact behavior and formatting as much as possible. Return only the updated code with comments, no prose or backticks.",
		PromptCodeActionRewriteUser:       "Instruction: {{instruction}}\n\nSelected code to transform:\n{{selection}}",
		PromptCodeActionDiagnosticsUser:   "Diagnostics to resolve (selection only):\n{{diagnostics}}\n\nSelected code:\n{{selection}}",
		PromptCodeActionDocumentUser:      "Add documentation comments to this code:\n{{selection}}",
		PromptCodeActionGoTestSystem:      "You are a precise Go unit test generator. Given a Go function, write one or more Test* functions using the testing package. Do NOT include package or imports, only the test function(s). Prefer table-driven tests. Keep it minimal and idiomatic.",
		PromptCodeActionGoTestUser:        "Function under test:\n{{function}}",
		PromptCodeActionSimplifySystem:    "You are a precise code improvement engine. Simplify and improve the given code while preserving behavior. Return only the improved code with no prose or backticks.",
		PromptCodeActionSimplifyUser:      "Improve this code:\n{{selection}}",

		PromptCLIDefaultSystem: "You are Hexai CLI. Default to very short, concise answers. If the user asks for commands, output only the commands (one per line) with no commentary or explanation. Only when the word 'explain' appears in the prompt, produce a verbose explanation.",
		PromptCLIExplainSystem: "You are Hexai CLI. The user requested an explanation. Provide a clear, verbose explanation with reasoning and details. If commands are needed, include them with brief context.",

		// Stats
		StatsWindowMinutes: 60,

		// Ignore: respect .gitignore by default, notify in LSP by default
		IgnoreGitignore: boolPtr(true),
		IgnoreLSPNotify: boolPtr(true),
	}
}

func boolPtr(b bool) *bool { return &b }

// Private helpers
// Sectioned (table-based) file format only.
type fileConfig struct {
	// Section tables only (flat keys are not allowed)
	General    sectionGeneral    `toml:"general"`
	Logging    sectionLogging    `toml:"logging"`
	Completion sectionCompletion `toml:"completion"`
	Triggers   sectionTriggers   `toml:"triggers"`
	Inline     sectionInline     `toml:"inline"`
	Chat       sectionChat       `toml:"chat"`
	Provider   sectionProvider   `toml:"provider"`
	OpenAI     sectionOpenAI     `toml:"openai"`
	OpenRouter sectionOpenRouter `toml:"openrouter"`
	Ollama     sectionOllama     `toml:"ollama"`
	Anthropic  sectionAnthropic  `toml:"anthropic"`
	Prompts    sectionPrompts    `toml:"prompts"`
	Tmux       sectionTmux       `toml:"tmux"`
	Stats      sectionStats      `toml:"stats"`
	Ignore     sectionIgnore     `toml:"ignore"`
	TmuxEdit   sectionTmuxEdit   `toml:"tmux_edit"`
	MCP        sectionMCP        `toml:"mcp"`
}

type sectionGeneral struct {
	MaxTokens          int      `toml:"max_tokens"`
	ContextMode        string   `toml:"context_mode"`
	ContextWindowLines int      `toml:"context_window_lines"`
	MaxContextTokens   int      `toml:"max_context_tokens"`
	CodingTemperature  *float64 `toml:"coding_temperature"`
	RequestTimeout     int      `toml:"request_timeout"`
}

type sectionLogging struct {
	LogPreviewLimit int `toml:"log_preview_limit"`
}

type sectionCompletion struct {
	CompletionDebounceMs  int   `toml:"completion_debounce_ms"`
	CompletionThrottleMs  int   `toml:"completion_throttle_ms"`
	ManualInvokeMinPrefix int   `toml:"manual_invoke_min_prefix"`
	CompletionWaitAll     *bool `toml:"completion_wait_all"`
}

type sectionTriggers struct {
	TriggerCharacters []string `toml:"trigger_characters"`
}

type sectionInline struct {
	InlineOpen  string `toml:"inline_open"`
	InlineClose string `toml:"inline_close"`
}

type sectionChat struct {
	ChatSuffix   string   `toml:"chat_suffix"`
	ChatPrefixes []string `toml:"chat_prefixes"`
}

type sectionProvider struct {
	Name string `toml:"name"`
}

type sectionStats struct {
	WindowMinutes int `toml:"window_minutes"`
}

// sectionIgnore controls gitignore-aware file filtering. Files matching
// these patterns are skipped for completions and code actions.
type sectionIgnore struct {
	Gitignore        *bool    `toml:"gitignore"`
	ExtraPatterns    []string `toml:"extra_patterns"`
	LSPNotifyIgnored *bool    `toml:"lsp_notify_ignored"`
}

// sectionTmuxEdit configures the tmux popup editor feature (hexai-tmux-edit).
type sectionTmuxEdit struct {
	PopupWidth   string                 `toml:"popup_width"`
	PopupHeight  string                 `toml:"popup_height"`
	DefaultAgent string                 `toml:"default_agent"`
	Agents       []sectionTmuxEditAgent `toml:"agents"`
}

// sectionTmuxEditAgent defines detection and interaction patterns for one AI agent.
type sectionTmuxEditAgent struct {
	Name           string   `toml:"name"`
	DisplayName    string   `toml:"display_name"`
	DetectPattern  string   `toml:"detect_pattern"`
	SectionPattern string   `toml:"section_pattern"`
	PromptPattern  string   `toml:"prompt_pattern"`
	StripPatterns  []string `toml:"strip_patterns"`
	ClearFirst     *bool    `toml:"clear_first"`
	ClearKeys      string   `toml:"clear_keys"`
	NewlineKeys    string   `toml:"newline_keys"`
	SubmitKeys     string   `toml:"submit_keys"`
}

// sectionMCP configures the MCP server settings.
type sectionMCP struct {
	PromptsDir       string `toml:"prompts_dir"`
	SlashCommandSync bool   `toml:"slashcommand_sync"`
	SlashCommandDir  string `toml:"slashcommand_dir"`
}

type sectionOpenAI struct {
	Model       string            `toml:"model"`
	BaseURL     string            `toml:"base_url"`
	Temperature *float64          `toml:"temperature"`
	Presets     map[string]string `toml:"presets"`
}

func (s sectionOpenAI) isZero() bool {
	return strings.TrimSpace(s.Model) == "" &&
		strings.TrimSpace(s.BaseURL) == "" &&
		s.Temperature == nil &&
		len(s.Presets) == 0
}

func (s sectionOpenAI) resolvedModel() string {
	model := strings.TrimSpace(s.Model)
	if model == "" {
		return ""
	}
	if len(s.Presets) == 0 {
		return model
	}
	if mapped := strings.TrimSpace(s.Presets[model]); mapped != "" {
		return mapped
	}
	lower := strings.ToLower(model)
	for k, v := range s.Presets {
		if strings.ToLower(strings.TrimSpace(k)) == lower {
			if mapped := strings.TrimSpace(v); mapped != "" {
				return mapped
			}
		}
	}
	return model
}

type sectionOpenRouter struct {
	Model       string   `toml:"model"`
	BaseURL     string   `toml:"base_url"`
	Temperature *float64 `toml:"temperature"`
}

type sectionOllama struct {
	Model       string   `toml:"model"`
	BaseURL     string   `toml:"base_url"`
	Temperature *float64 `toml:"temperature"`
}

type sectionAnthropic struct {
	Model       string   `toml:"model"`
	BaseURL     string   `toml:"base_url"`
	Temperature *float64 `toml:"temperature"`
}

// Prompts sections
type sectionPrompts struct {
	Completion     sectionPromptsCompletion     `toml:"completion"`
	Chat           sectionPromptsChat           `toml:"chat"`
	CodeAction     sectionPromptsCodeAction     `toml:"code_action"`
	CLI            sectionPromptsCLI            `toml:"cli"`
	ProviderNative sectionPromptsProviderNative `toml:"provider_native"`
}

type sectionPromptsCompletion struct {
	SystemGeneral string `toml:"system_general"`
	SystemParams  string `toml:"system_params"`
	SystemInline  string `toml:"system_inline"`
	UserGeneral   string `toml:"user_general"`
	UserParams    string `toml:"user_params"`
	ExtraHeader   string `toml:"additional_context"`
}

type sectionPromptsChat struct {
	System string `toml:"system"`
}

type sectionPromptsCodeAction struct {
	RewriteSystem     string                `toml:"rewrite_system"`
	DiagnosticsSystem string                `toml:"diagnostics_system"`
	DocumentSystem    string                `toml:"document_system"`
	RewriteUser       string                `toml:"rewrite_user"`
	DiagnosticsUser   string                `toml:"diagnostics_user"`
	DocumentUser      string                `toml:"document_user"`
	GoTestSystem      string                `toml:"go_test_system"`
	GoTestUser        string                `toml:"go_test_user"`
	SimplifySystem    string                `toml:"simplify_system"`
	SimplifyUser      string                `toml:"simplify_user"`
	Custom            []sectionCustomAction `toml:"custom"`
}

type sectionPromptsCLI struct {
	DefaultSystem string `toml:"default_system"`
	ExplainSystem string `toml:"explain_system"`
}

type sectionPromptsProviderNative struct {
	Completion string `toml:"completion"`
}

type sectionCustomAction struct {
	ID          string `toml:"id"`
	Title       string `toml:"title"`
	Kind        string `toml:"kind"`
	Scope       string `toml:"scope"`
	Hotkey      string `toml:"hotkey"`
	Instruction string `toml:"instruction"`
	System      string `toml:"system"`
	User        string `toml:"user"`
}

type sectionTmux struct {
	CustomMenuHotkey string `toml:"custom_menu_hotkey"`
}
