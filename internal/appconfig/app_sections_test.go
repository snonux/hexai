package appconfig

import (
	"reflect"
	"testing"
)

func TestSectionsRoundTrip(t *testing.T) {
	expected := App{}
	expected.ApplySections(testAppSections())

	got := App{}
	got.ApplySections(expected.Sections())

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", got, expected)
	}
}

func TestSectionsDefensiveCopies(t *testing.T) {
	sections := testAppSections()
	cfg := App{}
	cfg.ApplySections(sections)

	sections.Core.TriggerCharacters[0] = "mutated"
	sections.Core.ChatPrefixes[0] = "mutated"
	sections.Providers.CompletionConfigs[0].Provider = "mutated"
	sections.Providers.CLIConfigs[0].Model = "mutated"
	sections.Prompts.CustomActions[0].Title = "mutated"
	sections.Features.IgnoreExtraPatterns[0] = "mutated"
	sections.Features.TmuxEditAgents[0].Name = "mutated"

	assertNotEqual(t, cfg.TriggerCharacters[0], "mutated", "trigger characters")
	assertNotEqual(t, cfg.ChatPrefixes[0], "mutated", "chat prefixes")
	assertNotEqual(t, cfg.CompletionConfigs[0].Provider, "mutated", "completion configs")
	assertNotEqual(t, cfg.CLIConfigs[0].Model, "mutated", "cli configs")
	assertNotEqual(t, cfg.CustomActions[0].Title, "mutated", "custom actions")
	assertNotEqual(t, cfg.IgnoreExtraPatterns[0], "mutated", "ignore patterns")
	assertNotEqual(t, cfg.TmuxEditAgents[0].Name, "mutated", "tmux agents")

	out := cfg.Sections()
	out.Core.TriggerCharacters[0] = "mutated"
	out.Core.ChatPrefixes[0] = "mutated"
	out.Providers.CompletionConfigs[0].Provider = "mutated"
	out.Providers.CLIConfigs[0].Model = "mutated"
	out.Prompts.CustomActions[0].Title = "mutated"
	out.Features.IgnoreExtraPatterns[0] = "mutated"
	out.Features.TmuxEditAgents[0].Name = "mutated"

	assertNotEqual(t, cfg.TriggerCharacters[0], "mutated", "sections trigger characters")
	assertNotEqual(t, cfg.ChatPrefixes[0], "mutated", "sections chat prefixes")
	assertNotEqual(t, cfg.CompletionConfigs[0].Provider, "mutated", "sections completion configs")
	assertNotEqual(t, cfg.CLIConfigs[0].Model, "mutated", "sections cli configs")
	assertNotEqual(t, cfg.CustomActions[0].Title, "mutated", "sections custom actions")
	assertNotEqual(t, cfg.IgnoreExtraPatterns[0], "mutated", "sections ignore patterns")
	assertNotEqual(t, cfg.TmuxEditAgents[0].Name, "mutated", "sections tmux agents")
}

func assertNotEqual(t *testing.T, got, want, field string) {
	t.Helper()
	if got == want {
		t.Fatalf("%s unexpectedly changed to %q", field, got)
	}
}

func testAppSections() AppSections {
	return AppSections{
		Core:      testCoreConfig(),
		Providers: testProviderConfig(),
		Prompts:   testPromptConfig(),
		Features:  testFeatureConfig(),
	}
}

func testCoreConfig() CoreConfig {
	return CoreConfig{
		MaxTokens:             111,
		ContextMode:           "window",
		ContextWindowLines:    222,
		MaxContextTokens:      333,
		LogPreviewLimit:       444,
		RequestTimeout:        55,
		CodingTemperature:     sectionFloatPtr(0.4),
		ManualInvokeMinPrefix: 3,
		CompletionDebounceMs:  700,
		CompletionThrottleMs:  125,
		CompletionWaitAll:     sectionBoolPtr(false),
		TriggerCharacters:     []string{".", "("},
		Provider:              "openrouter",
		InlineOpen:            "<!",
		InlineClose:           "!>",
		ChatSuffix:            "?",
		ChatPrefixes:          []string{"@", "/"},
	}
}

