package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/askcli"
)

func TestMain_WiresDispatcher(t *testing.T) {
	var gotArgs []string
	d := askcli.NewDispatcher(&spyRunner{
		runFn: func(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			gotArgs = append([]string(nil), args...)
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Test","status":"pending","priority":"H","tags":["cli"],"urgency":15.0,"depends":[]}]`)
			return 0, nil
		},
	})
	code, err := d.Dispatch(context.Background(), []string{"list", "limit:1"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("dispatch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exitCode = %d, want 0", code)
	}
	if len(gotArgs) < 1 || gotArgs[len(gotArgs)-1] != "export" {
		t.Fatalf("args = %v, want [..., export]", gotArgs)
	}
}

func TestMain_ExitsNonZero(t *testing.T) {
	d := askcli.NewDispatcher(&spyRunner{
		runFn: func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error) {
			return 1, nil
		},
	})
	code, _ := d.Dispatch(context.Background(), []string{"list"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if code == 0 {
		t.Fatalf("exitCode = 0, want non-zero")
	}
}

type spyRunner struct {
	runFn func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error)
}

func (s *spyRunner) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return s.runFn(ctx, args, stdin, stdout, stderr)
}

// fakeDispatcher captures Dispatch arguments and returns canned values; used
// to exercise runMain without a real Taskwarrior binary on PATH.
type fakeDispatcher struct {
	gotArgs []string
	code    int
	err     error
}

func (f *fakeDispatcher) Dispatch(_ context.Context, args []string, _ io.Reader, _, _ io.Writer) (int, error) {
	f.gotArgs = append([]string(nil), args...)
	return f.code, f.err
}

// Driving runMain through a fake dispatcher proves the wiring (args
// forwarded, exit code returned) without touching the real ask CLI.
func TestRunMain_DelegatesAndReturnsCode(t *testing.T) {
	old := dispatcherFactory
	t.Cleanup(func() { dispatcherFactory = old })

	fake := &fakeDispatcher{code: 0}
	dispatcherFactory = func() dispatcher { return fake }

	var stdout, stderr bytes.Buffer
	got := runMain([]string{"list", "limit:1"}, nil, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runMain code = %d, want 0", got)
	}
	if len(fake.gotArgs) != 2 || fake.gotArgs[0] != "list" || fake.gotArgs[1] != "limit:1" {
		t.Fatalf("Dispatch args = %v", fake.gotArgs)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty on success, got %q", stderr.String())
	}
}

// The default dispatcherFactory must return a working real dispatcher (this
// is the path main() uses in production); fakes used elsewhere don't cover
// it, so verify it explicitly.
func TestDispatcherFactory_DefaultReturnsRealDispatcher(t *testing.T) {
	d := dispatcherFactory()
	if d == nil {
		t.Fatal("default dispatcherFactory returned nil")
	}
	if _, ok := d.(*askcli.Dispatcher); !ok {
		t.Fatalf("default dispatcherFactory returned %T, want *askcli.Dispatcher", d)
	}
}

// On a dispatcher error, runMain must print the error to stderr AND surface
// the dispatcher's exit code so the shell sees Taskwarrior's own status.
func TestRunMain_PrintsErrorAndPropagatesExitCode(t *testing.T) {
	old := dispatcherFactory
	t.Cleanup(func() { dispatcherFactory = old })

	fake := &fakeDispatcher{code: 7, err: errors.New("dispatch boom")}
	dispatcherFactory = func() dispatcher { return fake }

	var stdout, stderr bytes.Buffer
	got := runMain(nil, nil, &stdout, &stderr)
	if got != 7 {
		t.Fatalf("runMain code = %d, want 7", got)
	}
	if !strings.Contains(stderr.String(), "dispatch boom") {
		t.Fatalf("stderr missing error text: %q", stderr.String())
	}
}
