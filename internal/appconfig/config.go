// Summary: Application configuration model and loader; reads ~/.config/hexai/config.toml and merges defaults.
package appconfig

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// App holds user-configurable settings read from ~/.config/hexai/config.toml.
type App struct {
	MaxTokens          int    `json:"max_tokens" toml:"max_tokens"`
	ContextMode        string `json:"context_mode" toml:"context_mode"`
	ContextWindowLines int    `json:"context_window_lines" toml:"context_window_lines"`
	MaxContextTokens   int    `json:"max_context_tokens" toml:"max_context_tokens"`
	LogPreviewLimit    int    `json:"log_preview_limit" toml:"log_preview_limit"`
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

	TriggerCharacters []string `json:"trigger_characters" toml:"trigger_characters"`
	Provider          string   `json:"provider" toml:"provider"`

	// Inline prompt trigger characters (default: >text> and >>text>)
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
	OllamaBaseURL     string   `json:"ollama_base_url" toml:"ollama_base_url"`
	OllamaModel       string   `json:"ollama_model" toml:"ollama_model"`
	// Default temperature for Ollama requests (nil means use provider default)
	OllamaTemperature *float64 `json:"ollama_temperature" toml:"ollama_temperature"`
	CopilotBaseURL    string   `json:"copilot_base_url" toml:"copilot_base_url"`
	CopilotModel      string   `json:"copilot_model" toml:"copilot_model"`
	// Default temperature for Copilot requests (nil means use provider default)
	CopilotTemperature *float64 `json:"copilot_temperature" toml:"copilot_temperature"`

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
		CodingTemperature:     &t,
		OpenAITemperature:     &t,
		OllamaTemperature:     &t,
		CopilotTemperature:    &t,
		ManualInvokeMinPrefix: 0,
		CompletionDebounceMs:  800,
		CompletionThrottleMs:  0,
		// Inline/chat trigger defaults
		InlineOpen:   ">",
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
	}
}

// Load reads configuration from a file and merges with defaults.
// It respects the XDG Base Directory Specification.
func Load(logger *log.Logger) App {
	cfg := newDefaultConfig()
	if logger == nil {
		return cfg // Return defaults if no logger is provided (e.g. in tests)
	}

	configPath, err := getConfigPath()
	if err != nil {
		logger.Printf("%v", err)
		// Even if config path cannot be resolved, still allow env overrides below.
	} else {
		if fileCfg, err := loadFromFile(configPath, logger); err == nil && fileCfg != nil {
			cfg.mergeWith(fileCfg)
		}
		// When the config file is missing or invalid, we keep defaults and still
		// apply any environment overrides below.
	}

	// Environment overrides (take precedence over file)
	if envCfg := loadFromEnv(logger); envCfg != nil {
		cfg.mergeWith(envCfg)
	}
	return cfg
}

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
	Copilot    sectionCopilot    `toml:"copilot"`
	Ollama     sectionOllama     `toml:"ollama"`
	Prompts    sectionPrompts    `toml:"prompts"`
	Tmux       sectionTmux       `toml:"tmux"`
	Stats      sectionStats      `toml:"stats"`
}

type sectionGeneral struct {
	MaxTokens          int      `toml:"max_tokens"`
	ContextMode        string   `toml:"context_mode"`
	ContextWindowLines int      `toml:"context_window_lines"`
	MaxContextTokens   int      `toml:"max_context_tokens"`
	CodingTemperature  *float64 `toml:"coding_temperature"`
}

type sectionLogging struct {
	LogPreviewLimit int `toml:"log_preview_limit"`
}

