package tmuxedit

import (
	"fmt"
	"strings"
	"testing"
)

func TestDetectAgent(t *testing.T) {
	agents := builtinAgents()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"cursor box ui", "│ → type here │\n/ commands · @ files", "cursor"},
		// Cursor panes often show Claude model names; cursor's box UI must be detected first
		{"cursor not false claude", "Claude 4.5 Sonnet\n│ → test │\n/ commands · @ files", "cursor"},
		{"amp from banner", "Amp by Sourcegraph\n> ", "amp"},
		{"aider from banner", "aider v0.50\n> /help", "aider"},
		{"no match", "some random terminal output\n$ ", "generic"},
		{"empty content", "", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectAgent(tt.content, agents)
			if got.Name() != tt.want {
				t.Errorf("detectAgent() = %q, want %q", got.Name(), tt.want)
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
		{"CURSOR", "cursor"},
		{"amp", "amp"},
		{"nonexistent", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findAgentByName(tt.name, agents)
			if got.Name() != tt.want {
				t.Errorf("findAgentByName(%q) = %q, want %q", tt.name, got.Name(), tt.want)
			}
		})
	}
}

func TestDetectAgent_InvalidRegex(t *testing.T) {
	agents := []Agent{
		&configAgent{baseAgent{name: "bad", detectPattern: "[invalid"}},
	}
	got := detectAgent("anything", agents)
	if got.Name() != "generic" {
		t.Errorf("expected generic fallback for invalid regex, got %q", got.Name())
	}
}

func TestGenericAgent(t *testing.T) {
	g := genericAgent()
	if g.Name() != "generic" {
		t.Errorf("Name = %q, want generic", g.Name())
	}
}

func TestBaseAgent_SendText_Empty(t *testing.T) {
	b := &baseAgent{newlineKeys: "S-Enter"}
	err := b.SendText("%1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseAgent_ClearInput_Disabled(t *testing.T) {
	b := &baseAgent{clearFirst: false, clearKeys: "C-u"}
	err := b.ClearInput("%1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseAgent_ClearInput_EmptyKeys(t *testing.T) {
	// clearFirst=true but no clearKeys should be a no-op
	b := &baseAgent{clearFirst: true, clearKeys: ""}
	err := b.ClearInput("%1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseAgent_ClearInput_Enabled(t *testing.T) {
	noSleep(t)
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, fmt.Sprintf("send:%s:%s", paneID, strings.Join(keys, ",")))
		return nil
	}

	b := &baseAgent{clearFirst: true, clearKeys: "C-u"}
	err := b.ClearInput("%2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 || calls[0] != "send:%2:C-u" {
		t.Errorf("expected single C-u send call, got %v", calls)
	}
}

func TestBaseAgent_ClearInput_Error(t *testing.T) {
	noSleep(t)
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(string, ...string) error {
		return fmt.Errorf("send failed")
	}

	b := &baseAgent{clearFirst: true, clearKeys: "C-u"}
	err := b.ClearInput("%1")
	if err == nil {
		t.Fatal("expected error from sendClearSequence failure")
	}
}

func TestBaseAgent_ExtractPrompt_NoPattern(t *testing.T) {
	b := &baseAgent{}
	got := b.ExtractPrompt("some content")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestBaseAgent_ExtractPrompt_InvalidRegex(t *testing.T) {
	b := &baseAgent{promptPat: "[invalid"}
	got := b.ExtractPrompt("> test")
	if got != "" {
		t.Errorf("expected empty for invalid regex, got %q", got)
	}
}

func TestConfigurable_Interface(t *testing.T) {
	// Verify that all agent types implement Configurable
	agents := builtinAgents()
	for _, a := range agents {
		c, ok := a.(Configurable)
		if !ok {
			t.Errorf("agent %q does not implement Configurable", a.Name())
			continue
		}
		base := c.Base()
		if base.name != a.Name() {
			t.Errorf("Base().name = %q, want %q", base.name, a.Name())
		}
	}
}
