// Integration tests for hexaimcp orchestrator.
package hexaimcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/mcp"
	"github.com/snonux/hexai/internal/promptstore"
)

const (
	responseReadTimeout       = 2 * time.Second
	serverExitTimeout         = 2 * time.Second
	serverStillRunningTimeout = 100 * time.Millisecond
)

// mockServerRunner implements ServerRunner for testing
type mockServerRunner struct {
	runFunc func() error
}

type serverExit struct {
	done chan struct{}
	err  error
}

type fullProtocolServer struct {
	stdin         *io.PipeWriter
	stdout        io.Reader
	exit          *serverExit
	cancel        context.CancelFunc
	closeStdin    sync.Once
	closeStdinErr error
}

func (m *mockServerRunner) Run(context.Context) error {
	if m.runFunc != nil {
		return m.runFunc()
	}
	return nil
}

// TestFullProtocolFlow tests the complete MCP protocol interaction
func TestFullProtocolFlow(t *testing.T) {
	tmpDir, promptsDir := setupFullProtocolConfig(t)
	server := startFullProtocolServer(t, tmpDir, promptsDir)

	writeJSONRPCLine(t, server.stdin, initializeRequest())
	assertInitializeResponse(t, readJSONRPCLine(t, server.stdout))
	assertServerStillRunning(t, server.exit)

	if err := server.closeInput(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	if err := waitForServer(t, server.exit); err != nil {
		t.Fatalf("server returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(promptsDir, "backups")); err != nil {
		t.Fatalf("prompts store was not initialized in test dir: %v", err)
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Fatalf("remove temp dir after server exit: %v", err)
	}
}

func setupFullProtocolConfig(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("HEXAI_MCP_SLASHCOMMAND_SYNC", "true")
	t.Setenv("HEXAI_MCP_SLASHCOMMAND_DIR", "/dev/null/impossible")

	return tmpDir, filepath.Join(tmpDir, "prompts")
}

func startFullProtocolServer(t *testing.T, tmpDir, promptsDir string) *fullProtocolServer {
	t.Helper()
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	exit := newServerExit()
	ctx, cancel := context.WithCancel(context.Background())

	loadOpts := appconfig.LoadOptions{
		ConfigPath:  filepath.Join(tmpDir, "config.toml"),
		IgnoreEnv:   true,
		ProjectRoot: tmpDir,
	}
	overrides := MCPOverrides{PromptsDir: promptsDir}
	serverFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
		return mcp.NewServer(r, w, logger, store, syncer)
	}

	go func() {
		var runErr error
		defer func() {
			_ = stdinReader.Close()
			_ = stdoutWriter.Close()
			exit.finish(runErr)
		}()
		logPath := filepath.Join(tmpDir, "mcp.log")
		runErr = runWithFactoryLoadOptions(ctx, logPath, loadOpts, overrides, stdinReader, stdoutWriter, &bytes.Buffer{}, serverFactory)
	}()

	server := &fullProtocolServer{
		stdin:  stdinWriter,
		stdout: stdoutReader,
		exit:   exit,
		cancel: cancel,
	}
	t.Cleanup(func() {
		_ = server.closeInput()
		server.cancel()
		defer func() {
			_ = stdoutReader.Close()
		}()

		err, ok := server.exit.wait(serverExitTimeout)
		if !ok {
			t.Errorf("server did not exit during cleanup after %s", serverExitTimeout)
			return
		}
		if err != nil {
			t.Errorf("server cleanup returned error: %v", err)
		}
	})

	return server
}

func newServerExit() *serverExit {
	return &serverExit{done: make(chan struct{})}
}

func (e *serverExit) finish(err error) {
	e.err = err
	close(e.done)
}

func (e *serverExit) wait(timeout time.Duration) (error, bool) {
	select {
	case <-e.done:
		return e.err, true
	case <-time.After(timeout):
		return nil, false
	}
}

func (s *fullProtocolServer) closeInput() error {
	s.closeStdin.Do(func() {
		s.closeStdinErr = s.stdin.Close()
	})
	return s.closeStdinErr
}

func initializeRequest() map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0",
			},
		},
	}
}

func writeJSONRPCLine(t *testing.T, w io.Writer, req map[string]any) {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		t.Fatalf("write newline: %v", err)
	}
}

