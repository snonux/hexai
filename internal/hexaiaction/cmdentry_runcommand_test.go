package hexaiaction

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/snonux/hexai/internal/tmux"
)

func TestRunCommand_UIChild(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	out := filepath.Join(dir, "out.txt")
	_ = os.WriteFile(in, []byte("sel"), 0o600)
	r := commandRunner{run: func(_ context.Context, _ io.Reader, w io.Writer, _ io.Writer) error {
		_, _ = io.WriteString(w, "OK")
		return nil
	}}
	opts := Options{Infile: in, Outfile: out, UIChild: true}
	if err := r.RunCommand(context.Background(), opts, bytes.NewBuffer(nil), io.Discard, io.Discard); err != nil {
		t.Fatalf("RunCommand UIChild: %v", err)
	}
	b, _ := os.ReadFile(out)
	if string(b) != "OK" {
		t.Fatalf("outfile: %q", string(b))
	}
}

func TestRunCommand_Tmux(t *testing.T) {
	r := commandRunner{}
	r.osExecutable = func() (string, error) { return "/bin/hexai-tmux-action", nil }
	r.popupRun = func(_ tmux.PopupOpts, argv []string) error {
		for i := 0; i < len(argv)-1; i++ {
			if argv[i] == "-outfile" && i+1 < len(argv) {
				_ = os.WriteFile(argv[i+1], []byte("OUT"), 0o600)
				break
			}
		}
		return nil
	}
	var out bytes.Buffer
	if err := r.RunCommand(context.Background(), Options{}, bytes.NewBufferString("X"), &out, io.Discard); err != nil {
		t.Fatalf("RunCommand tmux: %v", err)
	}
	if out.String() != "OUT" {
		t.Fatalf("stdout: %q", out.String())
	}
}

// Inline TTY path is exercised implicitly via other helpers; testing it directly
// would require TTY simulation which is brittle in unit tests.

// Fallback echo path removed in tmux-only flow.
