package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunTaskSubcommandIfRequested_SkipsNonTaskArgs(t *testing.T) {
	handled, exitCode, err := runTaskSubcommandIfRequested([]string{"hello"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("expected non-task args to be ignored, got handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
}
