package hexaiaction

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/tmux"
)

// Options configures the command-line orchestration for hexai-tmux-action.
type Options struct {
	Infile          string
	Outfile         string
	UIChild         bool
	TmuxTarget      string
	TmuxPopupWidth  string // popup width, e.g. "80%" or "120"
	TmuxPopupHeight string // popup height, e.g. "80%" or "40"
}

// RunCommand is the CLI orchestrator used by cmd/hexai-tmux-action. It runs in tmux
// split-pane mode by default, or child mode when -ui-child is set.
func RunCommand(ctx context.Context, opts Options, stdin io.Reader, stdout, stderr io.Writer) error {
	return commandRunner{}.RunCommand(ctx, opts, stdin, stdout, stderr)
}

type commandRunner struct {
	popupRun     func(tmux.PopupOpts, []string) error
	osExecutable func() (string, error)
	run          func(context.Context, io.Reader, io.Writer, io.Writer) error
}

func (r commandRunner) popup(opts tmux.PopupOpts, argv []string) error {
	if r.popupRun != nil {
		return r.popupRun(opts, argv)
	}
	return tmux.PopupRun(opts, argv)
}

func (r commandRunner) executable() (string, error) {
	if r.osExecutable != nil {
		return r.osExecutable()
	}
	return os.Executable()
}

func (r commandRunner) runAction(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	if r.run != nil {
		return r.run(ctx, stdin, stdout, stderr)
	}
	return Run(ctx, stdin, stdout, stderr)
}

func (r commandRunner) RunCommand(ctx context.Context, opts Options, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := llm.RegisterAllProviders(); err != nil {
		return fmt.Errorf("failed to register LLM providers: %w", err)
	}
	if opts.UIChild {
		return r.runChild(ctx, opts.Infile, opts.Outfile, stdout, stderr)
	}
	// Always use tmux popup path
	return r.runInTmuxParent(ctx, stdin, stdout, opts.TmuxTarget, opts.TmuxPopupWidth, opts.TmuxPopupHeight)
}

// openIO returns readers/writers for infile/outfile flags with deferred closers.
func openIO(infile, outfile string) (io.Reader, io.Writer, func(), func(), error) {
	in := io.Reader(os.Stdin)
	out := io.Writer(os.Stdout)
	closeIn := func() {}
	closeOut := func() {}
	if path := infile; path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, func() {}, func() {}, fmt.Errorf("hexai-tmux-action: cannot open infile: %w", err)
		}
		in = f
		closeIn = func() { _ = f.Close() }
	}
	if path := outfile; path != "" {
		f, err := os.Create(path)
		if err != nil {
			return nil, nil, func() {}, func() {}, fmt.Errorf("hexai-tmux-action: cannot open outfile: %w", err)
		}
		out = f
		closeOut = func() { _ = f.Close() }
	}
	return in, out, closeIn, closeOut, nil
}

// runChild runs the interactive flow and writes the final output atomically when outfile is set.
func (r commandRunner) runChild(ctx context.Context, infile, outfile string, stdout, stderr io.Writer) error {
	if outfile == "" {
		// No atomic handoff needed; just run normally to provided stdout
		var in io.Reader = os.Stdin
		if infile != "" {
			f, err := os.Open(infile)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			in = f
		}
		return r.runAction(ctx, in, stdout, stderr)
	}
	tmp := outfile + ".tmp"
	in, out, closeIn, closeOut, err := openIO(infile, tmp)
	if err != nil {
		return err
	}
	defer closeIn()
	if err := r.runAction(ctx, in, out, stderr); err != nil {
		closeOut()
		if copyErr := echoThrough(infile, tmp, os.Stdin, stdout); copyErr != nil {
			// Wrap the primary child error with %w so callers can inspect it
			// via errors.Is/As; the echo failure is secondary context and
			// stays as %v (fmt.Errorf supports only a single %w).
			return fmt.Errorf("hexai-tmux-action child: %w; echo failed: %v", err, copyErr)
		}
	} else {
		closeOut()
	}
	return os.Rename(tmp, outfile)
}

func (r commandRunner) runInTmuxParent(ctx context.Context, stdin io.Reader, stdout io.Writer, target, popupWidth, popupHeight string) error {
	dir, err := os.MkdirTemp("", "hexai-tmux-action-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	inPath := filepath.Join(dir, "input.txt")
	outPath := filepath.Join(dir, "reply.txt")
	if err := persistStdin(inPath, stdin); err != nil {
		return err
	}
	exe, err := r.executable()
	if err != nil {
		return err
	}
	argv := []string{exe, "-ui-child", "-infile", inPath, "-outfile", outPath}
	opts := tmux.PopupOpts{Target: target, Width: popupWidth, Height: popupHeight}
	if err := r.popup(opts, argv); err != nil {
		return err
	}
	if err := waitForFile(ctx, outPath, 60*time.Second); err != nil {
		return err
	}
	return catFileTo(stdout, outPath)
}

func persistStdin(path string, stdin io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, stdin); err != nil {
		return err
	}
	return f.Sync()
}

// waitForFile polls for the existence of path until it appears, the deadline
// expires, or ctx is cancelled. Uses a ticker instead of time.Sleep so the
// context is honoured without blocking the full poll interval.
func waitForFile(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("hexai-tmux-action: timeout waiting for reply file")
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func catFileTo(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(w, f)
	return err
}

// echoThrough no longer used in tmux-only flow, but kept for potential reuse.
func echoThrough(infile, outfile string, stdin io.Reader, stdout io.Writer) error {
	in := stdin
	out := stdout
	if infile != "" {
		f, err := os.Open(infile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		in = f
	}
	if outfile != "" {
		f, err := os.Create(outfile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		out = f
	}
	_, err := io.Copy(out, in)
	return err
}
