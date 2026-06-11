package editor

import (
	"context"
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
// Override in tests to avoid launching a real editor. It uses
// exec.CommandContext so a cancelled ctx (e.g. process shutdown) kills the
// editor subprocess instead of leaving it blocking on terminal input.
var RunEditor = func(ctx context.Context, editor, path string) error {
	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// OpenTempAndEdit creates a temporary .md file, writes initial content if provided,
// opens it in the resolved editor, then reads the final content and removes the file.
// Returns the trimmed content. ctx is forwarded to the editor subprocess so it
// can be cancelled along with the surrounding command.
func OpenTempAndEdit(ctx context.Context, initial []byte) (string, error) {
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
	if err := RunEditor(ctx, ed, path); err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// OpenFile ensures the parent directory exists, then opens path in the editor
// from Resolve() (HEXAI_EDITOR or EDITOR). ctx is forwarded to the editor
// subprocess so it can be cancelled with the surrounding command.
func OpenFile(ctx context.Context, path string) error {
	ed, err := Resolve()
	if err != nil {
		return err
	}
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return errors.New("config path is empty")
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
			return mkErr
		}
	}
	return RunEditor(ctx, ed, path)
}
