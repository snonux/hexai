package tmuxedit

import (
	"fmt"
	"strings"
	"testing"
)

func noSleepDeps() tmuxEditDeps {
	return tmuxEditDeps{sleepAfterClear: func() {}, sleepAfterEscape: func() {}}
}

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

func TestSendLines_SingleLine(t *testing.T) {
	var calls []string
	deps := tmuxEditDeps{sendKeys: func(paneID string, keys ...string) error {
		calls = append(calls, fmt.Sprintf("send:%s:%s", paneID, strings.Join(keys, ",")))
		return nil
	}}

	err := deps.sendLines("%5", "hello", "S-Enter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1: %v", len(calls), calls)
	}
	if calls[0] != "send:%5:hello" {
		t.Errorf("call[0] = %q, want text", calls[0])
	}
}

func TestSendLines_MultiLine(t *testing.T) {
	var calls []string
	deps := tmuxEditDeps{sendKeys: func(paneID string, keys ...string) error {
		calls = append(calls, strings.Join(keys, ","))
		return nil
	}}

	err := deps.sendLines("%1", "line1\nline2\nline3", "S-Enter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestSendLines_FallbackNewline(t *testing.T) {
	var calls []string
	deps := tmuxEditDeps{sendKeys: func(paneID string, keys ...string) error {
		calls = append(calls, strings.Join(keys, ","))
		return nil
	}}

	// Empty newlineKeys should fallback to "Enter"
	err := deps.sendLines("%1", "a\nb", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3: %v", len(calls), calls)
	}
	if calls[1] != "Enter" {
		t.Errorf("newline key = %q, want Enter (fallback)", calls[1])
	}
}

func TestSendLines_Error(t *testing.T) {
	deps := tmuxEditDeps{sendKeys: func(string, ...string) error {
		return fmt.Errorf("send failed")
	}}

	err := deps.sendLines("%1", "hello", "Enter")
	if err == nil {
		t.Fatal("expected error on send failure")
	}
}
