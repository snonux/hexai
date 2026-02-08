package tmuxedit

import (
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

// configAgent uses baseAgent defaults for all operations. It serves
// user-defined agents from TOML config and simple built-ins (amp, aider)
// that don't need specialized extraction or clearing logic.
type configAgent struct{ baseAgent }

// builtinAgents returns the default set of agent implementations. Order
// matters: agents with distinctive UI elements (box-drawing, etc.) are
// checked first to avoid false positives from model names like "Claude
// 4.5 Sonnet" appearing in other agents' panes.
func builtinAgents() []Agent {
	return []Agent{
		newCursorAgent(),
		newClaudeAgent(),
		&configAgent{baseAgent{
			name:          "amp",
			displayName:   "Amp",
			detectPattern: `(?i)(amp|sourcegraph)`,
			promptPat:     `(?m)│\s*(.+?)\s*│\s*$`,
			clearFirst:    true,
			clearKeys:     "C-u",
			newlineKeys:   "S-Enter",
			submitKeys:    "Enter",
		}},
		&configAgent{baseAgent{
			name:          "aider",
			displayName:   "Aider",
			detectPattern: `(?i)aider`,
			promptPat:     `(?m)>\s*(.+)$`,
			clearFirst:    true,
			clearKeys:     "C-u",
			newlineKeys:   "",
			submitKeys:    "Enter",
		}},
	}
}

// genericAgent returns a fallback agent with no detection or prompt extraction.
// The user gets a blank editor and text is sent verbatim.
func genericAgent() Agent {
	return &configAgent{baseAgent{
		name:        "generic",
		displayName: "Generic",
		newlineKeys: "",
		submitKeys:  "Enter",
	}}
}

// resolveAgents merges built-in agent defaults with user-provided overrides
// from config. Agents are matched by name (case-insensitive); user config
// wins field-by-field over builtins. The Configurable interface provides
// access to baseAgent fields for merging.
func resolveAgents(cfgAgents []appconfig.TmuxEditAgentCfg) []Agent {
	agents := builtinAgents()
	for _, ca := range cfgAgents {
		merged := false
		for i, a := range agents {
			if !strings.EqualFold(a.Name(), ca.Name) {
				continue
			}
			if c, ok := a.(Configurable); ok {
				mergeAgentConfig(c.Base(), ca)
			}
			merged = true
			_ = i // index not needed; we modify through the pointer
			break
		}
		if !merged {
			agents = append(agents, agentFromConfig(ca))
		}
	}
	return agents
}

// mergeAgentConfig overrides fields in base with non-zero values from cfg.
// It modifies the baseAgent in place via pointer.
func mergeAgentConfig(base *baseAgent, cfg appconfig.TmuxEditAgentCfg) {
	if s := strings.TrimSpace(cfg.DisplayName); s != "" {
		base.displayName = s
	}
	if s := strings.TrimSpace(cfg.DetectPattern); s != "" {
		base.detectPattern = s
	}
	if s := strings.TrimSpace(cfg.SectionPattern); s != "" {
		base.sectionPat = s
	}
	if s := strings.TrimSpace(cfg.PromptPattern); s != "" {
		base.promptPat = s
	}
	if len(cfg.StripPatterns) > 0 {
		base.stripPatterns = cfg.StripPatterns
	}
	if cfg.ClearFirst != nil {
		base.clearFirst = *cfg.ClearFirst
	}
	if s := strings.TrimSpace(cfg.ClearKeys); s != "" {
		base.clearKeys = s
	}
	if s := strings.TrimSpace(cfg.NewlineKeys); s != "" {
		base.newlineKeys = s
	}
	if s := strings.TrimSpace(cfg.SubmitKeys); s != "" {
		base.submitKeys = s
	}
}

// agentFromConfig creates a new configAgent from a user config entry.
func agentFromConfig(cfg appconfig.TmuxEditAgentCfg) Agent {
	b := baseAgent{
		name:          strings.TrimSpace(cfg.Name),
		displayName:   strings.TrimSpace(cfg.DisplayName),
		detectPattern: strings.TrimSpace(cfg.DetectPattern),
		sectionPat:    strings.TrimSpace(cfg.SectionPattern),
		promptPat:     strings.TrimSpace(cfg.PromptPattern),
		stripPatterns: cfg.StripPatterns,
		clearKeys:     strings.TrimSpace(cfg.ClearKeys),
		newlineKeys:   strings.TrimSpace(cfg.NewlineKeys),
		submitKeys:    strings.TrimSpace(cfg.SubmitKeys),
	}
	if cfg.ClearFirst != nil {
		b.clearFirst = *cfg.ClearFirst
	}
	if b.displayName == "" {
		b.displayName = b.name
	}
	return &configAgent{b}
}
