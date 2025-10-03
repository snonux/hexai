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
	got := runtimeconfig.FormatSummary("Reloaded config", changes)
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
	if !strings.Contains(res.message, "/disable?>") || !strings.Contains(res.message, "/enable?>") {
		t.Fatalf("expected completion toggle commands in help output: %q", res.message)
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
	t.Setenv("HEXAI_PROVIDER", "")

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

func TestDetectAndHandleChatExecutesSlashCommand(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "hexai")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 128\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HEXAI_MAX_TOKENS", "")
	t.Setenv("HEXAI_PROVIDER", "")

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	initial := appconfig.Load(logger)
	store := runtimeconfig.New(initial)

	s := newTestServer()
	s.logger = logger
	s.configStore = store
	var out bytes.Buffer
	s.out = &out
	s.setCompletionsDisabled(true) // chat commands should remain available when completions are disabled

	uri := "file:///cmd.go"
	s.setDocument(uri, "/reload>\n")

	s.detectAndHandleChat(uri)

	outStr := out.String()
	if !strings.Contains(outStr, "Reloaded config") {
		t.Fatalf("expected reload summary in applyEdit payload, got %q", outStr)
	}
	if !strings.Contains(logBuf.String(), "Reloaded config") {
		t.Fatalf("expected reload summary logged, got %q", logBuf.String())
	}
}

func TestDisableEnableCommandsToggleCompletions(t *testing.T) {
	s := newTestServer()
	if s.completionDisabled() {
		t.Fatalf("expected completions enabled initially")
	}

	if res, ok := s.chatCommandResponse("file:///x", 0, "/disable>"); !ok {
		t.Fatalf("expected disable command to be handled")
	} else if !strings.Contains(res.message, "disabled") {
		t.Fatalf("unexpected disable message: %q", res.message)
	}
	if !s.completionDisabled() {
		t.Fatalf("expected completions disabled after command")
	}

	if res, ok := s.chatCommandResponse("file:///x", 0, "/disable>"); !ok {
		t.Fatalf("expected repeated disable command to be handled")
	} else if !strings.Contains(res.message, "already disabled") {
		t.Fatalf("expected already-disabled message, got %q", res.message)
	}

	if res, ok := s.chatCommandResponse("file:///x", 0, "/enable>"); !ok {
		t.Fatalf("expected enable command to be handled")
	} else if !strings.Contains(res.message, "enabled") {
		t.Fatalf("unexpected enable message: %q", res.message)
	}
	if s.completionDisabled() {
		t.Fatalf("expected completions enabled after command")
	}

	if res, ok := s.chatCommandResponse("file:///x", 0, "/enable>"); !ok {
		t.Fatalf("expected repeated enable command to be handled")
	} else if !strings.Contains(res.message, "already enabled") {
		t.Fatalf("expected already-enabled message, got %q", res.message)
	}
}
