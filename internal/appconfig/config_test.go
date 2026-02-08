package appconfig

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func newLogger() *log.Logger { return log.New(io.Discard, "", 0) }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// clearHexaiEnv removes any HEXAI_* variables to prevent environment leakage
// into tests that expect file-only configuration.
func clearHexaiEnv(t *testing.T) {
	t.Helper()
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "HEXAI_") {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) > 0 {
				t.Setenv(kv[0], "")
			}
		}
	}
}

func withEnv(t *testing.T, k, v string) {
	t.Helper()
	old := os.Getenv(k)
	_ = os.Setenv(k, v)
	t.Cleanup(func() { _ = os.Setenv(k, old) })
}

func TestLoad_Defaults_NoLogger(t *testing.T) {
	cfg := Load(nil)
	if cfg.MaxTokens == 0 || cfg.ContextMode == "" || cfg.ContextWindowLines == 0 || cfg.MaxContextTokens == 0 {
		t.Fatalf("expected defaults populated, got %+v", cfg)
	}
	if cfg.CodingTemperature == nil {
		t.Fatalf("expected default CodingTemperature")
	}
}

func TestLoad_Defaults_WithLogger_NoFile_NoEnv(t *testing.T) {
	clearHexaiEnv(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	logger := newLogger()
	cfg := Load(logger)
	def := newDefaultConfig()
	if cfg.MaxTokens != def.MaxTokens || cfg.ContextMode != def.ContextMode || cfg.ContextWindowLines != def.ContextWindowLines {
		t.Fatalf("expected defaults; got %+v want %+v", cfg, def)
	}
}

func TestParseSurfaceModels_CodeActionWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `
[models]
  [[models.code_action]]
  provider = "openai"
  model = "gpt-4o"

  [[models.code_action]]
  provider = "anthropic"
  model = "claude"
`)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	app, err := loadFromFile(path, logger)
	if err != nil {
		t.Fatalf("loadFromFile: %v", err)
	}
	if len(app.CodeActionConfigs) != 1 || app.CodeActionConfigs[0].Model != "gpt-4o" {
		t.Fatalf("expected single code action entry, got %+v", app.CodeActionConfigs)
	}
	if msg := buf.String(); !strings.Contains(msg, "models.code_action supports a single entry") {
		t.Fatalf("expected warning, got %q", msg)
	}
}