type sectionCompletion struct {
	CompletionDebounceMs  int `toml:"completion_debounce_ms"`
	CompletionThrottleMs  int `toml:"completion_throttle_ms"`
	ManualInvokeMinPrefix int `toml:"manual_invoke_min_prefix"`
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

type sectionOpenAI struct {
	Model       string   `toml:"model"`
	BaseURL     string   `toml:"base_url"`
	Temperature *float64 `toml:"temperature"`
}

type sectionCopilot struct {
	Model       string   `toml:"model"`
	BaseURL     string   `toml:"base_url"`
	Temperature *float64 `toml:"temperature"`
}

type sectionOllama struct {
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

func (fc *fileConfig) toApp() App {
	out := App{}

	// Merge section: general
	if (fc.General != sectionGeneral{}) || fc.General.CodingTemperature != nil {
		tmp := App{
			MaxTokens:          fc.General.MaxTokens,
			ContextMode:        fc.General.ContextMode,
			ContextWindowLines: fc.General.ContextWindowLines,
			MaxContextTokens:   fc.General.MaxContextTokens,
			CodingTemperature:  fc.General.CodingTemperature,
		}
		out.mergeBasics(&tmp)
	}

	// logging
	if (fc.Logging != sectionLogging{}) {
		tmp := App{LogPreviewLimit: fc.Logging.LogPreviewLimit}
		out.mergeBasics(&tmp)
	}

	// completion
	if (fc.Completion != sectionCompletion{}) {
		tmp := App{
			CompletionDebounceMs:  fc.Completion.CompletionDebounceMs,
			CompletionThrottleMs:  fc.Completion.CompletionThrottleMs,
			ManualInvokeMinPrefix: fc.Completion.ManualInvokeMinPrefix,
		}
		out.mergeBasics(&tmp)
	}

	// triggers
	if len(fc.Triggers.TriggerCharacters) > 0 {
		tmp := App{TriggerCharacters: fc.Triggers.TriggerCharacters}
		out.mergeBasics(&tmp)
	}

	// inline
	if (fc.Inline != sectionInline{}) {
		tmp := App{InlineOpen: fc.Inline.InlineOpen, InlineClose: fc.Inline.InlineClose}
		out.mergeBasics(&tmp)
	}

	// chat
	if strings.TrimSpace(fc.Chat.ChatSuffix) != "" || len(fc.Chat.ChatPrefixes) > 0 {
		tmp := App{ChatSuffix: fc.Chat.ChatSuffix, ChatPrefixes: fc.Chat.ChatPrefixes}
		out.mergeBasics(&tmp)
	}

	// provider
	if strings.TrimSpace(fc.Provider.Name) != "" {
		tmp := App{Provider: fc.Provider.Name}
		out.mergeBasics(&tmp)
	}

	// openai
	if (fc.OpenAI != sectionOpenAI{}) || fc.OpenAI.Temperature != nil {
		tmp := App{
			OpenAIBaseURL:     fc.OpenAI.BaseURL,
			OpenAIModel:       fc.OpenAI.Model,
			OpenAITemperature: fc.OpenAI.Temperature,
		}
		out.mergeProviderFields(&tmp)
	}

	// copilot
	if (fc.Copilot != sectionCopilot{}) || fc.Copilot.Temperature != nil {
		tmp := App{
			CopilotBaseURL:     fc.Copilot.BaseURL,
			CopilotModel:       fc.Copilot.Model,
			CopilotTemperature: fc.Copilot.Temperature,
		}
		out.mergeProviderFields(&tmp)
	}

	// ollama
	if (fc.Ollama != sectionOllama{}) || fc.Ollama.Temperature != nil {
		tmp := App{
			OllamaBaseURL:     fc.Ollama.BaseURL,
			OllamaModel:       fc.Ollama.Model,
			OllamaTemperature: fc.Ollama.Temperature,
		}
		out.mergeProviderFields(&tmp)
	}

	// prompts
	// completion
	if (fc.Prompts.Completion != sectionPromptsCompletion{}) {
		if strings.TrimSpace(fc.Prompts.Completion.SystemGeneral) != "" {
			out.PromptCompletionSystemGeneral = fc.Prompts.Completion.SystemGeneral
		}
		if strings.TrimSpace(fc.Prompts.Completion.SystemParams) != "" {
			out.PromptCompletionSystemParams = fc.Prompts.Completion.SystemParams
		}
		if strings.TrimSpace(fc.Prompts.Completion.SystemInline) != "" {
			out.PromptCompletionSystemInline = fc.Prompts.Completion.SystemInline
		}
		if strings.TrimSpace(fc.Prompts.Completion.UserGeneral) != "" {
			out.PromptCompletionUserGeneral = fc.Prompts.Completion.UserGeneral
		}
		if strings.TrimSpace(fc.Prompts.Completion.UserParams) != "" {
			out.PromptCompletionUserParams = fc.Prompts.Completion.UserParams
		}
		if strings.TrimSpace(fc.Prompts.Completion.ExtraHeader) != "" {
			out.PromptCompletionExtraHeader = fc.Prompts.Completion.ExtraHeader
		}
	}
	// chat
	if strings.TrimSpace(fc.Prompts.Chat.System) != "" {
		out.PromptChatSystem = fc.Prompts.Chat.System
	}
	// code action
	if strings.TrimSpace(fc.Prompts.CodeAction.RewriteSystem) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.DiagnosticsSystem) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.DocumentSystem) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.RewriteUser) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.DiagnosticsUser) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.DocumentUser) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.GoTestSystem) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.GoTestUser) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.SimplifySystem) != "" ||
		strings.TrimSpace(fc.Prompts.CodeAction.SimplifyUser) != "" ||
		len(fc.Prompts.CodeAction.Custom) > 0 {
		if strings.TrimSpace(fc.Prompts.CodeAction.RewriteSystem) != "" {
			out.PromptCodeActionRewriteSystem = fc.Prompts.CodeAction.RewriteSystem
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.DiagnosticsSystem) != "" {
			out.PromptCodeActionDiagnosticsSystem = fc.Prompts.CodeAction.DiagnosticsSystem
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.DocumentSystem) != "" {
			out.PromptCodeActionDocumentSystem = fc.Prompts.CodeAction.DocumentSystem
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.RewriteUser) != "" {
			out.PromptCodeActionRewriteUser = fc.Prompts.CodeAction.RewriteUser
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.DiagnosticsUser) != "" {
			out.PromptCodeActionDiagnosticsUser = fc.Prompts.CodeAction.DiagnosticsUser
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.DocumentUser) != "" {
			out.PromptCodeActionDocumentUser = fc.Prompts.CodeAction.DocumentUser
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.GoTestSystem) != "" {
			out.PromptCodeActionGoTestSystem = fc.Prompts.CodeAction.GoTestSystem
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.GoTestUser) != "" {
			out.PromptCodeActionGoTestUser = fc.Prompts.CodeAction.GoTestUser
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.SimplifySystem) != "" {
			out.PromptCodeActionSimplifySystem = fc.Prompts.CodeAction.SimplifySystem
		}
		if strings.TrimSpace(fc.Prompts.CodeAction.SimplifyUser) != "" {
			out.PromptCodeActionSimplifyUser = fc.Prompts.CodeAction.SimplifyUser
		}
		if len(fc.Prompts.CodeAction.Custom) > 0 {
			for _, ca := range fc.Prompts.CodeAction.Custom {
				out.CustomActions = append(out.CustomActions, CustomAction{
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
		}
	}
	// cli
	if (fc.Prompts.CLI != sectionPromptsCLI{}) {
		if strings.TrimSpace(fc.Prompts.CLI.DefaultSystem) != "" {
			out.PromptCLIDefaultSystem = fc.Prompts.CLI.DefaultSystem
		}
		if strings.TrimSpace(fc.Prompts.CLI.ExplainSystem) != "" {
			out.PromptCLIExplainSystem = fc.Prompts.CLI.ExplainSystem
		}
	}
	// provider-native
	if strings.TrimSpace(fc.Prompts.ProviderNative.Completion) != "" {
		out.PromptNativeCompletion = fc.Prompts.ProviderNative.Completion
	}

	// tmux
	if (fc.Tmux != sectionTmux{}) {
		out.TmuxCustomMenuHotkey = strings.TrimSpace(fc.Tmux.CustomMenuHotkey)
	}

	// stats
	if fc.Stats.WindowMinutes > 0 {
		out.StatsWindowMinutes = fc.Stats.WindowMinutes
	}

	return out
}

func loadFromFile(path string, logger *log.Logger) (*App, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) && logger != nil {
			logger.Printf("cannot open TOML config file %s: %v", path, err)
		}
		return nil, err
	}

	var tables fileConfig
	errTables := toml.NewDecoder(strings.NewReader(string(b))).Decode(&tables)
	// Raw map for validation/presence checks
	var raw map[string]any
	_ = toml.Unmarshal(b, &raw)
	if errTables != nil {
		if logger != nil {
			logger.Printf("invalid TOML config file %s: %v", path, errTables)
		}
		return nil, errTables
	}

	// Reject legacy flat keys at top-level (sectioned-only config is allowed)
	legacy := map[string]struct{}{
		"max_tokens": {}, "context_mode": {}, "context_window_lines": {}, "max_context_tokens": {},
		"log_preview_limit": {}, "completion_debounce_ms": {}, "completion_throttle_ms": {},
		"manual_invoke_min_prefix": {}, "trigger_characters": {}, "inline_open": {}, "inline_close": {},
		"chat_suffix": {}, "chat_prefixes": {}, "coding_temperature": {}, "provider": {},
		"openai_model": {}, "openai_base_url": {}, "openai_temperature": {},
		"ollama_model": {}, "ollama_base_url": {}, "ollama_temperature": {},
		"copilot_model": {}, "copilot_base_url": {}, "copilot_temperature": {},
	}
	for k := range raw {
		if _, isTable := map[string]struct{}{"general": {}, "logging": {}, "completion": {}, "triggers": {}, "inline": {}, "chat": {}, "provider": {}, "openai": {}, "copilot": {}, "ollama": {}, "prompts": {}}[k]; isTable {
			continue
		}
		if _, isLegacy := legacy[k]; isLegacy {
			return nil, fmt.Errorf("unsupported flat key '%s' in config; use sectioned tables (see config.toml.example)", k)
		}
	}

	if logger != nil {
		logger.Printf("loaded configuration from %s (TOML)", path)
	}

	// Merge order: flat first, then tables (so tables win over zero flat values)
	// Build App from tables only
	tab := tables.toApp()
	// Ensure explicit values from raw map are respected (defensive for ints)
	if t, ok := raw["completion"].(map[string]any); ok {
		if v, present := t["manual_invoke_min_prefix"]; present {
			switch vv := v.(type) {
			case int64:
				tab.ManualInvokeMinPrefix = int(vv)
			case int:
				tab.ManualInvokeMinPrefix = vv
			case float64:
				tab.ManualInvokeMinPrefix = int(vv)
			}
		}
	}
	if t, ok := raw["logging"].(map[string]any); ok {
		if v, present := t["log_preview_limit"]; present {
			switch vv := v.(type) {
			case int64:
				tab.LogPreviewLimit = int(vv)
			case int:
				tab.LogPreviewLimit = vv
			case float64:
				tab.LogPreviewLimit = int(vv)
			}
		}
	}
	return &tab, nil
}

