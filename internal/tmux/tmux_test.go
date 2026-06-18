package tmux

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestInSession(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-123,123,0")
	if !InSession() {
		t.Fatal("expected InSession true when TMUX is set")
	}
	t.Setenv("TMUX", "")
	if InSession() {
		t.Fatal("expected InSession false when TMUX is empty")
	}
}

func TestHasBinary_UsesLookPath(t *testing.T) {
	r := Runner{lookPath: func(file string) (string, error) { return "/bin/tmux", nil }}
	if !r.HasBinary() {
		t.Fatal("expected HasBinary true when lookPath succeeds")
	}
	r.lookPath = func(file string) (string, error) { return "", errors.New("nope") }
	if r.HasBinary() {
		t.Fatal("expected HasBinary false when lookPath fails")
	}
}

// --- Phase 3: Shell Utility Tests ---

func TestShellJoin(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		expected string
	}{
		{
			name:     "simple alphanumeric",
			argv:     []string{"ls", "-la"},
			expected: "ls -la",
		},
		{
			name:     "with spaces",
			argv:     []string{"echo", "hello world"},
			expected: "echo 'hello world'",
		},
		{
			name:     "with single quotes",
			argv:     []string{"echo", "it's fine"},
			expected: "echo 'it'\\''s fine'",
		},
		{
			name:     "with double quotes",
			argv:     []string{"echo", `say "hello"`},
			expected: `echo 'say "hello"'`,
		},
		{
			name:     "with dollar signs",
			argv:     []string{"echo", "$PATH"},
			expected: "echo '$PATH'",
		},
		{
			name:     "with backticks",
			argv:     []string{"echo", "`whoami`"},
			expected: "echo '`whoami`'",
		},
		{
			name:     "with backslashes",
			argv:     []string{"echo", `path\to\file`},
			expected: `echo 'path\to\file'`,
		},
		{
			name:     "empty string",
			argv:     []string{"echo", ""},
			expected: "echo ''",
		},
		{
			name:     "only empty strings",
			argv:     []string{"", "", ""},
			expected: "'' '' ''",
		},
		{
			name:     "with newlines",
			argv:     []string{"echo", "line1\nline2"},
			expected: "echo 'line1\nline2'",
		},
		{
			name:     "with tabs",
			argv:     []string{"echo", "col1\tcol2"},
			expected: "echo 'col1\tcol2'",
		},
		{
			name:     "with semicolons",
			argv:     []string{"echo", "a;b"},
			expected: "echo 'a;b'",
		},
		{
			name:     "with pipes",
			argv:     []string{"echo", "a|b"},
			expected: "echo 'a|b'",
		},
		{
			name:     "with ampersands",
			argv:     []string{"echo", "a&b"},
			expected: "echo 'a&b'",
		},
		{
			name:     "safe bare characters",
			argv:     []string{"cat", "/path/to/file-name_123.txt"},
			expected: "cat /path/to/file-name_123.txt",
		},
		{
			name:     "with colons and @",
			argv:     []string{"ssh", "user@host:22"},
			expected: "ssh 'user@host:22'", // @ is not safe, so gets quoted
		},
		{
			name:     "unicode characters",
			argv:     []string{"echo", "hello 世界"},
			expected: "echo 'hello 世界'",
		},
		{
			name:     "mixed safe and unsafe",
			argv:     []string{"git", "commit", "-m", "fix: resolve issue #123"},
			expected: "git commit -m 'fix: resolve issue #123'",
		},
		{
			name:     "multiple single quotes",
			argv:     []string{"echo", "can't won't shouldn't"},
			expected: "echo 'can'\\''t won'\\''t shouldn'\\''t'",
		},
		{
			name:     "only spaces",
			argv:     []string{"echo", "   "},
			expected: "echo '   '",
		},
		{
			name:     "parentheses",
			argv:     []string{"echo", "(hello)"},
			expected: "echo '(hello)'",
		},
		{
			name:     "brackets",
			argv:     []string{"echo", "[hello]"},
			expected: "echo '[hello]'",
		},
		{
			name:     "braces",
			argv:     []string{"echo", "{hello}"},
			expected: "echo '{hello}'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellJoin(tt.argv)
			if got != tt.expected {
				t.Errorf("shellJoin(%q) = %q, want %q", tt.argv, got, tt.expected)
			}
		})
	}
}

