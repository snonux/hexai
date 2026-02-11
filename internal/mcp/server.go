// Summary: MCP server over stdio; manages prompt store, dispatches requests, and handles protocol.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/promptstore"
)

// Server implements an MCP server over stdio using JSON-RPC 2.0.
// Follows the same pattern as the LSP server with dispatch table and thread safety.
type Server struct {
	in          *bufio.Reader
	out         io.Writer
	outMu       sync.Mutex
	logger      *log.Logger
	store       promptstore.PromptStore
	initialized bool
	mu          sync.RWMutex

	// Dispatch table for JSON-RPC methods
	handlers map[string]func(Request)
}

// NewServer creates a new MCP server with the given store and I/O streams.
// The store provides access to prompts; logger is used for debugging.
func NewServer(r io.Reader, w io.Writer, logger *log.Logger, store promptstore.PromptStore) *Server {
	s := &Server{
		in:     bufio.NewReader(r),
		out:    w,
		logger: logger,
		store:  store,
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
		"notifications/initialized": s.handleInitialized,
	}

	return s
}

// Run starts the server main loop, reading and dispatching requests.
// Returns on EOF or fatal error.
func (s *Server) Run() error {
	for {
		body, err := s.readMessage()
		if err == io.EOF {
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
				ListChanged: false,
				Mutable:     true, // Advertise that we support create/update/delete
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
	s.sendResponse(req.ID, result)
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
	s.sendResponse(req.ID, result)
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
	s.sendResponse(req.ID, result)
}