func TestLoad_FileMerge_And_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	// file configuration in TOML (sectioned)
	writeFile(t, cfgPath, `
[general]
max_tokens = 123
context_mode = "file-on-new-func"
context_window_lines = 50
max_context_tokens = 999
coding_temperature = 0.0

[logging]
log_preview_limit = 0

[completion]
manual_invoke_min_prefix = 2
completion_debounce_ms = 150
completion_throttle_ms = 300

[triggers]
trigger_characters = [".", ":"]

[[models.completion]]
model = "gpt-file-complete"
provider = "openai"

[[models.code_action]]
model = "claude-file-action"
temperature = 0.45
provider = "anthropic"

[[models.chat]]
model = "gpt-file-chat"
provider = "openai"

[[models.cli]]
model = "gpt-file-cli"
temperature = 0.15
provider = "ollama"

[provider]
name = "openai"

[openai]
base_url = "https://api.example"
model = "gpt-x"
temperature = 0.0

[ollama]
base_url = "http://ollama"
model = "llama"
temperature = 0.0
`)

	if _, err := loadFromFile(cfgPath, newLogger()); err != nil {
		t.Fatalf("loadFromFile: %v", err)
	}

	// Env overrides take precedence
	withEnv(t, "HEXAI_MAX_TOKENS", "321")
	withEnv(t, "HEXAI_CONTEXT_MODE", "always-full")
	withEnv(t, "HEXAI_CONTEXT_WINDOW_LINES", "77")
	withEnv(t, "HEXAI_MAX_CONTEXT_TOKENS", "888")
	withEnv(t, "HEXAI_LOG_PREVIEW_LIMIT", "7")
	withEnv(t, "HEXAI_CODING_TEMPERATURE", "0.7")
	withEnv(t, "HEXAI_MANUAL_INVOKE_MIN_PREFIX", "5")
	withEnv(t, "HEXAI_COMPLETION_DEBOUNCE_MS", "333")
	withEnv(t, "HEXAI_COMPLETION_THROTTLE_MS", "444")
	withEnv(t, "HEXAI_TRIGGER_CHARACTERS", "., / ,_")
	withEnv(t, "HEXAI_PROVIDER", "ollama")
	withEnv(t, "HEXAI_OPENAI_BASE_URL", "https://override")
	withEnv(t, "HEXAI_OPENAI_MODEL", "gpt-override")
	withEnv(t, "HEXAI_OPENAI_TEMPERATURE", "0.4")
	withEnv(t, "HEXAI_OLLAMA_BASE_URL", "http://ollama-override")
	withEnv(t, "HEXAI_OLLAMA_MODEL", "mistral")
	withEnv(t, "HEXAI_OLLAMA_TEMPERATURE", "0.6")
	withEnv(t, "HEXAI_MODEL_COMPLETION", "env-completion")
	withEnv(t, "HEXAI_TEMPERATURE_COMPLETION", "0.33")
	withEnv(t, "HEXAI_PROVIDER_COMPLETION", "ollama")
	withEnv(t, "HEXAI_MODEL_CODE_ACTION", "env-action")
	withEnv(t, "HEXAI_TEMPERATURE_CODE_ACTION", "0.55")
	withEnv(t, "HEXAI_PROVIDER_CODE_ACTION", "openai")
	withEnv(t, "HEXAI_MODEL_CHAT", "env-chat")
	withEnv(t, "HEXAI_TEMPERATURE_CHAT", "0.66")
	withEnv(t, "HEXAI_PROVIDER_CHAT", "anthropic")
	withEnv(t, "HEXAI_MODEL_CLI", "env-cli")
	withEnv(t, "HEXAI_TEMPERATURE_CLI", "0.77")
	withEnv(t, "HEXAI_PROVIDER_CLI", "ollama")

	logger := newLogger()
	cfg := Load(logger)

	// Check overrides
	if cfg.MaxTokens != 321 || cfg.ContextMode != "always-full" || cfg.ContextWindowLines != 77 || cfg.MaxContextTokens != 888 {
		t.Fatalf("env overrides (basic) not applied: %+v", cfg)
	}
	if cfg.LogPreviewLimit != 7 || cfg.ManualInvokeMinPrefix != 5 || cfg.CompletionDebounceMs != 333 || cfg.CompletionThrottleMs != 444 {
		t.Fatalf("env overrides (ints) not applied: %+v", cfg)
	}
	if cfg.CodingTemperature == nil || *cfg.CodingTemperature != 0.7 {
		t.Fatalf("env override (CodingTemperature) not applied: %+v", cfg.CodingTemperature)
	}
	if want := []string{".", "/", "_"}; !reflect.DeepEqual(cfg.TriggerCharacters, want) {
		t.Fatalf("env override (TriggerCharacters), got %v want %v", cfg.TriggerCharacters, want)
	}
	if cfg.Provider != "ollama" {
		t.Fatalf("provider override failed: %q", cfg.Provider)
	}
	// Provider-specific
	if cfg.OpenAIBaseURL != "https://override" || cfg.OpenAIModel != "gpt-override" || cfg.OpenAITemperature == nil || *cfg.OpenAITemperature != 0.4 {
		t.Fatalf("openai overrides not applied: %+v", cfg)
	}
	if cfg.OllamaBaseURL != "http://ollama-override" || cfg.OllamaModel != "mistral" || cfg.OllamaTemperature == nil || *cfg.OllamaTemperature != 0.6 {
		t.Fatalf("ollama overrides not applied: %+v", cfg)
	}
	if len(cfg.CompletionConfigs) != 1 || cfg.CompletionConfigs[0].Model != "env-completion" {
		t.Fatalf("completion overrides not applied: %+v", cfg.CompletionConfigs)
	}
	if cfg.CompletionConfigs[0].Temperature == nil || *cfg.CompletionConfigs[0].Temperature != 0.33 {
		t.Fatalf("completion temperature override missing: %+v", cfg.CompletionConfigs[0])
	}
	if cfg.CompletionConfigs[0].Provider != "ollama" {
		t.Fatalf("completion provider override not applied: %+v", cfg.CompletionConfigs[0])
	}
	if len(cfg.CodeActionConfigs) != 1 || cfg.CodeActionConfigs[0].Model != "env-action" {
		t.Fatalf("code action overrides not applied: %+v", cfg.CodeActionConfigs)
	}
	if cfg.CodeActionConfigs[0].Temperature == nil || *cfg.CodeActionConfigs[0].Temperature != 0.55 {
		t.Fatalf("code action temp override missing: %+v", cfg.CodeActionConfigs[0])
	}
	if cfg.CodeActionConfigs[0].Provider != "openai" {
		t.Fatalf("code action provider override not applied: %+v", cfg.CodeActionConfigs[0])
	}
	if len(cfg.ChatConfigs) != 1 || cfg.ChatConfigs[0].Model != "env-chat" {
		t.Fatalf("chat overrides not applied: %+v", cfg.ChatConfigs)
	}
	if cfg.ChatConfigs[0].Temperature == nil || *cfg.ChatConfigs[0].Temperature != 0.66 {
		t.Fatalf("chat temp override missing: %+v", cfg.ChatConfigs[0])
	}
	if cfg.ChatConfigs[0].Provider != "anthropic" {
		t.Fatalf("chat provider override not applied: %+v", cfg.ChatConfigs[0])
	}
	if len(cfg.CLIConfigs) != 1 || cfg.CLIConfigs[0].Model != "env-cli" {
		t.Fatalf("cli overrides not applied: %+v", cfg.CLIConfigs)
	}
	if cfg.CLIConfigs[0].Temperature == nil || *cfg.CLIConfigs[0].Temperature != 0.77 {
		t.Fatalf("cli temp override missing: %+v", cfg.CLIConfigs[0])
	}
	if cfg.CLIConfigs[0].Provider != "ollama" {
		t.Fatalf("cli provider override not applied: %+v", cfg.CLIConfigs[0])
	}

	// Ensure file values would have applied absent env
	// Spot-check: reset env and reload
	for _, k := range []string{
		"HEXAI_MAX_TOKENS", "HEXAI_CONTEXT_MODE", "HEXAI_CONTEXT_WINDOW_LINES", "HEXAI_MAX_CONTEXT_TOKENS", "HEXAI_LOG_PREVIEW_LIMIT", "HEXAI_CODING_TEMPERATURE", "HEXAI_MANUAL_INVOKE_MIN_PREFIX", "HEXAI_COMPLETION_DEBOUNCE_MS", "HEXAI_COMPLETION_THROTTLE_MS", "HEXAI_TRIGGER_CHARACTERS", "HEXAI_PROVIDER", "HEXAI_OPENAI_BASE_URL", "HEXAI_OPENAI_MODEL", "HEXAI_OPENAI_TEMPERATURE", "HEXAI_OLLAMA_BASE_URL", "HEXAI_OLLAMA_MODEL", "HEXAI_OLLAMA_TEMPERATURE", "HEXAI_MODEL_COMPLETION", "HEXAI_TEMPERATURE_COMPLETION", "HEXAI_MODEL_CODE_ACTION", "HEXAI_TEMPERATURE_CODE_ACTION", "HEXAI_MODEL_CHAT", "HEXAI_TEMPERATURE_CHAT", "HEXAI_MODEL_CLI", "HEXAI_TEMPERATURE_CLI", "HEXAI_PROVIDER_COMPLETION", "HEXAI_PROVIDER_CODE_ACTION", "HEXAI_PROVIDER_CHAT", "HEXAI_PROVIDER_CLI",
	} {
		t.Setenv(k, "")
	}
	cfg2 := Load(logger)
	if cfg2.MaxTokens != 123 || cfg2.ContextMode != "file-on-new-func" || cfg2.ContextWindowLines != 50 || cfg2.MaxContextTokens != 999 || cfg2.LogPreviewLimit != 0 {
		t.Fatalf("file merge not applied: %+v", cfg2)
	}
	if cfg2.CodingTemperature == nil || *cfg2.CodingTemperature != 0.0 {
		t.Fatalf("file merge (CodingTemperature) not applied: %+v", cfg2.CodingTemperature)
	}
	if cfg2.OpenAIBaseURL != "https://api.example" || cfg2.OpenAIModel != "gpt-x" || cfg2.OpenAITemperature == nil || *cfg2.OpenAITemperature != 0.0 {
		t.Fatalf("file merge (openai) not applied: %+v", cfg2)
	}
	if len(cfg2.CompletionConfigs) != 1 || cfg2.CompletionConfigs[0].Model != "gpt-file-complete" {
		t.Fatalf("file merge (completion) not applied: %+v", cfg2.CompletionConfigs)
	}
	if cfg2.CompletionConfigs[0].Temperature != nil {
		t.Fatalf("expected nil completion temperature, got %+v", cfg2.CompletionConfigs[0])
	}
	if cfg2.CompletionConfigs[0].Provider != "openai" {
		t.Fatalf("file merge (completion provider) not applied: %+v", cfg2.CompletionConfigs[0])
	}
	if len(cfg2.CodeActionConfigs) != 1 || cfg2.CodeActionConfigs[0].Model != "claude-file-action" {
		t.Fatalf("file merge (code action) not applied: %+v", cfg2.CodeActionConfigs)
	}
	if cfg2.CodeActionConfigs[0].Temperature == nil || *cfg2.CodeActionConfigs[0].Temperature != 0.45 {
		t.Fatalf("expected code action temp 0.45, got %+v", cfg2.CodeActionConfigs[0])
	}
	if cfg2.CodeActionConfigs[0].Provider != "anthropic" {
		t.Fatalf("file merge (code action provider) not applied: %+v", cfg2.CodeActionConfigs[0])
	}
	if len(cfg2.ChatConfigs) != 1 || cfg2.ChatConfigs[0].Model != "gpt-file-chat" {
		t.Fatalf("file merge (chat) not applied: %+v", cfg2.ChatConfigs)
	}
	if cfg2.ChatConfigs[0].Temperature != nil {
		t.Fatalf("expected nil chat temp, got %+v", cfg2.ChatConfigs[0])
	}
	if cfg2.ChatConfigs[0].Provider != "openai" {
		t.Fatalf("file merge (chat provider) not applied: %+v", cfg2.ChatConfigs[0])
	}
	if len(cfg2.CLIConfigs) != 1 || cfg2.CLIConfigs[0].Model != "gpt-file-cli" {
		t.Fatalf("file merge (cli) not applied: %+v", cfg2.CLIConfigs)
	}
	if cfg2.CLIConfigs[0].Temperature == nil || *cfg2.CLIConfigs[0].Temperature != 0.15 {
		t.Fatalf("expected CLI temp 0.15, got %+v", cfg2.CLIConfigs[0])
	}
	if cfg2.CLIConfigs[0].Provider != "ollama" {
		t.Fatalf("file merge (cli provider) not applied: %+v", cfg2.CLIConfigs[0])
	}
}

