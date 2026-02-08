package tmuxedit

import (
	"fmt"
	"strings"
	"testing"
)

func TestClaudeAgent_ExtractPrompt(t *testing.T) {
	agent := newClaudeAgent()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "single line",
			content: "──────\n❯ hello world\n──────",
			want:    "hello world",
		},
		{
			name: "multi-line between rules",
			content: "previous output\n" +
				"──────────────\n" +
				"❯ first line\n" +
				"\n" +
				"❯ second line\n" +
				"\n" +
				"❯ third line\n" +
				"──────────────\n" +
				"  -- INSERT --",
			want: "first line\nsecond line\nthird line",
		},
		{
			name: "wrapped long line",
			content: "──────────────\n" +
				"❯ This is a really long prompt that wraps\n" +
				"  to a second line in the terminal\n" +
				"──────────────\n" +
				"  -- INSERT --",
			want: "This is a really long prompt that wraps\nto a second line in the terminal",
		},
		{
			name: "ignores previous messages",
			content: "──────────────\n" +
				"❯ old user message\n" +
				"──────────────\n" +
				"assistant response here\n" +
				"──────────────\n" +
				"❯ current prompt\n" +
				"──────────────\n" +
				"  -- INSERT --",
			want: "current prompt",
		},
		{
			name:    "no match",
			content: "no prompt here",
			want:    "",
		},
		{
			name:    "no section delimiters",
			content: "❯ hello world",
			want:    "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agent.ExtractPrompt(tt.content)
			if got != tt.want {
				t.Errorf("ExtractPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeAgent_ClearInput(t *testing.T) {
	noSleep(t)
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, fmt.Sprintf("send:%s:%s", paneID, strings.Join(keys, ",")))
		return nil
	}

	agent := newClaudeAgent()
	err := agent.ClearInput("%3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "C-a C-k" (Emacs/readline style) should send each as separate send-keys call
	want := []string{
		"send:%3:C-a",
		"send:%3:C-k",
	}
	if len(calls) != len(want) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestClaudeAgent_Detect(t *testing.T) {
	agent := newClaudeAgent()
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"prompt symbol", "❯ hello", true},
		{"claude code banner", "claude code v1.0", true},
		{"anthropic mention", "Powered by Anthropic", true},
		{"no match", "some text", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agent.Detect(tt.content); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}
