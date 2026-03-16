package tmux

import (
	"strings"
	"testing"
	"time"
)

// --- Enabled ---

func TestEnabled_DefaultTrue(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "")
	if !Enabled() {
		t.Fatal("expected Enabled() true when env is empty")
	}
}

func TestEnabled_TruthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "on", " TRUE ", " On "} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("HEXAI_TMUX_STATUS", v)
			if !Enabled() {
				t.Fatalf("expected Enabled() true for %q", v)
			}
		})
	}
}

func TestEnabled_FalsyValues(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "off", "random"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("HEXAI_TMUX_STATUS", v)
			if Enabled() {
				t.Fatalf("expected Enabled() false for %q", v)
			}
		})
	}
}

// --- SetUserOption (logic paths, not actual tmux calls) ---

func TestSetUserOption_DisabledByEnv(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "off")
	// Should return nil immediately when disabled
	if err := SetUserOption("hexai_status", "test"); err != nil {
		t.Fatalf("expected nil error when disabled, got %v", err)
	}
}

func TestSetUserOption_EmptyKey(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "1")
	t.Setenv("TMUX", "/tmp/tmux-1,1,1")
	old := lookPath
	t.Cleanup(func() { lookPath = old })
	lookPath = func(string) (string, error) { return "/bin/tmux", nil }
	// Empty key after trimming should return nil
	if err := SetUserOption("  @  ", "test"); err != nil {
		t.Fatalf("expected nil for empty key, got %v", err)
	}
	if err := SetUserOption("  ", "test"); err != nil {
		t.Fatalf("expected nil for blank key, got %v", err)
	}
}

// --- SetStatus (just verifies it delegates; no tmux binary needed when disabled) ---

func TestSetStatus_DisabledNoOp(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "off")
	if err := SetStatus("anything"); err != nil {
		t.Fatalf("expected nil when status disabled, got %v", err)
	}
}

// --- FormatLLMStartStatus ---

func TestFormatLLMStartStatus(t *testing.T) {
	s := FormatLLMStartStatus("openai", "gpt-4.1")
	if !strings.Contains(s, "LLM:openai:gpt-4.1") {
		t.Fatalf("missing provider:model in %q", s)
	}
	if !strings.Contains(s, "⏳") {
		t.Fatalf("missing hourglass emoji in %q", s)
	}
	// Should contain baseFGToken placeholders (pre-theme)
	if !strings.Contains(s, baseFGToken) {
		t.Fatalf("expected baseFGToken placeholder in %q", s)
	}
}

// --- humanWindow ---

