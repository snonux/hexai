// Summary: MCP server over stdio; manages prompt store, dispatches requests, and handles protocol.
package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/promptstore"
)

// SlashCommandSyncer is the minimal sync contract the MCP server depends on.
type SlashCommandSyncer interface {
	SyncCreate(prompt *promptstore.Prompt) error
	SyncUpdate(prompt *promptstore.Prompt) error
	Delete(promptName string) error
}

// Server implements an MCP server over stdio using JSON-RPC 2.0.
// Follows the same pattern as the LSP server with dispatch table and thread safety.
type Server struct {
	in          *bufio.Reader
	out         io.Writer
	outMu       sync.Mutex
	logger      *log.Logger
	store       promptstore.PromptStore
	syncer      SlashCommandSyncer
	initialized bool
	mu          sync.RWMutex

	// Dispatch table for JSON-RPC methods
	handlers map[string]func(Request)
}

// NewServer creates a new MCP server with the given store and I/O streams.
// The store provides access to prompts; logger is used for debugging.
func NewServer(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore, syncer SlashCommandSyncer) *Server {
	s := &Server{
		in:     bufio.NewReader(r),
		out:    w,
		logger: logger,
		store:  store,
		syncer: syncer,
	}

	// Initialize dispatch table
	s.handlers = map[string]func(Request){
		"initialize":                s.handleInitialize,
		"initialized":               s.handleInitialized,
		"prompts/list":              s.handlePromptsList,
		"prompts/get":               s.handlePromptsGet,
		"prompts/create":            s.handlePromptsCreate,
		"prompts/update":            s.handlePromptsUpdate,
		"prompts/delete":            s.handlePromptsDelete,
		"tools/list":                s.handleToolsList,
		"tools/call":                s.handleToolsCall,
		"notifications/initialized": s.handleInitialized,
	}

	return s
}

// Run starts the server main loop, reading and dispatching requests.
// Returns on EOF or fatal error.
func (s *Server) Run() error {
	for {
		body, err := s.readMessage()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			s.logger.Printf("invalid JSON: %v", err)
			s.sendError(nil, ErrCodeParseError, "Parse error")
			continue
		}

		if req.Method == "" {
			// Response from client; ignore
			continue
		}

		// Dispatch request
		go s.handle(req)
	}
}

// handle dispatches a request to the appropriate handler.
func (s *Server) handle(req Request) {
	handler, ok := s.handlers[req.Method]
	if !ok {
		s.logger.Printf("method not found: %s", req.Method)
		s.sendError(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
		return
	}

	handler(req)
}

// handleInitialize processes the initialize request and returns server capabilities.
func (s *Server) handleInitialize(req Request) {
	var params InitializeRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid initialize params")
		return
	}

	s.logger.Printf("initialize from client: %s %s (protocol: %s)",
		params.ClientInfo.Name, params.ClientInfo.Version, params.ProtocolVersion)

	// Negotiate protocol version: echo client's version if valid, otherwise use latest.
	// This follows the MCP spec where the server responds with a version it supports.
	negotiatedVersion := negotiateProtocolVersion(params.ProtocolVersion)
	s.logger.Printf("negotiated protocol version: %s", negotiatedVersion)

	result := InitializeResult{
		ProtocolVersion: negotiatedVersion,
		Capabilities: ServerCapabilities{
			Prompts: &PromptsCapability{
				ListChanged: true, // Server sends notifications when prompt list changes
				Mutable:     true, // Advertise that we support create/update/delete
			},
			Tools: &ToolsCapability{
				ListChanged: false, // Tool list is static (no dynamic changes)
			},
		},
		ServerInfo: ServerInfo{
			Name:    "hexai-mcp-server",
			Version: internal.Version,
		},
	}

	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	s.sendResponse(req.ID, result)
}

// negotiateProtocolVersion returns the client's version if supported,
// otherwise returns the latest version this server supports.
func negotiateProtocolVersion(clientVersion string) string {
	for _, v := range ValidProtocolVersions {
		if v == clientVersion {
			return clientVersion
		}
	}
	return LatestProtocolVersion
}

// handleInitialized processes the initialized notification.
// This is sent by the client after receiving initialize response.
func (s *Server) handleInitialized(_ Request) {
	s.logger.Printf("client sent initialized notification")
	// No response required for notifications
}