func TestGetConfigPath_XDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(dir, "hexai")) || !strings.HasSuffix(path, "config.toml") {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestLoadFromFile_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("invalid ="), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadFromFile(cfgPath, newLogger())
	if err == nil {
		t.Fatalf("expected error for invalid TOML")
	}
}

func TestLoad_FileTables_Sectioned(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	content := `
[general]
max_tokens = 111
context_mode = "window"
context_window_lines = 42
max_context_tokens = 777
coding_temperature = 0.1

[logging]
log_preview_limit = 9

[completion]
completion_debounce_ms = 123
completion_throttle_ms = 456
manual_invoke_min_prefix = 3

[triggers]
trigger_characters = [".", ":"]

[inline]
inline_open = ">!"
inline_close = ">"

[chat]
chat_suffix = ">"
chat_prefixes = ["?", "!"]

[provider]
name = "openai"

[openai]
model = "gpt-x"
base_url = "https://api.example"
temperature = 0.0

[ollama]
model = "mistral"
base_url = "http://ollama"
temperature = 0.0
`
	writeFile(t, cfgPath, content)

	// Ensure no env override interferes with manual_invoke_min_prefix in this test
	t.Setenv("HEXAI_MANUAL_INVOKE_MIN_PREFIX", "")
	logger := newLogger()
	cfg := Load(logger)

	if cfg.MaxTokens != 111 || cfg.ContextMode != "window" || cfg.ContextWindowLines != 42 || cfg.MaxContextTokens != 777 {
		t.Fatalf("sectioned basics wrong: %+v", cfg)
	}
	if cfg.LogPreviewLimit != 9 || cfg.CompletionDebounceMs != 123 || cfg.CompletionThrottleMs != 456 || cfg.ManualInvokeMinPrefix != 3 {
		t.Fatalf("sectioned ints wrong: %+v", cfg)
	}
	if cfg.CodingTemperature == nil || *cfg.CodingTemperature != 0.1 {
		t.Fatalf("sectioned coding_temperature wrong: %+v", cfg.CodingTemperature)
	}
	if want := []string{".", ":"}; !reflect.DeepEqual(cfg.TriggerCharacters, want) {
		t.Fatalf("sectioned trigger chars wrong: got %v", cfg.TriggerCharacters)
	}
	if cfg.Provider != "openai" {
		t.Fatalf("sectioned provider name wrong: %q", cfg.Provider)
	}
	if cfg.OpenAIModel != "gpt-x" || cfg.OpenAIBaseURL != "https://api.example" || cfg.OpenAITemperature == nil || *cfg.OpenAITemperature != 0.0 {
		t.Fatalf("sectioned openai wrong: %+v", cfg)
	}
	if cfg.OllamaModel != "mistral" || cfg.OllamaBaseURL != "http://ollama" || cfg.OllamaTemperature == nil || *cfg.OllamaTemperature != 0.0 {
		t.Fatalf("sectioned ollama wrong: %+v", cfg)
	}
}