func TestHumanWindow(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "?"},
		{"negative", -5 * time.Minute, "?"},
		{"exact hour", time.Hour, "1h"},
		{"two hours", 2 * time.Hour, "2h"},
		{"30 minutes", 30 * time.Minute, "30m"},
		{"90 minutes", 90 * time.Minute, "90m"},
		{"45 minutes", 45 * time.Minute, "45m"},
		{"120 minutes", 120 * time.Minute, "2h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := humanWindow(tt.d)
			if got != tt.want {
				t.Errorf("humanWindow(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// --- truncateStatus ---

func TestTruncateStatus(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"zero limit", "hello", 0, ""},
		{"negative limit", "hello", -1, ""},
		{"within limit", "hi", 5, "hi"},
		{"exact limit", "hello", 5, "hello"},
		{"over limit", "hello world", 5, "hell…"},
		{"limit 1", "hello", 1, "h"},
		{"limit 2", "hello", 2, "h…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStatus(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("truncateStatus(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// --- strings.TrimSpace ---

func TestStringsTrim(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty", "", ""},
		{"no whitespace", "hello", "hello"},
		{"leading spaces", "  hello", "hello"},
		{"trailing spaces", "hello  ", "hello"},
		{"both sides", "  hello  ", "hello"},
		{"tabs", "\thello\t", "hello"},
		{"newlines", "\nhello\n", "hello"},
		{"carriage returns", "\rhello\r", "hello"},
		{"mixed whitespace", " \t\n\rhello \t\n\r", "hello"},
		{"all whitespace", "   \t\n  ", ""},
		{"internal spaces preserved", "hello world", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.TrimSpace(tt.s)
			if got != tt.want {
				t.Errorf("strings.TrimSpace(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

// --- maxStatusLen ---

func TestMaxStatusLen(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{"empty", "", 0},
		{"valid", "80", 80},
		{"negative", "-5", 0},
		{"zero", "0", 0},
		{"non-numeric", "abc", 0},
		{"whitespace padded", " 100 ", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HEXAI_TMUX_STATUS_MAXLEN", tt.env)
			got := maxStatusLen()
			if got != tt.want {
				t.Errorf("maxStatusLen() = %d, want %d (env=%q)", got, tt.want, tt.env)
			}
		})
	}
}

// --- narrowEnabled ---

func TestNarrowEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"empty", "", false},
		{"1", "1", true},
		{"true", "true", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"0", "0", false},
		{"false", "false", false},
		{"random", "random", false},
		{"TRUE uppercase", "TRUE", true},
		{"padded", " 1 ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HEXAI_TMUX_STATUS_NARROW", tt.env)
			got := narrowEnabled()
			if got != tt.want {
				t.Errorf("narrowEnabled() = %v, want %v (env=%q)", got, tt.want, tt.env)
			}
		})
	}
}

// --- applyTheme ---

func TestApplyTheme_NoTheme(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "")
	input := baseFGToken + "hello" + arrowUpToken + "up" + arrowDownToken + "down"
	got := applyTheme(input)
	// Should replace tokens with default fg and default arrow colors
	if strings.Contains(got, baseFGToken) {
		t.Fatalf("baseFGToken not replaced in %q", got)
	}
	if !strings.Contains(got, "#[fg=default]") {
		t.Fatalf("expected default fg in %q", got)
	}
	// No wrap, so no bg=default suffix
	if strings.HasSuffix(got, "#[fg=default,bg=default]") {
		t.Fatalf("should not wrap without theme in %q", got)
	}
}

func TestApplyTheme_PurpleTheme(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "purple")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "")
	input := baseFGToken + "hello"
	got := applyTheme(input)
	if !strings.Contains(got, "#[fg=white") {
		t.Fatalf("expected white fg for purple theme in %q", got)
	}
	if !strings.Contains(got, "bg=magenta") {
		t.Fatalf("expected magenta bg for purple theme in %q", got)
	}
	if !strings.HasSuffix(got, "#[fg=default,bg=default]") {
		t.Fatalf("expected reset suffix for themed output in %q", got)
	}
}

func TestApplyTheme_YellowTheme(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "yellow")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "")
	input := baseFGToken + "test" + arrowUpToken + "u" + arrowDownToken + "d"
	got := applyTheme(input)
	if !strings.Contains(got, "#[fg=black") {
		t.Fatalf("expected black fg for yellow theme in %q", got)
	}
	if !strings.Contains(got, "bg=yellow") {
		t.Fatalf("expected yellow bg in %q", got)
	}
}

func TestApplyTheme_BlueTheme(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "blue")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "")
	input := baseFGToken + "test" + arrowUpToken + "u" + arrowDownToken + "d"
	got := applyTheme(input)
	if !strings.Contains(got, "#[fg=white") {
		t.Fatalf("expected white fg for blue theme in %q", got)
	}
	if !strings.Contains(got, "bg=blue") {
		t.Fatalf("expected blue bg in %q", got)
	}
}

func TestApplyTheme_ExplicitFGBG(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "red")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "green")
	input := baseFGToken + "test" + arrowUpToken + "u" + arrowDownToken + "d"
	got := applyTheme(input)
	if !strings.Contains(got, "#[fg=red") {
		t.Fatalf("expected red fg in %q", got)
	}
	if !strings.Contains(got, "bg=green") {
		t.Fatalf("expected green bg in %q", got)
	}
	if !strings.HasSuffix(got, "#[fg=default,bg=default]") {
		t.Fatalf("expected reset suffix in %q", got)
	}
}