// handlePromptsList processes the prompts/list request.
func (s *Server) handlePromptsList(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params ListPromptsRequest
	if req.Params != nil && len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/list params")
			return
		}
	}

	// List prompts from store
	prompts, nextCursor, err := s.store.List(params.Cursor, 100)
	if err != nil {
		s.logger.Printf("store list error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, "Failed to list prompts")
		return
	}

	// Convert to PromptInfo (initialize as empty slice so JSON marshals as [] not null)
	infos := make([]PromptInfo, 0, len(prompts))
	for _, p := range prompts {
		args := make([]PromptArgument, len(p.Arguments))
		for i, a := range p.Arguments {
			args[i] = PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			}
		}
		infos = append(infos, PromptInfo{
			Name:        p.Name,
			Title:       p.Title,
			Description: p.Description,
			Arguments:   args,
		})
	}

	result := ListPromptsResult{
		Prompts:    infos,
		NextCursor: nextCursor,
	}

	s.sendResponse(req.ID, result)
}

// handlePromptsGet processes the prompts/get request.
func (s *Server) handlePromptsGet(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params GetPromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/get params")
		return
	}

	if params.Name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Missing prompt name")
		return
	}

	// Get prompt from store
	prompt, err := s.store.Get(params.Name)
	if err != nil {
		s.logger.Printf("store get error: %v", err)
		s.sendError(req.ID, ErrCodeInvalidParams, fmt.Sprintf("Prompt not found: %s", params.Name))
		return
	}

	// Render prompt with arguments
	messages, err := s.renderPrompt(prompt, params.Arguments)
	if err != nil {
		s.logger.Printf("render error: %v", err)
		s.sendError(req.ID, ErrCodeInvalidParams, err.Error())
		return
	}

	result := GetPromptResult{
		Description: prompt.Description,
		Messages:    messages,
	}

	s.sendResponse(req.ID, result)
}

// renderPrompt substitutes template arguments in prompt messages.
// Returns error if required arguments are missing.
func (s *Server) renderPrompt(prompt *promptstore.Prompt, args map[string]string) ([]PromptMessage, error) {
	// Validate required arguments
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if _, ok := args[arg.Name]; !ok {
				return nil, fmt.Errorf("missing required argument: %s", arg.Name)
			}
		}
	}

	// Render each message
	var rendered []PromptMessage
	for _, msg := range prompt.Messages {
		text := msg.Content.Text

		// Simple template substitution: {{arg}} -> value
		for key, val := range args {
			placeholder := "{{" + key + "}}"
			text = strings.ReplaceAll(text, placeholder, val)
		}

		rendered = append(rendered, PromptMessage{
			Role: msg.Role,
			Content: MessageContent{
				Type: msg.Content.Type,
				Text: text,
			},
		})
	}

	return rendered, nil
}

// sendResponse sends a successful JSON-RPC response.
func (s *Server) sendResponse(id any, result any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	if err := s.writeMessage(resp); err != nil {
		s.logger.Printf("write response error: %v", err)
	}
}

// sendError sends an error JSON-RPC response.
func (s *Server) sendError(id any, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RespError{
			Code:    code,
			Message: message,
		},
	}
	if err := s.writeMessage(resp); err != nil {
		s.logger.Printf("write error response error: %v", err)
	}
}

