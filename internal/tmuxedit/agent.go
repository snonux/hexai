// Package tmuxedit implements a tmux popup editor for composing AI agent prompts.
// agent.go defines the Agent interface, the baseAgent struct with default
// implementations, and agent detection/resolution helpers.
package tmuxedit

import (
	"regexp"
	"strings"
)

// Agent defines how to interact with a specific AI agent in a tmux pane.
// Each implementation encapsulates its own detection, extraction, clearing,
// and sending logic since agents differ fundamentally in their UI structure.
type Agent interface {
	Name() string
	DisplayName() string
	Detect(paneContent string) bool
	ExtractPrompt(paneContent string) string
	ClearInput(paneID string) error
	SendText(paneID, text string) error
}

// Configurable provides access to a baseAgent's fields for config merging.
// Agent implementations that embed baseAgent automatically satisfy this.
type Configurable interface {
	Base() *baseAgent
}

// baseAgent holds configurable fields and provides default implementations
// of the Agent interface. Specialized agents (e.g. cursor) embed baseAgent
// and override methods where behavior differs from the defaults.
type baseAgent struct {
	name          string
	displayName   string
	detectPattern string
	sectionPat    string   // optional regex to delimit the prompt area
	promptPat     string   // regex with capture group (1) for prompt text
	stripPatterns []string // substrings removed from extracted text
	clearFirst    bool     // whether to clear existing input before sending
	clearKeys     string   // tmux key sequence to clear input
	newlineKeys   string   // tmux key to insert a newline
	submitKeys    string   // tmux key to submit the prompt
	deps          tmuxEditDeps
}

// Base returns a pointer to the baseAgent for config merging.
func (b *baseAgent) Base() *baseAgent { return b }

// Name returns the agent's short identifier (e.g. "cursor", "amp").
func (b *baseAgent) Name() string { return b.name }

// DisplayName returns the agent's human-readable name.
func (b *baseAgent) DisplayName() string { return b.displayName }

// Detect checks whether the pane content matches this agent's detection
// pattern. Returns false if no pattern is set or the regex is invalid.
func (b *baseAgent) Detect(paneContent string) bool {
	if b.detectPattern == "" {
		return false
	}
	re, err := regexp.Compile(b.detectPattern)
	if err != nil {
		return false
	}
	return re.MatchString(paneContent)
}

// ExtractPrompt uses the agent's prompt pattern to extract the current prompt
// text from pane content. If sectionPat is set, extraction is scoped to the
// last section between two delimiter lines and all matches are joined.
// Without sectionPat, the last contiguous group of matched lines is used.
// Returns empty string if no pattern or no match.
func (b *baseAgent) ExtractPrompt(paneContent string) string {
	if b.promptPat == "" {
		return ""
	}
	re, err := regexp.Compile(b.promptPat)
	if err != nil {
		return ""
	}
	scoped := b.sectionPat != ""
	content := scopeToLastSection(paneContent, b.sectionPat)
	allMatches := matchPromptLines(re, content)
	if len(allMatches) == 0 {
		return ""
	}
	if scoped {
		return joinAllMatches(allMatches, b.stripPatterns)
	}
	return joinLastContiguousBlock(allMatches, b.stripPatterns)
}

// ClearInput clears existing input in the pane using the configured key
// sequence. Skipped if clearFirst is false or clearKeys is empty.
func (b *baseAgent) ClearInput(paneID string) error {
	if !b.clearFirst || b.clearKeys == "" {
		return nil
	}
	if err := b.deps.sendClearSequence(paneID, b.clearKeys); err != nil {
		return err
	}
	b.deps.sleep()
	return nil
}

// SendText sends the given text to the target pane line-by-line, using the
// agent's newline key between lines.
func (b *baseAgent) SendText(paneID, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return b.deps.sendLines(paneID, text, b.newlineKeys)
}

func withAgentDeps(agents []Agent, deps tmuxEditDeps) []Agent {
	for _, agent := range agents {
		withAgentDep(agent, deps)
	}
	return agents
}

func withAgentDep(agent Agent, deps tmuxEditDeps) Agent {
	if c, ok := agent.(Configurable); ok {
		c.Base().deps = deps
	}
	return agent
}

// detectAgent tries each agent's Detect method against pane content.
// First match wins. Returns genericAgent() if no agent matches.
func detectAgent(paneContent string, agents []Agent) Agent {
	for _, a := range agents {
		if a.Detect(paneContent) {
			return a
		}
	}
	return genericAgent()
}

// findAgentByName returns the agent with the given name (case-insensitive),
// falling back to genericAgent() if not found.
func findAgentByName(name string, agents []Agent) Agent {
	for _, a := range agents {
		if strings.EqualFold(a.Name(), name) {
			return a
		}
	}
	return genericAgent()
}
