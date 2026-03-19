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

func TestAppRunnerRun_TaskDispatchAfterConfigFlag(t *testing.T) {
	var gotConfigPath string
	var gotArgs []string
	runner := appRunner{
		loadConfig: func(path string) appconfig.App {
			gotConfigPath = path
			return appconfig.App{}
		},
		runCLI: func(context.Context, []string, io.Reader, io.Writer, io.Writer) error {
			t.Fatal("runCLI should not be called when task subcommand is handled")
			return nil
		},
		runTaskSubcommand: func(args []string, stdin io.Reader, stdout, stderr io.Writer) (bool, int, error) {
			gotArgs = append([]string(nil), args...)
			return true, 0, nil
		},
	}

	exitCode := runner.run([]string{"--config", "/tmp/hexai.toml", "task", "list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if gotConfigPath != "/tmp/hexai.toml" {
		t.Fatalf("configPath = %q, want /tmp/hexai.toml", gotConfigPath)
	}
	wantArgs := []string{"task", "list"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("task args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestAppRunnerRun_SingleArgumentTaskListFallsThroughToCLI(t *testing.T) {
	var taskArgs []string
	var cliArgs []string
	runner := appRunner{
		loadConfig: func(string) appconfig.App { return appconfig.App{} },
		runCLI: func(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			cliArgs = append([]string(nil), args...)
			return nil
		},
		runTaskSubcommand: func(args []string, stdin io.Reader, stdout, stderr io.Writer) (bool, int, error) {
			taskArgs = append([]string(nil), args...)
			return false, 0, nil
		},
	}

	exitCode := runner.run([]string{"task list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	wantArgs := []string{"task list"}
	if !reflect.DeepEqual(taskArgs, wantArgs) {
		t.Fatalf("task dispatch args = %v, want %v", taskArgs, wantArgs)
	}
	if !reflect.DeepEqual(cliArgs, wantArgs) {
		t.Fatalf("cli args = %v, want %v", cliArgs, wantArgs)
	}
}