func TestLoad_FileTables_Prompts_AllSections(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	content := `
[prompts.completion]
system_general = "SYS-GENERAL"
system_params  = "SYS-PARAMS"
system_inline  = "SYS-INLINE"
user_general   = "USER-GENERAL {{file}} {{char}}"
user_params    = "USER-PARAMS {{function}}"
additional_context = "EXTRA {{context}}"

[prompts.provider_native]
completion = "NATIVE {{path}} {{before}}"

[prompts.chat]
system = "CHAT-SYS"

[prompts.code_action]
rewrite_system     = "REWRITE-SYS"
diagnostics_system = "DIAG-SYS"
document_system    = "DOC-SYS"
rewrite_user       = "REWRITE-USER {{instruction}} {{selection}}"
diagnostics_user   = "DIAG-USER {{diagnostics}} {{selection}}"
document_user      = "DOC-USER {{selection}}"
go_test_system     = "GOTEST-SYS"
go_test_user       = "GOTEST-USER {{function}}"

[prompts.cli]
default_system = "CLI-DEFAULT"
explain_system = "CLI-EXPLAIN"
`
	writeFile(t, cfgPath, content)

	cfg := Load(newLogger())

	// completion
	if cfg.PromptCompletionSystemGeneral != "SYS-GENERAL" || cfg.PromptCompletionSystemParams != "SYS-PARAMS" || cfg.PromptCompletionSystemInline != "SYS-INLINE" {
		t.Fatalf("completion system prompts wrong: %+v", cfg)
	}
	if cfg.PromptCompletionUserGeneral == "" || cfg.PromptCompletionUserParams == "" || cfg.PromptCompletionExtraHeader == "" {
		t.Fatalf("completion user/extra prompts not loaded")
	}
	// provider-native
	if cfg.PromptNativeCompletion != "NATIVE {{path}} {{before}}" {
		t.Fatalf("provider-native prompt wrong: %q", cfg.PromptNativeCompletion)
	}
	// chat
	if cfg.PromptChatSystem != "CHAT-SYS" {
		t.Fatalf("chat system wrong: %q", cfg.PromptChatSystem)
	}
	// code action
	if cfg.PromptCodeActionRewriteSystem != "REWRITE-SYS" || cfg.PromptCodeActionDiagnosticsSystem != "DIAG-SYS" || cfg.PromptCodeActionDocumentSystem != "DOC-SYS" {
		t.Fatalf("code action system prompts wrong")
	}
	if cfg.PromptCodeActionRewriteUser == "" || cfg.PromptCodeActionDiagnosticsUser == "" || cfg.PromptCodeActionDocumentUser == "" {
		t.Fatalf("code action user prompts not loaded")
	}
	if cfg.PromptCodeActionGoTestSystem != "GOTEST-SYS" || cfg.PromptCodeActionGoTestUser == "" {
		t.Fatalf("go test prompts wrong")
	}
	// CLI
	if cfg.PromptCLIDefaultSystem != "CLI-DEFAULT" || cfg.PromptCLIExplainSystem != "CLI-EXPLAIN" {
		t.Fatalf("cli prompts wrong: %q %q", cfg.PromptCLIDefaultSystem, cfg.PromptCLIExplainSystem)
	}
}

