package appconfig

import (
	"log"
	"os"
	"testing"
)

// Test that HEXAI_MODEL applies to provider model fields and that
// provider-specific envs take precedence when both are set.
func TestEnv_GenericModelOverrideAndPrecedence(t *testing.T) {
	t.Setenv("HEXAI_MODEL", "gpt-5-codex")
	t.Setenv("HEXAI_PROVIDER", "openai")
	// No provider-specific env set yet: HEXAI_MODEL should flow into OpenAIModel
	cfg := Load(log.New(os.Stderr, "test ", 0))
	if cfg.OpenAIModel != "gpt-5-codex" {
		t.Fatalf("expected OpenAIModel=gpt-5-codex via HEXAI_MODEL, got %q", cfg.OpenAIModel)
	}

	// Now set a provider-specific model; it should win over HEXAI_MODEL
	t.Setenv("HEXAI_OPENAI_MODEL", "gpt-5-thinking")
	cfg2 := Load(log.New(os.Stderr, "test ", 0))
	if cfg2.OpenAIModel != "gpt-5-thinking" {
		t.Fatalf("expected OpenAIModel from HEXAI_OPENAI_MODEL to win, got %q", cfg2.OpenAIModel)
	}
}

// Test that HEXAI_MODEL_FORCE overrides provider-specific envs (used by CLI --model).
func TestEnv_ModelForce_OverridesProviderSpecific(t *testing.T) {
	t.Setenv("HEXAI_OPENAI_MODEL", "gpt-5-main")
	t.Setenv("HEXAI_MODEL_FORCE", "gpt-5-codex")
	t.Setenv("HEXAI_PROVIDER", "openai")
	cfg := Load(log.New(os.Stderr, "test ", 0))
	if cfg.OpenAIModel != "gpt-5-codex" {
		t.Fatalf("expected OpenAIModel forced to gpt-5-codex, got %q", cfg.OpenAIModel)
	}
}

func TestEnv_SurfaceModelOverrides(t *testing.T) {
	t.Setenv("HEXAI_MODEL_COMPLETION", "gpt-c")
	t.Setenv("HEXAI_TEMPERATURE_COMPLETION", "0.44")
	t.Setenv("HEXAI_PROVIDER_COMPLETION", "copilot")
	t.Setenv("HEXAI_MODEL_CLI", "gpt-cli")
	t.Setenv("HEXAI_TEMPERATURE_CLI", "0.22")
	t.Setenv("HEXAI_PROVIDER_CLI", "ollama")
	cfg := Load(log.New(os.Stderr, "test ", 0))
	if len(cfg.CompletionConfigs) != 1 {
		t.Fatalf("expected single completion entry, got %+v", cfg.CompletionConfigs)
	}
	comp := cfg.CompletionConfigs[0]
	if comp.Model != "gpt-c" {
		t.Fatalf("expected completion model override, got %+v", comp)
	}
	if comp.Temperature == nil || *comp.Temperature != 0.44 {
		t.Fatalf("expected completion temperature override, got %+v", comp)
	}
	if comp.Provider != "copilot" {
		t.Fatalf("expected completion provider override, got %+v", comp)
	}
	if len(cfg.CLIConfigs) != 1 {
		t.Fatalf("expected single CLI entry, got %+v", cfg.CLIConfigs)
	}
	cli := cfg.CLIConfigs[0]
	if cli.Model != "gpt-cli" {
		t.Fatalf("expected cli model override, got %+v", cli)
	}
	if cli.Temperature == nil || *cli.Temperature != 0.22 {
		t.Fatalf("expected cli temperature override, got %+v", cli)
	}
	if cli.Provider != "ollama" {
		t.Fatalf("expected cli provider override, got %+v", cli)
	}
}
