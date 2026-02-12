// Summary: Tests for MCP prompt management handlers
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/promptstore"
)

func TestServer_PromptsCreate(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/create request
	params := CreatePromptRequest{
		Name:        "test_create",
		Title:       "Test Create Prompt",
		Description: "A test prompt",
		Arguments: []PromptArgument{
			{Name: "input", Description: "Test input", Required: true},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Test: {{input}}",
				},
			},
		},
		Tags: []string{"test"},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "prompts/create",
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
	var result PromptOperationResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true")
	}
}

func TestServer_PromptsUpdate(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test_update": {
				Name:    "test_update",
				Title:   "Original Title",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Original text",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request
	params := UpdatePromptRequest{
		Name:  "test_update",
		Title: "Updated Title",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "prompts/update",
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
	var result PromptOperationResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true")
	}
}

func TestServer_PromptsDelete(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test_delete": {
				Name:     "test_delete",
				Title:    "To Be Deleted",
				Created:  now,
				Updated:  now,
				Messages: []promptstore.PromptMessage{},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/delete request
	params := DeletePromptRequest{
		Name: "test_delete",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "prompts/delete",
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
	var result PromptOperationResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true")
	}
}

func TestServer_PromptsCreate_MissingName(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/create request without name
	params := CreatePromptRequest{
		Title: "No Name",
		Messages: []PromptMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "Test"}},
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "prompts/create",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for missing name")
	}

	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

// Update mockPromptStore to support Create, Update, Delete
func (m *mockPromptStore) Create(prompt *promptstore.Prompt) error {
	if _, exists := m.prompts[prompt.Name]; exists {
		return fmt.Errorf("prompt already exists: %s", prompt.Name)
	}
	m.prompts[prompt.Name] = prompt
	return nil
}

func (m *mockPromptStore) Update(prompt *promptstore.Prompt) error {
	if _, exists := m.prompts[prompt.Name]; !exists {
		return fmt.Errorf("prompt not found: %s", prompt.Name)
	}
	m.prompts[prompt.Name] = prompt
	return nil
}

func (m *mockPromptStore) Delete(name string) error {
	if _, exists := m.prompts[name]; !exists {
		return fmt.Errorf("prompt not found: %s", name)
	}
	delete(m.prompts, name)
	return nil
}

func TestServer_PromptsUpdate_NotFound(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request for non-existent prompt
	params := UpdatePromptRequest{
		Name:  "nonexistent",
		Title: "Updated Title",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "prompts/update",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for non-existent prompt")
	}
}

func TestServer_PromptsUpdate_MissingName(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request without name
	params := UpdatePromptRequest{
		Title: "Updated Title",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "prompts/update",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for missing name")
	}
}

func TestServer_PromptsDelete_NotFound(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/delete request for non-existent prompt
	params := DeletePromptRequest{
		Name: "nonexistent",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "prompts/delete",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for non-existent prompt")
	}
}

func TestServer_PromptsDelete_MissingName(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/delete request without name
	params := DeletePromptRequest{}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "prompts/delete",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for missing name")
	}
}

func TestServer_PromptsCreate_AlreadyExists(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"existing": {
				Name:     "existing",
				Title:    "Existing Prompt",
				Created:  now,
				Updated:  now,
				Messages: []promptstore.PromptMessage{},
			},
		},
	}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/create request with existing name
	params := CreatePromptRequest{
		Name:  "existing",
		Title: "Duplicate",
		Messages: []PromptMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "Test"}},
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      24,
		Method:  "prompts/create",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for duplicate prompt")
	}
}

func TestServer_PromptsGet_NotFound(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/get request for non-existent prompt
	params := GetPromptRequest{
		Name:      "nonexistent",
		Arguments: map[string]string{},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      25,
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
		t.Fatal("Expected error for non-existent prompt")
	}
}

