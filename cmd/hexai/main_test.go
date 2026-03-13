package main

import (
	"io"
	"os"
	"strings"
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

func TestMain_TPSSimulation(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"hexai", "--tps-simulation=1000000", "simulated", "output"}
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	main()
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe: %v", err)
	}
	b, _ := io.ReadAll(r)
	if !strings.Contains(string(b), "simulated output") {
		t.Fatalf("expected simulation output, got %q", string(b))
	}
}
