package main

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "codeberg.org/snonux/hexai/internal/tmux"
)

func TestShouldRunInTmux_Preferences(t *testing.T) {
    // no-tmux overrides
    if shouldRunInTmux(false, true) {
        t.Fatal("expected false when no-tmux is set")
    }
    // force tmux overrides
    if !shouldRunInTmux(true, false) {
        t.Fatal("expected true when -tmux is set")
    }
}

func TestShouldRunInTmux_Auto(t *testing.T) {
    oldIsTTY := isTTYFn
    oldAvail := tmuxAvailableFn
    t.Cleanup(func() { isTTYFn = oldIsTTY; tmuxAvailableFn = oldAvail })
    // Simulate Helix :pipe (no TTY) and tmux available
    isTTYFn = func(_ uintptr) bool { return false }
    tmuxAvailableFn = func() bool { return true }
    if !shouldRunInTmux(false, false) {
        t.Fatal("expected true when not TTY and tmux available")
    }
    // Simulate TTY present: prefer inline
    isTTYFn = func(_ uintptr) bool { return true }
    if shouldRunInTmux(false, false) {
        t.Fatal("expected false when TTY present")
    }
    // Simulate tmux not available
    isTTYFn = func(_ uintptr) bool { return false }
    tmuxAvailableFn = func() bool { return false }
    if shouldRunInTmux(false, false) {
        t.Fatal("expected false when tmux unavailable")
    }
}

func TestPersistStdin_WritesFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "in.txt")
    // Point os.Stdin to a temp file with content
    src := filepath.Join(dir, "src.txt")
    if err := os.WriteFile(src, []byte("hello world"), 0o600); err != nil {
        t.Fatalf("write src: %v", err)
    }
    f, err := os.Open(src)
    if err != nil { t.Fatalf("open src: %v", err) }
    old := os.Stdin
    os.Stdin = f
    t.Cleanup(func() { os.Stdin = old; _ = f.Close() })
    if err := persistStdin(path); err != nil {
        t.Fatalf("persistStdin error: %v", err)
    }
    b, err := os.ReadFile(path)
    if err != nil { t.Fatalf("read out: %v", err) }
    if string(b) != "hello world" {
        t.Fatalf("unexpected content %q", string(b))
    }
}

func TestEchoThrough(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    out := filepath.Join(dir, "out.txt")
    if err := os.WriteFile(in, []byte("hello"), 0o600); err != nil {
        t.Fatalf("write in: %v", err)
    }
    if err := echoThrough(in, out); err != nil {
        t.Fatalf("echoThrough: %v", err)
    }
    b, _ := os.ReadFile(out)
    if string(b) != "hello" {
        t.Fatalf("unexpected: %q", string(b))
    }
}

func TestEchoThrough_StdinStdout(t *testing.T) {
    // set stdin
    rIn, wIn, _ := os.Pipe()
    _, _ = wIn.Write([]byte("PIPE"))
    _ = wIn.Close()
    oldIn := os.Stdin
    os.Stdin = rIn
    defer func() { os.Stdin = oldIn; _ = rIn.Close() }()
    // capture stdout
    r, w, _ := os.Pipe()
    oldOut := os.Stdout
    os.Stdout = w
    defer func() { os.Stdout = oldOut; _ = r.Close(); _ = w.Close() }()
    if err := echoThrough("", ""); err != nil { t.Fatalf("echoThrough: %v", err) }
    _ = w.Close()
    data, _ := io.ReadAll(r)
    if string(data) != "PIPE" {
        t.Fatalf("stdout: %q", string(data))
    }
}

func TestWaitForFile(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "x")
    go func() {
        // create shortly after
        f, _ := os.Create(p)
        defer f.Close()
        f.WriteString("ok")
    }()
    if err := waitForFile(p, 2_000_000_000); err != nil { // 2s
        t.Fatalf("waitForFile: %v", err)
    }
}

