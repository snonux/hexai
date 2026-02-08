package tmuxedit

import (
	"fmt"
	"strings"
	"testing"
)

func TestDeduplicateText(t *testing.T) {
	tests := []struct {
		name     string
		original string
		edited   string
		want     string
	}{
		{"empty both", "", "", ""},
		{"empty original", "", "new text", "new text"},
		{"empty edited", "original", "", ""},
		{"unchanged", "hello world", "hello world", ""},
		{"appended", "hello", "hello world", "hello world"},
		{"rewritten", "hello world", "goodbye world", "goodbye world"},
		{"whitespace handling", "  hello  ", "  hello  world  ", "hello  world"},
		{"appended with newlines", "line1\nline2", "line1\nline2\nline3", "line1\nline2\nline3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateText(tt.original, tt.edited)
			if got != tt.want {
				t.Errorf("deduplicateText(%q, %q) = %q, want %q",
					tt.original, tt.edited, got, tt.want)
			}
		})
	}
}

// noSleep disables the post-clear sleep in tests and restores it on cleanup.
func noSleep(t *testing.T) {
	t.Helper()
	old := sleepAfterClear
	sleepAfterClear = func() {}
	t.Cleanup(func() { sleepAfterClear = old })
}

func TestSendTextToPane_SingleLine(t *testing.T) {
	noSleep(t)
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, fmt.Sprintf("send:%s:%s", paneID, strings.Join(keys, ",")))
		return nil
	}
	agent := AgentConfig{ClearFirst: true, ClearKeys: "C-u", NewlineKeys: "S-Enter"}
	err := sendTextToPane("%5", "hello", agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect: clear, then single line (no newline after last line)
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2: %v", len(calls), calls)
	}
	if calls[0] != "send:%5:C-u" {
		t.Errorf("call[0] = %q, want clear", calls[0])
	}
	if calls[1] != "send:%5:hello" {
		t.Errorf("call[1] = %q, want text", calls[1])
	}
}

func TestSendTextToPane_MultiLine(t *testing.T) {
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, strings.Join(keys, ","))
		return nil
	}
	agent := AgentConfig{NewlineKeys: "S-Enter"}
	err := sendTextToPane("%1", "line1\nline2\nline3", agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect: line1, S-Enter, line2, S-Enter, line3 (no trailing newline)
	want := []string{"line1", "S-Enter", "line2", "S-Enter", "line3"}
	if len(calls) != len(want) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestSendTextToPane_NoClear(t *testing.T) {
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, strings.Join(keys, ","))
		return nil
	}
	agent := AgentConfig{ClearFirst: false, ClearKeys: "C-u"}
	err := sendTextToPane("%1", "hello", agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No clear call; just the text
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1: %v", len(calls), calls)
	}
}

func TestSendTextToPane_Empty(t *testing.T) {
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(string, ...string) error {
		t.Fatal("sendKeys should not be called for empty text")
		return nil
	}
	err := sendTextToPane("%1", "", AgentConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendTextToPane_ClearError(t *testing.T) {
	noSleep(t)
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		return fmt.Errorf("tmux error")
	}
	agent := AgentConfig{ClearFirst: true, ClearKeys: "C-u"}
	err := sendTextToPane("%1", "hello", agent)
	if err == nil {
		t.Fatal("expected error on clear failure")
	}
}

func TestSendTextToPane_SendError(t *testing.T) {
	noSleep(t)
	callCount := 0
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		callCount++
		if callCount == 2 { // fail on second call (first line text)
			return fmt.Errorf("send failed")
		}
		return nil
	}
	agent := AgentConfig{ClearFirst: true, ClearKeys: "C-u"}
	err := sendTextToPane("%1", "hello", agent)
	if err == nil {
		t.Fatal("expected error on send failure")
	}
}

func TestSendTextToPane_BulkClear(t *testing.T) {
	noSleep(t)
	var calls []string
	oldSend := sendKeys
	oldRepeat := sendRepeatedKey
	defer func() {
		sendKeys = oldSend
		sendRepeatedKey = oldRepeat
	}()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, fmt.Sprintf("send:%s:%s", paneID, strings.Join(keys, ",")))
		return nil
	}
	sendRepeatedKey = func(paneID, key string, count int) error {
		calls = append(calls, fmt.Sprintf("repeat:%s:%s*%d", paneID, key, count))
		return nil
	}
	// "End BSpace*200" should send End normally, then BSpace 200 times via -N
	agent := AgentConfig{ClearFirst: true, ClearKeys: "End BSpace*200", NewlineKeys: "S-Enter"}
	err := sendTextToPane("%5", "new text", agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"send:%5:End",
		"repeat:%5:BSpace*200",
		"send:%5:new text",
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

func TestParseKeyRepeat(t *testing.T) {
	tests := []struct {
		token     string
		wantKey   string
		wantCount int
	}{
		{"BSpace*200", "BSpace", 200},
		{"End", "End", 1},
		{"C-u", "C-u", 1},
		{"BSpace*1", "BSpace", 1},
		{"BSpace*0", "BSpace*0", 1},     // invalid count
		{"BSpace*abc", "BSpace*abc", 1}, // non-numeric
		{"*200", "*200", 1},             // no key name
		{"x*3", "x", 3},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			key, count := parseKeyRepeat(tt.token)
			if key != tt.wantKey || count != tt.wantCount {
				t.Errorf("parseKeyRepeat(%q) = (%q, %d), want (%q, %d)",
					tt.token, key, count, tt.wantKey, tt.wantCount)
			}
		})
	}
}

func TestSendTextToPane_FallbackNewline(t *testing.T) {
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, strings.Join(keys, ","))
		return nil
	}
	// Agent with empty NewlineKeys should fallback to "Enter"
	agent := AgentConfig{NewlineKeys: ""}
	err := sendTextToPane("%1", "a\nb", agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect: a, Enter, b
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3: %v", len(calls), calls)
	}
	if calls[1] != "Enter" {
		t.Errorf("newline key = %q, want Enter (fallback)", calls[1])
	}
}
