package tmux

import (
    "errors"
    "os"
    "os/exec"
    "testing"
)

func TestInSession(t *testing.T) {
    t.Setenv("TMUX", "/tmp/tmux-123,123,0")
    if !InSession() {
        t.Fatal("expected InSession true when TMUX is set")
    }
    t.Setenv("TMUX", "")
    if InSession() {
        t.Fatal("expected InSession false when TMUX is empty")
    }
}

func TestHasBinary_UsesLookPath(t *testing.T) {
    old := lookPath
    t.Cleanup(func() { lookPath = old })
    lookPath = func(file string) (string, error) { return "/bin/tmux", nil }
    if !HasBinary() {
        t.Fatal("expected HasBinary true when lookPath succeeds")
    }
    lookPath = func(file string) (string, error) { return "", errors.New("nope") }
    if HasBinary() {
        t.Fatal("expected HasBinary false when lookPath fails")
    }
}

func TestSplitRun_AssemblesArgs(t *testing.T) {
    captured := struct{ name string; args []string }{}
    oldCmd := command
    t.Cleanup(func() { command = oldCmd })
    command = func(name string, args ...string) *exec.Cmd {
        captured.name = name
        captured.args = append([]string(nil), args...)
        // Use a benign command that exits 0
        return exec.Command("true")
    }
    opts := SplitOpts{Target: ":.", Vertical: true, Percent: 40}
    argv := []string{"/path/to/bin", "-flag", "value with spaces", "and'quote"}
    if err := SplitRun(opts, argv); err != nil {
        t.Fatalf("SplitRun error: %v", err)
    }
    if captured.name != "tmux" {
        t.Fatalf("expected tmux, got %q", captured.name)
    }
    wantFlags := map[string]bool{"split-window": true, "-v": true, "-p": true, "40": true, "-t": true, ":.": true}
    for _, a := range captured.args[:len(captured.args)-1] {
        if wantFlags[a] {
            delete(wantFlags, a)
        }
    }
    if len(wantFlags) != 0 {
        t.Fatalf("missing expected flags: %v", wantFlags)
    }
    last := captured.args[len(captured.args)-1]
    if last == "" || last == argv[0] {
        t.Fatalf("expected last arg to be joined command string, got %q", last)
    }
    _ = os.Unsetenv("TMUX")
}

func TestAvailable(t *testing.T) {
    oldLook := lookPath
    t.Cleanup(func() { lookPath = oldLook })
    // Present binary + TMUX set -> available
    lookPath = func(file string) (string, error) { return "/bin/tmux", nil }
    t.Setenv("TMUX", "/tmp/tmux-1,1,1")
    if !Available() {
        t.Fatal("expected Available true with TMUX + binary")
    }
    // No binary -> not available
    lookPath = func(file string) (string, error) { return "", errors.New("nope") }
    if Available() {
        t.Fatal("expected Available false without binary")
    }
}
