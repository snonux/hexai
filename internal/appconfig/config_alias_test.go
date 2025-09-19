package appconfig

import (
    "log"
    "os"
    "path/filepath"
    "testing"
)

func TestOpenAIPresets_AliasResolution(t *testing.T) {
    dir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", dir)
    cfgDir := filepath.Join(dir, "hexai")
    if err := os.MkdirAll(cfgDir, 0o755); err != nil {
        t.Fatalf("mkdir: %v", err)
    }
    toml := `
[provider]
name = "openai"

[openai]
model = "codex"

[openai.presets]
codex = "gpt-5-codex"
`
    path := filepath.Join(cfgDir, "config.toml")
    if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
        t.Fatalf("write: %v", err)
    }
    cfg := Load(log.New(os.Stderr, "test ", 0))
    if cfg.OpenAIModel != "gpt-5-codex" {
        t.Fatalf("expected alias to resolve to gpt-5-codex, got %q", cfg.OpenAIModel)
    }
}

