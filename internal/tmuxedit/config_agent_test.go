package tmuxedit

import (
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func boolP(b bool) *bool { return &b }

func TestResolveAgents_MergeOverride(t *testing.T) {
	// Override the built-in "amp" agent to verify config merging preserves
	// builtin fields (detectPattern) while applying user overrides (DisplayName, ClearFirst).
	cfgAgents := []appconfig.TmuxEditAgentCfg{
		{
			Name:        "amp",
			DisplayName: "My Amp",
			ClearFirst:  boolP(false),
		},
	}
	agents := resolveAgents(cfgAgents)
	var amp Agent
	for _, a := range agents {
		if a.Name() == "amp" {
			amp = a
			break
		}
	}
	if amp == nil {
		t.Fatal("amp agent not found")
	}
	if amp.DisplayName() != "My Amp" {
		t.Errorf("DisplayName = %q, want My Amp", amp.DisplayName())
	}
	// ClearInput should be no-op after override to false
	c := amp.(Configurable)
	if c.Base().clearFirst {
		t.Error("clearFirst should be false after override")
	}
	// DetectPattern should be preserved from builtin
	if c.Base().detectPattern == "" {
		t.Error("detectPattern should be preserved from builtin")
	}
}

func TestResolveAgents_MergeAllFields(t *testing.T) {
	// Override the built-in "aider" agent with all fields to verify full merging.
	cfgAgents := []appconfig.TmuxEditAgentCfg{
		{
			Name:          "aider",
			DisplayName:   "Custom Aider",
			DetectPattern: "(?i)custom-aider",
			PromptPattern: `>\s+(.*)$`,
			StripPatterns: []string{"NOISE"},
			ClearFirst:    boolP(true),
			ClearKeys:     "C-k",
			NewlineKeys:   "C-Enter",
			SubmitKeys:    "C-m",
		},
	}
	agents := resolveAgents(cfgAgents)
	var a Agent
	for _, ag := range agents {
		if ag.Name() == "aider" {
			a = ag
			break
		}
	}
	if a == nil {
		t.Fatal("aider agent not found")
	}
	c := a.(Configurable)
	base := c.Base()
	if base.detectPattern != "(?i)custom-aider" {
		t.Errorf("detectPattern = %q", base.detectPattern)
	}
	if base.promptPat != `>\s+(.*)$` {
		t.Errorf("promptPat = %q", base.promptPat)
	}
	if len(base.stripPatterns) != 1 || base.stripPatterns[0] != "NOISE" {
		t.Errorf("stripPatterns = %v", base.stripPatterns)
	}
	if base.clearKeys != "C-k" {
		t.Errorf("clearKeys = %q", base.clearKeys)
	}
	if base.newlineKeys != "C-Enter" {
		t.Errorf("newlineKeys = %q", base.newlineKeys)
	}
	if base.submitKeys != "C-m" {
		t.Errorf("submitKeys = %q", base.submitKeys)
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
		if a.Name() == "custom" {
			found = true
			if a.DisplayName() != "Custom Agent" {
				t.Errorf("DisplayName = %q, want Custom Agent", a.DisplayName())
			}
			c := a.(Configurable)
			if !c.Base().clearFirst {
				t.Error("clearFirst should be true")
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
	if a.DisplayName() != "test" {
		t.Errorf("DisplayName = %q, want test (defaulted from Name)", a.DisplayName())
	}
}

func TestConfigAgent_ExtractPrompt(t *testing.T) {
	// Config agent uses baseAgent's default extraction (section-aware)
	agent := &configAgent{baseAgent{
		promptPat: `(?m)>\s*(.+)$`,
	}}
	content := "> hello world"
	got := agent.ExtractPrompt(content)
	if got != "hello world" {
		t.Errorf("ExtractPrompt() = %q, want %q", got, "hello world")
	}
}

func TestConfigAgent_Amp(t *testing.T) {
	agents := builtinAgents()
	var amp Agent
	for _, a := range agents {
		if a.Name() == "amp" {
			amp = a
			break
		}
	}
	if amp == nil {
		t.Fatal("amp agent not found")
	}
	if !amp.Detect("Amp by Sourcegraph") {
		t.Error("amp should detect 'Amp by Sourcegraph'")
	}
	// Amp uses box-drawing TUI format (like cursor), not shell-style > prompt
	got := amp.ExtractPrompt("│ fix the bug                                                       │")
	if got != "fix the bug" {
		t.Errorf("ExtractPrompt() = %q, want %q", got, "fix the bug")
	}
}

func TestConfigAgent_Aider(t *testing.T) {
	agents := builtinAgents()
	var aider Agent
	for _, a := range agents {
		if a.Name() == "aider" {
			aider = a
			break
		}
	}
	if aider == nil {
		t.Fatal("aider agent not found")
	}
	if !aider.Detect("aider v0.50") {
		t.Error("aider should detect 'aider v0.50'")
	}
}