func TestApplyTheme_ExplicitBGOnly(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "cyan")
	input := baseFGToken + "test"
	got := applyTheme(input)
	// When only bg is set, fg defaults to "default"
	if !strings.Contains(got, "#[fg=default") {
		t.Fatalf("expected default fg when only bg set in %q", got)
	}
	if !strings.Contains(got, "bg=cyan") {
		t.Fatalf("expected cyan bg in %q", got)
	}
}

func TestApplyTheme_NoTokensInInput(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "")
	// Input without any tokens should pass through unchanged
	got := applyTheme("plain text")
	if got != "plain text" {
		t.Fatalf("expected unchanged output for tokenless input, got %q", got)
	}
}

func TestApplyTheme_ThemeAliases(t *testing.T) {
	// Test theme aliases that map to the same preset
	aliases := map[string]string{
		"white-on-purple":  "magenta",
		"magenta":          "magenta",
		"white-on-magenta": "magenta",
		"black-on-yellow":  "yellow",
		"black-on-gold":    "yellow",
		"white-on-blue":    "blue",
		"white-on-navy":    "blue",
	}
	for theme, expectBG := range aliases {
		t.Run(theme, func(t *testing.T) {
			t.Setenv("HEXAI_TMUX_STATUS_THEME", theme)
			t.Setenv("HEXAI_TMUX_STATUS_FG", "")
			t.Setenv("HEXAI_TMUX_STATUS_BG", "")
			got := applyTheme(baseFGToken + "x")
			if !strings.Contains(got, "bg="+expectBG) {
				t.Fatalf("theme %q: expected bg=%s in %q", theme, expectBG, got)
			}
		})
	}
}

// --- FormatGlobalStatusColored branch coverage ---

func TestFormatGlobalStatusColored_EmptyProvider(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_NARROW", "")
	t.Setenv("HEXAI_TMUX_STATUS_MAXLEN", "")
	// Empty provider should return head only (no tail)
	s := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "", "model", 1.1, 4, time.Hour)
	if strings.Contains(s, "|") {
		t.Fatalf("expected no tail with empty provider: %q", s)
	}
}

func TestFormatGlobalStatusColored_EmptyModel(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_NARROW", "")
	t.Setenv("HEXAI_TMUX_STATUS_MAXLEN", "")
	// Empty model should return head only
	s := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "openai", "", 1.1, 4, time.Hour)
	if strings.Contains(s, "|") {
		t.Fatalf("expected no tail with empty model: %q", s)
	}
}

func TestFormatGlobalStatusColored_WithTail(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_NARROW", "")
	t.Setenv("HEXAI_TMUX_STATUS_MAXLEN", "")
	// With valid provider and model, should include tail
	s := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "openai", "gpt-4.1", 1.1, 4, time.Hour)
	if !strings.Contains(s, "|") || !strings.Contains(s, "openai:gpt-4.1") {
		t.Fatalf("expected tail with provider:model: %q", s)
	}
}

func TestFormatGlobalStatusColored_MaxLenTruncatesHead(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_NARROW", "")
	t.Setenv("HEXAI_TMUX_STATUS_THEME", "")
	t.Setenv("HEXAI_TMUX_STATUS_FG", "")
	t.Setenv("HEXAI_TMUX_STATUS_BG", "")
	// Set maxlen very small so even head gets truncated
	t.Setenv("HEXAI_TMUX_STATUS_MAXLEN", "5")
	s := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "openai", "gpt-4.1", 1.1, 4, time.Hour)
	// The string contains control-char tokens; truncateStatus works on raw bytes.
	// Just verify it ends with the ellipsis character and is shorter than untruncated.
	if !strings.HasSuffix(s, "…") {
		t.Fatalf("expected truncated output ending with ellipsis, got %q", s)
	}
}

func TestFormatGlobalStatusColored_MaxLenFitsBoth(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS_NARROW", "")
	// Set maxlen very large so both head and tail fit
	t.Setenv("HEXAI_TMUX_STATUS_MAXLEN", "500")
	s := FormatGlobalStatusColored(10, 3.3, 1000, 2000, "openai", "gpt-4.1", 1.1, 4, time.Hour)
	if !strings.Contains(s, "|") || !strings.Contains(s, "openai:gpt-4.1") {
		t.Fatalf("expected full output with large maxlen: %q", s)
	}
}
