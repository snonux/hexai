package runtimeconfig

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
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
	t.Setenv("HEXAI_PROVIDER", "")

	initial := appconfig.Load(context.Background(), logger)
	if initial.MaxTokens != 321 {
		t.Fatalf("expected env override to win initial load, got %d", initial.MaxTokens)
	}

	store := New(initial)
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 128\n"), 0o644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	changes, err := store.Reload(context.Background(), logger, appconfig.LoadOptions{IgnoreEnv: true})
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
	t.Setenv("HEXAI_PROVIDER", "")

	initial := appconfig.Load(context.Background(), logger)
	store := New(initial)
	if err := os.WriteFile(configPath, []byte("[general]\nmax_tokens = 128\n"), 0o644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	_, err := store.Reload(context.Background(), logger, appconfig.LoadOptions{IgnoreEnv: true})
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

func TestSubscribe_NilListener(t *testing.T) {
	store := New(appconfig.App{})
	unsub := store.Subscribe(nil)
	// Should return a no-op unsubscribe without panicking.
	unsub()
}

func TestSubscribe_ReceivesUpdates(t *testing.T) {
	store := New(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 100}})

	var gotOld, gotNew appconfig.App
	callCount := 0
	unsub := store.Subscribe(func(old, new appconfig.App) {
		gotOld = old
		gotNew = new
		callCount++
	})

	store.Set(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 200}})
	if callCount != 1 {
		t.Fatalf("expected listener called once, got %d", callCount)
	}
	if gotOld.MaxTokens != 100 || gotNew.MaxTokens != 200 {
		t.Fatalf("unexpected old/new: %d / %d", gotOld.MaxTokens, gotNew.MaxTokens)
	}

	// After unsubscribe, listener must not be called again.
	unsub()
	store.Set(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 300}})
	if callCount != 1 {
		t.Fatalf("expected listener not called after unsubscribe, got %d", callCount)
	}
}

func TestSubscribe_MultipleListeners(t *testing.T) {
	store := New(appconfig.App{})
	calls := [2]int{}
	unsub0 := store.Subscribe(func(_, _ appconfig.App) { calls[0]++ })
	unsub1 := store.Subscribe(func(_, _ appconfig.App) { calls[1]++ })

	store.Set(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 1}})
	if calls[0] != 1 || calls[1] != 1 {
		t.Fatalf("expected both listeners called once: %v", calls)
	}

	// Unsubscribe first listener only.
	unsub0()
	store.Set(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 2}})
	if calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("expected only second listener called: %v", calls)
	}
	unsub1()
}

func TestSet_ReturnsChanges(t *testing.T) {
	store := New(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 10, Provider: "ollama"}})
	changes := store.Set(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 20, Provider: "ollama"}})
	found := false
	for _, ch := range changes {
		if ch.Key == "max_tokens" {
			found = true
			if ch.Old != "10" || ch.New != "20" {
				t.Fatalf("unexpected change values: %+v", ch)
			}
		}
	}
	if !found {
		t.Fatalf("expected max_tokens in changes, got %+v", changes)
	}
}

func TestSet_NoChanges(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 10}}
	store := New(cfg)
	changes := store.Set(cfg)
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %+v", changes)
	}
}

func TestReload_NilLogger(t *testing.T) {
	// Reload with nil logger should not panic; it exercises the nil-logger guard
	// in Reload (skipping logger.Print). LoadWithOptions returns defaults when
	// logger is nil, so the store gets default config applied.
	store := New(appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 1}})
	changes, err := store.Reload(context.Background(), nil, appconfig.LoadOptions{IgnoreEnv: true})
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	// Config was updated from our custom value (1) to defaults (4000).
	if snap := store.Snapshot(); snap.MaxTokens != 4000 {
		t.Fatalf("expected default 4000, got %d", snap.MaxTokens)
	}
	// Should report a change for max_tokens at minimum.
	found := false
	for _, ch := range changes {
		if ch.Key == "max_tokens" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected max_tokens change, got %+v", changes)
	}
}

func TestFormatSummary_NoChanges(t *testing.T) {
	result := FormatSummary("Test", nil)
	if result != "Test (no changes detected)." {
		t.Fatalf("unexpected: %q", result)
	}
}

func TestFormatSummary_WithChanges(t *testing.T) {
	changes := []Change{
		{Key: "a", Old: "1", New: "2"},
		{Key: "b", Old: "x", New: "y"},
	}
	result := FormatSummary("Reloaded", changes)
	if !strings.Contains(result, "Reloaded (2 changes):") {
		t.Fatalf("missing header: %q", result)
	}
	if !strings.Contains(result, "- a: 1 → 2") || !strings.Contains(result, "- b: x → y") {
		t.Fatalf("missing details: %q", result)
	}
}

func TestStringifyValue_BoolAndFloat(t *testing.T) {
	// Exercise the bool and float branches via Diff on App fields.
	temp1 := 0.5
	temp2 := 0.9
	oldCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{CodingTemperature: &temp1}}
	newCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{CodingTemperature: &temp2}}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "coding_temperature" {
			found = true
			if ch.Old != "0.5" || ch.New != "0.9" {
				t.Fatalf("unexpected values: %+v", ch)
			}
		}
	}
	if !found {
		t.Fatalf("expected coding_temperature change, got %+v", changes)
	}
}