func (a *App) mergeWith(other *App) {
	a.mergeBasics(other)
	a.mergeProviderFields(other)
	a.mergePrompts(other)
}

// mergeBasics merges general (non-provider) fields.
func (a *App) mergeBasics(other *App) {
	if other.MaxTokens > 0 {
		a.MaxTokens = other.MaxTokens
	}
	if s := strings.TrimSpace(other.ContextMode); s != "" {
		a.ContextMode = s
	}
	if other.ContextWindowLines > 0 {
		a.ContextWindowLines = other.ContextWindowLines
	}
	if other.MaxContextTokens > 0 {
		a.MaxContextTokens = other.MaxContextTokens
	}
	if other.LogPreviewLimit >= 0 {
		a.LogPreviewLimit = other.LogPreviewLimit
	}
	if other.CodingTemperature != nil { // allow explicit 0.0
		a.CodingTemperature = other.CodingTemperature
	}
	if other.ManualInvokeMinPrefix >= 0 {
		a.ManualInvokeMinPrefix = other.ManualInvokeMinPrefix
	}
	if other.CompletionDebounceMs > 0 {
		a.CompletionDebounceMs = other.CompletionDebounceMs
	}
	if other.CompletionThrottleMs > 0 {
		a.CompletionThrottleMs = other.CompletionThrottleMs
	}
	if len(other.TriggerCharacters) > 0 {
		a.TriggerCharacters = slices.Clone(other.TriggerCharacters)
	}
	if s := strings.TrimSpace(other.InlineOpen); s != "" {
		a.InlineOpen = s
	}
	if s := strings.TrimSpace(other.InlineClose); s != "" {
		a.InlineClose = s
	}
	if s := strings.TrimSpace(other.ChatSuffix); s != "" {
		a.ChatSuffix = s
	}
	if len(other.ChatPrefixes) > 0 {
		a.ChatPrefixes = slices.Clone(other.ChatPrefixes)
	}
	if s := strings.TrimSpace(other.Provider); s != "" {
		a.Provider = s
	}
}

