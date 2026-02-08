// Package tmuxedit implements a tmux popup editor for composing AI agent prompts.
// agent.go defines agent detection, prompt extraction, and noise stripping.
package tmuxedit

import (
	"regexp"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

// AgentConfig describes how to detect and interact with a specific AI agent
// running in a tmux pane. All behavior is driven by regex patterns so new
// agents can be added via config without code changes.
type AgentConfig struct {
	Name          string   // short key: "claude", "cursor", "amp"
	DisplayName   string   // human-readable: "Claude Code"
	DetectPattern string   // regex matched against pane content for auto-detection
	PromptPattern string   // regex with capture group (1) to extract current prompt text
	StripPatterns []string // substrings removed from extracted text
	ClearFirst    bool     // whether to clear existing input before sending
	ClearKeys     string   // tmux key sequence to clear input (e.g. "C-u")
	NewlineKeys   string   // tmux key to insert a newline (e.g. "S-Enter")
	SubmitKeys    string   // tmux key to submit the prompt (e.g. "Enter")
}

// builtinAgents returns the default set of agent configurations. These are
// overridden/extended by user config in [tmux_edit.agents].
func builtinAgents() []AgentConfig {
	return []AgentConfig{
		{
			Name:          "claude",
			DisplayName:   "Claude Code",
			DetectPattern: `(?i)(claude|anthropic)`,
			PromptPattern: `(?m)>\s*(.+)$`,
			ClearFirst:    true,
			ClearKeys:     "C-u",
			NewlineKeys:   "S-Enter",
			SubmitKeys:    "Enter",
		},
		{
			Name:          "cursor",
			DisplayName:   "Cursor",
			DetectPattern: `(?i)cursor`,
			PromptPattern: `(?m)│\s*(.+)$`,
			StripPatterns: []string{"INSERT", "Add a follow-up"},
			ClearFirst:    true,
			ClearKeys:     "C-u",
			NewlineKeys:   "S-Enter",
			SubmitKeys:    "Enter",
		},
		{
			Name:          "amp",
			DisplayName:   "Amp",
			DetectPattern: `(?i)(amp|sourcegraph)`,
			PromptPattern: `(?m)>\s*(.+)$`,
			ClearFirst:    true,
			ClearKeys:     "C-u",
			NewlineKeys:   "S-Enter",
			SubmitKeys:    "Enter",
		},
		{
			Name:          "aider",
			DisplayName:   "Aider",
			DetectPattern: `(?i)aider`,
			PromptPattern: `(?m)>\s*(.+)$`,
			ClearFirst:    true,
			ClearKeys:     "C-u",
			NewlineKeys:   "",
			SubmitKeys:    "Enter",
		},
	}
}

// genericAgent returns a fallback agent with no detection or prompt extraction.
// The user gets a blank editor and text is sent verbatim.
func genericAgent() AgentConfig {
	return AgentConfig{
		Name:        "generic",
		DisplayName: "Generic",
		NewlineKeys: "",
		SubmitKeys:  "Enter",
	}
}

// resolveAgents merges built-in agent defaults with user-provided overrides
// from config. Agents are matched by name (case-insensitive); user config
// wins field-by-field over builtins.
func resolveAgents(cfgAgents []appconfig.TmuxEditAgentCfg) []AgentConfig {
	agents := builtinAgents()
	for _, ca := range cfgAgents {
		merged := false
		for i, a := range agents {
			if !strings.EqualFold(a.Name, ca.Name) {
				continue
			}
			agents[i] = mergeAgentConfig(a, ca)
			merged = true
			break
		}
		if !merged {
			agents = append(agents, agentFromConfig(ca))
		}
	}
	return agents
}

// mergeAgentConfig overrides fields in base with non-zero values from cfg.
func mergeAgentConfig(base AgentConfig, cfg appconfig.TmuxEditAgentCfg) AgentConfig {
	if s := strings.TrimSpace(cfg.DisplayName); s != "" {
		base.DisplayName = s
	}
	if s := strings.TrimSpace(cfg.DetectPattern); s != "" {
		base.DetectPattern = s
	}
	if s := strings.TrimSpace(cfg.PromptPattern); s != "" {
		base.PromptPattern = s
	}
	if len(cfg.StripPatterns) > 0 {
		base.StripPatterns = cfg.StripPatterns
	}
	if cfg.ClearFirst != nil {
		base.ClearFirst = *cfg.ClearFirst
	}
	if s := strings.TrimSpace(cfg.ClearKeys); s != "" {
		base.ClearKeys = s
	}
	if s := strings.TrimSpace(cfg.NewlineKeys); s != "" {
		base.NewlineKeys = s
	}
	if s := strings.TrimSpace(cfg.SubmitKeys); s != "" {
		base.SubmitKeys = s
	}
	return base
}

// agentFromConfig creates a new AgentConfig from a user config entry.
func agentFromConfig(cfg appconfig.TmuxEditAgentCfg) AgentConfig {
	a := AgentConfig{
		Name:          strings.TrimSpace(cfg.Name),
		DisplayName:   strings.TrimSpace(cfg.DisplayName),
		DetectPattern: strings.TrimSpace(cfg.DetectPattern),
		PromptPattern: strings.TrimSpace(cfg.PromptPattern),
		StripPatterns: cfg.StripPatterns,
		ClearKeys:     strings.TrimSpace(cfg.ClearKeys),
		NewlineKeys:   strings.TrimSpace(cfg.NewlineKeys),
		SubmitKeys:    strings.TrimSpace(cfg.SubmitKeys),
	}
	if cfg.ClearFirst != nil {
		a.ClearFirst = *cfg.ClearFirst
	}
	if a.DisplayName == "" {
		a.DisplayName = a.Name
	}
	return a
}

// detectAgent tries each agent's DetectPattern against pane content.
// First match wins. Returns genericAgent() if no agent matches.
func detectAgent(paneContent string, agents []AgentConfig) AgentConfig {
	for _, a := range agents {
		if a.DetectPattern == "" {
			continue
		}
		re, err := regexp.Compile(a.DetectPattern)
		if err != nil {
			continue
		}
		if re.MatchString(paneContent) {
			return a
		}
	}
	return genericAgent()
}

// findAgentByName returns the agent with the given name (case-insensitive),
// falling back to genericAgent() if not found.
func findAgentByName(name string, agents []AgentConfig) AgentConfig {
	for _, a := range agents {
		if strings.EqualFold(a.Name, name) {
			return a
		}
	}
	return genericAgent()
}

// extractPrompt uses the agent's PromptPattern to extract the current prompt
// text from pane content. Returns empty string if no pattern or no match.
func extractPrompt(paneContent string, agent AgentConfig) string {
	if agent.PromptPattern == "" {
		return ""
	}
	re, err := regexp.Compile(agent.PromptPattern)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(paneContent)
	if len(m) < 2 {
		return ""
	}
	text := m[1]
	return stripNoise(text, agent.StripPatterns)
}

// stripNoise removes each of the agent's StripPatterns from text and trims
// whitespace.
func stripNoise(text string, patterns []string) string {
	for _, p := range patterns {
		text = strings.ReplaceAll(text, p, "")
	}
	return strings.TrimSpace(text)
}
