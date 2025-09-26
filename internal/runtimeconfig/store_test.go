package runtimeconfig

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestStoreReloadSkipsEnvOverrides(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "hexai")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 64\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HEXAI_MAX_TOKENS", "321")

	initial := appconfig.Load(logger)
	if initial.MaxTokens != 321 {
		t.Fatalf("expected env override to win initial load, got %d", initial.MaxTokens)
	}

	store := New(initial)
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 128\n"), 0o644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	changes, err := store.Reload(logger, appconfig.LoadOptions{IgnoreEnv: true})
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if snap := store.Snapshot(); snap.MaxTokens != 128 {
		t.Fatalf("expected reload to apply file value, got %d", snap.MaxTokens)
	}

	found := false
	for _, change := range changes {
		if change.Key == "max_tokens" {
			found = true
			if change.Old != "321" || change.New != "128" {
				t.Fatalf("unexpected change diff: %+v", change)
			}
		}
	}
	if !found {
		t.Fatalf("expected max_tokens change in diff, got %#v", changes)
	}
}

func TestStoreReloadLogsSummary(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
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

	initial := appconfig.Load(logger)
	store := New(initial)
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 128\n"), 0o644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	_, err := store.Reload(logger, appconfig.LoadOptions{IgnoreEnv: true})
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	logOutput := buf.String()
	if !strings.Contains(logOutput, "Reloaded config (1 changes):") {
		t.Fatalf("expected summary line in log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "- max_tokens: 321 → 128") {
		t.Fatalf("expected change details in log, got %q", logOutput)
	}
}

func TestDiff_SurfaceModel(t *testing.T) {
	oldCfg := appconfig.App{CompletionModel: "gpt-4o", CompletionProvider: "openai"}
	newCfg := appconfig.App{CompletionModel: "gpt-4.1", CompletionProvider: "copilot"}
	changes := Diff(oldCfg, newCfg)
	if len(changes) != 2 {
		t.Fatalf("expected single change, got %+v", changes)
	}
	if changes[0].Key != "completion_model" || changes[0].Old != "gpt-4o" || changes[0].New != "gpt-4.1" {
		t.Fatalf("unexpected diff entry: %+v", changes[0])
	}
	if changes[1].Key != "completion_provider" || changes[1].Old != "openai" || changes[1].New != "copilot" {
		t.Fatalf("unexpected provider diff: %+v", changes[1])
	}
}