// mergePrompts copies non-empty prompt templates from other.
func (a *App) mergePrompts(other *App) {
	// Completion
	if strings.TrimSpace(other.PromptCompletionSystemGeneral) != "" {
		a.PromptCompletionSystemGeneral = other.PromptCompletionSystemGeneral
	}
	if strings.TrimSpace(other.PromptCompletionSystemParams) != "" {
		a.PromptCompletionSystemParams = other.PromptCompletionSystemParams
	}
	if strings.TrimSpace(other.PromptCompletionSystemInline) != "" {
		a.PromptCompletionSystemInline = other.PromptCompletionSystemInline
	}
	if strings.TrimSpace(other.PromptCompletionUserGeneral) != "" {
		a.PromptCompletionUserGeneral = other.PromptCompletionUserGeneral
	}
	if strings.TrimSpace(other.PromptCompletionUserParams) != "" {
		a.PromptCompletionUserParams = other.PromptCompletionUserParams
	}
	if strings.TrimSpace(other.PromptCompletionExtraHeader) != "" {
		a.PromptCompletionExtraHeader = other.PromptCompletionExtraHeader
	}
	// Provider-native
	if strings.TrimSpace(other.PromptNativeCompletion) != "" {
		a.PromptNativeCompletion = other.PromptNativeCompletion
	}
	// Chat
	if strings.TrimSpace(other.PromptChatSystem) != "" {
		a.PromptChatSystem = other.PromptChatSystem
	}
	// Code actions
	if strings.TrimSpace(other.PromptCodeActionRewriteSystem) != "" {
		a.PromptCodeActionRewriteSystem = other.PromptCodeActionRewriteSystem
	}
	if strings.TrimSpace(other.PromptCodeActionDiagnosticsSystem) != "" {
		a.PromptCodeActionDiagnosticsSystem = other.PromptCodeActionDiagnosticsSystem
	}
	if strings.TrimSpace(other.PromptCodeActionDocumentSystem) != "" {
		a.PromptCodeActionDocumentSystem = other.PromptCodeActionDocumentSystem
	}
	if strings.TrimSpace(other.PromptCodeActionRewriteUser) != "" {
		a.PromptCodeActionRewriteUser = other.PromptCodeActionRewriteUser
	}
	if strings.TrimSpace(other.PromptCodeActionDiagnosticsUser) != "" {
		a.PromptCodeActionDiagnosticsUser = other.PromptCodeActionDiagnosticsUser
	}
	if strings.TrimSpace(other.PromptCodeActionDocumentUser) != "" {
		a.PromptCodeActionDocumentUser = other.PromptCodeActionDocumentUser
	}
	if strings.TrimSpace(other.PromptCodeActionGoTestSystem) != "" {
		a.PromptCodeActionGoTestSystem = other.PromptCodeActionGoTestSystem
	}
	if strings.TrimSpace(other.PromptCodeActionGoTestUser) != "" {
		a.PromptCodeActionGoTestUser = other.PromptCodeActionGoTestUser
	}
	if strings.TrimSpace(other.PromptCodeActionSimplifySystem) != "" {
		a.PromptCodeActionSimplifySystem = other.PromptCodeActionSimplifySystem
	}
	if strings.TrimSpace(other.PromptCodeActionSimplifyUser) != "" {
		a.PromptCodeActionSimplifyUser = other.PromptCodeActionSimplifyUser
	}
	// CLI
	if strings.TrimSpace(other.PromptCLIDefaultSystem) != "" {
		a.PromptCLIDefaultSystem = other.PromptCLIDefaultSystem
	}
	if strings.TrimSpace(other.PromptCLIExplainSystem) != "" {
		a.PromptCLIExplainSystem = other.PromptCLIExplainSystem
	}
	// Custom actions
	if len(other.CustomActions) > 0 {
		a.CustomActions = append([]CustomAction{}, other.CustomActions...)
	}
	if strings.TrimSpace(other.TmuxCustomMenuHotkey) != "" {
		a.TmuxCustomMenuHotkey = other.TmuxCustomMenuHotkey
	}
}

