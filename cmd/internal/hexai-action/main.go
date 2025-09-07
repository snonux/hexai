package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/snonux/hexai/internal/hexaiaction"
	"codeberg.org/snonux/hexai/internal/tmux"
	"golang.org/x/term"
)

func main() {
	infile := flag.String("infile", "", "Read input from this file instead of stdin")
	outfile := flag.String("outfile", "", "Write output to this file instead of stdout")
	// Tmux/UI flags
	forceTmux := flag.Bool("tmux", false, "Force running the UI in a tmux split-pane (auto if not set)")
	noTmux := flag.Bool("no-tmux", false, "Disable tmux mode even if available")
	uiChild := flag.Bool("ui-child", false, "INTERNAL: run interactive UI and write to -outfile atomically")
	tmuxTarget := flag.String("tmux-target", "", "tmux split target (advanced)")
	tmuxSplit := flag.String("tmux-split", "v", "tmux split orientation: v or h")
	tmuxPercent := flag.Int("tmux-percent", 33, "tmux split size percentage (1-100)")
	flag.Parse()

	// Child mode: run TUI and write atomically to -outfile
	if *uiChild {
		if err := runChild(*infile, *outfile); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Parent mode: decide inline vs tmux
	if shouldRunInTmux(*forceTmux, *noTmux) {
		if err := runInTmuxParent(*tmuxTarget, *tmuxSplit, *tmuxPercent); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Inline path: only if we have a TTY for UI; otherwise echo input
	if isTTY(os.Stdout.Fd()) && isTTY(os.Stdin.Fd()) {
		in, out, closeIn, closeOut, err := openIO(*infile, *outfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer closeIn()
		defer closeOut()
        if err := hexaiactionRun(context.Background(), in, out, os.Stderr); err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }
        return
    }

	// Fallback: no TTY and tmux not available; echo input to output
	if err := echoThrough(*infile, *outfile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
			return nil, nil, func() {}, func() {}, fmt.Errorf("hexai-action: cannot open infile: %w", err)
		}
		in = f
		closeIn = func() { _ = f.Close() }
	}
	if path := outfile; path != "" {
		f, err := os.Create(path)
		if err != nil {
			return nil, nil, func() {}, func() {}, fmt.Errorf("hexai-action: cannot open outfile: %w", err)
		}
		out = f
		closeOut = func() { _ = f.Close() }
	}
	return in, out, closeIn, closeOut, nil
}

// runChild runs the interactive flow and writes the final output atomically to outfile.
var hexaiactionRun = hexaiaction.Run

func runChild(infile, outfile string) error {
	if outfile == "" {
		// No atomic handoff needed; just run normally to stdout
		in, out, closeIn, closeOut, err := openIO(infile, "")
		if err != nil {
			return err
		}
		defer closeIn()
		defer closeOut()
        return hexaiactionRun(context.Background(), in, out, os.Stderr)
	}
	tmp := outfile + ".tmp"
	in, out, closeIn, closeOut, err := openIO(infile, tmp)
	if err != nil {
		return err
	}
	defer closeIn()
    if err := hexaiactionRun(context.Background(), in, out, os.Stderr); err != nil {
        // On error, try to echo input to tmp to avoid blocking
        closeOut()
        if copyErr := echoThrough(infile, tmp); copyErr != nil {
            return fmt.Errorf("hexai-action child: %v; echo failed: %v", err, copyErr)
        }
	} else {
		closeOut()
	}
	return os.Rename(tmp, outfile)
}

var isTTYFn = isTTY
var tmuxAvailableFn = tmux.Available
var splitRunFn = tmux.SplitRun
var osExecutableFn = os.Executable

func shouldRunInTmux(forceTmux, noTmux bool) bool {
	if noTmux {
		return false
	}
	if forceTmux {
		return true
	}
	// Auto: prefer tmux when stdio are not TTYs (Helix :pipe scenario)
    if !(isTTYFn(os.Stdin.Fd()) && isTTYFn(os.Stdout.Fd())) && tmuxAvailableFn() {
        return true
    }
	return false
}

func isTTY(fd uintptr) bool { return term.IsTerminal(int(fd)) }

func runInTmuxParent(target, split string, percent int) error {
	// Prepare temp files
	dir, err := os.MkdirTemp("", "hexai-action-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	inPath := filepath.Join(dir, "input.txt")
	outPath := filepath.Join(dir, "reply.txt")
	// Read stdin and persist to inPath
	if err := persistStdin(inPath); err != nil {
		return err
	}
	// Build child argv
	exe, err := osExecutableFn()
	if err != nil {
		return err
	}
	argv := []string{exe, "-ui-child", "-infile", inPath, "-outfile", outPath}
	// Spawn tmux split
	opts := tmux.SplitOpts{Target: target, Vertical: split != "h", Percent: percent}
	if err := splitRunFn(opts, argv); err != nil {
		return err
	}
	// Wait for outfile to appear
	if err := waitForFile(outPath, 60*time.Second); err != nil {
		return err
	}
	// Print to stdout
	return catFileToStdout(outPath)
}

func persistStdin(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, os.Stdin); err != nil {
		return err
	}
	return f.Sync()
}

func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("hexai-action: timeout waiting for reply file")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func catFileToStdout(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(os.Stdout, f)
	return err
}

func echoThrough(infile, outfile string) error {
	// Read from infile or stdin and write to outfile or stdout
	var in io.Reader = os.Stdin
	var out io.Writer = os.Stdout
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