func TestCustomActions_ParseAndValidate_OK(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	content := `
[prompts.code_action]

[[prompts.code_action.custom]]
id = "extract-function"
title = "Extract function"
kind = "refactor.extract"
scope = "selection"
hotkey = "e"
instruction = "Extract the selected code into a new function named 'extracted' and replace with a call. Return only code."

[[prompts.code_action.custom]]
id = "fix-lints"
title = "Fix linters"
kind = "quickfix"
scope = "diagnostics"
hotkey = "l"
system = "You are a precise code fixer."
user = "Diagnostics to resolve (selection only):\n{{diagnostics}}\n\nSelected code:\n{{selection}}"

[tmux]
custom_menu_hotkey = "a"
`
	writeFile(t, cfgPath, content)
	cfg := Load(newLogger())
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(cfg.CustomActions) != 2 {
		t.Fatalf("expected 2 custom actions, got %d", len(cfg.CustomActions))
	}
	if cfg.TmuxCustomMenuHotkey != "a" {
		t.Fatalf("tmux hotkey wrong: %q", cfg.TmuxCustomMenuHotkey)
	}
	// spot-check mapping
	if cfg.CustomActions[0].ID != "extract-function" || cfg.CustomActions[0].Scope != "selection" || cfg.CustomActions[0].Instruction == "" {
		t.Fatalf("first action mapping wrong: %+v", cfg.CustomActions[0])
	}
	if cfg.CustomActions[1].User == "" || cfg.CustomActions[1].Scope != "diagnostics" {
		t.Fatalf("second action mapping wrong: %+v", cfg.CustomActions[1])
	}
}