// handlePromptsCreate processes the prompts/create request.
// Creates a new custom prompt in user.jsonl.
func (s *Server) handlePromptsCreate(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params CreatePromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/create params")
		return
	}

	// Validate required fields
	if err := validateCreateParams(params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, err.Error())
		return
	}

	// Build prompt from params
	prompt := buildPromptFromCreateParams(params)

	if err := s.store.Create(prompt); err != nil {
		s.logger.Printf("create prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, fmt.Sprintf("Failed to create prompt: %v", err))
		return
	}

	result := PromptOperationResult{
		Success: true,
		Message: fmt.Sprintf("Created prompt: %s", params.Name),
	}

	s.logger.Printf("created prompt: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncCreate(prompt); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendResponse(req.ID, result)

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// validateCreateParams validates required fields for prompt creation.
func validateCreateParams(params CreatePromptRequest) error {
	if params.Name == "" {
		return fmt.Errorf("prompt name is required")
	}
	if params.Title == "" {
		return fmt.Errorf("prompt title is required")
	}
	if len(params.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}
	return nil
}

// buildPromptFromCreateParams converts CreatePromptRequest to Prompt.
func buildPromptFromCreateParams(params CreatePromptRequest) *promptstore.Prompt {
	prompt := &promptstore.Prompt{
		Name:        params.Name,
		Title:       params.Title,
		Description: params.Description,
		Tags:        params.Tags,
		Created:     time.Now(),
		Updated:     time.Now(),
	}

	// Convert arguments
	for _, arg := range params.Arguments {
		prompt.Arguments = append(prompt.Arguments, promptstore.PromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}

	// Convert messages
	for _, msg := range params.Messages {
		prompt.Messages = append(prompt.Messages, promptstore.PromptMessage{
			Role: msg.Role,
			Content: promptstore.MessageContent{
				Type: msg.Content.Type,
				Text: msg.Content.Text,
			},
		})
	}

	return prompt
}

// handlePromptsUpdate processes the prompts/update request.
// Updates an existing custom prompt in user.jsonl.
func (s *Server) handlePromptsUpdate(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params UpdatePromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/update params")
		return
	}

	if params.Name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Prompt name is required")
		return
	}

	// Get existing prompt
	existing, err := s.store.Get(params.Name)
	if err != nil {
		s.logger.Printf("get prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInvalidParams, fmt.Sprintf("Prompt not found: %s", params.Name))
		return
	}

	// Apply updates to existing prompt
	applyPromptUpdates(existing, params)

	if err := s.store.Update(existing); err != nil {
		s.logger.Printf("update prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, fmt.Sprintf("Failed to update prompt: %v", err))
		return
	}

	result := PromptOperationResult{
		Success: true,
		Message: fmt.Sprintf("Updated prompt: %s", params.Name),
	}

	s.logger.Printf("updated prompt: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncUpdate(existing); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendResponse(req.ID, result)

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// applyPromptUpdates applies update parameters to an existing prompt.
func applyPromptUpdates(existing *promptstore.Prompt, params UpdatePromptRequest) {
	// Update fields (only if provided)
	if params.Title != "" {
		existing.Title = params.Title
	}
	if params.Description != "" {
		existing.Description = params.Description
	}
	if len(params.Arguments) > 0 {
		existing.Arguments = nil
		for _, arg := range params.Arguments {
			existing.Arguments = append(existing.Arguments, promptstore.PromptArgument{
				Name:        arg.Name,
				Description: arg.Description,
				Required:    arg.Required,
			})
		}
	}
	if len(params.Messages) > 0 {
		existing.Messages = nil
		for _, msg := range params.Messages {
			existing.Messages = append(existing.Messages, promptstore.PromptMessage{
				Role: msg.Role,
				Content: promptstore.MessageContent{
					Type: msg.Content.Type,
					Text: msg.Content.Text,
				},
			})
		}
	}
	if len(params.Tags) > 0 {
		existing.Tags = params.Tags
	}

	existing.Updated = time.Now()
}

// handlePromptsDelete processes the prompts/delete request.
// Deletes a custom prompt from user.jsonl.
func (s *Server) handlePromptsDelete(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params DeletePromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/delete params")
		return
	}

	if params.Name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Prompt name is required")
		return
	}

	if err := s.store.Delete(params.Name); err != nil {
		s.logger.Printf("delete prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, fmt.Sprintf("Failed to delete prompt: %v", err))
		return
	}

	result := PromptOperationResult{
		Success: true,
		Message: fmt.Sprintf("Deleted prompt: %s", params.Name),
	}

	s.logger.Printf("deleted prompt: %s", params.Name)

	// Delete slash command file if enabled
	if s.syncer != nil {
		if err := s.syncer.Delete(params.Name); err != nil {
			s.logger.Printf("slash command sync delete failed: %v", err)
		}
	}

	s.sendResponse(req.ID, result)

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// handleToolsList processes the tools/list request.
// Returns available tools for prompt management operations.
func (s *Server) handleToolsList(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	// Define 3 tools for prompt management
	tools := []Tool{
		s.makeCreatePromptTool(),
		s.makeUpdatePromptTool(),
		s.makeDeletePromptTool(),
	}

	result := ListToolsResult{
		Tools:      tools,
		NextCursor: "", // No pagination needed for 3 tools
	}

	s.sendResponse(req.ID, result)
}

// makeCreatePromptTool returns the JSON Schema definition for create_prompt tool.
func (s *Server) makeCreatePromptTool() Tool {
	return Tool{
		Name:        "create_prompt",
		Description: "Creates a new custom prompt template that can be invoked via the MCP prompts capability. The prompt is saved to user.jsonl and becomes immediately available for use.",
		InputSchema: s.buildCreatePromptSchema(),
	}
}

// buildCreatePromptSchema constructs the JSON Schema for create_prompt tool inputs.
func (s *Server) buildCreatePromptSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Unique identifier for the prompt (lowercase, underscores allowed)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable display name for the prompt",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Detailed explanation of what the prompt does",
			},
			"arguments": buildArgumentArraySchema(),
			"messages":  buildMessageArraySchema(),
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "Optional tags for categorizing the prompt",
				"items":       map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"name", "title", "messages"},
	}
}

// makeUpdatePromptTool returns the JSON Schema definition for update_prompt tool.
func (s *Server) makeUpdatePromptTool() Tool {
	return Tool{
		Name:        "update_prompt",
		Description: "Updates an existing custom prompt template. Only custom prompts (stored in user.jsonl) can be updated; built-in prompts cannot be modified.",
		InputSchema: s.buildUpdatePromptSchema(),
	}
}

// buildUpdatePromptSchema constructs the JSON Schema for update_prompt tool inputs.
func (s *Server) buildUpdatePromptSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the existing prompt to update",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "New display name (optional, only if changing)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "New description (optional, only if changing)",
			},
			"arguments": buildArgumentArraySchema(),
			"messages":  buildMessageArraySchema(),
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "New tags array (optional, replaces existing if provided)",
				"items":       map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"name"},
	}
}