func TestServer_PromptsList_WithError(t *testing.T) {
	store := &mockPromptStore{
		listFn: func(cursor string, limit int) ([]promptstore.Prompt, string, error) {
			return nil, "", fmt.Errorf("store error")
		},
	}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/list request
	req := Request{
		JSONRPC: "2.0",
		ID:      26,
		Method:  "prompts/list",
		Params:  json.RawMessage(`{}`),
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error from store")
	}
}

func TestServer_PromptsUpdate_WithMessages(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test": {
				Name:    "test",
				Title:   "Original",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Original",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request with new messages
	params := UpdatePromptRequest{
		Name:  "test",
		Title: "Updated",
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Updated message",
				},
			},
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      27,
		Method:  "prompts/update",
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
}

func TestServer_PromptsCreate_WithAllFields(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/create request with all fields
	params := CreatePromptRequest{
		Name:        "complete",
		Title:       "Complete Prompt",
		Description: "Full description",
		Arguments: []PromptArgument{
			{Name: "arg1", Description: "First arg", Required: true},
			{Name: "arg2", Description: "Second arg", Required: false},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Test {{arg1}} and {{arg2}}",
				},
			},
			{
				Role: "assistant",
				Content: MessageContent{
					Type: "text",
					Text: "Response",
				},
			},
		},
		Tags: []string{"tag1", "tag2"},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      28,
		Method:  "prompts/create",
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
	var result PromptOperationResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true")
	}
}

