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

type Runner struct {
	runEditor func(context.Context, string, string) error
}

func (r Runner) edit(ctx context.Context, editor, path string) error {
	if r.runEditor != nil {
		return r.runEditor(ctx, editor, path)
	}
	return runEditor(ctx, editor, path)
}

// RunEditor invokes the editor on the given file path. It uses
// exec.CommandContext so a cancelled ctx kills the editor subprocess instead
// of leaving it blocking on terminal input.
func RunEditor(ctx context.Context, editor, path string) error {
	return runEditor(ctx, editor, path)
}

func runEditor(ctx context.Context, editor, path string) error {
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
	return Runner{}.OpenTempAndEdit(ctx, initial)
}

func (r Runner) OpenTempAndEdit(ctx context.Context, initial []byte) (string, error) {
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
	if err := r.edit(ctx, ed, path); err != nil {
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
	return Runner{}.OpenFile(ctx, path)
}

func (r Runner) OpenFile(ctx context.Context, path string) error {
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
	return r.edit(ctx, ed, path)
}
