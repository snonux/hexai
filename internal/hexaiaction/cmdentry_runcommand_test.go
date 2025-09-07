package hexaiaction

import (
    "bytes"
    "context"
    "io"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "codeberg.org/snonux/hexai/internal/tmux"
)

func TestRunCommand_UIChild(t *testing.T) {
    dir := t.TempDir()
    in := filepath.Join(dir, "in.txt")
    out := filepath.Join(dir, "out.txt")
    _ = os.WriteFile(in, []byte("sel"), 0o600)
    old := runFn
    runFn = func(_ context.Context, _ io.Reader, w io.Writer, _ io.Writer) error { _, _ = io.WriteString(w, "OK"); return nil }
    t.Cleanup(func(){ runFn = old })
    opts := Options{Infile: in, Outfile: out, UIChild: true}
    if err := RunCommand(context.Background(), opts, bytes.NewBuffer(nil), io.Discard, io.Discard); err != nil {
        t.Fatalf("RunCommand UIChild: %v", err)
    }
    b, _ := os.ReadFile(out)
    if string(b) != "OK" { t.Fatalf("outfile: %q", string(b)) }
}

func TestRunCommand_Tmux(t *testing.T) {
    oldTTY := isTTYFn
    oldAvail := tmuxAvailableFn
    oldExec := osExecutableFn
    oldSplit := splitRunFn
    isTTYFn = func(_ uintptr) bool { return false }
    tmuxAvailableFn = func() bool { return true }
    osExecutableFn = func() (string, error) { return "/bin/hexai-action", nil }
    splitRunFn = func(_ tmux.SplitOpts, argv []string) error {
        for i := 0; i < len(argv)-1; i++ {
            if argv[i] == "-outfile" && i+1 < len(argv) {
                _ = os.WriteFile(argv[i+1], []byte("OUT"), 0o600)
                break
            }
        }
        return nil
    }
    defer func(){ isTTYFn = oldTTY; tmuxAvailableFn = oldAvail; osExecutableFn = oldExec; splitRunFn = oldSplit }()
    var out bytes.Buffer
    if err := RunCommand(context.Background(), Options{ForceTmux: true}, bytes.NewBufferString("X"), &out, io.Discard); err != nil {
        t.Fatalf("RunCommand tmux: %v", err)
    }
    if out.String() != "OUT" { t.Fatalf("stdout: %q", out.String()) }
}

// Inline TTY path is exercised implicitly via other helpers; testing it directly
// would require TTY simulation which is brittle in unit tests.

func TestRunCommand_FallbackEcho(t *testing.T) {
    oldTTY := isTTYFn
    oldAvail := tmuxAvailableFn
    isTTYFn = func(_ uintptr) bool { return false }
    tmuxAvailableFn = func() bool { return false }
    defer func(){ isTTYFn = oldTTY; tmuxAvailableFn = oldAvail }()
    var out bytes.Buffer
    if err := RunCommand(context.Background(), Options{NoTmux: true}, bytes.NewBufferString("Z"), &out, io.Discard); err != nil {
        t.Fatalf("RunCommand fallback: %v", err)
    }
    if strings.TrimSpace(out.String()) != "Z" { t.Fatalf("stdout: %q", out.String()) }
}