func TestCustomActions_DuplicateID_Error(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	writeFile(t, cfgPath, `
[prompts.code_action]
[[prompts.code_action.custom]]
id = "dup"
title = "A"
instruction = "x"
[[prompts.code_action.custom]]
id = "DUP"
title = "B"
instruction = "y"
`)
	cfg := Load(newLogger())
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate custom action id") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestCustomActions_DuplicateHotkey_Error(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	writeFile(t, cfgPath, `
[prompts.code_action]
[[prompts.code_action.custom]]
id = "a1"
title = "A"
instruction = "x"
hotkey = "e"
[[prompts.code_action.custom]]
id = "a2"
title = "B"
instruction = "y"
hotkey = "E"
`)
	cfg := Load(newLogger())
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate custom action hotkey") {
		t.Fatalf("expected duplicate hotkey error, got %v", err)
	}
}

func TestCustomActions_InvalidScope_Error(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	writeFile(t, cfgPath, `
[prompts.code_action]
[[prompts.code_action.custom]]
id = "a1"
title = "A"
instruction = "x"
scope = "bad"
`)
	cfg := Load(newLogger())
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "invalid scope") {
		t.Fatalf("expected invalid scope error, got %v", err)
	}
}

func TestTmuxMenuHotkey_Clash_Error(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	writeFile(t, cfgPath, `
[tmux]
custom_menu_hotkey = "r"
`)
	cfg := Load(newLogger())
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "clashes with built-in") {
		t.Fatalf("expected clash error, got %v", err)
	}
}

func TestFindGitRoot(t *testing.T) {
	// Create a temp dir with a .git subdirectory to simulate a git repo
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	// Create a nested subdir to test walking up
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Test from nested subdir — should find the git root
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	root := FindGitRoot()
	if root != dir {
		t.Fatalf("FindGitRoot() = %q, want %q", root, dir)
	}

	// Test from a dir with no .git ancestor
	noGit := t.TempDir()
	if err := os.Chdir(noGit); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	root = FindGitRoot()
	if root != "" {
		t.Fatalf("FindGitRoot() = %q, want empty", root)
	}
}

func TestLoadWithOptions_ProjectConfig(t *testing.T) {
	clearHexaiEnv(t)

	// Set up global config
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalCfgPath := filepath.Join(globalDir, "hexai", "config.toml")
	writeFile(t, globalCfgPath, `
[general]
max_tokens = 2000
context_mode = "always-full"

[provider]
name = "openai"
`)

	// Set up project root with .git and .hexaiconfig.toml
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	writeFile(t, filepath.Join(projectDir, ProjectConfigFilename), `
[general]
max_tokens = 8000

[provider]
name = "anthropic"
`)

	// Load using explicit ProjectRoot (avoids needing to chdir)
	logger := newLogger()
	cfg := LoadWithOptions(logger, LoadOptions{ProjectRoot: projectDir})

	// Project config should override global values
	if cfg.MaxTokens != 8000 {
		t.Fatalf("expected project max_tokens=8000, got %d", cfg.MaxTokens)
	}
	if cfg.Provider != "anthropic" {
		t.Fatalf("expected project provider=anthropic, got %q", cfg.Provider)
	}
	// Values not overridden by project config should come from global config
	if cfg.ContextMode != "always-full" {
		t.Fatalf("expected global context_mode=always-full, got %q", cfg.ContextMode)
	}
}

func TestLoadWithOptions_ProjectConfig_EnvOverridesProject(t *testing.T) {
	clearHexaiEnv(t)

	// Set up global config
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalCfgPath := filepath.Join(globalDir, "hexai", "config.toml")
	writeFile(t, globalCfgPath, `
[general]
max_tokens = 2000
`)

	// Set up project config
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	writeFile(t, filepath.Join(projectDir, ProjectConfigFilename), `
[general]
max_tokens = 8000
`)

	// Env var should override project config
	withEnv(t, "HEXAI_MAX_TOKENS", "9999")

	logger := newLogger()
	cfg := LoadWithOptions(logger, LoadOptions{ProjectRoot: projectDir})

	if cfg.MaxTokens != 9999 {
		t.Fatalf("expected env max_tokens=9999 to override project, got %d", cfg.MaxTokens)
	}
}

func TestLoadWithOptions_ProjectConfig_NoGitRoot(t *testing.T) {
	clearHexaiEnv(t)

	// Set up global config
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalCfgPath := filepath.Join(globalDir, "hexai", "config.toml")
	writeFile(t, globalCfgPath, `
[general]
max_tokens = 2000
`)

	// No ProjectRoot, no .git — should work as before
	noGit := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(noGit); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	logger := newLogger()
	cfg := LoadWithOptions(logger, LoadOptions{})

	// Should get global config values, not defaults
	if cfg.MaxTokens != 2000 {
		t.Fatalf("expected global max_tokens=2000 without project config, got %d", cfg.MaxTokens)
	}
}

func TestProjectConfigPath(t *testing.T) {
	// Set up a fake git repo
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	path := ProjectConfigPath()
	want := filepath.Join(dir, ProjectConfigFilename)
	if path != want {
		t.Fatalf("ProjectConfigPath() = %q, want %q", path, want)
	}

	// No git root
	noGit := t.TempDir()
	if err := os.Chdir(noGit); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	path = ProjectConfigPath()
	if path != "" {
		t.Fatalf("ProjectConfigPath() = %q, want empty", path)
	}
}

func TestIgnoreConfig_Defaults(t *testing.T) {
	clearHexaiEnv(t)
	cfg := Load(nil)
	if cfg.IgnoreGitignore == nil || !*cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore default true")
	}
	if cfg.IgnoreLSPNotify == nil || !*cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify default true")
	}
	if len(cfg.IgnoreExtraPatterns) != 0 {
		t.Errorf("expected empty IgnoreExtraPatterns, got %v", cfg.IgnoreExtraPatterns)
	}
}

func TestIgnoreConfig_FromFile(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = false
extra_patterns = ["*.min.js", "dist/**"]
lsp_notify_ignored = false
`)
	cfg := LoadWithOptions(newLogger(), LoadOptions{ConfigPath: cfgPath})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore false from file")
	}
	if cfg.IgnoreLSPNotify == nil || *cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify false from file")
	}
	want := []string{"*.min.js", "dist/**"}
	if !reflect.DeepEqual(cfg.IgnoreExtraPatterns, want) {
		t.Errorf("IgnoreExtraPatterns = %v, want %v", cfg.IgnoreExtraPatterns, want)
	}
}