func TestIsSafeBare(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Safe cases - only alphanumeric, dash, underscore, dot, slash, colon
		{"simple word", "hello", true},
		{"with numbers", "file123", true},
		{"with dash", "my-file", true},
		{"with underscore", "my_file", true},
		{"with dot", "file.txt", true},
		{"with slash", "/path/to/file", true},
		{"with colon only", "path:to:file", true},
		{"uppercase", "README", true},
		{"mixed alphanumeric", "AaBb123", true},
		{"complex safe", "path/to/file-name_v1.2.txt", true},
		{"just numbers", "12345", true},
		{"just dashes", "---", true},
		{"just underscores", "___", true},
		{"just dots", "...", true},
		{"just slashes", "///", true},
		{"just colons", ":::", true},

		// Unsafe cases - contain special characters
		{"with space", "hello world", false},
		{"with single quote", "it's", false},
		{"with double quote", `say "hi"`, false},
		{"with dollar", "$VAR", false},
		{"with backtick", "`cmd`", false},
		{"with backslash", `path\file`, false},
		{"with newline", "line1\nline2", false},
		{"with tab", "col1\tcol2", false},
		{"with semicolon", "cmd;cmd", false},
		{"with pipe", "a|b", false},
		{"with ampersand", "a&b", false},
		{"with asterisk", "*.txt", false},
		{"with question mark", "file?.txt", false},
		{"with exclamation", "hello!", false},
		{"with at sign", "user@host", false},
		{"with hash", "tag#123", false},
		{"with percent", "50%", false},
		{"with caret", "a^b", false},
		{"with tilde", "~/path", false},
		{"with equals", "key=value", false},
		{"with plus", "a+b", false},
		{"with parenthesis", "(test)", false},
		{"with bracket", "[test]", false},
		{"with brace", "{test}", false},
		{"with less than", "a<b", false},
		{"with greater than", "a>b", false},
		{"with comma", "a,b", false},
		{"empty string", "", true}, // edge case: no unsafe chars
		{"unicode", "hello世界", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSafeBare(tt.input)
			if got != tt.expected {
				t.Errorf("isSafeBare(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSplitRun_AssemblesArgs(t *testing.T) {
	captured := struct {
		name string
		args []string
	}{}
	r := Runner{command: func(name string, args ...string) *exec.Cmd {
		captured.name = name
		captured.args = append([]string(nil), args...)
		// Use a benign command that exits 0
		return exec.Command("true")
	}}
	opts := SplitOpts{Target: ":.", Vertical: true, Percent: 40}
	argv := []string{"/path/to/bin", "-flag", "value with spaces", "and'quote"}
	if err := r.SplitRun(opts, argv); err != nil {
		t.Fatalf("SplitRun error: %v", err)
	}
	if captured.name != "tmux" {
		t.Fatalf("expected tmux, got %q", captured.name)
	}
	wantFlags := map[string]bool{"split-window": true, "-v": true, "-p": true, "40": true, "-t": true, ":.": true}
	for _, a := range captured.args[:len(captured.args)-1] {
		if wantFlags[a] {
			delete(wantFlags, a)
		}
	}
	if len(wantFlags) != 0 {
		t.Fatalf("missing expected flags: %v", wantFlags)
	}
	last := captured.args[len(captured.args)-1]
	if last == "" || last == argv[0] {
		t.Fatalf("expected last arg to be joined command string, got %q", last)
	}
	_ = os.Unsetenv("TMUX")
}

func TestAvailable(t *testing.T) {
	// Present binary + TMUX set -> available
	r := Runner{lookPath: func(file string) (string, error) { return "/bin/tmux", nil }}
	t.Setenv("TMUX", "/tmp/tmux-1,1,1")
	if !r.Available() {
		t.Fatal("expected Available true with TMUX + binary")
	}
	// No binary -> not available
	r.lookPath = func(file string) (string, error) { return "", errors.New("nope") }
	if r.Available() {
		t.Fatal("expected Available false without binary")
	}
}
