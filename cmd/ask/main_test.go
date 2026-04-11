package main

import (
	"bytes"
	"context"
	"io"
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
