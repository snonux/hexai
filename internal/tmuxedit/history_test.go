package tmuxedit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/textutil"
)

func TestAppendHistory(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	text := "test prompt text"
	agent := "claude"
	cwd := "/tmp/test"

	// Append first entry
	if err := AppendHistory(text, agent, cwd); err != nil {
		t.Fatalf("AppendHistory failed: %v", err)
	}

	// Verify file was created
	historyPath := filepath.Join(tmpDir, "state", "tmux-edit-history.jsonl")
	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("history file not created: %v", err)
	}

	// Read and verify content
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("cannot read history file: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Fatal("history file is empty")
	}

	// Verify it contains expected fields
	if !containsString(content, "test prompt text") {
		t.Error("history doesn't contain text")
	}
	if !containsString(content, "claude") {
		t.Error("history doesn't contain agent")
	}
	if !containsString(content, "/tmp/test") {
		t.Error("history doesn't contain cwd")
	}
}

func TestGetHistory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Append multiple entries
	entries := []struct {
		text  string
		agent string
		cwd   string
	}{
		{"first prompt", "claude", "/home/user"},
		{"second prompt", "aider", "/tmp/project"},
		{"third prompt", "claude", "/var/tmp"},
	}

	for _, e := range entries {
		if err := AppendHistory(e.text, e.agent, e.cwd); err != nil {
			t.Fatalf("AppendHistory failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Get all history
	history, err := GetHistory(0)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}

	// Verify first entry
	if history[0].Text != "first prompt" {
		t.Errorf("first entry text: got %q, want %q", history[0].Text, "first prompt")
	}
	if history[0].Agent != "claude" {
		t.Errorf("first entry agent: got %q, want %q", history[0].Agent, "claude")
	}

	// Test limit
	limited, err := GetHistory(2)
	if err != nil {
		t.Fatalf("GetHistory with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("expected 2 entries with limit, got %d", len(limited))
	}

	// Should get the most recent 2
	if limited[0].Text != "second prompt" {
		t.Errorf("limited[0] should be second entry")
	}
	if limited[1].Text != "third prompt" {
		t.Errorf("limited[1] should be third entry")
	}
}

func TestGetHistory_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Get history when file doesn't exist
	history, err := GetHistory(0)
	if err != nil {
		t.Fatalf("GetHistory should not error on missing file: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(history))
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "unix newlines",
			input: "line1\nline2\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "windows newlines",
			input: "line1\r\nline2\r\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "mixed newlines",
			input: "line1\nline2\r\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "trailing newline",
			input: "line1\nline2\n",
			want:  []string{"line1", "line2"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "single line no newline",
			input: "single",
			want:  []string{"single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textutil.SplitLinesBytes([]byte(tt.input))
			gotStr := make([]string, len(got))
			for i, b := range got {
				gotStr[i] = string(b)
			}

			if len(gotStr) != len(tt.want) {
				t.Fatalf("got %d lines, want %d", len(gotStr), len(tt.want))
			}

			for i := range gotStr {
				if gotStr[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, gotStr[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetHistory_MalformedEntries(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Create state directory and write a file with some valid and invalid lines
	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("cannot create state dir: %v", err)
	}
	historyPath := filepath.Join(stateDir, "tmux-edit-history.jsonl")
	content := `{"timestamp":"2025-01-01T00:00:00Z","agent":"claude","cwd":"/tmp","text":"valid"}
not json at all
{"timestamp":"2025-01-02T00:00:00Z","agent":"aider","cwd":"/home","text":"also valid"}
`
	if err := os.WriteFile(historyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("cannot write history file: %v", err)
	}

	entries, err := GetHistory(0)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	// Should skip the malformed line and return the 2 valid entries
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "valid" {
		t.Errorf("entries[0].Text = %q, want 'valid'", entries[0].Text)
	}
	if entries[1].Text != "also valid" {
		t.Errorf("entries[1].Text = %q, want 'also valid'", entries[1].Text)
	}
}

func TestGetHistory_EmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("cannot create state dir: %v", err)
	}
	historyPath := filepath.Join(stateDir, "tmux-edit-history.jsonl")
	// File with empty lines interspersed
	content := "\n" +
		`{"timestamp":"2025-01-01T00:00:00Z","agent":"claude","cwd":"/tmp","text":"entry1"}` + "\n" +
		"\n\n" +
		`{"timestamp":"2025-01-02T00:00:00Z","agent":"aider","cwd":"/home","text":"entry2"}` + "\n"
	if err := os.WriteFile(historyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("cannot write history file: %v", err)
	}

	entries, err := GetHistory(0)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping empty lines), got %d", len(entries))
	}
}

func TestGetHistory_LimitZeroReturnsAll(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	for i := 0; i < 5; i++ {
		if err := AppendHistory(fmt.Sprintf("entry%d", i), "claude", "/tmp"); err != nil {
			t.Fatalf("AppendHistory failed: %v", err)
		}
	}

	entries, err := GetHistory(0)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 entries with limit=0, got %d", len(entries))
	}
}

func TestGetHistory_LimitLargerThanEntries(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	if err := AppendHistory("only one", "claude", "/tmp"); err != nil {
		t.Fatalf("AppendHistory failed: %v", err)
	}

	entries, err := GetHistory(100)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry with large limit, got %d", len(entries))
	}
}

func TestAppendHistory_InvalidStateDir(t *testing.T) {
	// Point XDG_STATE_HOME to a path that can't be created (file, not dir)
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("cannot create blocking file: %v", err)
	}
	// Set state home to a path under the file (impossible to mkdir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(blockingFile, "sub"))

	err := AppendHistory("text", "agent", "/cwd")
	if err == nil {
		t.Fatal("expected error when state directory cannot be created")
	}
}

func TestGetHistory_InvalidStateDir(t *testing.T) {
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("cannot create blocking file: %v", err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(blockingFile, "sub"))

	_, err := GetHistory(0)
	if err == nil {
		t.Fatal("expected error when state directory cannot be created")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