// Validate checks custom actions and tmux settings for duplicates and consistency.
func (a App) Validate() error {
	// Normalize and check duplicates for IDs and hotkeys
	seenID := make(map[string]struct{})
	seenHK := make(map[string]struct{})
	for _, ca := range a.CustomActions {
		id := strings.ToLower(strings.TrimSpace(ca.ID))
		if id == "" {
			return fmt.Errorf("config: custom action missing required field id")
		}
		if _, ok := seenID[id]; ok {
			return fmt.Errorf("config: duplicate custom action id: %s", ca.ID)
		}
		seenID[id] = struct{}{}
		if strings.TrimSpace(ca.Title) == "" {
			return fmt.Errorf("config: custom action %s missing required field title", ca.ID)
		}
		// Validate scope
		scope := strings.TrimSpace(ca.Scope)
		if scope != "" && scope != "selection" && scope != "diagnostics" {
			return fmt.Errorf("config: custom action %s has invalid scope: %s", ca.ID, ca.Scope)
		}
		// Instruction vs user
		hasInstr := strings.TrimSpace(ca.Instruction) != ""
		hasUser := strings.TrimSpace(ca.User) != ""
		if hasInstr && hasUser {
			return fmt.Errorf("config: custom action %s must set either instruction or user, not both", ca.ID)
		}
		if !hasInstr && !hasUser {
			return fmt.Errorf("config: custom action %s requires instruction or user", ca.ID)
		}
		// Hotkey unique (case-insensitive), one rune if provided
		if hk := strings.TrimSpace(ca.Hotkey); hk != "" {
			if []rune(hk) == nil || len([]rune(hk)) != 1 {
				return fmt.Errorf("config: custom action %s hotkey must be a single character", ca.ID)
			}
			lhk := strings.ToLower(hk)
			if _, ok := seenHK[lhk]; ok {
				return fmt.Errorf("config: duplicate custom action hotkey: %s", hk)
			}
			seenHK[lhk] = struct{}{}
		}
	}
	// Tmux custom menu hotkey validation
	if hk := strings.TrimSpace(a.TmuxCustomMenuHotkey); hk != "" {
		if len([]rune(hk)) != 1 {
			return fmt.Errorf("config: invalid tmux.custom_menu_hotkey: %s", hk)
		}
		// built-in hotkeys in tmux TUI: r,i,c,t,p,s
		switch strings.ToLower(hk) {
		case "r", "i", "c", "t", "p", "s":
			return fmt.Errorf("config: invalid tmux.custom_menu_hotkey: %s (clashes with built-in)", hk)
		}
	}
	return nil
}

