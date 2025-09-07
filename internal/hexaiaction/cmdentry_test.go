package hexaiaction

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "codeberg.org/snonux/hexai/internal/tmux"
)

func TestShouldRunInTmux_Preferences(t *testing.T) {
    if shouldRunInTmux(false, true) { t.Fatal("expected false when no-tmux is set") }
    if !shouldRunInTmux(true, false) { t.Fatal("expected true when -tmux is set") }
}

func TestShouldRunInTmux_Auto(t *testing.T) {
    oldIsTTY := isTTYFn
    oldAvail := tmuxAvailableFn
    t.Cleanup(func() { isTTYFn = oldIsTTY; tmuxAvailableFn = oldAvail })
    isTTYFn = func(_ uintptr) bool { return false }
    tmuxAvailableFn = func() bool { return true }
    if !shouldRunInTmux(false, false) { t.Fatal("expected true when not TTY and tmux available") }
    isTTYFn = func(_ uintptr) bool { return true }
    if shouldRunInTmux(false, false) { t.Fatal("expected false when TTY present") }
    isTTYFn = func(_ uintptr) bool { return false }
    tmuxAvailableFn = func() bool { return false }
    if shouldRunInTmux(false, false) { t.Fatal("expected false when tmux unavailable") }
}

func TestPersistStdin_WritesFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "in.txt")
    // Point stdin to content
    src := filepath.Join(dir, "src.txt")
    if err := os.WriteFile(src, []byte("hello world"), 0o600); err != nil { t.Fatalf("write src: %v", err) }
    f, _ := os.Open(src)
    defer f.Close()
    if err := persistStdin(path, f); err != nil { t.Fatalf("persistStdin: %v", err) }
    b, _ := os.ReadFile(path)
    if string(b) != "hello world" { t.Fatalf("unexpected content %q", string(b)) }
}

func TestEchoThrough(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    out := filepath.Join(dir, "out.txt")
    _ = os.WriteFile(in, []byte("hello"), 0o600)
    if err := echoThrough(in, out, os.Stdin, os.Stdout); err != nil { t.Fatalf("echoThrough: %v", err) }
    b, _ := os.ReadFile(out)
    if string(b) != "hello" { t.Fatalf("unexpected: %q", string(b)) }
}

func TestEchoThrough_StdinStdout(t *testing.T) {
    // set stdin
    rIn, wIn, _ := os.Pipe()
    _, _ = wIn.Write([]byte("PIPE"))
    _ = wIn.Close()
    // capture stdout
    r, w, _ := os.Pipe()
    if err := echoThrough("", "", rIn, w); err != nil { t.Fatalf("echoThrough: %v", err) }
    _ = w.Close()
    data, _ := io.ReadAll(r)
    if string(data) != "PIPE" { t.Fatalf("stdout: %q", string(data)) }
}

func TestRunInTmuxParent_Stubbed(t *testing.T) {
    dir := t.TempDir()
    // set stdin content
    r, w, _ := os.Pipe()
    _, _ = w.Write([]byte("input"))
    _ = w.Close()
    // capture stdout
    rout, wout, _ := os.Pipe()
    oldExec := osExecutableFn
    oldSplit := splitRunFn
    osExecutableFn = func() (string, error) { return "/bin/hexai-action", nil }
    splitRunFn = func(opts tmux.SplitOpts, argv []string) error {
        for i := 0; i < len(argv)-1; i++ {
            if argv[i] == "-outfile" && i+1 < len(argv) {
                _ = os.WriteFile(argv[i+1], []byte("OUT:"+strings.Join(argv, ",")), 0o600)
                break
            }
        }
        return nil
    }
    t.Cleanup(func() { osExecutableFn = oldExec; splitRunFn = oldSplit })
    if err := runInTmuxParent(r, wout, "", "v", 33); err != nil { t.Fatalf("runInTmuxParent: %v", err) }
    _ = wout.Close()
    got, _ := io.ReadAll(rout)
    if !strings.HasPrefix(string(got), "OUT:") { t.Fatalf("unexpected stdout: %q", string(got)) }
    _ = dir
}

func TestRunInTmuxParent_ExecutableError(t *testing.T) {
    old := osExecutableFn
    osExecutableFn = func() (string, error) { return "", fmt.Errorf("no exe") }
    t.Cleanup(func() { osExecutableFn = old })
    r, w, _ := os.Pipe(); _, _ = w.Write([]byte("x")); _ = w.Close()
    if err := runInTmuxParent(r, io.Discard, "", "v", 33); err == nil { t.Fatal("expected error from missing executable") }
}

func TestRunInTmuxParent_SplitError(t *testing.T) {
    oldExec := osExecutableFn
    osExecutableFn = func() (string, error) { return "/bin/hexai-action", nil }
    oldSplit := splitRunFn
    splitRunFn = func(_ tmux.SplitOpts, _ []string) error { return fmt.Errorf("split failed") }
    t.Cleanup(func() { osExecutableFn = oldExec; splitRunFn = oldSplit })
    r, w, _ := os.Pipe(); _, _ = w.Write([]byte("x")); _ = w.Close()
    if err := runInTmuxParent(r, io.Discard, "", "v", 33); err == nil { t.Fatal("expected split error") }
}

func TestRunChild_StdoutAndOutfile(t *testing.T) {
    // Outfile mode
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    out := filepath.Join(dir, "out.txt")
    _ = os.WriteFile(in, []byte("sel"), 0o600)
    oldRun := runFn
    runFn = func(_ context.Context, _ io.Reader, w io.Writer, _ io.Writer) error { _, _ = io.WriteString(w, "RESULT"); return nil }
    t.Cleanup(func(){ runFn = oldRun })
    if err := runChild(context.Background(), in, out, io.Discard, io.Discard); err != nil { t.Fatalf("runChild: %v", err) }
    b, _ := os.ReadFile(out)
    if len(b) == 0 { t.Fatalf("expected some output") }
    // Stdout mode
    r, w, _ := os.Pipe()
    if err := runChild(context.Background(), in, "", w, io.Discard); err != nil { t.Fatalf("runChild: %v", err) }
    _ = w.Close(); buf, _ := io.ReadAll(r)
    if len(buf) == 0 { t.Fatalf("expected stdout output") }
}

func TestWaitForFile_Timeout(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "nope")
    if err := waitForFile(p, 10*time.Millisecond); err == nil { t.Fatal("expected timeout error") }
}

func TestOpenIO_InfileOutfile(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "i"); out := filepath.Join(dir, "o")
    _ = os.WriteFile(in, []byte("X"), 0o600)
    r, w, ci, co, err := openIO(in, out)
    if err != nil { t.Fatalf("openIO: %v", err) }
    defer ci(); defer co()
    if _, err := io.Copy(w, r); err != nil { t.Fatalf("copy: %v", err) }
    b, _ := os.ReadFile(out)
    if string(b) != "X" { t.Fatalf("got %q", string(b)) }
}
