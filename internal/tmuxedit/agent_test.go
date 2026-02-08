package tmuxedit

import (
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func boolP(b bool) *bool { return &b }

func TestDetectAgent(t *testing.T) {
	agents := builtinAgents()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"claude from banner", "Welcome to Claude Code v1.2\n> ", "claude"},
		{"claude from anthropic", "Powered by Anthropic\n> ", "claude"},
		{"cursor from prompt", "cursor agent ready\n│ type here", "cursor"},
		{"amp from banner", "Amp by Sourcegraph\n> ", "amp"},
		{"aider from banner", "aider v0.50\n> /help", "aider"},
		{"no match", "some random terminal output\n$ ", "generic"},
		{"empty content", "", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectAgent(tt.content, agents)
			if got.Name != tt.want {
				t.Errorf("detectAgent() = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestFindAgentByName(t *testing.T) {
	agents := builtinAgents()
	tests := []struct {
		name string
		want string
	}{
		{"claude", "claude"},
		{"Claude", "claude"},
		{"CURSOR", "cursor"},
		{"amp", "amp"},
		{"nonexistent", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findAgentByName(tt.name, agents)
			if got.Name != tt.want {
				t.Errorf("findAgentByName(%q) = %q, want %q", tt.name, got.Name, tt.want)
			}
		})
	}
}

func TestExtractPrompt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		agent   AgentConfig
		want    string
	}{
		{
			name:    "claude prompt",
			content: "Claude Code v1\n> hello world",
			agent:   builtinAgents()[0], // claude
			want:    "hello world",
		},
		{
			name:    "cursor prompt with strip",
			content: "Cursor Agent\n│ fix the bug INSERT",
			agent:   builtinAgents()[1], // cursor
			want:    "fix the bug",
		},
		{
			name:    "cursor prompt strips follow-up",
			content: "Cursor\n│ Add a follow-up",
			agent:   builtinAgents()[1], // cursor
			want:    "",
		},
		{
			name:    "no pattern",
			content: "some text",
			agent:   genericAgent(),
			want:    "",
		},
		{
			name:    "no match",
			content: "no prompt here",
			agent:   builtinAgents()[0], // claude
			want:    "",
		},
		{
			name:    "invalid regex",
			content: "> test",
			agent:   AgentConfig{PromptPattern: "[invalid"},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPrompt(tt.content, tt.agent)
			if got != tt.want {
				t.Errorf("extractPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripNoise(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		patterns []string
		want     string
	}{
		{"no patterns", "hello world", nil, "hello world"},
		{"strip INSERT", "fix the bug INSERT", []string{"INSERT"}, "fix the bug"},
		{"strip multiple", "INSERT fix the bug Add a follow-up", []string{"INSERT", "Add a follow-up"}, "fix the bug"},
		{"strip to empty", "INSERT", []string{"INSERT"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripNoise(tt.text, tt.patterns)
			if got != tt.want {
				t.Errorf("stripNoise() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAgents_MergeOverride(t *testing.T) {
	cfgAgents := []appconfig.TmuxEditAgentCfg{
		{
			Name:        "claude",
			DisplayName: "My Claude",
			ClearFirst:  boolP(false),
		},
	}
	agents := resolveAgents(cfgAgents)
	var claude AgentConfig
	for _, a := range agents {
		if a.Name == "claude" {
			claude = a
			break
		}
	}
	if claude.DisplayName != "My Claude" {
		t.Errorf("DisplayName = %q, want My Claude", claude.DisplayName)
	}
	if claude.ClearFirst {
		t.Error("ClearFirst should be false after override")
	}
	// DetectPattern should be preserved from builtin
	if claude.DetectPattern == "" {
		t.Error("DetectPattern should be preserved from builtin")
	}
}

func TestResolveAgents_MergeAllFields(t *testing.T) {
	cfgAgents := []appconfig.TmuxEditAgentCfg{
		{
			Name:          "claude",
			DisplayName:   "Custom Claude",
			DetectPattern: "(?i)custom-claude",
			PromptPattern: `>\s+(.*)$`,
			StripPatterns: []string{"NOISE"},
			ClearFirst:    boolP(true),
			ClearKeys:     "C-k",
			NewlineKeys:   "C-Enter",
			SubmitKeys:    "C-m",
		},
	}
	agents := resolveAgents(cfgAgents)
	var a AgentConfig
	for _, ag := range agents {
		if ag.Name == "claude" {
			a = ag
			break
		}
	}
	if a.DetectPattern != "(?i)custom-claude" {
		t.Errorf("DetectPattern = %q", a.DetectPattern)
	}
	if a.PromptPattern != `>\s+(.*)$` {
		t.Errorf("PromptPattern = %q", a.PromptPattern)
	}
	if len(a.StripPatterns) != 1 || a.StripPatterns[0] != "NOISE" {
		t.Errorf("StripPatterns = %v", a.StripPatterns)
	}
	if a.ClearKeys != "C-k" {
		t.Errorf("ClearKeys = %q", a.ClearKeys)
	}
	if a.NewlineKeys != "C-Enter" {
		t.Errorf("NewlineKeys = %q", a.NewlineKeys)
	}
	if a.SubmitKeys != "C-m" {
		t.Errorf("SubmitKeys = %q", a.SubmitKeys)
	}
}

func TestResolveAgents_AddNew(t *testing.T) {
	cfgAgents := []appconfig.TmuxEditAgentCfg{
		{
			Name:          "custom",
			DisplayName:   "Custom Agent",
			DetectPattern: "(?i)custom",
			PromptPattern: `>\s*(.+)$`,
			ClearFirst:    boolP(true),
		},
	}
	agents := resolveAgents(cfgAgents)
	found := false
	for _, a := range agents {
		if a.Name == "custom" {
			found = true
			if a.DisplayName != "Custom Agent" {
				t.Errorf("DisplayName = %q, want Custom Agent", a.DisplayName)
			}
			if !a.ClearFirst {
				t.Error("ClearFirst should be true")
			}
		}
	}
	if !found {
		t.Error("custom agent not found in resolved agents")
	}
}

func TestAgentFromConfig_DefaultDisplayName(t *testing.T) {
	cfg := appconfig.TmuxEditAgentCfg{
		Name: "test",
	}
	a := agentFromConfig(cfg)
	if a.DisplayName != "test" {
		t.Errorf("DisplayName = %q, want test (defaulted from Name)", a.DisplayName)
	}
}

func TestDetectAgent_InvalidRegex(t *testing.T) {
	agents := []AgentConfig{
		{Name: "bad", DetectPattern: "[invalid"},
	}
	got := detectAgent("anything", agents)
	if got.Name != "generic" {
		t.Errorf("expected generic fallback for invalid regex, got %q", got.Name)
	}
}

func TestGenericAgent(t *testing.T) {
	g := genericAgent()
	if g.Name != "generic" {
		t.Errorf("Name = %q, want generic", g.Name)
	}
	if g.SubmitKeys != "Enter" {
		t.Errorf("SubmitKeys = %q, want Enter", g.SubmitKeys)
	}
}