func TestServer_PromptsList_WithCursorAndLimit(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test1": {
				Name:    "test1",
				Title:   "Test 1",
				Created: now,
				Updated: now,
			},
			"test2": {
				Name:    "test2",
				Title:   "Test 2",
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

	// Send prompts/list request with cursor and limit
	params := map[string]interface{}{
		"cursor": "test1",
		"limit":  10,
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "prompts/list",
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
}

func TestServer_PromptsUpdate_WithDescription(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test": {
				Name:    "test",
				Title:   "Original",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Original",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request with description
	params := UpdatePromptRequest{
		Name:        "test",
		Description: "Updated description",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "prompts/update",
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
}

func TestServer_PromptsUpdate_WithArguments(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test": {
				Name:    "test",
				Title:   "Original",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Original",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request with arguments
	params := UpdatePromptRequest{
		Name: "test",
		Arguments: []PromptArgument{
			{Name: "newarg", Description: "New argument", Required: true},
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "prompts/update",
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
}

func TestServer_PromptsUpdate_WithTags(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"test": {
				Name:    "test",
				Title:   "Original",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Original",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/update request with tags
	params := UpdatePromptRequest{
		Name: "test",
		Tags: []string{"newtag1", "newtag2"},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      33,
		Method:  "prompts/update",
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
}

func TestServer_Run_InvalidJSON(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	logger := log.New(io.Discard, "", 0)
	server := NewServer(inBuf, outBuf, logger, store, nil)

	// Write invalid JSON
	msg := []byte(`{invalid json}`)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg))
	inBuf.WriteString(header)
	inBuf.Write(msg)

	// Run in background
	done := make(chan error, 1)
	go func() {
		done <- server.Run()
	}()

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Should have written error response
	if outBuf.Len() == 0 {
		t.Error("Expected error response to be written")
	}
}

func TestServer_PromptsCreate_MissingMessages(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send prompts/create request without messages
	params := CreatePromptRequest{
		Name:  "test",
		Title: "Test",
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      34,
		Method:  "prompts/create",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for missing messages")
	}
}

func TestServer_HandleInitialize_InvalidParams(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Send initialize request with invalid params
	req := Request{
		JSONRPC: "2.0",
		ID:      35,
		Method:  "initialize",
		Params:  json.RawMessage(`{invalid}`),
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for invalid params")
	}
}

// ==================== Tools Tests ====================

func TestServer_ToolsList(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/list request
	req := Request{
		JSONRPC: "2.0",
		ID:      40,
		Method:  "tools/list",
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
	var result ListToolsResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	// Verify 3 tools returned
	if len(result.Tools) != 3 {
		t.Errorf("Tools count = %d, want 3", len(result.Tools))
	}

	// Verify tool names
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("Tool %s has nil InputSchema", tool.Name)
		}
	}

	expectedTools := []string{"create_prompt", "update_prompt", "delete_prompt"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Missing expected tool: %s", name)
		}
	}
}

func TestServer_ToolsCall_CreatePrompt(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call request
	params := CallToolRequest{
		Name: "create_prompt",
		Arguments: map[string]interface{}{
			"name":  "tool_test",
			"title": "Tool Test Prompt",
			"messages": []interface{}{
				map[string]interface{}{
					"role": "user",
					"content": map[string]interface{}{
						"type": "text",
						"text": "Test message",
					},
				},
			},
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      41,
		Method:  "tools/call",
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
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if result.IsError {
		t.Errorf("IsError = true, want false. Content: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	// Verify prompt was created
	if _, exists := store.prompts["tool_test"]; !exists {
		t.Error("Prompt was not created in store")
	}
}

func TestServer_ToolsCall_UpdatePrompt(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"tool_update": {
				Name:    "tool_update",
				Title:   "Original Title",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Original",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call request
	params := CallToolRequest{
		Name: "update_prompt",
		Arguments: map[string]interface{}{
			"name":  "tool_update",
			"title": "Updated Title",
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      42,
		Method:  "tools/call",
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
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if result.IsError {
		t.Errorf("IsError = true, want false. Content: %v", result.Content)
	}

	// Verify prompt was updated
	if store.prompts["tool_update"].Title != "Updated Title" {
		t.Errorf("Title not updated, got %s", store.prompts["tool_update"].Title)
	}
}

func TestServer_ToolsCall_DeletePrompt(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"tool_delete": {
				Name:    "tool_delete",
				Title:   "To Delete",
				Created: now,
				Updated: now,
				Messages: []promptstore.PromptMessage{
					{
						Role: "user",
						Content: promptstore.MessageContent{
							Type: "text",
							Text: "Test",
						},
					},
				},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call request
	params := CallToolRequest{
		Name: "delete_prompt",
		Arguments: map[string]interface{}{
			"name": "tool_delete",
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      43,
		Method:  "tools/call",
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
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if result.IsError {
		t.Errorf("IsError = true, want false. Content: %v", result.Content)
	}

	// Verify prompt was deleted
	if _, exists := store.prompts["tool_delete"]; exists {
		t.Error("Prompt was not deleted from store")
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call request with unknown tool
	params := CallToolRequest{
		Name:      "nonexistent_tool",
		Arguments: map[string]interface{}{},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      44,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	// Should not be a protocol error
	if resp.Error != nil {
		t.Fatalf("Unexpected protocol error: %v", resp.Error)
	}

	// Parse result - should be a tool error
	resultBytes, _ := json.Marshal(resp.Result)
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true for unknown tool")
	}
}

func TestServer_ToolsCall_InvalidArguments(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call with invalid arguments (missing required fields)
	params := CallToolRequest{
		Name: "create_prompt",
		Arguments: map[string]interface{}{
			"name": "test", // Missing title and messages
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      45,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	// Should not be a protocol error
	if resp.Error != nil {
		t.Fatalf("Unexpected protocol error: %v", resp.Error)
	}

	// Parse result - should be a tool error
	resultBytes, _ := json.Marshal(resp.Result)
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true for invalid arguments")
	}
}

func TestServer_ToolsCall_NotInitialized(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// DO NOT initialize server

	// Send tools/call request
	params := CallToolRequest{
		Name:      "create_prompt",
		Arguments: map[string]interface{}{},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      46,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for uninitialized server")
	}

	if resp.Error.Code != ErrCodeInvalidRequest {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, ErrCodeInvalidRequest)
	}
}

func TestServer_ToolsCall_CreatePrompt_AlreadyExists(t *testing.T) {
	now := time.Now()
	store := &mockPromptStore{
		prompts: map[string]*promptstore.Prompt{
			"existing": {
				Name:     "existing",
				Title:    "Existing",
				Created:  now,
				Updated:  now,
				Messages: []promptstore.PromptMessage{},
			},
		},
	}

	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call to create duplicate
	params := CallToolRequest{
		Name: "create_prompt",
		Arguments: map[string]interface{}{
			"name":  "existing",
			"title": "Duplicate",
			"messages": []interface{}{
				map[string]interface{}{
					"role": "user",
					"content": map[string]interface{}{
						"type": "text",
						"text": "Test",
					},
				},
			},
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      47,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	// Should not be a protocol error
	if resp.Error != nil {
		t.Fatalf("Unexpected protocol error: %v", resp.Error)
	}

	// Parse result - should be a tool error
	resultBytes, _ := json.Marshal(resp.Result)
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true for duplicate prompt")
	}
}

func TestServer_Initialize_AdvertisesTools(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Send initialize request
	params := InitializeRequest{
		ProtocolVersion: "2025-11-25",
		Capabilities:    ClientCapabilities{},
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0",
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      48,
		Method:  "initialize",
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
	var result InitializeResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	// Verify Tools capability is advertised
	if result.Capabilities.Tools == nil {
		t.Fatal("Tools capability not advertised")
	}

	// Verify Prompts capability is also still advertised
	if result.Capabilities.Prompts == nil {
		t.Fatal("Prompts capability not advertised")
	}
}

func TestServer_ToolsList_NotInitialized(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// DO NOT initialize server

	// Send tools/list request
	req := Request{
		JSONRPC: "2.0",
		ID:      49,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for uninitialized server")
	}

	if resp.Error.Code != ErrCodeInvalidRequest {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, ErrCodeInvalidRequest)
	}
}

func TestServer_ToolsCall_UpdatePrompt_NotFound(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call to update non-existent prompt
	params := CallToolRequest{
		Name: "update_prompt",
		Arguments: map[string]interface{}{
			"name":  "nonexistent",
			"title": "Updated",
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      50,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	// Should not be a protocol error
	if resp.Error != nil {
		t.Fatalf("Unexpected protocol error: %v", resp.Error)
	}

	// Parse result - should be a tool error
	resultBytes, _ := json.Marshal(resp.Result)
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true for non-existent prompt")
	}
}

func TestServer_ToolsCall_DeletePrompt_NotFound(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call to delete non-existent prompt
	params := CallToolRequest{
		Name: "delete_prompt",
		Arguments: map[string]interface{}{
			"name": "nonexistent",
		},
	}
	paramsBytes, _ := json.Marshal(params)
	req := Request{
		JSONRPC: "2.0",
		ID:      51,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	// Should not be a protocol error
	if resp.Error != nil {
		t.Fatalf("Unexpected protocol error: %v", resp.Error)
	}

	// Parse result - should be a tool error
	resultBytes, _ := json.Marshal(resp.Result)
	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true for non-existent prompt")
	}
}

func TestServer_ToolsCall_InvalidParams(t *testing.T) {
	store := &mockPromptStore{prompts: make(map[string]*promptstore.Prompt)}
	server, _, outBuf := createTestServer(t, store)

	// Initialize server
	server.mu.Lock()
	server.initialized = true
	server.mu.Unlock()

	// Send tools/call with invalid JSON params
	req := Request{
		JSONRPC: "2.0",
		ID:      52,
		Method:  "tools/call",
		Params:  json.RawMessage(`{invalid}`),
	}

	server.handle(req)

	// Read response
	resp, err := readResponse(outBuf)
	if err != nil {
		t.Fatalf("readResponse() error = %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected protocol error for invalid params")
	}

	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}
