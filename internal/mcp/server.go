// Package mcp provides the MCP server core — struct definition, constructor, main loop, and message dispatch.
package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

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
	inflight    sync.WaitGroup // tracks handler goroutines; Run waits before returning

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

	// Initialize dispatch table mapping JSON-RPC methods to handler functions
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
// Returns on EOF or fatal error, after waiting for all in-flight handlers.
func (s *Server) Run() error {
	for {
		body, err := s.readMessage()
		if errors.Is(err, io.EOF) {
			s.inflight.Wait() // drain handlers before signalling callers
			return nil
		}
		if err != nil {
			s.inflight.Wait()
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

		// Dispatch request in a goroutine, tracked so Run can wait on completion.
		s.inflight.Add(1)
		go func(r Request) {
			defer s.inflight.Done()
			s.handle(r)
		}(req)
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
