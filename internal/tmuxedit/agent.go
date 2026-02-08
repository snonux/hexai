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

// builtinAgents returns the default set of agent configurations. Order
// matters: agents with distinctive UI elements (box-drawing, etc.) are
// checked first to avoid false positives from model names like "Claude
// 4.5 Sonnet" appearing in other agents' panes. Overridden/extended by
// user config in [tmux_edit.agents].
func builtinAgents() []AgentConfig {
	return []AgentConfig{
		{
			// Cursor Agent uses a distinctive box-drawing │ → prompt │ UI.
			// Detect by the box structure or "/ commands" footer. Checked
			// first because cursor panes show model names like "Claude 4.5".
			// Clear uses End + bulk backspace to delete all existing text.
			// The *200 suffix sends 200 backspaces via tmux send-keys -N.
			Name:          "cursor",
			DisplayName:   "Cursor",
			DetectPattern: `(│\s*→|/ commands · @ files)`,
			PromptPattern: `(?m)│\s*→?\s*(.+?)\s*│\s*$`,
			StripPatterns: []string{"INSERT", "Add a follow-up", "ctrl+c to stop"},
			ClearFirst:    true,
			ClearKeys:     "End BSpace*200",
			NewlineKeys:   "S-Enter",
			SubmitKeys:    "Enter",
		},
		{
			// Claude Code uses ❯ prompt between ──── horizontal rules.
			// Detect by the ❯ prompt or explicit "claude code" banner.
			Name:          "claude",
			DisplayName:   "Claude Code",
			DetectPattern: `(❯|claude code|anthropic)`,
			PromptPattern: `(?m)❯\s*(.+)$`,
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
// text from pane content. For multi-line prompts (e.g. cursor's box-drawing
// │...│ UI) it takes only the last contiguous group of matched lines, which
// avoids picking up command-review or dialog boxes that use the same border
// characters. Returns empty string if no pattern or no match.
func extractPrompt(paneContent string, agent AgentConfig) string {
	if agent.PromptPattern == "" {
		return ""
	}
	re, err := regexp.Compile(agent.PromptPattern)
	if err != nil {
		return ""
	}
	allMatches := matchPromptLines(re, paneContent)
	if len(allMatches) == 0 {
		return ""
	}
	return joinLastContiguousBlock(allMatches, agent.StripPatterns)
}

// promptMatch holds a regex match result with its line number in the pane.
type promptMatch struct {
	lineNum int
	text    string // capture group 1
}

// matchPromptLines runs the prompt regex against each pane line, returning
// matches with their line numbers for contiguity analysis.
func matchPromptLines(re *regexp.Regexp, paneContent string) []promptMatch {
	paneLines := strings.Split(paneContent, "\n")
	var matches []promptMatch
	for i, line := range paneLines {
		m := re.FindStringSubmatch(line)
		if len(m) >= 2 {
			matches = append(matches, promptMatch{lineNum: i, text: m[1]})
		}
	}
	return matches
}

// joinLastContiguousBlock takes the last group of matches on consecutive line
// numbers, strips noise from each, and joins the non-empty results with
// newlines. This ensures that only the bottom-most box (the input prompt)
// is captured when multiple box-drawing sections exist in the pane.
func joinLastContiguousBlock(matches []promptMatch, strips []string) string {
	last := len(matches) - 1
	start := last
	for start > 0 && matches[start].lineNum-matches[start-1].lineNum == 1 {
		start--
	}
	var lines []string
	for i := start; i <= last; i++ {
		line := stripNoise(matches[i].text, strips)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// stripNoise removes each of the agent's StripPatterns from text and trims
// whitespace.
func stripNoise(text string, patterns []string) string {
	for _, p := range patterns {
		text = strings.ReplaceAll(text, p, "")
	}
	return strings.TrimSpace(text)
}