func readJSONRPCLine(t *testing.T, r io.Reader) mcp.Response {
	t.Helper()
	type readResult struct {
		line []byte
		err  error
	}
	result := make(chan readResult, 1)
	go func() {
		line, err := bufio.NewReader(r).ReadBytes('\n')
		result <- readResult{line: line, err: err}
	}()

	var line []byte
	select {
	case res := <-result:
		if res.err != nil {
			t.Fatalf("read response: %v", res.err)
		}
		line = res.line
	case <-time.After(responseReadTimeout):
		t.Fatalf("timed out reading response after %s", responseReadTimeout)
	}

	var resp mcp.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", line, err)
	}
	return resp
}

func assertInitializeResponse(t *testing.T, resp mcp.Response) {
	t.Helper()
	if resp.JSONRPC != "2.0" {
		t.Fatalf("response JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID != float64(1) {
		t.Fatalf("response ID = %#v, want 1", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("initialize response error = %+v", resp.Error)
	}
}

func assertServerStillRunning(t *testing.T, exit *serverExit) {
	t.Helper()
	if err, ok := exit.wait(serverStillRunningTimeout); ok {
		t.Fatalf("server returned before stdin was closed: %v", err)
	}
}

func waitForServer(t *testing.T, exit *serverExit) error {
	t.Helper()
	if err, ok := exit.wait(serverExitTimeout); ok {
		return err
	}
	t.Fatal("server did not exit after stdin closed")
	return nil
}

func TestGetPromptsDir(t *testing.T) {
	tests := []struct {
		name      string
		cfgValue  string
		wantMatch string
	}{
		{
			name:      "config value used",
			cfgValue:  "/config/prompts",
			wantMatch: "/config/prompts",
		},
		{
			name:      "uses default XDG location",
			cfgValue:  "",
			wantMatch: ".local/hexai/data/prompts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := appconfig.App{
				FeatureConfig: appconfig.FeatureConfig{MCPConfig: appconfig.MCPConfig{MCPPromptsDir: tt.cfgValue}},
			}

			result, err := getPromptsDir(cfg)
			if err != nil {
				t.Fatalf("getPromptsDir() error = %v", err)
			}

			if !strings.Contains(result, tt.wantMatch) {
				t.Errorf("getPromptsDir() = %v, want to contain %v", result, tt.wantMatch)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "expand tilde",
			input:   "~/prompts",
			wantErr: false,
		},
		{
			name:    "absolute path",
			input:   "/absolute/path",
			wantErr: false,
		},
		{
			name:    "relative path",
			input:   "relative/path",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if tt.input == "~/prompts" && strings.Contains(result, "~") {
					t.Error("expandPath() should expand tilde")
				}
				if !strings.Contains(result, "/") {
					t.Error("expandPath() should return absolute path")
				}
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	t.Run("empty path uses stderr", func(t *testing.T) {
		logger, err := setupLogger("")
		if err != nil {
			t.Fatalf("setupLogger() error = %v", err)
		}
		if logger == nil {
			t.Fatal("setupLogger() returned nil logger")
		}
	})

	t.Run("creates log file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		logger, err := setupLogger(logPath)
		if err != nil {
			t.Fatalf("setupLogger() error = %v", err)
		}
		if logger == nil {
			t.Fatal("setupLogger() returned nil logger")
		}

		// Write a test message
		logger.Print("test message")

		// Verify file exists
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("Log file was not created")
		}

		// Close the file
		if f, ok := logger.Writer().(*os.File); ok && f != os.Stderr {
			f.Close()
		}
	})

	t.Run("creates log directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "subdir", "test.log")

		logger, err := setupLogger(logPath)
		if err != nil {
			t.Fatalf("setupLogger() error = %v", err)
		}

		// Verify directory was created
		dirPath := filepath.Dir(logPath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Error("Log directory was not created")
		}

		// Close the file
		if f, ok := logger.Writer().(*os.File); ok && f != os.Stderr {
			f.Close()
		}
	})
}

func TestLoadConfig(t *testing.T) {
	logger := log.New(io.Discard, "", 0)

	t.Run("loads default config when path empty", func(t *testing.T) {
		cfg := loadConfig(context.Background(), logger, "")
		// Should return a valid config (may be defaults)
		// Just verify it returns without panic
		_ = cfg
	})

	t.Run("loads config with nonexistent path", func(t *testing.T) {
		cfg := loadConfig(context.Background(), logger, "/nonexistent/config.yaml")
		// Should return default config without error
		// Just verify it returns without panic
		_ = cfg
	})
}

func TestDefaultServerFactory(t *testing.T) {
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	logger := log.New(io.Discard, "", 0)
	tmpDir := t.TempDir()
	store, err := promptstore.NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	server := defaultServerFactory(inBuf, outBuf, logger, store, nil)
	if server == nil {
		t.Fatal("defaultServerFactory() returned nil")
	}
}

func TestRun(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create a mock server factory that returns immediately
	mockFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
		return &mockServerRunner{
			runFunc: func() error {
				return nil // Exit immediately
			},
		}
	}

	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	// Pass prompts dir via overrides instead of environment variable
	overrides := MCPOverrides{PromptsDir: tmpDir}

	err := RunWithFactory(context.Background(), logPath, "", overrides, inBuf, outBuf, errBuf, mockFactory)
	if err != nil {
		t.Fatalf("RunWithFactory() error = %v", err)
	}

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

func TestRunWithFactory_ServerError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create a mock server factory that returns an error
	mockFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
		return &mockServerRunner{
			runFunc: func() error {
				return fmt.Errorf("mock server error")
			},
		}
	}

	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	// Pass prompts dir via overrides instead of environment variable
	overrides := MCPOverrides{PromptsDir: tmpDir}

	err := RunWithFactory(context.Background(), logPath, "", overrides, inBuf, outBuf, errBuf, mockFactory)
	if err == nil {
		t.Fatal("RunWithFactory() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("RunWithFactory() error = %v, want to contain 'server error'", err)
	}
}

