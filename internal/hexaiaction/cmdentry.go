package hexaiaction

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "time"

    "codeberg.org/snonux/hexai/internal/tmux"
    "golang.org/x/term"
)

// Options configures the command-line orchestration for hexai-tmux-action.
type Options struct {
    Infile       string
    Outfile      string
    UIChild      bool
    TmuxTarget   string
    TmuxSplit    string // "v" or "h"
    TmuxPercent  int    // 1-100
}

// RunCommand is the CLI orchestrator used by cmd/hexai-tmux-action. It runs in tmux
// split-pane mode by default, or child mode when -ui-child is set.
func RunCommand(ctx context.Context, opts Options, stdin io.Reader, stdout, stderr io.Writer) error {
    if opts.UIChild {
        return runChild(ctx, opts.Infile, opts.Outfile, stdout, stderr)
    }
    // Always use tmux path
    return runInTmuxParent(stdin, stdout, opts.TmuxTarget, opts.TmuxSplit, opts.TmuxPercent)
}

// seams for unit tests
var isTTYFn = func(fd uintptr) bool { return term.IsTerminal(int(fd)) }
var splitRunFn = tmux.SplitRun
var osExecutableFn = os.Executable
var runFn = Run

// openIO returns readers/writers for infile/outfile flags with deferred closers.
func openIO(infile, outfile string) (io.Reader, io.Writer, func(), func(), error) {
    in := io.Reader(os.Stdin)
    out := io.Writer(os.Stdout)
    closeIn := func() {}
    closeOut := func() {}
    if path := infile; path != "" {
        f, err := os.Open(path)
        if err != nil { return nil, nil, func(){}, func(){}, fmt.Errorf("hexai-tmux-action: cannot open infile: %w", err) }
        in = f
        closeIn = func() { _ = f.Close() }
    }
    if path := outfile; path != "" {
        f, err := os.Create(path)
        if err != nil { return nil, nil, func(){}, func(){}, fmt.Errorf("hexai-tmux-action: cannot open outfile: %w", err) }
        out = f
        closeOut = func() { _ = f.Close() }
    }
    return in, out, closeIn, closeOut, nil
}

// runChild runs the interactive flow and writes the final output atomically when outfile is set.
func runChild(ctx context.Context, infile, outfile string, stdout, stderr io.Writer) error {
    if outfile == "" {
        // No atomic handoff needed; just run normally to provided stdout
        var in io.Reader = os.Stdin
        if infile != "" {
            f, err := os.Open(infile)
            if err != nil { return err }
            defer func(){ _ = f.Close() }()
            in = f
        }
        return runFn(ctx, in, stdout, stderr)
    }
    tmp := outfile + ".tmp"
    in, out, closeIn, closeOut, err := openIO(infile, tmp)
    if err != nil { return err }
    defer closeIn()
    if err := runFn(ctx, in, out, stderr); err != nil {
        closeOut()
        if copyErr := echoThrough(infile, tmp, os.Stdin, stdout); copyErr != nil {
            return fmt.Errorf("hexai-tmux-action child: %v; echo failed: %v", err, copyErr)
        }
    } else {
        closeOut()
    }
    return os.Rename(tmp, outfile)
}

func runInTmuxParent(stdin io.Reader, stdout io.Writer, target, split string, percent int) error {
    dir, err := os.MkdirTemp("", "hexai-tmux-action-")
    if err != nil { return err }
    defer func() { _ = os.RemoveAll(dir) }()
    inPath := filepath.Join(dir, "input.txt")
    outPath := filepath.Join(dir, "reply.txt")
    if err := persistStdin(inPath, stdin); err != nil { return err }
    exe, err := osExecutableFn()
    if err != nil { return err }
    argv := []string{exe, "-ui-child", "-infile", inPath, "-outfile", outPath}
    opts := tmux.SplitOpts{Target: target, Vertical: split != "h", Percent: percent}
    if err := splitRunFn(opts, argv); err != nil { return err }
    if err := waitForFile(outPath, 60*time.Second); err != nil { return err }
    return catFileTo(stdout, outPath)
}

func persistStdin(path string, stdin io.Reader) error {
    f, err := os.Create(path)
    if err != nil { return err }
    defer func() { _ = f.Close() }()
    if _, err := io.Copy(f, stdin); err != nil { return err }
    return f.Sync()
}

func waitForFile(path string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for {
        if _, err := os.Stat(path); err == nil { return nil }
        if time.Now().After(deadline) { return fmt.Errorf("hexai-tmux-action: timeout waiting for reply file") }
        time.Sleep(200 * time.Millisecond)
    }
}

func catFileTo(w io.Writer, path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer func() { _ = f.Close() }()
    _, err = io.Copy(w, f)
    return err
}

// echoThrough no longer used in tmux-only flow, but kept for potential reuse.
func echoThrough(infile, outfile string, stdin io.Reader, stdout io.Writer) error {
    var in io.Reader = stdin
    var out io.Writer = stdout
    if infile != "" {
        f, err := os.Open(infile)
        if err != nil { return err }
        defer func() { _ = f.Close() }()
        in = f
    }
    if outfile != "" {
        f, err := os.Create(outfile)
        if err != nil { return err }
        defer func() { _ = f.Close() }()
        out = f
    }
    _, err := io.Copy(out, in)
    return err
}
