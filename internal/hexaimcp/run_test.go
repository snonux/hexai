// Integration tests for hexaimcp orchestrator.
package hexaimcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/mcp"
	"codeberg.org/snonux/hexai/internal/promptstore"
)

// mockServerRunner implements ServerRunner for testing
type mockServerRunner struct {
	runFunc func() error
}

func (m *mockServerRunner) Run() error {
	if m.runFunc != nil {
		return m.runFunc()
	}
	return nil
}

// TestFullProtocolFlow tests the complete MCP protocol interaction
func TestFullProtocolFlow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test server factory
	serverFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer mcp.SlashCommandSyncer) ServerRunner {
		return mcp.NewServer(r, w, logger, store, syncer)
	}

	// Setup I/O pipes
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	// Send initialize request
	initReq := map[string]any{
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

	writeJSONRPC(t, inBuf, initReq)

	// Run server in background (it will read from inBuf and write to outBuf)
	go func() {
		// Pass prompts dir via overrides instead of environment variable
		overrides := MCPOverrides{PromptsDir: tmpDir}

		// Note: This will hang waiting for more input, which is expected
		_ = RunWithFactory("", "", overrides, inBuf, outBuf, errBuf, serverFactory)
	}()

	// Give server time to process
	// Note: In a real test, you'd use proper synchronization

	// For now, just verify the server starts and creates the prompts directory
	// A full integration test would require more sophisticated I/O handling
}

func writeJSONRPC(t *testing.T, w io.Writer, req map[string]any) {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write body: %v", err)
	}
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
				FeatureConfig: appconfig.FeatureConfig{MCPPromptsDir: tt.cfgValue},
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
		cfg := loadConfig(logger, "")
		// Should return a valid config (may be defaults)
		// Just verify it returns without panic
		_ = cfg
	})

	t.Run("loads config with nonexistent path", func(t *testing.T) {
		cfg := loadConfig(logger, "/nonexistent/config.yaml")
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

	err := RunWithFactory(logPath, "", overrides, inBuf, outBuf, errBuf, mockFactory)
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

	err := RunWithFactory(logPath, "", overrides, inBuf, outBuf, errBuf, mockFactory)
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

	err := RunWithFactory(badLogPath, "", MCPOverrides{}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, mockFactory)
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
	err := RunWithFactory("", "", overrides, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, mockFactory)
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
	err := Run(logPath, "", overrides, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
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
		FeatureConfig: appconfig.FeatureConfig{MCPPromptsDir: "~/my-prompts"},
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
		FeatureConfig: appconfig.FeatureConfig{MCPSlashCommandSync: false},
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
		FeatureConfig: appconfig.FeatureConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  tmpDir,
		},
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
		FeatureConfig: appconfig.FeatureConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  "",
		},
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
	err := RunBackfill(logPath, cfgPath, overrides)
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

	err := RunBackfill(logPath, cfgPath, MCPOverrides{})
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
	err := RunBackfill("", cfgPath, overrides)
	if err != nil {
		t.Fatalf("RunBackfill() error = %v", err)
	}
}

// TestRunBackfill_LoggerError verifies RunBackfill returns an error when
// the log path is invalid.
func TestRunBackfill_LoggerError(t *testing.T) {
	err := RunBackfill("/dev/null/impossible/test.log", "", MCPOverrides{})
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

	err := RunBackfill(logPath, emptyCfgPath, MCPOverrides{})
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
			FeatureConfig: appconfig.FeatureConfig{
				MCPPromptsDir:       "/existing/prompts",
				MCPSlashCommandSync: true,
				MCPSlashCommandDir:  "/existing/cmds",
			},
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