func testProviderConfig() ProviderConfig {
	return ProviderConfig{
		OpenAIBaseURL:         "https://api.openai.test",
		OpenAIModel:           "gpt-test",
		OpenAITemperature:     sectionFloatPtr(0.6),
		OpenRouterBaseURL:     "https://openrouter.test",
		OpenRouterModel:       "or-test",
		OpenRouterTemperature: sectionFloatPtr(0.7),
		OllamaBaseURL:         "http://localhost:11434",
		OllamaModel:           "llama3.1",
		OllamaTemperature:     sectionFloatPtr(0.8),
		AnthropicBaseURL:      "https://api.anthropic.test",
		AnthropicModel:        "claude-test",
		AnthropicTemperature:  sectionFloatPtr(0.9),
		CompletionConfigs: []SurfaceConfig{{
			Provider:    "openai",
			Model:       "gpt-test",
			Temperature: sectionFloatPtr(0.2),
		}},
		CodeActionConfigs: []SurfaceConfig{{
			Provider:    "anthropic",
			Model:       "claude-test",
			Temperature: sectionFloatPtr(0.3),
		}},
		ChatConfigs: []SurfaceConfig{{
			Provider:    "openrouter",
			Model:       "or-test",
			Temperature: sectionFloatPtr(0.4),
		}},
		CLIConfigs: []SurfaceConfig{{
			Provider:    "ollama",
			Model:       "llama3.1",
			Temperature: sectionFloatPtr(0.5),
		}},
	}
}

func testPromptConfig() PromptConfig {
	return PromptConfig{
		PromptCompletionSystemGeneral:     "comp sys",
		PromptCompletionSystemParams:      "comp params",
		PromptCompletionSystemInline:      "comp inline",
		PromptCompletionUserGeneral:       "comp user",
		PromptCompletionUserParams:        "comp user params",
		PromptCompletionExtraHeader:       "comp header",
		PromptNativeCompletion:            "native",
		PromptChatSystem:                  "chat",
		PromptCodeActionRewriteSystem:     "rewrite sys",
		PromptCodeActionDiagnosticsSystem: "diag sys",
		PromptCodeActionDocumentSystem:    "doc sys",
		PromptCodeActionRewriteUser:       "rewrite user",
		PromptCodeActionDiagnosticsUser:   "diag user",
		PromptCodeActionDocumentUser:      "doc user",
		PromptCodeActionGoTestSystem:      "gotest sys",
		PromptCodeActionGoTestUser:        "gotest user",
		PromptCodeActionSimplifySystem:    "simplify sys",
		PromptCodeActionSimplifyUser:      "simplify user",
		PromptCLIDefaultSystem:            "cli default",
		PromptCLIExplainSystem:            "cli explain",
		CustomActions: []CustomAction{{
			ID:          "a1",
			Title:       "Action 1",
			Kind:        "refactor",
			Scope:       "selection",
			Hotkey:      "r",
			Instruction: "do it",
			System:      "sys",
			User:        "user",
		}},
		TmuxCustomMenuHotkey: "M-a",
	}
}

func testFeatureConfig() FeatureConfig {
	return FeatureConfig{
		StatsConfig: StatsConfig{StatsWindowMinutes: 15},
		IgnoreConfig: IgnoreConfig{
			IgnoreGitignore:     sectionBoolPtr(true),
			IgnoreExtraPatterns: []string{"vendor/**", "tmp/**"},
			IgnoreLSPNotify:     sectionBoolPtr(false),
		},
		TmuxEditConfig: TmuxEditConfig{
			TmuxEditPopupWidth:   "80%",
			TmuxEditPopupHeight:  "75%",
			TmuxEditDefaultAgent: "codex",
			TmuxEditAgents: []TmuxEditAgentCfg{{
				Name:           "codex",
				DisplayName:    "Codex",
				DetectPattern:  "(?i)codex",
				SectionPattern: "section",
				PromptPattern:  "prompt",
				StripPatterns:  []string{"x", "y"},
				ClearFirst:     sectionBoolPtr(true),
				ClearKeys:      "C-u",
				NewlineKeys:    "S-Enter",
				SubmitKeys:     "Enter",
			}},
		},
		MCPConfig: MCPConfig{
			MCPPromptsDir:       ".hexai/prompts",
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  ".hexai/slash",
		},
	}
}

func sectionFloatPtr(v float64) *float64 { return &v }

func sectionBoolPtr(v bool) *bool { return &v }
