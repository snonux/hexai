package tmuxedit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
			got := splitLines([]byte(tt.input))
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
