package editor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolve returns the editor command from HEXAI_EDITOR or EDITOR.
func Resolve() (string, error) {
	ed := strings.TrimSpace(os.Getenv("HEXAI_EDITOR"))
	if ed == "" {
		ed = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if ed == "" {
		return "", errors.New("no editor configured (set HEXAI_EDITOR or EDITOR)")
	}
	return ed, nil
}

// RunEditor is the seam that invokes the editor on the given file path.
// Override in tests to avoid launching a real editor.
var RunEditor = func(editor, path string) error {
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// OpenTempAndEdit creates a temporary .md file, writes initial content if provided,
// opens it in the resolved editor, then reads the final content and removes the file.
// Returns the trimmed content.
func OpenTempAndEdit(initial []byte) (string, error) {
	ed, err := Resolve()
	if err != nil {
		return "", err
	}
	// Create temp file under system temp dir; ensure .md suffix
	dir := os.TempDir()
	f, err := os.CreateTemp(dir, "hexai-*.md")
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer func() { _ = os.Remove(path) }()
	if len(initial) > 0 {
		if _, err := f.Write(initial); err != nil {
			_ = f.Close()
			return "", err
		}
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := RunEditor(ed, path); err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
