package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleDenotate_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"denotate", "test-uuid", "old annotation"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("denotate code = %d, want 0", code)
	}
	if err != nil {
		t.Fatalf("denotate returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ok") || !strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + uuid", stdout.String())
	}
}

func TestHandleDenotate_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"denotate", "123", "text"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("denotate code = %d, want 1 for numeric ID", code)
	}
}

func TestHandleDenotate_MissingArgs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing args")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"denotate", "uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("denotate code = %d, want 1 for missing args", code)
	}
}

func TestHandleModify_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"modify", "test-uuid", "priority:H"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("modify code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok") || !strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + uuid", stdout.String())
	}
}

func TestHandleModify_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"modify", "123", "priority:H"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("modify code = %d, want 1 for numeric ID", code)
	}
}

func TestHandleAnnotate_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"annotate", "test-uuid", "new note"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("annotate code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok") || !strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + uuid", stdout.String())
	}
}

func TestHandleAnnotate_MissingArgs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing args")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"annotate", "uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("annotate code = %d, want 1 for missing args", code)
	}
}

func TestHandleStart_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start", "test-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("stdout = %q, want ok", stdout.String())
	}
}

func TestHandleStart_MissingUUID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing UUID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("start code = %d, want 1 for missing UUID", code)
	}
}

func TestHandleStop_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"stop", "test-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stop code = %d, want 0", code)
	}
}

func TestHandleDone_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"done", "test-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("done code = %d, want 0", code)
	}
}

func TestHandlePriority_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"priority", "test-uuid", "H"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("priority code = %d, want 0", code)
	}
}

func TestHandlePriority_MissingArgs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing args")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"priority", "uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("priority code = %d, want 1 for missing args", code)
	}
}

func TestHandleTag_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"tag", "test-uuid", "+cli"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("tag code = %d, want 0", code)
	}
}

func TestHandleTag_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"tag", "123", "+cli"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("tag code = %d, want 1 for numeric ID", code)
	}
}

func TestAllWriteHandlers_PassCorrectArgs(t *testing.T) {
	testCases := []struct {
		subcommand string
		args       []string
		wantArgs   []string
	}{
		{"denotate", []string{"denotate", "my-uuid", "text"}, []string{"denotate", "my-uuid", "text"}},
		{"modify", []string{"modify", "my-uuid", "priority:H"}, []string{"modify", "my-uuid", "priority:H"}},
		{"annotate", []string{"annotate", "my-uuid", "note"}, []string{"annotate", "my-uuid", "note"}},
		{"start", []string{"start", "my-uuid"}, []string{"start", "my-uuid"}},
		{"stop", []string{"stop", "my-uuid"}, []string{"stop", "my-uuid"}},
		{"done", []string{"done", "my-uuid"}, []string{"done", "my-uuid"}},
		{"priority", []string{"priority", "my-uuid", "H"}, []string{"priority", "my-uuid", "H"}},
		{"tag", []string{"tag", "my-uuid", "+cli"}, []string{"tag", "my-uuid", "+cli"}},
	}

	for _, tc := range testCases {
		t.Run(tc.subcommand, func(t *testing.T) {
			var capturedArgs []string
			d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
				capturedArgs = args
				return 0, nil
			}})
			var stdout, stderr bytes.Buffer
			d.Dispatch(context.Background(), tc.args, &bytes.Buffer{}, &stdout, &stderr)
			if len(capturedArgs) != len(tc.wantArgs) {
				t.Fatalf("capturedArgs = %v, want %v", capturedArgs, tc.wantArgs)
			}
			for i, want := range tc.wantArgs {
				if capturedArgs[i] != want {
					t.Fatalf("capturedArgs[%d] = %q, want %q", i, capturedArgs[i], want)
				}
			}
		})
	}
}