func TestStringifyValue_NilPointer(t *testing.T) {
	// nil *float64 should produce "(unset)".
	oldCfg := appconfig.App{}
	temp := 0.3
	newCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{CodingTemperature: &temp}}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "coding_temperature" {
			found = true
			if ch.Old != "(unset)" {
				t.Fatalf("expected (unset) for nil ptr, got %q", ch.Old)
			}
		}
	}
	if !found {
		t.Fatalf("expected coding_temperature change")
	}
}

func TestStringifyValue_NilBoolPointer(t *testing.T) {
	// CompletionWaitAll is *bool; nil should produce "(unset)".
	b := true
	oldCfg := appconfig.App{}
	newCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{CompletionWaitAll: &b}}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "completion_wait_all" {
			found = true
			if ch.Old != "(unset)" || ch.New != "true" {
				t.Fatalf("unexpected: old=%q new=%q", ch.Old, ch.New)
			}
		}
	}
	if !found {
		t.Fatalf("expected completion_wait_all change")
	}
}

func TestStringifyValue_StringSlice(t *testing.T) {
	// TriggerCharacters is []string; exercise the string-slice branch.
	oldCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{TriggerCharacters: []string{".", ":"}}}
	newCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{TriggerCharacters: []string{".", ":", "("}}}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "trigger_characters" {
			found = true
			if ch.Old != ".,::" || ch.New != ".,:,(" {
				// Join uses comma separator.
				if ch.Old != ".,:" || ch.New != ".,:,(" {
					t.Fatalf("unexpected: old=%q new=%q", ch.Old, ch.New)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected trigger_characters change, got %+v", changes)
	}
}

func TestStringifyValue_NilSlice(t *testing.T) {
	// nil slice vs non-nil slice.
	oldCfg := appconfig.App{}
	newCfg := appconfig.App{CoreConfig: appconfig.CoreConfig{TriggerCharacters: []string{"x"}}}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "trigger_characters" {
			found = true
			if ch.Old != "" {
				t.Fatalf("expected empty for nil slice, got %q", ch.Old)
			}
		}
	}
	if !found {
		t.Fatalf("expected trigger_characters change")
	}
}

func TestStringifyValue_SurfaceConfigWithTemperature(t *testing.T) {
	// Exercise the SurfaceConfig temperature branch.
	temp := 0.750
	oldCfg := appconfig.App{
		ProviderConfig: appconfig.ProviderConfig{
			CompletionConfigs: []appconfig.SurfaceConfig{
				{Provider: "openai", Model: "gpt-4o", Temperature: &temp},
			},
		},
	}
	newCfg := appconfig.App{
		ProviderConfig: appconfig.ProviderConfig{
			CompletionConfigs: []appconfig.SurfaceConfig{
				{Provider: "openai", Model: "gpt-4o"},
			},
		},
	}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "completion_configs" {
			found = true
			if !strings.Contains(ch.Old, "@0.750") {
				t.Fatalf("expected temperature in old value, got %q", ch.Old)
			}
		}
	}
	if !found {
		t.Fatalf("expected completion_configs change")
	}
}

func TestStringifyValue_SurfaceConfigEmptyProvider(t *testing.T) {
	// Exercise the SurfaceConfig branch where provider is empty.
	oldCfg := appconfig.App{
		ProviderConfig: appconfig.ProviderConfig{
			ChatConfigs: []appconfig.SurfaceConfig{
				{Provider: "", Model: "some-model"},
			},
		},
	}
	newCfg := appconfig.App{}
	changes := Diff(oldCfg, newCfg)
	found := false
	for _, ch := range changes {
		if ch.Key == "chat_configs" {
			found = true
			if ch.Old != "some-model" {
				t.Fatalf("expected 'some-model', got %q", ch.Old)
			}
		}
	}
	if !found {
		t.Fatalf("expected chat_configs change")
	}
}

func TestDiff_SurfaceModel(t *testing.T) {
	oldCfg := appconfig.App{ProviderConfig: appconfig.ProviderConfig{CompletionConfigs: []appconfig.SurfaceConfig{{Provider: "openai", Model: "gpt-4o"}}}}
	newCfg := appconfig.App{ProviderConfig: appconfig.ProviderConfig{CompletionConfigs: []appconfig.SurfaceConfig{{Provider: "anthropic", Model: "claude-3-5-sonnet"}}}}
	changes := Diff(oldCfg, newCfg)
	if len(changes) == 0 {
		t.Fatalf("expected diff entries, got none")
	}
	found := false
	for _, ch := range changes {
		if ch.Key == "completion_configs" {
			if !strings.Contains(ch.Old, "gpt-4o") || !strings.Contains(ch.New, "claude-3-5-sonnet") {
				t.Fatalf("unexpected diff contents: %+v", ch)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected completion configs diff, got %+v", changes)
	}
}