// mergeProviderFields merges per-provider configuration.
func (a *App) mergeProviderFields(other *App) {
	if s := strings.TrimSpace(other.OpenAIBaseURL); s != "" {
		a.OpenAIBaseURL = s
	}
	if s := strings.TrimSpace(other.OpenAIModel); s != "" {
		a.OpenAIModel = s
	}
	if other.OpenAITemperature != nil { // allow explicit 0.0
		a.OpenAITemperature = other.OpenAITemperature
	}
	if s := strings.TrimSpace(other.OllamaBaseURL); s != "" {
		a.OllamaBaseURL = s
	}
	if s := strings.TrimSpace(other.OllamaModel); s != "" {
		a.OllamaModel = s
	}
	if other.OllamaTemperature != nil { // allow explicit 0.0
		a.OllamaTemperature = other.OllamaTemperature
	}
	if s := strings.TrimSpace(other.CopilotBaseURL); s != "" {
		a.CopilotBaseURL = s
	}
	if s := strings.TrimSpace(other.CopilotModel); s != "" {
		a.CopilotModel = s
	}
	if other.CopilotTemperature != nil { // allow explicit 0.0
		a.CopilotTemperature = other.CopilotTemperature
	}
}

func getConfigPath() (string, error) {
	var configPath string
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		configPath = filepath.Join(xdgConfigHome, "hexai", "config.toml")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot find user home directory: %v", err)
		}
		configPath = filepath.Join(home, ".config", "hexai", "config.toml")
	}
	return configPath, nil
}

// --- Environment overrides ---

