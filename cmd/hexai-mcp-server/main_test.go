package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/hexaimcp"
)

// The deprecation banner is unconditional: it must reach stderr on every
// invocation so users notice this binary is unmaintained.
func TestDeprecationWarning_Content(t *testing.T) {
	for _, want := range []string{"DEPRECATION NOTICE", "EXPERIMENTAL", "NOT ACTIVELY MAINTAINED"} {
		if !strings.Contains(deprecationWarning, want) {
			t.Errorf("expected %q in deprecationWarning", want)
		}
	}
}

func TestDefaultLogPath(t *testing.T) {
	path, err := defaultLogPath()
	if err != nil {
		t.Fatalf("defaultLogPath returned error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty log path")
	}
	if !strings.HasSuffix(path, "hexai-mcp-server.log") {
		t.Errorf("expected path to end with hexai-mcp-server.log, got %q", path)
	}
}

func TestRun_ShowVersion(t *testing.T) {
	var stdout bytes.Buffer
	opts := mcpOptions{showVersion: true}
	if err := run(opts, nil, &stdout, nil); err != nil {
		t.Fatalf("run --version: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != internal.Version {
		t.Fatalf("expected version %q, got %q", internal.Version, got)
	}
}

func TestBuildOverrides(t *testing.T) {
	opts := mcpOptions{
		promptsDir:       "/tmp/test-prompts",
		slashCommandSync: true,
		slashCommandDir:  "/tmp/test-cmds",
	}
	overrides := buildOverrides(opts)
	if overrides.PromptsDir != "/tmp/test-prompts" {
		t.Fatalf("expected PromptsDir=/tmp/test-prompts, got %q", overrides.PromptsDir)
	}
	if !overrides.SlashCommandSync {
		t.Fatal("expected SlashCommandSync=true")
	}
	if overrides.SlashCommandDir != "/tmp/test-cmds" {
		t.Fatalf("expected SlashCommandDir=/tmp/test-cmds, got %q", overrides.SlashCommandDir)
	}
}

func TestRun_SyncAll(t *testing.T) {
	old := runBackfill
	t.Cleanup(func() { runBackfill = old })

	var gotLog, gotConfig string
	var gotOverrides hexaimcp.MCPOverrides
	runBackfill = func(logPath, configPath string, overrides hexaimcp.MCPOverrides) error {
		gotLog = logPath
		gotConfig = configPath
		gotOverrides = overrides
		return nil
	}

	opts := mcpOptions{
		syncAll:          true,
		logPath:          "/tmp/test.log",
		configPath:       "/tmp/cfg.toml",
		promptsDir:       "/tmp/prompts",
		slashCommandSync: true,
		slashCommandDir:  "/tmp/cmds",
	}
	if err := run(opts, nil, nil, nil); err != nil {
		t.Fatalf("run syncAll: %v", err)
	}
	if gotLog != "/tmp/test.log" {
		t.Fatalf("expected logPath=/tmp/test.log, got %q", gotLog)
	}
	if gotConfig != "/tmp/cfg.toml" {
		t.Fatalf("expected configPath=/tmp/cfg.toml, got %q", gotConfig)
	}
	if gotOverrides.PromptsDir != "/tmp/prompts" {
		t.Fatalf("expected overrides.PromptsDir=/tmp/prompts, got %q", gotOverrides.PromptsDir)
	}
	if !gotOverrides.SlashCommandSync {
		t.Fatal("expected overrides.SlashCommandSync=true")
	}
	if gotOverrides.SlashCommandDir != "/tmp/cmds" {
		t.Fatalf("expected overrides.SlashCommandDir=/tmp/cmds, got %q", gotOverrides.SlashCommandDir)
	}
}

func TestRun_SyncAllError(t *testing.T) {
	old := runBackfill
	t.Cleanup(func() { runBackfill = old })

	wantErr := errors.New("backfill failed")
	runBackfill = func(_, _ string, _ hexaimcp.MCPOverrides) error { return wantErr }

	opts := mcpOptions{syncAll: true}
	if err := run(opts, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected backfill error, got: %v", err)
	}
}

func TestRun_MCPServer(t *testing.T) {
	old := runMCP
	t.Cleanup(func() { runMCP = old })

	called := false
	runMCP = func(logPath, configPath string, overrides hexaimcp.MCPOverrides, stdin io.Reader, stdout, stderr io.Writer) error {
		called = true
		return nil
	}

	opts := mcpOptions{logPath: "/tmp/mcp.log"}
	if err := run(opts, nil, nil, nil); err != nil {
		t.Fatalf("run MCP: %v", err)
	}
	if !called {
		t.Fatal("expected runMCP to be called")
	}
}

func TestRun_MCPServerError(t *testing.T) {
	old := runMCP
	t.Cleanup(func() { runMCP = old })

	wantErr := errors.New("server failed")
	runMCP = func(_, _ string, _ hexaimcp.MCPOverrides, _ io.Reader, _, _ io.Writer) error { return wantErr }

	if err := run(mcpOptions{}, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected server error, got: %v", err)
	}
}

// runMain version path: -version must short-circuit and write the version
// to stdout (not stderr, so it stays scriptable). Stderr still gets the
// deprecation banner — that's correct since the binary is leaving anyway.
func TestRunMain_VersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runMain([]string{"-version"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runMain code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != internal.Version {
		t.Fatalf("stdout = %q, want version %q", got, internal.Version)
	}
	if !strings.Contains(stderr.String(), "DEPRECATION NOTICE") {
		t.Fatalf("stderr missing deprecation banner: %q", stderr.String())
	}
}

// runMain --sync-all path: forwards parsed options to runBackfill and
// returns 0 on success.
func TestRunMain_SyncAllSuccess(t *testing.T) {
	old := runBackfill
	t.Cleanup(func() { runBackfill = old })

	var gotLog string
	runBackfill = func(logPath string, _ string, _ hexaimcp.MCPOverrides) error {
		gotLog = logPath
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := runMain([]string{"-sync-all", "-log", "/tmp/sync.log"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runMain code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if gotLog != "/tmp/sync.log" {
		t.Fatalf("logPath forwarded = %q, want /tmp/sync.log", gotLog)
	}
}

// runMain run-error path: when the underlying server fails, runMain must
// return 1 (the production exit code) and write the error to stderr.
func TestRunMain_ServerErrorReturnsOne(t *testing.T) {
	old := runMCP
	t.Cleanup(func() { runMCP = old })
	runMCP = func(string, string, hexaimcp.MCPOverrides, io.Reader, io.Writer, io.Writer) error {
		return errors.New("mcp boom")
	}

	var stdout, stderr bytes.Buffer
	code := runMain(nil, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "mcp boom") {
		t.Fatalf("stderr missing error: %q", stderr.String())
	}
}

// Bad flag must yield exit 2 without ever invoking the server stub.
func TestRunMain_BadFlagReturnsTwo(t *testing.T) {
	old := runMCP
	t.Cleanup(func() { runMCP = old })
	called := false
	runMCP = func(string, string, hexaimcp.MCPOverrides, io.Reader, io.Writer, io.Writer) error {
		called = true
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := runMain([]string{"--bogus"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runMain code = %d, want 2", code)
	}
	if called {
		t.Fatal("runMCP must not be called on flag-parse failure")
	}
}
