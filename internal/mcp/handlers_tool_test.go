// Summary: Tests for MCP tool handlers (tools/list, tools/call for create/update/delete)
package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/promptstore"
)

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
