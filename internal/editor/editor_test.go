package editor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRunEditor_Default exercises the default RunEditor function with a harmless command.
func TestRunEditor_Default(t *testing.T) {
	t.Setenv("HEXAI_EDITOR", "true") // /usr/bin/true — exits 0 immediately
	tmp := filepath.Join(t.TempDir(), "test.md")
	if err := os.WriteFile(tmp, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunEditor("true", tmp); err != nil {
		t.Fatalf("RunEditor with 'true': %v", err)
	}
}

// TestRunEditor_Default_BadCommand verifies RunEditor returns an error for a nonexistent command.
func TestRunEditor_Default_BadCommand(t *testing.T) {
	err := RunEditor("nonexistent-editor-cmd-12345", "/dev/null")
	if err == nil {
		t.Fatal("expected error for nonexistent editor command")
	}
}

func TestResolve_EnvPriority(t *testing.T) {
	t.Setenv("HEXAI_EDITOR", "ed1")
	t.Setenv("EDITOR", "ed2")
	ed, err := Resolve()
	if err != nil || ed != "ed1" {
		t.Fatalf("Resolve failed: %v %q", err, ed)
	}
	t.Setenv("HEXAI_EDITOR", "")
	ed, err = Resolve()
	if err != nil || ed != "ed2" {
		t.Fatalf("Resolve fallback failed: %v %q", err, ed)
	}
}

// TestResolve_NoEditor verifies the error when neither HEXAI_EDITOR nor EDITOR is set.
func TestResolve_NoEditor(t *testing.T) {
	t.Setenv("HEXAI_EDITOR", "")
	t.Setenv("EDITOR", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error when no editor is configured")
	}
}

// TestResolve_WhitespaceOnly verifies that whitespace-only values are treated as empty.
func TestResolve_WhitespaceOnly(t *testing.T) {
	t.Setenv("HEXAI_EDITOR", "   ")
	t.Setenv("EDITOR", "  \t ")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for whitespace-only editor values")
	}
}

func TestOpenTempAndEdit_UsesRunEditor(t *testing.T) {
	old := RunEditor
	t.Cleanup(func() { RunEditor = old })
	// Ensure Resolve() succeeds
	t.Setenv("HEXAI_EDITOR", "dummy")
	var capturedPath string
	RunEditor = func(editor, path string) error {
		capturedPath = path
		// simulate user writing content
		return os.WriteFile(path, []byte("Hello\nWorld\n"), 0o600)
	}
	out, err := OpenTempAndEdit([]byte("# Start\n\n"))
	if err != nil {
		t.Fatalf("OpenTempAndEdit: %v", err)
	}
	if out != "Hello\nWorld" {
		t.Fatalf("unexpected content: %q", out)
	}
	if filepath.Ext(capturedPath) != ".md" {
		t.Fatalf("expected .md suffix: %s", capturedPath)
	}
}

// TestOpenTempAndEdit_NoEditor verifies error propagation when no editor is configured.
func TestOpenTempAndEdit_NoEditor(t *testing.T) {
	t.Setenv("HEXAI_EDITOR", "")
	t.Setenv("EDITOR", "")
	_, err := OpenTempAndEdit(nil)
	if err == nil {
		t.Fatal("expected error when no editor is set")
	}
}

// TestOpenTempAndEdit_NilInitial verifies that nil initial content works (empty file).
func TestOpenTempAndEdit_NilInitial(t *testing.T) {
	old := RunEditor
	t.Cleanup(func() { RunEditor = old })
	t.Setenv("HEXAI_EDITOR", "dummy")
	RunEditor = func(editor, path string) error {
		// simulate user writing content into a file that started empty
		return os.WriteFile(path, []byte("result"), 0o600)
	}
	out, err := OpenTempAndEdit(nil)
	if err != nil {
		t.Fatalf("OpenTempAndEdit with nil initial: %v", err)
	}
	if out != "result" {
		t.Fatalf("unexpected content: %q", out)
	}
}

// TestOpenTempAndEdit_EmptyInitial verifies that empty (zero-length) initial content
// skips the write branch but still works end-to-end.
func TestOpenTempAndEdit_EmptyInitial(t *testing.T) {
	old := RunEditor
	t.Cleanup(func() { RunEditor = old })
	t.Setenv("HEXAI_EDITOR", "dummy")
	RunEditor = func(editor, path string) error {
		return os.WriteFile(path, []byte("  trimmed  "), 0o600)
	}
	out, err := OpenTempAndEdit([]byte{})
	if err != nil {
		t.Fatalf("OpenTempAndEdit with empty initial: %v", err)
	}
	if out != "trimmed" {
		t.Fatalf("expected trimmed content, got %q", out)
	}
}

// TestOpenTempAndEdit_EditorError verifies that an editor failure propagates the error.
func TestOpenTempAndEdit_EditorError(t *testing.T) {
	old := RunEditor
	t.Cleanup(func() { RunEditor = old })
	t.Setenv("HEXAI_EDITOR", "dummy")
	editorErr := errors.New("editor crashed")
	RunEditor = func(editor, path string) error {
		return editorErr
	}
	_, err := OpenTempAndEdit([]byte("some content"))
	if err == nil {
		t.Fatal("expected error when editor fails")
	}
	if !errors.Is(err, editorErr) {
		t.Fatalf("expected editor error, got: %v", err)
	}
}

// TestOpenTempAndEdit_EditorDeletesFile verifies error when the editor removes the temp file.
func TestOpenTempAndEdit_EditorDeletesFile(t *testing.T) {
	old := RunEditor
	t.Cleanup(func() { RunEditor = old })
	t.Setenv("HEXAI_EDITOR", "dummy")
	RunEditor = func(editor, path string) error {
		// simulate the editor deleting the file
		return os.Remove(path)
	}
	_, err := OpenTempAndEdit([]byte("content"))
	if err == nil {
		t.Fatal("expected error when temp file is deleted by editor")
	}
}

// TestOpenTempAndEdit_TempFileCleanup verifies the temp file is removed after success.
func TestOpenTempAndEdit_TempFileCleanup(t *testing.T) {
	old := RunEditor
	t.Cleanup(func() { RunEditor = old })
	t.Setenv("HEXAI_EDITOR", "dummy")
	var capturedPath string
	RunEditor = func(editor, path string) error {
		capturedPath = path
		return os.WriteFile(path, []byte("done"), 0o600)
	}
	_, err := OpenTempAndEdit(nil)
	if err != nil {
		t.Fatalf("OpenTempAndEdit: %v", err)
	}
	// The deferred os.Remove should have cleaned up the temp file
	if _, err := os.Stat(capturedPath); !os.IsNotExist(err) {
		t.Fatalf("temp file was not cleaned up: %s", capturedPath)
	}
}