func TestCatFileToStdout(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "f")
    if err := os.WriteFile(p, []byte("abc"), 0o600); err != nil {
        t.Fatalf("write: %v", err)
    }
    // capture stdout
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    defer func() { os.Stdout = old; _ = r.Close(); _ = w.Close() }()
    if err := catFileToStdout(p); err != nil {
        t.Fatalf("catFileToStdout: %v", err)
    }
    _ = w.Close()
    buf, _ := io.ReadAll(r)
    if string(buf) != "abc" {
        t.Fatalf("stdout = %q", string(buf))
    }
}

func TestRunInTmuxParent_Stubbed(t *testing.T) {
    dir := t.TempDir()
    // set stdin content
    src := filepath.Join(dir, "stdin.txt")
    _ = os.WriteFile(src, []byte("input"), 0o600)
    f, _ := os.Open(src)
    oldStdin := os.Stdin
    os.Stdin = f
    defer func() { os.Stdin = oldStdin; _ = f.Close() }()

    // capture stdout
    oldStdout := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    defer func() { os.Stdout = oldStdout; _ = r.Close(); _ = w.Close() }()

    // stub seams
    oldExec := osExecutableFn
    oldSplit := splitRunFn
    oldRun := hexaiactionRun
    osExecutableFn = func() (string, error) { return "/bin/hexai-action", nil }
    splitRunFn = func(opts tmux.SplitOpts, argv []string) error {
        // find -outfile path and write content to simulate child
        for i := 0; i < len(argv)-1; i++ {
            if argv[i] == "-outfile" && i+1 < len(argv) {
                _ = os.WriteFile(argv[i+1], []byte("OUT:"+strings.Join(argv, ",")), 0o600)
                break
            }
        }
        return nil
    }
    // Ensure child mode won't try to run the real TUI if invoked in tests.
    hexaiactionRun = func(_ context.Context, _ io.Reader, w io.Writer, _ io.Writer) error {
        _, _ = io.WriteString(w, "child-stub")
        return nil
    }
    defer func() { osExecutableFn = oldExec; splitRunFn = oldSplit; hexaiactionRun = oldRun }()

    if err := runInTmuxParent("", "v", 33); err != nil {
        t.Fatalf("runInTmuxParent: %v", err)
    }
    _ = w.Close()
    got, _ := io.ReadAll(r)
    if !strings.HasPrefix(string(got), "OUT:") {
        t.Fatalf("unexpected stdout: %q", string(got))
    }
}

func TestRunChild_StubbedOutfile(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    out := filepath.Join(dir, "out.txt")
    _ = os.WriteFile(in, []byte("sel"), 0o600)
    old := hexaiactionRun
    hexaiactionRun = func(_ context.Context, _ io.Reader, w io.Writer, _ io.Writer) error {
        _, _ = io.WriteString(w, "RESULT")
        return nil
    }
    defer func() { hexaiactionRun = old }()
    if err := runChild(in, out); err != nil {
        t.Fatalf("runChild: %v", err)
    }
    b, _ := os.ReadFile(out)
    if string(b) != "RESULT" {
        t.Fatalf("unexpected outfile: %q", string(b))
    }
}

func TestRunChild_StubbedStdout(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    _ = os.WriteFile(in, []byte("sel"), 0o600)
    // capture stdout
    oldStdout := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    defer func() { os.Stdout = oldStdout; _ = r.Close(); _ = w.Close() }()
    old := hexaiactionRun
    hexaiactionRun = func(_ context.Context, _ io.Reader, w io.Writer, _ io.Writer) error {
        _, _ = io.WriteString(w, "STDOUT-RESULT")
        return nil
    }
    defer func() { hexaiactionRun = old }()
    if err := runChild(in, ""); err != nil {
        t.Fatalf("runChild: %v", err)
    }
    _ = w.Close()
    data, _ := io.ReadAll(r)
    if string(data) != "STDOUT-RESULT" {
        t.Fatalf("stdout: %q", string(data))
    }
}

