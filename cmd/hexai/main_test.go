package main

import (
	"io"
	"os"
	"testing"
)

func TestMain_Version(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"hexai", "-version"}
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	main()
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe: %v", err)
	}
	b, _ := io.ReadAll(r)
	if len(b) == 0 {
		t.Fatalf("expected version output")
	}
}