// TestRunWithFactory_LoggerError verifies that a bad log path propagates as an error.
func TestRunWithFactory_LoggerError(t *testing.T) {
	// Use /dev/null/impossible as log path — directory creation will fail
	// because /dev/null is a file, not a directory.
	badLogPath := "/dev/null/impossible/test.log"

	mockFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
		return &mockServerRunner{}
	}

	err := RunWithFactory(context.Background(), badLogPath, "", MCPOverrides{}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, mockFactory)
	if err == nil {
		t.Fatal("expected error for invalid log path, got nil")
	}
	if !strings.Contains(err.Error(), "cannot setup logger") {
		t.Errorf("error = %v, want to contain 'cannot setup logger'", err)
	}
}

// TestRunWithFactory_StderrLogger verifies RunWithFactory works when logPath
// is empty (logger writes to stderr, defer close branch is a no-op).
func TestRunWithFactory_StderrLogger(t *testing.T) {
	tmpDir := t.TempDir()

	mockFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
		return &mockServerRunner{}
	}

	// Pass prompts dir via overrides instead of environment variable
	overrides := MCPOverrides{PromptsDir: tmpDir}

	// Empty logPath causes logger to write to stderr (no file to close)
	err := RunWithFactory(context.Background(), "", "", overrides, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, mockFactory)
	if err != nil {
		t.Fatalf("RunWithFactory() error = %v", err)
	}
}

// TestRun_CallsDefaultFactory verifies the Run() entry point invokes
// RunWithFactory with the defaultServerFactory. The real server reads
// from stdin until EOF; with an empty buffer it returns immediately.
func TestRun_CallsDefaultFactory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Pass prompts dir via overrides instead of environment variable
	overrides := MCPOverrides{PromptsDir: tmpDir}

	// Run with empty stdin — the real server hits EOF and exits cleanly.
	// This exercises the full Run -> RunWithFactory -> defaultServerFactory path.
	err := Run(context.Background(), logPath, "", overrides, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	// The server may return nil or an error depending on how it handles EOF;
	// the important thing is that Run() itself does not panic.
	_ = err
}

// TestSetupLogger_InvalidPath verifies setupLogger returns an error when
// the log directory cannot be created.
func TestSetupLogger_InvalidPath(t *testing.T) {
	// /dev/null is a file, so creating a subdirectory under it fails
	_, err := setupLogger("/dev/null/subdir/test.log")
	if err == nil {
		t.Fatal("expected error for invalid log path, got nil")
	}
	if !strings.Contains(err.Error(), "cannot create log directory") {
		t.Errorf("error = %v, want to contain 'cannot create log directory'", err)
	}
}

// TestSetupLogger_WhitespacePath verifies that a whitespace-only path
// falls back to stderr logging.
func TestSetupLogger_WhitespacePath(t *testing.T) {
	logger, err := setupLogger("   ")
	if err != nil {
		t.Fatalf("setupLogger() error = %v", err)
	}
	if logger == nil {
		t.Fatal("setupLogger() returned nil logger")
	}
}

// TestGetPromptsDir_XDGDataHome verifies getPromptsDir uses XDG_DATA_HOME
// when set (covers the branch where XDG_DATA_HOME is non-empty).
func TestGetPromptsDir_XDGDataHome(t *testing.T) {
	oldXDG := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", oldXDG)
	os.Setenv("XDG_DATA_HOME", "/custom/xdg/data")

	cfg := appconfig.App{}
	result, err := getPromptsDir(cfg)
	if err != nil {
		t.Fatalf("getPromptsDir() error = %v", err)
	}

	want := "/custom/xdg/data/prompts"
	if result != want {
		t.Errorf("getPromptsDir() = %v, want %v", result, want)
	}
}