func TestRunChild_ErrorFallback(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    out := filepath.Join(dir, "out.txt")
    _ = os.WriteFile(in, []byte("INPUT"), 0o600)
    old := hexaiactionRun
    hexaiactionRun = func(_ context.Context, _ io.Reader, _ io.Writer, _ io.Writer) error {
        return fmt.Errorf("boom")
    }
    defer func() { hexaiactionRun = old }()
    if err := runChild(in, out); err != nil {
        t.Fatalf("runChild: %v", err)
    }
    b, _ := os.ReadFile(out)
    if string(b) != "INPUT" {
        t.Fatalf("expected fallback echo, got %q", string(b))
    }
}

func TestWaitForFile_Timeout(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "nope")
    if err := waitForFile(p, 10_000_000); err == nil { // 10ms
        t.Fatal("expected timeout error")
    }
}

func TestOpenIO_InfileOutfile(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "i")
    out := filepath.Join(dir, "o")
    _ = os.WriteFile(in, []byte("X"), 0o600)
    r, w, ci, co, err := openIO(in, out)
    if err != nil { t.Fatalf("openIO: %v", err) }
    defer ci(); defer co()
    if _, err := io.Copy(w, r); err != nil { t.Fatalf("copy: %v", err) }
    b, _ := os.ReadFile(out)
    if string(b) != "X" { t.Fatalf("got %q", string(b)) }
}

func TestRunInTmuxParent_ExecutableError(t *testing.T) {
    old := osExecutableFn
    osExecutableFn = func() (string, error) { return "", fmt.Errorf("no exe") }
    defer func() { osExecutableFn = old }()
    // set stdin content
    r, w, _ := os.Pipe()
    _, _ = w.Write([]byte("x"))
    _ = w.Close()
    oldIn := os.Stdin
    os.Stdin = r
    defer func() { os.Stdin = oldIn; _ = r.Close() }()
    if err := runInTmuxParent("", "v", 33); err == nil {
        t.Fatal("expected error from missing executable")
    }
}

func TestRunInTmuxParent_SplitError(t *testing.T) {
    oldExec := osExecutableFn
    osExecutableFn = func() (string, error) { return "/bin/hexai-action", nil }
    oldSplit := splitRunFn
    splitRunFn = func(_ tmux.SplitOpts, _ []string) error { return fmt.Errorf("split failed") }
    defer func() { osExecutableFn = oldExec; splitRunFn = oldSplit }()
    // set stdin
    r, w, _ := os.Pipe()
    _, _ = w.Write([]byte("x"))
    _ = w.Close()
    oldIn := os.Stdin
    os.Stdin = r
    defer func() { os.Stdin = oldIn; _ = r.Close() }()
    if err := runInTmuxParent("", "v", 33); err == nil {
        t.Fatal("expected split error")
    }
}

func TestEchoThrough_OutfileError(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "i.txt")
    _ = os.WriteFile(in, []byte("x"), 0o600)
    // Outfile inside non-existent subdir -> Create should fail
    out := filepath.Join(dir, "nope", "out.txt")
    if err := echoThrough(in, out); err == nil {
        t.Fatal("expected echoThrough outfile error")
    }
}

func TestPersistStdin_Error(t *testing.T) {
    // Parent directory missing -> Create should fail
    dir := t.TempDir()
    p := filepath.Join(dir, "missing", "x.txt")
    // set stdin to something
    r, w, _ := os.Pipe()
    _, _ = w.Write([]byte("x"))
    _ = w.Close()
    old := os.Stdin
    os.Stdin = r
    defer func() { os.Stdin = old; _ = r.Close() }()
    if err := persistStdin(p); err == nil {
        t.Fatal("expected persistStdin error")
    }
}

func TestCatFileToStdout_Error(t *testing.T) {
    if err := catFileToStdout("/nonexistent/path/file.txt"); err == nil {
        t.Fatal("expected error for missing file")
    }
}

func TestOpenIO_Errors(t *testing.T) {
    // Non-existent infile
    if _, _, _, _, err := openIO("/definitely/missing/file.txt", ""); err == nil {
        t.Fatal("expected infile error")
    }
    // Outfile in missing dir
    dir := t.TempDir()
    out := filepath.Join(dir, "nope", "x.txt")
    if _, _, _, _, err := openIO("", out); err == nil {
        t.Fatal("expected outfile error")
    }
}
