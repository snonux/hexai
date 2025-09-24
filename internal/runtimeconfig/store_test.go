package runtimeconfig

import (
	"io"
	"log"
	"os"
	"path/filepath"
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
