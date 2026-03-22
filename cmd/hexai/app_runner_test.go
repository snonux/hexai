package main

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestAppRunnerRun_ConfigFlagPassedToLoader(t *testing.T) {
	var gotConfigPath string
	var gotArgs []string
	runner := appRunner{
		loadConfig: func(path string) appconfig.App {
			gotConfigPath = path
			return appconfig.App{}
		},
		runCLI: func(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	exitCode := runner.run([]string{"--config", "/tmp/hexai.toml", "hello"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if gotConfigPath != "/tmp/hexai.toml" {
		t.Fatalf("configPath = %q, want /tmp/hexai.toml", gotConfigPath)
	}
	wantArgs := []string{"hello"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("cli args = %v, want %v", gotArgs, wantArgs)
	}
}
