package lsp

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/runtimeconfig"
)

func TestFormatReloadSummary(t *testing.T) {
	changes := []runtimeconfig.Change{
		{Key: "max_tokens", Old: "200", New: "128"},
		{Key: "provider", Old: "openai", New: "ollama"},
	}
	got := formatReloadSummary(changes)
	if !strings.Contains(got, "Reloaded config (2 changes):") {
		t.Fatalf("expected change count line, got %q", got)
	}
	if !strings.Contains(got, "max_tokens: 200") || !strings.Contains(got, "provider: openai") {
		t.Fatalf("expected formatted entries, got %q", got)
	}
}

func TestHandleHelpCommandListsReload(t *testing.T) {
	s := newTestServer()
	res := s.handleHelpCommand()
	if !strings.Contains(res.message, "/reload?>") {
		t.Fatalf("expected reload command in help output: %q", res.message)
	}
}

func TestHandleReloadCommandReloadsStore(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "hexai")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 64\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HEXAI_MAX_TOKENS", "321")

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	initial := appconfig.Load(logger)
	if initial.MaxTokens != 321 {
		t.Fatalf("expected env override to win initial load, got %d", initial.MaxTokens)
	}

	store := runtimeconfig.New(initial)

	s := newTestServer()
	s.logger = logger
	s.configStore = store

	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 128\n"), 0o644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	res := s.handleReloadCommand()
	if !strings.Contains(res.message, "Reloaded config (1 changes):") {
		t.Fatalf("unexpected reload summary: %q", res.message)
	}
	if !strings.Contains(res.message, "max_tokens: 321") || !strings.Contains(res.message, "128") {
		t.Fatalf("expected diff for max_tokens: %q", res.message)
	}
	if store.Snapshot().MaxTokens != 128 {
		t.Fatalf("expected snapshot to reflect new value, got %d", store.Snapshot().MaxTokens)
	}
	if !strings.Contains(logBuf.String(), "Reloaded config") {
		t.Fatalf("expected summary logged, got %q", logBuf.String())
	}
}
