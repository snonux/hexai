package hexaicli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

func TestCLIResponseCacheFingerprintChanges(t *testing.T) {
	base := newCLIResponseCacheKey("openai", "gpt-4.1", requestArgs{maxTokens: 42}, []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
	})
	baseFingerprint, ok := cliResponseCacheFingerprint(base)
	if !ok {
		t.Fatal("expected fingerprint for base key")
	}

	tests := []struct {
		name string
		key  cliResponseCacheKey
	}{
		{name: "provider", key: newCLIResponseCacheKey("anthropic", "gpt-4.1", requestArgs{maxTokens: 42}, base.Messages)},
		{name: "model", key: newCLIResponseCacheKey("openai", "gpt-5", requestArgs{maxTokens: 42}, base.Messages)},
		{name: "prompt", key: newCLIResponseCacheKey("openai", "gpt-4.1", requestArgs{maxTokens: 42}, []llm.Message{{Role: "system", Content: "different"}, {Role: "user", Content: "hello"}})},
		{name: "temperature", key: newCLIResponseCacheKey("openai", "gpt-4.1", requestArgs{maxTokens: 42, temperature: floatPtr(0.7)}, base.Messages)},
	}

	for _, tc := range tests {
		fingerprint, ok := cliResponseCacheFingerprint(tc.key)
		if !ok {
			t.Fatalf("%s: expected fingerprint", tc.name)
		}
		if fingerprint == baseFingerprint {
			t.Fatalf("%s: expected fingerprint change", tc.name)
		}
	}
}

func TestLookupCLIResponseCacheExpiresEntries(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	oldNow := nowCLIResponseCache
	nowCLIResponseCache = func() time.Time { return time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC) }
	defer func() { nowCLIResponseCache = oldNow }()

	key := newCLIResponseCacheKey("openai", "gpt-4.1", requestArgs{maxTokens: 10}, []llm.Message{{Role: "user", Content: "hello"}})
	storeCLIResponseCache(key, "cached")

	path, ok := cliResponseCachePath(key)
	if !ok {
		t.Fatal("expected cache path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected cache file: %v", err)
	}

	nowCLIResponseCache = func() time.Time { return time.Date(2026, 3, 16, 11, 0, 0, 0, time.UTC) }
	if _, _, hit := lookupCLIResponseCache(key); hit {
		t.Fatal("expected expired cache miss")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected expired cache file removal, got %v", err)
	}
}

func TestRun_UsesCachedResponseWithoutClientCall(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	// This test asserts an "openai/gpt-4.1" cache-hit label, so pin the provider
	// (the in-code default switched to ollama when no config is present).
	t.Setenv("HEXAI_PROVIDER", "openai")

	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()

	calls := 0
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		calls++
		return &fakeClient{name: cfg.Provider, model: "gpt-4.1", resp: "cached output"}, nil
	}

	var firstOut, firstErr bytes.Buffer
	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &firstOut, &firstErr); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one live client call, got %d", calls)
	}

	newClientFromApp = func(appconfig.App) (llm.Client, error) {
		t.Fatal("client should not be constructed on cache hit")
		return nil, nil
	}

	var secondOut, secondErr bytes.Buffer
	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &secondOut, &secondErr); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if got := secondOut.String(); got != "cached output" {
		t.Fatalf("stdout = %q, want cached output", got)
	}
	if !strings.Contains(secondErr.String(), "cache hit provider=openai model=gpt-4.1") {
		t.Fatalf("expected cache hit note, got %q", secondErr.String())
	}
}

func TestRun_WithSelectionUsesChosenCachedResponse(t *testing.T) {
	workDir := t.TempDir()
	configHome := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	configPath := filepath.Join(configHome, "hexai", "config.toml")
	writeConfigString(t, configPath, `
[provider]
name = "openai"

[[models.cli]]
provider = "openai"
model = "gpt-4.1"

[[models.cli]]
provider = "anthropic"
model = "claude-3-5-sonnet-20240620"
`)

	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		switch cfg.Provider {
		case "anthropic":
			return &fakeClient{name: "anthropic", model: "claude-3-5-sonnet-20240620", resp: "RIGHT"}, nil
		default:
			return &fakeClient{name: "openai", model: "gpt-4.1", resp: "LEFT"}, nil
		}
	}

	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("warm cache Run: %v", err)
	}

	newClientFromApp = func(appconfig.App) (llm.Client, error) {
		t.Fatal("client should not be constructed for selected cache hit")
		return nil, nil
	}

	ctx := WithCLISelection(context.Background(), []int{1})
	var out, errb bytes.Buffer
	if err := Run(ctx, []string{"hello"}, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("selected Run: %v", err)
	}
	if got := out.String(); got != "RIGHT" {
		t.Fatalf("stdout = %q, want RIGHT", got)
	}
	if strings.Contains(out.String(), "LEFT") {
		t.Fatalf("unexpected other provider output: %q", out.String())
	}
	if !strings.Contains(errb.String(), "anthropic:claude-3-5-sonnet-20240620") {
		t.Fatalf("expected selected provider header, got %q", errb.String())
	}
}

func TestRun_ExpiredCacheFallsBackToProvider(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	oldNow := nowCLIResponseCache
	nowCLIResponseCache = func() time.Time { return time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC) }
	defer func() { nowCLIResponseCache = oldNow }()

	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()

	calls := 0
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		calls++
		resp := "first"
		if calls > 1 {
			resp = "second"
		}
		return &fakeClient{name: cfg.Provider, model: "gpt-4.1", resp: resp}, nil
	}

	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	nowCLIResponseCache = func() time.Time { return time.Date(2026, 3, 16, 11, 0, 0, 0, time.UTC) }
	var out, errb bytes.Buffer
	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected second live provider call after expiry, got %d", calls)
	}
	if got := out.String(); got != "second" {
		t.Fatalf("stdout = %q, want second", got)
	}
}
