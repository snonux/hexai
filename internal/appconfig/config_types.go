// Package appconfig provides the application configuration model and defaults.
package appconfig

import "strings"

// SurfaceConfig describes a provider/model pairing (with optional temperature).
type SurfaceConfig struct {
	Provider    string
	Model       string
	Temperature *float64
}

// App holds user-configurable settings read from ~/.config/hexai/config.toml.
// Fields are organized into embedded section structs. Go promotes all fields,
// so existing code like cfg.MaxTokens continues to work. App is never directly
// TOML-decoded; the fileConfig struct handles TOML parsing and fields are copied
// to App in loadFromFile. JSON marshalling works because section structs carry
// the appropriate json tags.
type App struct {
	CoreConfig
	ProviderConfig
	PromptConfig
	FeatureConfig
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

// Constructor: defaults for App (kept first among functions).
// Initializes via embedded section structs; see CoreConfig, ProviderConfig,
// PromptConfig, and FeatureConfig for field documentation.
func newDefaultConfig() App {
	// Coding-friendly default temperature across providers.
	// Users can override per provider in config.toml (including 0.0).
	t := 0.2
	return App{
		CoreConfig: CoreConfig{
			MaxTokens:             4000,
			ContextMode:           "always-full",
			ContextWindowLines:    120,
			MaxContextTokens:      4000,
			LogPreviewLimit:       100,
			RequestTimeout:        600,
			CodingTemperature:     &t,
			ManualInvokeMinPrefix: 0,
			CompletionDebounceMs:  800,
			CompletionThrottleMs:  0,
			// Inline/chat trigger defaults
			InlineOpen:   ">!",
			InlineClose:  ">",
			ChatSuffix:   ">",
			ChatPrefixes: []string{"?", "!", ":", ";"},
		},
		ProviderConfig: ProviderConfig{
			OpenAITemperature:    &t,
			OllamaTemperature:    &t,
			AnthropicTemperature: &t,
		},
		PromptConfig: defaultPromptConfig(),
		FeatureConfig: FeatureConfig{
			StatsWindowMinutes: 60,
			// Ignore: respect .gitignore by default, notify in LSP by default
			IgnoreGitignore: boolPtr(true),
			IgnoreLSPNotify: boolPtr(true),
		},
	}
}

// defaultPromptConfig returns the default prompt template values.
func defaultPromptConfig() PromptConfig {
	return PromptConfig{
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
