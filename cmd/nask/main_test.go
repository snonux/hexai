package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func setupTaskFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Taskwarrior fixture requires Bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Taskwarrior fixture requires Bash")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	script := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$NASK_TEST_ARGS"
printf '%s' "${NASK_TEST_OUTPUT:-[]}"
exit "${NASK_TEST_EXIT:-0}"
`
	if err := os.WriteFile(filepath.Join(dir, "task"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("NASK_TEST_ARGS", argsPath)
	t.Setenv("NASK_TEST_OUTPUT", "[]")
	t.Setenv("NASK_TEST_EXIT", "0")
	return argsPath
}

func fixtureTaskArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestRunMainListsNonAgentTasks(t *testing.T) {
	argsPath := setupTaskFixture(t)
	t.Setenv("NASK_TEST_OUTPUT", `[{"uuid":"personal-task","description":"Read the guide","status":"pending","tags":[]}]`)
	for _, args := range [][]string{nil, {"list"}, {"proj:personal", "--json", "list"}} {
		var stdout, stderr bytes.Buffer
		if code := runMain(args, nil, &stdout, &stderr); code != 0 {
			t.Fatalf("runMain(%v) = %d: %s", args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Read the guide") {
			t.Fatalf("missing task in output: %s", stdout.String())
		}
		taskArgs := fixtureTaskArgs(t, argsPath)
		if !slices.Contains(taskArgs, "-agent") || slices.Contains(taskArgs, "+agent") {
			t.Fatalf("wrong task scope: %v", taskArgs)
		}
		if len(args) > 0 && args[0] == "proj:personal" {
			if !slices.Contains(taskArgs, "(project.is:personal or project:personal.)") {
				t.Fatalf("missing project override: %v", taskArgs)
			}
			var tasks []struct{ Description string }
			if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil || len(tasks) != 1 || tasks[0].Description != "Read the guide" {
				t.Fatalf("missing JSON output: %s", stdout.String())
			}
		}
	}
}

func TestRunMainAddsWithoutAgentTag(t *testing.T) {
	argsPath := setupTaskFixture(t)
	uuid := "12345678-1234-1234-1234-123456789abc"
	t.Setenv("NASK_TEST_OUTPUT", "Created task "+uuid+".")
	var stdout, stderr bytes.Buffer
	code := runMain([]string{"proj:personal", "add", "+reading", "Read the guide"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add = %d: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "created task ") {
		t.Fatalf("missing created-task alias: %s", stdout.String())
	}
	args := fixtureTaskArgs(t, argsPath)
	for _, want := range []string{"project:personal", "add", "+reading", "Read the guide"} {
		if !slices.Contains(args, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
	if slices.Contains(args, "+agent") || slices.Contains(args, "-agent") {
		t.Fatalf("unexpected agent modifier on add: %v", args)
	}
}

func TestRunMainPropagatesTaskFailure(t *testing.T) {
	setupTaskFixture(t)
	t.Setenv("NASK_TEST_EXIT", "7")
	if code := runMain([]string{"list"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
}

func TestRunMainRejectsInvalidProject(t *testing.T) {
	argsPath := setupTaskFixture(t)
	var stderr bytes.Buffer
	code := runMain([]string{"proj:", "list"}, nil, &bytes.Buffer{}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "requires a project name") {
		t.Fatalf("invalid project = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(argsPath); !os.IsNotExist(err) {
		t.Fatalf("Taskwarrior ran for an invalid project: %v", err)
	}
}

func TestRunMainHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		if code := runMain(args, nil, &stdout, &stderr); code != 0 {
			t.Fatalf("help = %d: %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Standalone alias for ask na") {
			t.Fatalf("help missing nask usage: %s", stdout.String())
		}
	}
}