// makeDeletePromptTool returns the JSON Schema definition for delete_prompt tool.
func (s *Server) makeDeletePromptTool() Tool {
	return Tool{
		Name:        "delete_prompt",
		Description: "Deletes a custom prompt template. Only custom prompts (stored in user.jsonl) can be deleted; built-in prompts cannot be removed.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the prompt to delete",
				},
			},
			"required": []string{"name"},
		},
	}
}

// handleToolsCall processes the tools/call request.
// Dispatches to appropriate tool wrapper based on tool name.
func (s *Server) handleToolsCall(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params CallToolRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid tools/call params")
		return
	}

	// Dispatch to appropriate tool wrapper
	switch params.Name {
	case "create_prompt":
		s.callCreatePromptTool(req.ID, params.Arguments)
	case "update_prompt":
		s.callUpdatePromptTool(req.ID, params.Arguments)
	case "delete_prompt":
		s.callDeletePromptTool(req.ID, params.Arguments)
	default:
		s.sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

// callCreatePromptTool executes the create_prompt tool.
// Converts map arguments to CreatePromptRequest and delegates to store.Create.
func (s *Server) callCreatePromptTool(id any, args map[string]interface{}) {
	// Convert map to CreatePromptRequest
	params, err := convertToCreatePromptRequest(args)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	// Validate required fields
	if err := validateCreateParams(params); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	// Build prompt and create
	prompt := buildPromptFromCreateParams(params)
	if err := s.store.Create(prompt); err != nil {
		s.logger.Printf("create prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Failed to create prompt: %v", err))
		return
	}

	s.logger.Printf("created prompt via tool: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncCreate(prompt); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendToolSuccess(id, fmt.Sprintf("Successfully created prompt: %s", params.Name))

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// callUpdatePromptTool executes the update_prompt tool.
// Converts map arguments to UpdatePromptRequest and delegates to store.Update.
func (s *Server) callUpdatePromptTool(id any, args map[string]interface{}) {
	// Convert map to UpdatePromptRequest
	params, err := convertToUpdatePromptRequest(args)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	if params.Name == "" {
		s.sendToolError(id, "Prompt name is required")
		return
	}

	// Get existing prompt
	existing, err := s.store.Get(params.Name)
	if err != nil {
		s.logger.Printf("get prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Prompt not found: %s", params.Name))
		return
	}

	// Apply updates
	applyPromptUpdates(existing, params)

	if err := s.store.Update(existing); err != nil {
		s.logger.Printf("update prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Failed to update prompt: %v", err))
		return
	}

	s.logger.Printf("updated prompt via tool: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncUpdate(existing); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendToolSuccess(id, fmt.Sprintf("Successfully updated prompt: %s", params.Name))

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// callDeletePromptTool executes the delete_prompt tool.
// Extracts name from arguments and delegates to store.Delete.
func (s *Server) callDeletePromptTool(id any, args map[string]interface{}) {
	// Extract name from arguments
	name, ok := args["name"].(string)
	if !ok || name == "" {
		s.sendToolError(id, "Prompt name is required")
		return
	}

	if err := s.store.Delete(name); err != nil {
		s.logger.Printf("delete prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Failed to delete prompt: %v", err))
		return
	}

	s.logger.Printf("deleted prompt via tool: %s", name)

	// Delete slash command file if enabled
	if s.syncer != nil {
		if err := s.syncer.Delete(name); err != nil {
			s.logger.Printf("slash command sync delete failed: %v", err)
		}
	}

	s.sendToolSuccess(id, fmt.Sprintf("Successfully deleted prompt: %s", name))

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// sendToolSuccess sends a successful tool result.
func (s *Server) sendToolSuccess(id any, message string) {
	result := CallToolResult{
		Content: []ToolContent{{Type: "text", Text: message}},
		IsError: false,
	}
	s.sendResponse(id, result)
}

// sendToolError sends a tool error result (business logic error, not protocol error).
func (s *Server) sendToolError(id any, message string) {
	result := CallToolResult{
		Content: []ToolContent{{Type: "text", Text: message}},
		IsError: true,
	}
	s.sendResponse(id, result)
}

// convertToCreatePromptRequest converts map arguments to CreatePromptRequest.
func convertToCreatePromptRequest(args map[string]interface{}) (CreatePromptRequest, error) {
	// Marshal to JSON then unmarshal to struct for type conversion
	data, err := json.Marshal(args)
	if err != nil {
		return CreatePromptRequest{}, fmt.Errorf("marshal args: %w", err)
	}

	var req CreatePromptRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return CreatePromptRequest{}, fmt.Errorf("unmarshal to CreatePromptRequest: %w", err)
	}

	return req, nil
}

// convertToUpdatePromptRequest converts map arguments to UpdatePromptRequest.
func convertToUpdatePromptRequest(args map[string]interface{}) (UpdatePromptRequest, error) {
	// Marshal to JSON then unmarshal to struct for type conversion
	data, err := json.Marshal(args)
	if err != nil {
		return UpdatePromptRequest{}, fmt.Errorf("marshal args: %w", err)
	}

	var req UpdatePromptRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return UpdatePromptRequest{}, fmt.Errorf("unmarshal to UpdatePromptRequest: %w", err)
	}

	return req, nil
}

