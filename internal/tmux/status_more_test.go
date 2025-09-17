package tmux

import (
	"os"
	"testing"
	"time"
)

func TestFormatLLMStatsStatus_Basic(t *testing.T) {
	s := FormatLLMStatsStatus("gpt-4.1", 5, 0.8, 1234, 5678)
	if s == "" || !containsAll(s, []string{"LLM:gpt-4.1", "5r", "0.8rpm"}) {
		t.Fatalf("unexpected status: %q", s)
	}
}

func TestFormatLLMStatsStatusColored_Basic(t *testing.T) {
	s := FormatLLMStatsStatusColored("openai", "gpt-4.1", 2, 1.2, 100, 200)
	if s == "" || !containsAll(s, []string{"LLM:openai:gpt-4.1", "rpm", "2r"}) {
		t.Fatalf("colored status missing parts: %q", s)
	}
}

func TestFormatGlobalStatusColored_NarrowAndMaxLen(t *testing.T) {
	// Narrow mode should elide the tail
	os.Setenv("HEXAI_TMUX_STATUS_NARROW", "1")
	defer os.Unsetenv("HEXAI_TMUX_STATUS_NARROW")
	s := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "prov", "model", 1.1, 4, 30*time.Minute)
	if containsAll(s, []string{"|", "prov:model"}) {
		t.Fatalf("narrow mode should not include tail: %q", s)
	}
	// Max length should also drop the tail when it would overflow
	os.Unsetenv("HEXAI_TMUX_STATUS_NARROW")
	os.Setenv("HEXAI_TMUX_STATUS_MAXLEN", "10")
	defer os.Unsetenv("HEXAI_TMUX_STATUS_MAXLEN")
	s2 := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "prov", "model", 1.1, 4, 30*time.Minute)
	if containsAll(s2, []string{"|", "prov:model"}) {
		t.Fatalf("maxlen should drop tail when overflowing: %q", s2)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || (len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	// tiny wrapper to avoid importing strings
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