// loadFromEnv constructs an App containing only fields set via HEXAI_* env vars.
// These values should take precedence over file config when merged.
func loadFromEnv(logger *log.Logger) *App {
	var out App
	var any bool

	// helpers
	getenv := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
	parseInt := func(k string) (int, bool) {
		v := getenv(k)
		if v == "" {
			return 0, false
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			if logger != nil {
				logger.Printf("invalid %s: %v", k, err)
			}
			return 0, false
		}
		return n, true
	}
	parseFloatPtr := func(k string) (*float64, bool) {
		v := getenv(k)
		if v == "" {
			return nil, false
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			if logger != nil {
				logger.Printf("invalid %s: %v", k, err)
			}
			return nil, false
		}
		return &f, true
	}

	if n, ok := parseInt("HEXAI_MAX_TOKENS"); ok {
		out.MaxTokens = n
		any = true
	}
	if s := getenv("HEXAI_CONTEXT_MODE"); s != "" {
		out.ContextMode = s
		any = true
	}
	if n, ok := parseInt("HEXAI_CONTEXT_WINDOW_LINES"); ok {
		out.ContextWindowLines = n
		any = true
	}
	if n, ok := parseInt("HEXAI_MAX_CONTEXT_TOKENS"); ok {
		out.MaxContextTokens = n
		any = true
	}
	if n, ok := parseInt("HEXAI_LOG_PREVIEW_LIMIT"); ok {
		out.LogPreviewLimit = n
		any = true
	}
	if n, ok := parseInt("HEXAI_MANUAL_INVOKE_MIN_PREFIX"); ok {
		out.ManualInvokeMinPrefix = n
		any = true
	}
	if n, ok := parseInt("HEXAI_COMPLETION_DEBOUNCE_MS"); ok {
		out.CompletionDebounceMs = n
		any = true
	}
	if n, ok := parseInt("HEXAI_COMPLETION_THROTTLE_MS"); ok {
		out.CompletionThrottleMs = n
		any = true
	}
	if f, ok := parseFloatPtr("HEXAI_CODING_TEMPERATURE"); ok {
		out.CodingTemperature = f
		any = true
	}
	if s := getenv("HEXAI_TRIGGER_CHARACTERS"); s != "" {
		parts := strings.Split(s, ",")
		out.TriggerCharacters = nil
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out.TriggerCharacters = append(out.TriggerCharacters, t)
			}
		}
		any = true
	}
	if s := getenv("HEXAI_INLINE_OPEN"); s != "" {
		out.InlineOpen = s
		any = true
	}
	if s := getenv("HEXAI_INLINE_CLOSE"); s != "" {
		out.InlineClose = s
		any = true
	}
	if s := getenv("HEXAI_CHAT_SUFFIX"); s != "" {
		out.ChatSuffix = s
		any = true
	}
	if s := getenv("HEXAI_CHAT_PREFIXES"); s != "" {
		parts := strings.Split(s, ",")
		out.ChatPrefixes = nil
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out.ChatPrefixes = append(out.ChatPrefixes, t)
			}
		}
		any = true
	}
	if s := getenv("HEXAI_PROVIDER"); s != "" {
		out.Provider = s
		any = true
	}

	// Provider-specific
	if s := getenv("HEXAI_OPENAI_BASE_URL"); s != "" {
		out.OpenAIBaseURL = s
		any = true
	}
	if s := getenv("HEXAI_OPENAI_MODEL"); s != "" {
		out.OpenAIModel = s
		any = true
	}
	if f, ok := parseFloatPtr("HEXAI_OPENAI_TEMPERATURE"); ok {
		out.OpenAITemperature = f
		any = true
	}

	if s := getenv("HEXAI_OLLAMA_BASE_URL"); s != "" {
		out.OllamaBaseURL = s
		any = true
	}
	if s := getenv("HEXAI_OLLAMA_MODEL"); s != "" {
		out.OllamaModel = s
		any = true
	}
	if f, ok := parseFloatPtr("HEXAI_OLLAMA_TEMPERATURE"); ok {
		out.OllamaTemperature = f
		any = true
	}

	if s := getenv("HEXAI_COPILOT_BASE_URL"); s != "" {
		out.CopilotBaseURL = s
		any = true
	}
	if s := getenv("HEXAI_COPILOT_MODEL"); s != "" {
		out.CopilotModel = s
		any = true
	}
	if f, ok := parseFloatPtr("HEXAI_COPILOT_TEMPERATURE"); ok {
		out.CopilotTemperature = f
		any = true
	}

	if !any {
		return nil
	}
	return &out
}