// buildArgumentArraySchema creates the JSON Schema for prompt arguments array.
func buildArgumentArraySchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": "Template variables that can be substituted when invoking the prompt",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Argument name (used in {{name}} placeholders)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the argument's purpose",
				},
				"required": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this argument must be provided",
				},
			},
			"required": []string{"name"},
		},
	}
}

// buildMessageArraySchema creates the JSON Schema for prompt messages array.
func buildMessageArraySchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": "Array of message objects (role + content) that form the prompt",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"role": map[string]interface{}{
					"type":        "string",
					"description": "Message role: 'user' or 'assistant'",
					"enum":        []string{"user", "assistant"},
				},
				"content": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Content type (currently only 'text' is supported)",
							"enum":        []string{"text"},
						},
						"text": map[string]interface{}{
							"type":        "string",
							"description": "The message text (can include {{arg}} placeholders)",
						},
					},
					"required": []string{"type", "text"},
				},
			},
			"required": []string{"role", "content"},
		},
	}
}

// sendPromptsListChangedNotification notifies the client that the prompt list has changed.
// This allows clients to refresh their cached prompt lists.
func (s *Server) sendPromptsListChangedNotification() {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/prompts/list_changed",
	}

	if err := s.writeMessage(notification); err != nil {
		s.logger.Printf("failed to send prompts/list_changed notification: %v", err)
	}
}
