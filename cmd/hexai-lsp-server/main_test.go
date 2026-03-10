package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestMain_Version(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"hexai-lsp-server", "-version"}
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	main()
	if buf.Len() == 0 {
		t.Fatalf("expected version log")
	}
}

func TestDefaultLogPathFallsBackToTempDirOnStateDirFailure(t *testing.T) {
	stateHome := filepath.Join(t.TempDir(), "state-home-file")
	if err := os.WriteFile(stateHome, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("write state home marker: %v", err)
	}
	t.Setenv("XDG_STATE_HOME", stateHome)

	got := defaultLogPath()
	want := filepath.Join(os.TempDir(), "hexai-lsp-server.log")
	if got != want {
		t.Fatalf("expected temp-dir fallback %q, got %q", want, got)
	}
}