func TestIgnoreConfig_EnvOverrides(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = true
lsp_notify_ignored = true
`)
	withEnv(t, "HEXAI_IGNORE_GITIGNORE", "false")
	withEnv(t, "HEXAI_IGNORE_LSP_NOTIFY", "0")
	withEnv(t, "HEXAI_IGNORE_EXTRA_PATTERNS", "*.bak,*.tmp")
	cfg := LoadWithOptions(newLogger(), LoadOptions{ConfigPath: cfgPath})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore false from env override")
	}
	if cfg.IgnoreLSPNotify == nil || *cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify false from env override")
	}
	want := []string{"*.bak", "*.tmp"}
	if !reflect.DeepEqual(cfg.IgnoreExtraPatterns, want) {
		t.Errorf("IgnoreExtraPatterns = %v, want %v", cfg.IgnoreExtraPatterns, want)
	}
}

func TestIgnoreConfig_ProjectOverride(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = true
`)
	// Set up a fake git repo with project override
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	projectCfg := filepath.Join(projectDir, ProjectConfigFilename)
	writeFile(t, projectCfg, `
[ignore]
gitignore = false
extra_patterns = ["build/**"]
`)
	cfg := LoadWithOptions(newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: projectDir})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected project override to set IgnoreGitignore false")
	}
	want := []string{"build/**"}
	if !reflect.DeepEqual(cfg.IgnoreExtraPatterns, want) {
		t.Errorf("IgnoreExtraPatterns = %v, want %v", cfg.IgnoreExtraPatterns, want)
	}
}

func TestIgnoreConfig_DisableGitignore(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = false
`)
	cfg := LoadWithOptions(newLogger(), LoadOptions{ConfigPath: cfgPath})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore false")
	}
	// LSP notify should still be true (default, not overridden)
	if cfg.IgnoreLSPNotify == nil || !*cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify to remain true (default)")
	}
}
