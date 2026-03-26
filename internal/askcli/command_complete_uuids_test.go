package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleCompleteUUIDs_PrintsPendingUUIDs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		want := []string{"status:pending", "export"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("args = %v, want %v", args, want)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1"},{"uuid":"uuid-2"},{"uuid":""}]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 0", code)
	}
	if got := stdout.String(); got != "uuid-1\nuuid-2\n" {
		t.Fatalf("stdout = %q, want UUID list", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestHandleCompleteUUIDs_ParseError(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, `not-json`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "failed to parse task data") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
}
