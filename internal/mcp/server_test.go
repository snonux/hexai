// Summary: Tests for MCP server operations
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/promptstore"
)

// mockPromptStore implements PromptStore for testing
type mockPromptStore struct {
	prompts map[string]*promptstore.Prompt
	listFn  func(string, int) ([]promptstore.Prompt, string, error)
	getFn   func(string) (*promptstore.Prompt, error)
}

func (m *mockPromptStore) List(cursor string, limit int) ([]promptstore.Prompt, string, error) {
	if m.listFn != nil {
		return m.listFn(cursor, limit)
	}
	var all []promptstore.Prompt
	for _, p := range m.prompts {
		all = append(all, *p)
	}
	return all, "", nil
}

func (m *mockPromptStore) Get(name string) (*promptstore.Prompt, error) {
	if m.getFn != nil {
		return m.getFn(name)
	}
	p, ok := m.prompts[name]
	if !ok {
		return nil, fmt.Errorf("prompt not found: %s", name)
	}
	return p, nil
}

// Create, Update, Delete methods moved to handlers_test.go to avoid duplication

func (m *mockPromptStore) SearchByTags(tags []string) ([]promptstore.Prompt, error) {
	return nil, fmt.Errorf("not implemented")
}

func createTestServer(t *testing.T, store promptstore.PromptStore) (*Server, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	logger := log.New(io.Discard, "", 0)
	return NewServer(inBuf, outBuf, logger, store), inBuf, outBuf
}

// sendRequest writes a JSON-RPC request as newline-delimited JSON (MCP stdio protocol).
func sendRequest(w io.Writer, req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return nil
}

// readResponse reads a newline-delimited JSON-RPC response (MCP stdio protocol).
// Skips notification messages (those without an ID) and returns the actual response.
func readResponse(r io.Reader) (*Response, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Parse newline-delimited JSON; find the first line with an ID (skip notifications)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no response data")
	}
	
	// Try to find a response (message with an ID)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		// Quick check if this might be a response (has "id" field)
		if !strings.Contains(line, `"id"`) {
			continue // Skip notifications
		}
		
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue // Not a valid response, try next line
		}
		
		// Valid response found
		return &resp, nil
	}
	
	return nil, fmt.Errorf("no response found in output (only notifications)")
}

func TestServer_Initialize(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, inBuf, outBuf := createTestServer(t, store)

	// Send initialize request
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	params := InitializeRequest{
		ProtocolVersion: "2025-06-18",
		Capabilities:    ClientCapabilities{},
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0",
		},
	}
	req.Params, _ = json.Marshal(params)

	if err := sendRequest(inBuf, req); err != nil {
		t.Fatalf("sendRequest() error = %v", err)
	}

	// Handle request
	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %v, want 2.0", resp.JSONRPC)
	}
	// ID comparison: JSON unmarshaling may convert int to float64
	if fmt.Sprintf("%v", resp.ID) != "1" {
		t.Errorf("ID = %v (type %T), want 1", resp.ID, resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Error = %v, want nil", resp.Error)
	}

	// Verify server is initialized
	server.mu.RLock()
	initialized := server.initialized
	server.mu.RUnlock()
	if !initialized {
		t.Error("Server not initialized")
	}
}

func TestServer_PromptsList(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test1": {
				Name:        "test1",
				Title:       "Test Prompt 1",
				Description: "First test prompt",
				Messages:    []promptstore.PromptMessage{},
				Created:     now,
				Updated:     now,
			},
			"test2": {
				Name:        "test2",
				Title:       "Test Prompt 2",
				Description: "Second test prompt",
				Messages:    []promptstore.PromptMessage{},
				Created:     now,
				Updated:     now,
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server first
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/list request
	req := Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "prompts/list",
		Params:  json.RawMessage(`{}`),
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Error = %v, want nil", resp.Error)
	}

	// Parse result
	resultBytes, _ := json.Marshal(resp.Result)
	var result ListPromptsResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if len(result.Prompts) != 2 {
		t.Errorf("Prompts count = %d, want 2", len(result.Prompts))
	}
}

func TestServer_PromptsGet(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test_prompt": {
				Name:        "test_prompt",
				Title:       "Test Prompt",
				Description: "A test prompt",
				Arguments: []promptstore.PromptArgument{
					{Name: "name", Description: "User name", Required: true},
				},
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Hello {{name}}!",
						},
					},
				},
				Created: now,
				Updated: now,
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/get request
	params := GetPromptRequest{
		Name: "test_prompt",
		Arguments: map[string]string{
			"name": "World",
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "prompts/get",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Error = %v, want nil", resp.Error)
	}

	// Parse result
	resultBytes, _ := json.Marshal(resp.Result)
	var result GetPromptResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if len(result.Messages) != 1 {
		t.Errorf("Messages count = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Content.Text != "Hello World!" {
		t.Errorf("Message text = %v, want 'Hello World!'", result.Messages[0].Content.Text)
	}
}

func TestServer_PromptsGet_MissingArg(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test_prompt": {
				Name:        "test_prompt",
				Title:       "Test Prompt",
				Description: "A test prompt",
				Arguments: []promptstore.PromptArgument{
					{Name: "required_arg", Description: "Required", Required: true},
				},
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Test: {{required_arg}}",
						},
					},
				},
				Created: now,
				Updated: now,
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/get request without required argument
	params := GetPromptRequest{
		Name:      "test_prompt",
		Arguments: map[string]string{},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "prompts/get",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for missing required argument")
	}
	if !strings.Contains(resp.Error.Message, "required_arg") {
		t.Errorf("Error message = %v, want to contain 'required_arg'", resp.Error.Message)
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Send request with unknown method
	req := Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "unknown/method",
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for unknown method")
	}
	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, ErrCodeMethodNotFound)
	}
}

