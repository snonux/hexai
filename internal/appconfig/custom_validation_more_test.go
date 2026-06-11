package appconfig

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomActions_MissingFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	writeFile(t, cfgPath, `
[prompts.code_action]
[[prompts.code_action.custom]]
title = "No ID"
instruction = "x"
[[prompts.code_action.custom]]
id = "no-title"
instruction = "x"
`)
	cfg := Load(context.Background(), newLogger())
	if err := cfg.Validate(); err == nil || (!strings.Contains(err.Error(), "missing required field id") && !strings.Contains(err.Error(), "missing required field title")) {
		t.Fatalf("expected missing field error, got %v", err)
	}
}

func TestCustomActions_InvalidHotkeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "hexai", "config.toml")
	writeFile(t, cfgPath, `
[prompts.code_action]
[[prompts.code_action.custom]]
id = "a"
title = "A"
instruction = "x"
hotkey = "too"

[tmux]
custom_menu_hotkey = "ab"
`)
	cfg := Load(context.Background(), newLogger())
	if err := cfg.Validate(); err == nil || (!strings.Contains(err.Error(), "hotkey must be a single character") && !strings.Contains(err.Error(), "invalid tmux.custom_menu_hotkey")) {
		t.Fatalf("expected invalid hotkey error, got %v", err)
	}
}
