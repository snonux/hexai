package llmutils

import (
	"os"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
)

// TestMain registers all built-in LLM providers before tests run.
func TestMain(m *testing.M) {
	if err := llm.RegisterAllProviders(); err != nil {
		panic(err)
	}
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
	if got := DefaultModelForProvider(cfg, "ollama"); got != appconfig.DefaultOllamaModel {
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

func TestConfigForProvider_ProfileOverridesEndpointAndModel(t *testing.T) {
	base := appconfig.App{}
	base.ProviderProfiles = map[string]appconfig.ProviderProfile{
		"local-qwen": {Type: "ollama", BaseURL: "http://127.0.0.1:11434", Model: "qwen3.8:27b"},
	}
	got := ConfigForProvider(base, "local-qwen", "")
	if got.Provider != "ollama" || got.OllamaBaseURL != "http://127.0.0.1:11434" || got.OllamaModel != "qwen3.8:27b" {
		t.Fatalf("unexpected profile config: %+v", got.ProviderConfig)
	}
	got = ConfigForProvider(base, "local-qwen", "override")
	if got.OllamaModel != "override" {
		t.Fatalf("model override = %q, want override", got.OllamaModel)
	}
}

// TestConfigForProvider_ProfileAppliesToEachProviderType checks that a profile's
// base URL, model and temperature land in the section of the provider type the
// profile names, and in no other section.
func TestConfigForProvider_ProfileAppliesToEachProviderType(t *testing.T) {
	temp := 0.3
	cases := []struct {
		typ  string
		pick func(appconfig.App) (string, string, *float64)
	}{
		{"ollama", func(a appconfig.App) (string, string, *float64) {
			return a.OllamaBaseURL, a.OllamaModel, a.OllamaTemperature
		}},
		{"openrouter", func(a appconfig.App) (string, string, *float64) {
			return a.OpenRouterBaseURL, a.OpenRouterModel, a.OpenRouterTemperature
		}},
		{"anthropic", func(a appconfig.App) (string, string, *float64) {
			return a.AnthropicBaseURL, a.AnthropicModel, a.AnthropicTemperature
		}},
		{"openai", func(a appconfig.App) (string, string, *float64) {
			return a.OpenAIBaseURL, a.OpenAIModel, a.OpenAITemperature
		}},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			base := appconfig.App{}
			base.ProviderProfiles = map[string]appconfig.ProviderProfile{
				"mine": {Type: tc.typ, BaseURL: "http://example.invalid/" + tc.typ, Model: "m-" + tc.typ, Temperature: &temp},
			}
			got := ConfigForProvider(base, "mine", "")
			if got.Provider != tc.typ {
				t.Fatalf("provider = %q, want %q", got.Provider, tc.typ)
			}
			url, model, gotTemp := tc.pick(got)
			if url != "http://example.invalid/"+tc.typ || model != "m-"+tc.typ {
				t.Fatalf("url/model = %q/%q", url, model)
			}
			if gotTemp == nil || *gotTemp != temp {
				t.Fatalf("temperature = %v, want %v", gotTemp, temp)
			}
			// Other provider sections must stay untouched.
			for _, other := range cases {
				if other.typ == tc.typ {
					continue
				}
				if u, m, tp := other.pick(got); u != "" || m != "" || tp != nil {
					t.Fatalf("%s section leaked into %s: %q %q %v", tc.typ, other.typ, u, m, tp)
				}
			}
		})
	}
}

func TestConfigForProvider_ModelOverrideForOpenRouterAndAnthropic(t *testing.T) {
	got := ConfigForProvider(appconfig.App{}, "openrouter", "or-model")
	if got.Provider != "openrouter" || got.OpenRouterModel != "or-model" {
		t.Fatalf("openrouter override: %q %q", got.Provider, got.OpenRouterModel)
	}
	got = ConfigForProvider(appconfig.App{}, "anthropic", "an-model")
	if got.Provider != "anthropic" || got.AnthropicModel != "an-model" {
		t.Fatalf("anthropic override: %q %q", got.Provider, got.AnthropicModel)
	}
}

func TestConfigForProvider_EmptyProviderUsesConfigProvider(t *testing.T) {
	base := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "Anthropic"}}
	got := ConfigForProvider(base, "  ", "")
	if got.Provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", got.Provider)
	}
}

func TestProviderProfileFor_UnknownProvider(t *testing.T) {
	if p, ok := ProviderProfileFor(appconfig.App{}, "nonesuch"); ok {
		t.Fatalf("unknown provider resolved to %+v", p)
	}
}

func TestProviderProfileFor_BuiltinsReadLegacySections(t *testing.T) {
	cfg := appconfig.App{ProviderConfig: appconfig.ProviderConfig{
		OpenRouterBaseURL: "http://or", OpenRouterModel: "orm",
		AnthropicBaseURL: "http://an", AnthropicModel: "anm",
	}}
	if p, ok := ProviderProfileFor(cfg, "openrouter"); !ok || p.BaseURL != "http://or" || p.Model != "orm" || p.Type != "openrouter" {
		t.Fatalf("openrouter profile = %+v ok=%v", p, ok)
	}
	if p, ok := ProviderProfileFor(cfg, "anthropic"); !ok || p.BaseURL != "http://an" || p.Model != "anm" {
		t.Fatalf("anthropic profile = %+v ok=%v", p, ok)
	}
}

func TestDefaultModelForProvider_ProfileModelWins(t *testing.T) {
	cfg := appconfig.App{ProviderConfig: appconfig.ProviderConfig{OpenAIModel: "legacy"}}
	cfg.ProviderProfiles = map[string]appconfig.ProviderProfile{
		"work": {Type: "openai", Model: " profile-model "},
	}
	if got := DefaultModelForProvider(cfg, "work"); got != "profile-model" {
		t.Fatalf("model = %q, want profile-model", got)
	}
}

func TestNewClientFromAppForProvider_UsesProviderAndModel(t *testing.T) {
	t.Setenv("HEXAI_OPENAI_API_KEY", "test-key")
	base := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "ollama"}}
	c, err := NewClientFromAppForProvider(base, "openai", "gpt-test")
	if err != nil || c == nil {
		t.Fatalf("client: %v %v", c, err)
	}
	if c.Name() != "openai" || c.DefaultModel() != "gpt-test" {
		t.Fatalf("client = %s/%s, want openai/gpt-test", c.Name(), c.DefaultModel())
	}
}

func TestNewClientFromApp_UnknownProviderErrors(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "nonesuch"}}
	if c, err := NewClientFromApp(cfg); err == nil {
		t.Fatalf("expected error for unknown provider, got client %v", c)
	}
}