func TestServer_Run(t *testing.T) {
	t.Run("exits on EOF", func(t *testing.T) {
		store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
		// Empty reader will cause immediate EOF
		inBuf := &bytes.Buffer{}
		outBuf := &bytes.Buffer{}
		logger := log.New(io.Discard, "", 0)
		server := NewServer(inBuf, outBuf, logger, store)

		err := server.Run()
		if err != nil {
			t.Errorf("Run() error = %v, want nil on EOF", err)
		}
	})

	t.Run("processes initialize request", func(t *testing.T) {
		store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
		inBuf := &bytes.Buffer{}
		outBuf := &bytes.Buffer{}
		logger := log.New(io.Discard, "", 0)
		server := NewServer(inBuf, outBuf, logger, store)

		// Send initialize request
		req := Request{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
		}
		params := InitializeRequest{
			ProtocolVersion: "2025-06-18",
			Capabilities:    ClientCapabilities{},
			ClientInfo: ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		}
		req.Params, _ = json.Marshal(params)

		if err := sendRequest(inBuf, req); err != nil {
			t.Fatalf("sendRequest() error = %v", err)
		}

		// Run in background
		done := make(chan error, 1)
		go func() {
			done <- server.Run()
		}()

		// Give time for processing (server will block waiting for more input)
		time.Sleep(50 * time.Millisecond)

		// Read response
		resp, err := readResponse(outBuf)
		if err != nil {
			t.Fatalf("readResponse() error = %v", err)
		}

		if resp.Error != nil {
			t.Errorf("Error = %v, want nil", resp.Error)
		}

		// Verify server is initialized
		server.mu.RLock()
		initialized := server.initialized
		server.mu.RUnlock()
		if !initialized {
			t.Error("Server not initialized")
		}
	})
}

func TestServer_ReadMessage(t *testing.T) {
	t.Run("reads valid message", func(t *testing.T) {
		store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
		inBuf := &bytes.Buffer{}
		outBuf := &bytes.Buffer{}
		logger := log.New(io.Discard, "", 0)
		server := NewServer(inBuf, outBuf, logger, store)

		// Write a newline-delimited JSON message (MCP stdio protocol)
		msg := `{"jsonrpc":"2.0","id":1,"method":"test"}`
		inBuf.WriteString(msg + "\n")

		// Read it back
		body, err := server.readMessage()
		if err != nil {
			t.Fatalf("readMessage() error = %v", err)
		}

		if string(body) != msg {
			t.Errorf("readMessage() = %s, want %s", body, msg)
		}
	})

	t.Run("skips empty lines", func(t *testing.T) {
		store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
		inBuf := &bytes.Buffer{}
		outBuf := &bytes.Buffer{}
		logger := log.New(io.Discard, "", 0)
		server := NewServer(inBuf, outBuf, logger, store)

		// Write empty lines followed by a valid message
		msg := `{"jsonrpc":"2.0","id":1,"method":"test"}`
		inBuf.WriteString("\n\n" + msg + "\n")

		body, err := server.readMessage()
		if err != nil {
			t.Fatalf("readMessage() error = %v", err)
		}

		if string(body) != msg {
			t.Errorf("readMessage() = %s, want %s", body, msg)
		}
	})

	t.Run("returns EOF on empty stream", func(t *testing.T) {
		store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
		inBuf := &bytes.Buffer{}
		outBuf := &bytes.Buffer{}
		logger := log.New(io.Discard, "", 0)
		server := NewServer(inBuf, outBuf, logger, store)

		_, err := server.readMessage()
		if err != io.EOF {
			t.Errorf("readMessage() error = %v, want EOF", err)
		}
	})
}

func TestNegotiateProtocolVersion(t *testing.T) {
	tests := []struct {
		name     string
		client   string
		expected string
	}{
		{"echoes supported version", "2025-11-25", "2025-11-25"},
		{"echoes older version", "2025-06-18", "2025-06-18"},
		{"echoes oldest version", "2024-11-05", "2024-11-05"},
		{"returns latest for unknown", "9999-01-01", LatestProtocolVersion},
		{"returns latest for empty", "", LatestProtocolVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := negotiateProtocolVersion(tt.client)
			if got != tt.expected {
				t.Errorf("negotiateProtocolVersion(%q) = %q, want %q", tt.client, got, tt.expected)
			}
		})
	}
}

func TestServer_HandleInitialized(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, _ := createTestServer(t, store)

	// Send initialized notification
	req := Request{
		JSONRPC: "2.0",
		Method:  "initialized",
	}

	// This should not error (it's a notification, no response expected)
	server.handleInitialized(req)
}
