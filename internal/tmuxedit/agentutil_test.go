package tmuxedit

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestScopeToLastSection(t *testing.T) {
	tests := []struct {
		name    string
		content string
		pattern string
		want    string
	}{
		{
			name:    "no pattern returns full content",
			content: "line1\nline2\nline3",
			pattern: "",
			want:    "line1\nline2\nline3",
		},
		{
			name:    "invalid regex returns full content",
			content: "line1\nline2",
			pattern: "[invalid",
			want:    "line1\nline2",
		},
		{
			name:    "fewer than two delimiters returns full content",
			content: "─────\nhello",
			pattern: `^─{5,}`,
			want:    "─────\nhello",
		},
		{
			name:    "extracts last section between two delimiters",
			content: "─────\nold message\n─────\n❯ prompt text\n─────",
			pattern: `^─{5,}`,
			want:    "❯ prompt text",
		},
		{
			name: "skips earlier sections",
			content: "─────\n❯ old msg1\n─────\n" +
				"─────\n❯ old msg2\n─────\n" +
				"─────\n❯ current prompt\n─────",
			pattern: `^─{5,}`,
			want:    "❯ current prompt",
		},
		{
			name: "claude multi-line prompt between rules",
			content: "previous output\n" +
				"─────────────\n" +
				"❯ first line\n" +
				"\n" +
				"❯ second line\n" +
				"\n" +
				"❯ third line\n" +
				"─────────────\n" +
				"  -- INSERT --",
			pattern: `^─{5,}`,
			want:    "❯ first line\n\n❯ second line\n\n❯ third line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scopeToLastSection(tt.content, tt.pattern)
			if got != tt.want {
				t.Errorf("scopeToLastSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripNoise(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		patterns []string
		want     string
	}{
		{"no patterns", "hello world", nil, "hello world"},
		{"strip INSERT", "fix the bug INSERT", []string{"INSERT"}, "fix the bug"},
		{"strip multiple", "INSERT fix the bug Add a follow-up", []string{"INSERT", "Add a follow-up"}, "fix the bug"},
		{"strip to empty", "INSERT", []string{"INSERT"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripNoise(tt.text, tt.patterns)
			if got != tt.want {
				t.Errorf("stripNoise() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchPromptLines(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		content string
		want    int
	}{
		{"no matches", `❯\s*(.+)$`, "no prompt here", 0},
		{"single match", `❯\s*(.+)$`, "❯ hello", 1},
		{"multiple matches", `❯\s*(.+)$`, "❯ first\nother\n❯ second", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := mustCompile(t, tt.pattern)
			got := matchPromptLines(re, tt.content)
			if len(got) != tt.want {
				t.Errorf("matchPromptLines() returned %d matches, want %d", len(got), tt.want)
			}
		})
	}
}

func TestJoinAllMatches(t *testing.T) {
	matches := []promptMatch{
		{lineNum: 0, text: "first"},
		{lineNum: 2, text: "INSERT"},
		{lineNum: 4, text: "third"},
	}
	got := joinAllMatches(matches, []string{"INSERT"})
	if got != "first\nthird" {
		t.Errorf("joinAllMatches() = %q, want %q", got, "first\nthird")
	}
}

func TestJoinLastContiguousBlock(t *testing.T) {
	tests := []struct {
		name    string
		matches []promptMatch
		strips  []string
		want    string
	}{
		{
			name: "single block",
			matches: []promptMatch{
				{lineNum: 5, text: "first"},
				{lineNum: 6, text: "second"},
			},
			want: "first\nsecond",
		},
		{
			name: "two blocks takes last",
			matches: []promptMatch{
				{lineNum: 1, text: "old"},
				{lineNum: 2, text: "old2"},
				{lineNum: 10, text: "new"},
				{lineNum: 11, text: "new2"},
			},
			want: "new\nnew2",
		},
		{
			name: "strips noise",
			matches: []promptMatch{
				{lineNum: 0, text: "fix INSERT"},
			},
			strips: []string{"INSERT"},
			want:   "fix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinLastContiguousBlock(tt.matches, tt.strips)
			if got != tt.want {
				t.Errorf("joinLastContiguousBlock() = %q, want %q", got, tt.want)
			}
		})
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

func TestSendClearSequence_EscapeKey(t *testing.T) {
	var calls []string
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, strings.Join(keys, ","))
		return nil
	}

	// sendClearSequence with "Escape" should succeed and send the key.
	// The 150ms Escape delay is real but acceptable in tests.
	err := sendClearSequence("%1", "Escape C-k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Escape", "C-k"}
	if len(calls) != len(want) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestSendClearSequence_SingleKeyError(t *testing.T) {
	oldSend := sendKeys
	defer func() { sendKeys = oldSend }()
	sendKeys = func(string, ...string) error {
		return fmt.Errorf("send failed")
	}

	err := sendClearSequence("%1", "C-u")
	if err == nil {
		t.Fatal("expected error from sendKeys failure")
	}
	if !strings.Contains(err.Error(), "clear key") {
		t.Errorf("error should mention 'clear key', got: %v", err)
	}
}

func TestSendClearSequence_RepeatedKeyError(t *testing.T) {
	oldRepeat := sendRepeatedKey
	defer func() { sendRepeatedKey = oldRepeat }()
	sendRepeatedKey = func(string, string, int) error {
		return fmt.Errorf("repeat failed")
	}

	err := sendClearSequence("%1", "BSpace*200")
	if err == nil {
		t.Fatal("expected error from sendRepeatedKey failure")
	}
	if !strings.Contains(err.Error(), "clear key") {
		t.Errorf("error should mention 'clear key', got: %v", err)
	}
}

// mustCompile is a test helper that compiles a regex or fails the test.
func mustCompile(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("regexp.Compile(%q) failed: %v", pattern, err)
	}
	return re
}
