package editor

import (
	"os"
	"path/filepath"
	"testing"
)

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
