package askcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeWatchTicker struct {
	ch      <-chan time.Time
	stopped bool
}

func (t *fakeWatchTicker) C() <-chan time.Time {
	return t.ch
}

func (t *fakeWatchTicker) Stop() {
	t.stopped = true
}

func TestHandleWatch_ForwardsNonZeroCode(t *testing.T) {
	ticks := make(chan time.Time)
	ctx := context.Background()
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 1, nil
	}})
	d.newTicker = func(time.Duration) watchTicker { return &fakeWatchTicker{ch: ticks} }

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"watch", "urgency"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("watch code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no output, got %q", stdout.String())
	}
}

func TestHandleWatch_ForwardsInnerError(t *testing.T) {
	ctx := context.Background()
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 1, errors.New("boom")
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"watch", "urgency"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want boom", err)
	}
	if code != 1 {
		t.Fatalf("watch code = %d, want 1", code)
	}
}

func TestHandleWatch_DrawsStderrOnNonZero(t *testing.T) {
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		callCount++
		if len(args) >= 2 && args[0] == "started" && args[1] == "export" {
			if callCount == 1 {
				cancel()
				return 1, nil
			}
		}
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	d.newTicker = func(time.Duration) watchTicker { return &fakeWatchTicker{ch: ticks} }

	var out bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"watch", "info"}, nil, &out, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("watch code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), ansiClearScreen) {
		t.Fatalf("stdout missing clear screen: %q", out.String())
	}
	if !strings.Contains(out.String(), "no started task found") {
		t.Fatalf("stdout missing expected error message: %q", out.String())
	}
}

func TestHandleWatch_DefaultsToListAndRedrawsOnChange(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	ticks := make(chan time.Time, 1)
	ticks <- time.Now()
	fakeTicker := &fakeWatchTicker{ch: ticks}

	ctx, cancel := context.WithCancel(context.Background())
	var calls [][]string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(calls) == 1 {
			_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"M","tags":["agent"],"urgency":3,"depends":[]}]`)
			return 0, nil
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Task 1 updated","status":"pending","priority":"M","tags":["agent"],"urgency":3,"depends":[]}]`)
		cancel()
		return 0, nil
	}})
	d.newTicker = func(interval time.Duration) watchTicker {
		if interval != watchInterval {
			t.Fatalf("watch interval = %s, want %s", interval, watchInterval)
		}
		return fakeTicker
	}

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"watch"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("watch code = %d, want 0: stderr=%s", code, stderr.String())
	}
	wantCalls := [][]string{{"status:pending", "export"}, {"status:pending", "export"}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("runner calls = %#v, want %#v", calls, wantCalls)
	}
	if got := strings.Count(stdout.String(), "\033[2J\033[H"); got != 2 {
		t.Fatalf("clear count = %d, want 2; output=%q", got, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Task 1 updated") {
		t.Fatalf("stdout missing updated task: %q", stdout.String())
	}
	if !fakeTicker.stopped {
		t.Fatal("watch ticker was not stopped")
	}
}

func TestHandleWatch_DoesNotRedrawUnchangedOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	ticks := make(chan time.Time, 1)
	ticks <- time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	runCount := 0
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		runCount++
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"M","tags":["agent"],"urgency":3,"depends":[]}]`)
		if runCount == 2 {
			cancel()
		}
		return 0, nil
	}})
	d.newTicker = func(time.Duration) watchTicker { return &fakeWatchTicker{ch: ticks} }

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"watch", "list"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("watch code = %d, want 0: stderr=%s", code, stderr.String())
	}
	if got := strings.Count(stdout.String(), "\033[2J\033[H"); got != 1 {
		t.Fatalf("clear count = %d, want 1; output=%q", got, stdout.String())
	}
}

func TestHandleWatch_ForwardsSubcommandArgs(t *testing.T) {
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	var gotArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		gotArgs = append([]string(nil), args...)
		cancel()
		_, _ = io.WriteString(stdout, `[]`)
		return 0, nil
	}})
	d.newTicker = func(time.Duration) watchTicker { return &fakeWatchTicker{ch: ticks} }

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"watch", "ready", "limit:2"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("watch code = %d, want 0: stderr=%s", code, stderr.String())
	}
	if want := []string{"+READY", "limit:2", "export"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("runner args = %v, want %v", gotArgs, want)
	}
}

func TestHandleWatch_RejectsUnsafeSubcommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "recursive watch", args: []string{"watch", "watch", "list"}},
		{name: "implicit add typo", args: []string{"watch", "typo"}},
		{name: "write command", args: []string{"watch", "start", "0"}},
		{name: "write dependency command", args: []string{"watch", "dep", "add", "0", "1"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
				t.Fatalf("runner should not be called for unsafe watch command")
				return 0, nil
			}})

			var stdout, stderr bytes.Buffer
			code, err := d.Dispatch(context.Background(), tc.args, nil, &stdout, &stderr)
			if err != nil {
				t.Fatalf("watch returned error: %v", err)
			}
			if code != 1 {
				t.Fatalf("watch code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "read-only subcommands") {
				t.Fatalf("stderr = %q, want read-only subcommand error", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

type mockFdWriter struct {
	io.Writer
	fd uintptr
}

func (m *mockFdWriter) Fd() uintptr { return m.fd }

func TestDeferredWriter_PreservesFd(t *testing.T) {
	w := &mockFdWriter{Writer: &bytes.Buffer{}, fd: 42}
	dw := &deferredWriter{w: w}
	if got := dw.Fd(); got != 42 {
		t.Fatalf("Fd() = %d, want 42", got)
	}
	_, _ = dw.Write([]byte("hello"))
	if !bytes.Equal(dw.buf.Bytes(), []byte("hello")) {
		t.Fatalf("buf = %q, want hello", dw.buf.Bytes())
	}
}

func TestDeferredWriter_FdZeroWhenUnsupported(t *testing.T) {
	dw := &deferredWriter{w: &bytes.Buffer{}}
	if got := dw.Fd(); got != 0 {
		t.Fatalf("Fd() = %d, want 0", got)
	}
}
