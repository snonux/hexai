package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal"
)

func TestPrintDeprecationWarning(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	oldStderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	printDeprecationWarning()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}

	output := string(b)
	for _, want := range []string{"DEPRECATION NOTICE", "EXPERIMENTAL", "NOT ACTIVELY MAINTAINED"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got %q", want, output)
		}
	}
}

func TestDefaultLogPath(t *testing.T) {
	path := defaultLogPath()
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

func TestRun_SetsPromptsDir(t *testing.T) {
	t.Setenv("HEXAI_MCP_PROMPTS_DIR", "")
	opts := mcpOptions{showVersion: true, promptsDir: "/tmp/test-prompts"}
	var stdout bytes.Buffer
	if err := run(opts, nil, &stdout, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := os.Getenv("HEXAI_MCP_PROMPTS_DIR"); got != "/tmp/test-prompts" {
		t.Fatalf("expected HEXAI_MCP_PROMPTS_DIR=/tmp/test-prompts, got %q", got)
	}
}

func TestRun_SetsSlashCommandSync(t *testing.T) {
	t.Setenv("HEXAI_MCP_SLASHCOMMAND_SYNC", "")
	opts := mcpOptions{showVersion: true, slashCommandSync: true}
	var stdout bytes.Buffer
	if err := run(opts, nil, &stdout, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := os.Getenv("HEXAI_MCP_SLASHCOMMAND_SYNC"); got != "true" {
		t.Fatalf("expected HEXAI_MCP_SLASHCOMMAND_SYNC=true, got %q", got)
	}
}

func TestRun_SetsSlashCommandDir(t *testing.T) {
	t.Setenv("HEXAI_MCP_SLASHCOMMAND_DIR", "")
	opts := mcpOptions{showVersion: true, slashCommandDir: "/tmp/test-cmds"}
	var stdout bytes.Buffer
	if err := run(opts, nil, &stdout, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := os.Getenv("HEXAI_MCP_SLASHCOMMAND_DIR"); got != "/tmp/test-cmds" {
		t.Fatalf("expected HEXAI_MCP_SLASHCOMMAND_DIR=/tmp/test-cmds, got %q", got)
	}
}

func TestRun_SyncAll(t *testing.T) {
	old := runBackfill
	t.Cleanup(func() { runBackfill = old })

	var gotLog, gotConfig string
	runBackfill = func(logPath, configPath string) error {
		gotLog = logPath
		gotConfig = configPath
		return nil
	}

	opts := mcpOptions{syncAll: true, logPath: "/tmp/test.log", configPath: "/tmp/cfg.toml"}
	if err := run(opts, nil, nil, nil); err != nil {
		t.Fatalf("run syncAll: %v", err)
	}
	if gotLog != "/tmp/test.log" {
		t.Fatalf("expected logPath=/tmp/test.log, got %q", gotLog)
	}
	if gotConfig != "/tmp/cfg.toml" {
		t.Fatalf("expected configPath=/tmp/cfg.toml, got %q", gotConfig)
	}
}

func TestRun_SyncAllError(t *testing.T) {
	old := runBackfill
	t.Cleanup(func() { runBackfill = old })

	wantErr := errors.New("backfill failed")
	runBackfill = func(_, _ string) error { return wantErr }

	opts := mcpOptions{syncAll: true}
	if err := run(opts, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected backfill error, got: %v", err)
	}
}

func TestRun_MCPServer(t *testing.T) {
	old := runMCP
	t.Cleanup(func() { runMCP = old })

	called := false
	runMCP = func(logPath, configPath string, stdin io.Reader, stdout, stderr io.Writer) error {
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
	runMCP = func(_, _ string, _ io.Reader, _, _ io.Writer) error { return wantErr }

	if err := run(mcpOptions{}, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected server error, got: %v", err)
	}
}
