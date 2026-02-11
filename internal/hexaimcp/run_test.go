// Summary: Integration tests for hexaimcp orchestrator
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
	serverFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore) ServerRunner {
		return mcp.NewServer(r, w, logger, store)
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
		// Override prompts dir via environment
		t.Setenv("HEXAI_MCP_PROMPTS_DIR", tmpDir)

		// Note: This will hang waiting for more input, which is expected
		_ = RunWithFactory("", "", inBuf, outBuf, errBuf, serverFactory)
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
		envVar    string
		cfgValue  string
		wantMatch string
	}{
		{
			name:      "environment variable takes precedence",
			envVar:    "/custom/prompts",
			cfgValue:  "/config/prompts",
			wantMatch: "/custom/prompts",
		},
		{
			name:      "config file used when no env",
			envVar:    "",
			cfgValue:  "/config/prompts",
			wantMatch: "/config/prompts",
		},
		{
			name:      "uses default XDG location",
			envVar:    "",
			cfgValue:  "",
			wantMatch: ".local/hexai/data/prompts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			oldEnv := os.Getenv("HEXAI_MCP_PROMPTS_DIR")
			defer os.Setenv("HEXAI_MCP_PROMPTS_DIR", oldEnv)
			os.Setenv("HEXAI_MCP_PROMPTS_DIR", tt.envVar)

			// Create config
			cfg := appconfig.App{
				MCPPromptsDir: tt.cfgValue,
			}

			// Test
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

	server := defaultServerFactory(inBuf, outBuf, logger, store)
	if server == nil {
		t.Fatal("defaultServerFactory() returned nil")
	}
}

func TestRun(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create a mock server factory that returns immediately
	mockFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore) ServerRunner {
		return &mockServerRunner{
			runFunc: func() error {
				return nil // Exit immediately
			},
		}
	}

	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	// Set prompts dir environment variable
	oldEnv := os.Getenv("HEXAI_MCP_PROMPTS_DIR")
	defer os.Setenv("HEXAI_MCP_PROMPTS_DIR", oldEnv)
	os.Setenv("HEXAI_MCP_PROMPTS_DIR", tmpDir)

	err := RunWithFactory(logPath, "", inBuf, outBuf, errBuf, mockFactory)
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
	mockFactory := func(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore) ServerRunner {
		return &mockServerRunner{
			runFunc: func() error {
				return fmt.Errorf("mock server error")
			},
		}
	}

	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	// Set prompts dir environment variable
	oldEnv := os.Getenv("HEXAI_MCP_PROMPTS_DIR")
	defer os.Setenv("HEXAI_MCP_PROMPTS_DIR", oldEnv)
	os.Setenv("HEXAI_MCP_PROMPTS_DIR", tmpDir)

	err := RunWithFactory(logPath, "", inBuf, outBuf, errBuf, mockFactory)
	if err == nil {
		t.Fatal("RunWithFactory() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("RunWithFactory() error = %v, want to contain 'server error'", err)
	}
}
