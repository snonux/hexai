package llmutils

import (
	"os"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

// TestMain registers all built-in LLM providers before tests run.
func TestMain(m *testing.M) {
	llm.RegisterAllProviders()
	os.Exit(m.Run())
}

func TestNewClientFromApp_Ollama(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "ollama"}}
	c, err := NewClientFromApp(cfg)
	if err != nil || c == nil {
		t.Fatalf("ollama client failed: %v %v", c, err)
	}
}

func TestNewClientFromApp_OpenAI_WithKey(t *testing.T) {
	t.Setenv("HEXAI_OPENAI_API_KEY", "test-key")
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}}
	c, err := NewClientFromApp(cfg)
	if err != nil || c == nil {
		t.Fatalf("openai client failed: %v %v", c, err)
	}
	// ensure env override precedence
	_ = os.Unsetenv("OPENAI_API_KEY")
}

func TestCanonicalProvider(t *testing.T) {
	if got := CanonicalProvider("  OpenRouter "); got != "openrouter" {
		t.Fatalf("CanonicalProvider(openrouter) = %q", got)
	}
	if got := CanonicalProvider(" "); got != "ollama" {
		t.Fatalf("CanonicalProvider(empty) = %q", got)
	}
}

func TestDefaultModelForProvider(t *testing.T) {
	cfg := appconfig.App{
		ProviderConfig: appconfig.ProviderConfig{
			OpenAIModel:     "gpt-4.1",
			OpenRouterModel: "openrouter/auto",
			OllamaModel:     "qwen3",
			AnthropicModel:  "claude",
		},
	}
	if got := DefaultModelForProvider(cfg, "openai"); got != "gpt-4.1" {
		t.Fatalf("openai model = %q", got)
	}
	if got := DefaultModelForProvider(cfg, "openrouter"); got != "openrouter/auto" {
		t.Fatalf("openrouter model = %q", got)
	}
	if got := DefaultModelForProvider(cfg, "ollama"); got != "qwen3" {
		t.Fatalf("ollama model = %q", got)
	}
	if got := DefaultModelForProvider(cfg, "anthropic"); got != "claude" {
		t.Fatalf("anthropic model = %q", got)
	}
}

func TestDefaultModelForProvider_Fallbacks(t *testing.T) {
	cfg := appconfig.App{}
	if got := DefaultModelForProvider(cfg, "openai"); got != "gpt-4.1" {
		t.Fatalf("openai fallback = %q", got)
	}
	if got := DefaultModelForProvider(cfg, "openrouter"); got != "openrouter/auto" {
		t.Fatalf("openrouter fallback = %q", got)
	}
	if got := DefaultModelForProvider(cfg, "ollama"); got != "gemma4" {
		t.Fatalf("ollama fallback = %q", got)
	}
	if got := DefaultModelForProvider(cfg, "anthropic"); got != "claude-3-5-sonnet-20240620" {
		t.Fatalf("anthropic fallback = %q", got)
	}
}

func TestConfigForProvider(t *testing.T) {
	base := appconfig.App{
		CoreConfig: appconfig.CoreConfig{Provider: "openai"},
		ProviderConfig: appconfig.ProviderConfig{
			OpenAIModel:    "gpt-4.1",
			OllamaModel:    "qwen3",
			AnthropicModel: "claude",
		},
	}
	got := ConfigForProvider(base, "ollama", "qwen3-coder")
	if got.Provider != "ollama" {
		t.Fatalf("provider = %q", got.Provider)
	}
	if got.OllamaModel != "qwen3-coder" {
		t.Fatalf("ollama model = %q", got.OllamaModel)
	}
	if got.OpenAIModel != "gpt-4.1" {
		t.Fatalf("openai model unexpectedly changed: %q", got.OpenAIModel)
	}
}