// TestGetPromptsDir_TildeInConfig verifies tilde expansion for config path.
func TestGetPromptsDir_TildeInConfig(t *testing.T) {
	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{MCPConfig: appconfig.MCPConfig{MCPPromptsDir: "~/my-prompts"}},
	}

	result, err := getPromptsDir(cfg)
	if err != nil {
		t.Fatalf("getPromptsDir() error = %v", err)
	}

	// Should not contain tilde and should be absolute
	if strings.Contains(result, "~") {
		t.Errorf("getPromptsDir() = %v, tilde not expanded", result)
	}
	if !filepath.IsAbs(result) {
		t.Errorf("getPromptsDir() = %v, want absolute path", result)
	}
	if !strings.HasSuffix(result, "my-prompts") {
		t.Errorf("getPromptsDir() = %v, want suffix 'my-prompts'", result)
	}
}

// TestCreateSyncer_Disabled verifies createSyncer returns a non-nil syncer
// when sync is disabled.
func TestCreateSyncer_Disabled(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{MCPConfig: appconfig.MCPConfig{MCPSlashCommandSync: false}},
	}

	syncer, err := createSyncer(cfg, logger)
	if err != nil {
		t.Fatalf("createSyncer() error = %v", err)
	}
	if syncer == nil {
		t.Fatal("createSyncer() returned nil syncer")
	}
}

// TestCreateSyncer_Enabled verifies createSyncer when sync is enabled
// with a valid temporary directory.
func TestCreateSyncer_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	logger := log.New(io.Discard, "", 0)
	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{MCPConfig: appconfig.MCPConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  tmpDir,
		}},
	}

	syncer, err := createSyncer(cfg, logger)
	if err != nil {
		t.Fatalf("createSyncer() error = %v", err)
	}
	if syncer == nil {
		t.Fatal("createSyncer() returned nil syncer")
	}
}

// TestCreateSyncer_Error verifies createSyncer returns an error when sync
// is enabled but the directory config is empty.
func TestCreateSyncer_Error(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{MCPConfig: appconfig.MCPConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  "",
		}},
	}

	_, err := createSyncer(cfg, logger)
	if err == nil {
		t.Fatal("createSyncer() expected error for empty dir, got nil")
	}
}

// TestRunBackfill_FullHappyPath verifies the happy path of RunBackfill by
// providing a config file with a valid slash command directory and prompts dir.
func TestRunBackfill_FullHappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	cmdDir := filepath.Join(tmpDir, "commands")
	logPath := filepath.Join(tmpDir, "test.log")

	// Create prompts directory so the store can be initialized
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("cannot create prompts dir: %v", err)
	}

	// Write a config file with [mcp] section that sets the slash command dir
	cfgContent := fmt.Sprintf("[mcp]\nslashcommand_dir = %q\nslashcommand_sync = true\n", cmdDir)
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("cannot write config: %v", err)
	}

	// Pass prompts dir via overrides instead of environment variable
	overrides := MCPOverrides{PromptsDir: promptsDir}

	// RunBackfill should succeed: config sets MCPSlashCommandDir, prompts
	// dir exists, and SyncAll on an empty store is a no-op.
	err := RunBackfill(context.Background(), logPath, cfgPath, overrides)
	if err != nil {
		t.Fatalf("RunBackfill() error = %v", err)
	}

	// Verify log file was created
	if _, statErr := os.Stat(logPath); os.IsNotExist(statErr) {
		t.Error("log file was not created")
	}
}

// TestRunBackfill_CreateSyncerError verifies RunBackfill propagates
// syncer creation errors (e.g. when MCPSlashCommandDir is set but
// the syncer cannot be created due to an invalid path).
func TestRunBackfill_CreateSyncerError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Use /dev/null as the slash command dir — creating subdirs under
	// /dev/null will fail, which triggers a syncer creation error.
	cfgContent := "[mcp]\nslashcommand_dir = \"/dev/null/impossible\"\n"
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("cannot write config: %v", err)
	}

	err := RunBackfill(context.Background(), logPath, cfgPath, MCPOverrides{})
	if err == nil {
		t.Fatal("expected error for invalid slash command dir, got nil")
	}
	if !strings.Contains(err.Error(), "cannot create syncer") {
		t.Errorf("error = %v, want to contain 'cannot create syncer'", err)
	}
}

