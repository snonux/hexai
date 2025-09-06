package appconfig

import (
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	logger := newLogger()
	cfg := Load(logger)
	def := newDefaultConfig()
	if cfg.MaxTokens != def.MaxTokens || cfg.ContextMode != def.ContextMode || cfg.ContextWindowLines != def.ContextWindowLines {
		t.Fatalf("expected defaults; got %+v want %+v", cfg, def)
	}
}

func TestLoad_FileMerge_And_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	// file configuration in TOML
	writeFile(t, cfgPath, `
max_tokens = 123
context_mode = "file-on-new-func"
context_window_lines = 50
max_context_tokens = 999
log_preview_limit = 0
coding_temperature = 0.0
manual_invoke_min_prefix = 2
completion_debounce_ms = 150
completion_throttle_ms = 300
trigger_characters = [".", ":"]
provider = "openai"
openai_base_url = "https://api.example"
openai_model = "gpt-x"
openai_temperature = 0.0
ollama_base_url = "http://ollama"
ollama_model = "llama"
ollama_temperature = 0.0
copilot_base_url = "http://copilot"
copilot_model = "ghost"
copilot_temperature = 0.0
`)

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
	withEnv(t, "HEXAI_COPILOT_BASE_URL", "http://copilot-override")
	withEnv(t, "HEXAI_COPILOT_MODEL", "ghost-override")
	withEnv(t, "HEXAI_COPILOT_TEMPERATURE", "0.3")

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
	if cfg.CopilotBaseURL != "http://copilot-override" || cfg.CopilotModel != "ghost-override" || cfg.CopilotTemperature == nil || *cfg.CopilotTemperature != 0.3 {
		t.Fatalf("copilot overrides not applied: %+v", cfg)
	}

	// Ensure file values would have applied absent env
	// Spot-check: reset env and reload
	for _, k := range []string{
		"HEXAI_MAX_TOKENS", "HEXAI_CONTEXT_MODE", "HEXAI_CONTEXT_WINDOW_LINES", "HEXAI_MAX_CONTEXT_TOKENS", "HEXAI_LOG_PREVIEW_LIMIT", "HEXAI_CODING_TEMPERATURE", "HEXAI_MANUAL_INVOKE_MIN_PREFIX", "HEXAI_COMPLETION_DEBOUNCE_MS", "HEXAI_COMPLETION_THROTTLE_MS", "HEXAI_TRIGGER_CHARACTERS", "HEXAI_PROVIDER", "HEXAI_OPENAI_BASE_URL", "HEXAI_OPENAI_MODEL", "HEXAI_OPENAI_TEMPERATURE", "HEXAI_OLLAMA_BASE_URL", "HEXAI_OLLAMA_MODEL", "HEXAI_OLLAMA_TEMPERATURE", "HEXAI_COPILOT_BASE_URL", "HEXAI_COPILOT_MODEL", "HEXAI_COPILOT_TEMPERATURE",
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