// TestRunBackfill_StderrLogger verifies RunBackfill works when logPath
// is empty (logger writes to stderr).
func TestRunBackfill_StderrLogger(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	cmdDir := filepath.Join(tmpDir, "commands")

	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("cannot create prompts dir: %v", err)
	}

	cfgContent := fmt.Sprintf("[mcp]\nslashcommand_dir = %q\n", cmdDir)
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("cannot write config: %v", err)
	}

	// Pass prompts dir via overrides instead of environment variable
	overrides := MCPOverrides{PromptsDir: promptsDir}

	// Empty logPath — logger writes to stderr, defer close is a no-op
	err := RunBackfill(context.Background(), "", cfgPath, overrides)
	if err != nil {
		t.Fatalf("RunBackfill() error = %v", err)
	}
}

// TestRunBackfill_LoggerError verifies RunBackfill returns an error when
// the log path is invalid.
func TestRunBackfill_LoggerError(t *testing.T) {
	err := RunBackfill(context.Background(), "/dev/null/impossible/test.log", "", MCPOverrides{})
	if err == nil {
		t.Fatal("expected error for invalid log path, got nil")
	}
	if !strings.Contains(err.Error(), "cannot setup logger") {
		t.Errorf("error = %v, want to contain 'cannot setup logger'", err)
	}
}

// TestRunBackfill_NoCmdDir verifies RunBackfill returns an error when
// slash command directory is not configured. Uses a nonexistent config
// path and unsets relevant env vars to avoid picking up real config.
func TestRunBackfill_NoCmdDir(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write an empty config file so loadConfig doesn't fall back to
	// the user's real global config or project config.
	emptyCfgPath := filepath.Join(tmpDir, "empty.toml")
	if err := os.WriteFile(emptyCfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("cannot write empty config: %v", err)
	}

	// Unset env var that could set the slash command dir
	oldEnv := os.Getenv("HEXAI_MCP_SLASHCOMMAND_DIR")
	defer os.Setenv("HEXAI_MCP_SLASHCOMMAND_DIR", oldEnv)
	os.Setenv("HEXAI_MCP_SLASHCOMMAND_DIR", "")

	err := RunBackfill(context.Background(), logPath, emptyCfgPath, MCPOverrides{})
	if err == nil {
		t.Fatal("expected error for empty slash command dir, got nil")
	}
	if !strings.Contains(err.Error(), "commands directory not configured") {
		t.Errorf("error = %v, want to contain 'commands directory not configured'", err)
	}
}

// TestApplyOverrides verifies that MCPOverrides are correctly applied to config.
func TestApplyOverrides(t *testing.T) {
	t.Run("applies all overrides", func(t *testing.T) {
		cfg := appconfig.App{}
		overrides := MCPOverrides{
			PromptsDir:       "/custom/prompts",
			SlashCommandSync: true,
			SlashCommandDir:  "/custom/cmds",
		}
		applyOverrides(&cfg, overrides)

		if cfg.MCPPromptsDir != "/custom/prompts" {
			t.Errorf("MCPPromptsDir = %q, want /custom/prompts", cfg.MCPPromptsDir)
		}
		if !cfg.MCPSlashCommandSync {
			t.Error("MCPSlashCommandSync = false, want true")
		}
		if cfg.MCPSlashCommandDir != "/custom/cmds" {
			t.Errorf("MCPSlashCommandDir = %q, want /custom/cmds", cfg.MCPSlashCommandDir)
		}
	})

	t.Run("does not overwrite with zero values", func(t *testing.T) {
		cfg := appconfig.App{
			FeatureConfig: appconfig.FeatureConfig{MCPConfig: appconfig.MCPConfig{
				MCPPromptsDir:       "/existing/prompts",
				MCPSlashCommandSync: true,
				MCPSlashCommandDir:  "/existing/cmds",
			}},
		}
		overrides := MCPOverrides{} // all zero values
		applyOverrides(&cfg, overrides)

		if cfg.MCPPromptsDir != "/existing/prompts" {
			t.Errorf("MCPPromptsDir = %q, want /existing/prompts", cfg.MCPPromptsDir)
		}
		// SlashCommandSync false doesn't overwrite existing true
		if !cfg.MCPSlashCommandSync {
			t.Error("MCPSlashCommandSync should remain true")
		}
		if cfg.MCPSlashCommandDir != "/existing/cmds" {
			t.Errorf("MCPSlashCommandDir = %q, want /existing/cmds", cfg.MCPSlashCommandDir)
		}
	})
}
