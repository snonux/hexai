// Summary: MCP protocol types for JSON-RPC 2.0 messages and MCP-specific structures.
package mcp

import "encoding/json"

// Request represents an MCP JSON-RPC 2.0 request from the client.
// All MCP communication follows JSON-RPC 2.0 specification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"` // Always "2.0"
	ID      any             `json:"id"`      // Request ID (string or number)
	Method  string          `json:"method"`  // Method name
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC 2.0 response to the client.
// Contains either a result or an error, never both.
type Response struct {
	JSONRPC string     `json:"jsonrpc"` // Always "2.0"
	ID      any        `json:"id"`      // Matching request ID
	Result  any        `json:"result,omitempty"`
	Error   *RespError `json:"error,omitempty"`
}

// RespError represents a JSON-RPC error with code and message.
// Follows JSON-RPC 2.0 error object specification.
type RespError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC error codes
const (
	ErrCodeParseError     = -32700 // Invalid JSON
	ErrCodeInvalidRequest = -32600 // Invalid Request structure
	ErrCodeMethodNotFound = -32601 // Method doesn't exist
	ErrCodeInvalidParams  = -32602 // Invalid parameters
	ErrCodeInternalError  = -32603 // Server internal error
)

// LatestProtocolVersion is the newest MCP protocol version this server supports.
const LatestProtocolVersion = "2025-11-25"

// ValidProtocolVersions lists all MCP protocol versions this server accepts.
// Server echoes back the client's version if valid; otherwise responds with latest.
var ValidProtocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

// InitializeRequest is the first message from client to establish connection.
// Client sends capabilities and protocol version; server responds with its capabilities.
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"` // "2025-06-18"
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
	Meta            map[string]interface{} `json:"_meta,omitempty"`
}

// ClientCapabilities describes what features the client supports.
// Used during handshake to negotiate supported features.
type ClientCapabilities struct {
	Sampling map[string]interface{} `json:"sampling,omitempty"`
}

// ClientInfo identifies the connecting client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the server's response to initialize.
// Contains server capabilities and identification.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes what features this server supports.
// Currently only prompts are supported; tools/resources reserved for future.
type ServerCapabilities struct {
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
}

// PromptsCapability indicates server supports prompt listing and retrieval.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // Server notifies when prompts change
	Mutable     bool `json:"mutable,omitempty"`     // Server supports create/update/delete
}

// ResourcesCapability indicates server supports resource listing and retrieval.
// Reserved for future use (runbooks, documentation files).
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability indicates server supports tool execution.
// Reserved for future use (script execution, git operations).
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo identifies this server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListPromptsRequest contains parameters for prompts/list method.
// Supports pagination via cursor-based iteration.
type ListPromptsRequest struct {
	Cursor string `json:"cursor,omitempty"` // Pagination cursor (empty for first page)
}

// ListPromptsResult contains paginated list of available prompts.
// Includes nextCursor for fetching additional pages.
type ListPromptsResult struct {
	Prompts    []PromptInfo `json:"prompts"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

// PromptInfo describes a prompt in the list (metadata only, no messages).
// Client uses this to display available prompts before fetching full content.
type PromptInfo struct {
	Name        string           `json:"name"`                  // Unique identifier
	Title       string           `json:"title,omitempty"`       // Display name
	Description string           `json:"description,omitempty"` // Human-readable
	Arguments   []PromptArgument `json:"arguments,omitempty"`   // Template variables
}

// PromptArgument describes a template variable in a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// GetPromptRequest contains parameters for prompts/get method.
// Includes argument values to render the prompt template.
type GetPromptRequest struct {
	Name      string            `json:"name"`                // Prompt identifier
	Arguments map[string]string `json:"arguments,omitempty"` // Argument values
}

// GetPromptResult contains a fully rendered prompt ready for use.
// Template variables have been substituted with provided arguments.
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage represents a message in the rendered prompt.
type PromptMessage struct {
	Role    string         `json:"role"` // "user" or "assistant"
	Content MessageContent `json:"content"`
}

// MessageContent contains the message text or other content types.
type MessageContent struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
}

// CreatePromptRequest contains parameters for prompts/create method.
type CreatePromptRequest struct {
	Name        string           `json:"name"`
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Messages    []PromptMessage  `json:"messages"`
	Tags        []string         `json:"tags,omitempty"`
}

// UpdatePromptRequest contains parameters for prompts/update method.
type UpdatePromptRequest struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Messages    []PromptMessage  `json:"messages,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
}

// DeletePromptRequest contains parameters for prompts/delete method.
type DeletePromptRequest struct {
	Name string `json:"name"`
}

// Tool defines an MCP tool that can be invoked via tools/call.
// Each tool has a name, description, and JSON Schema for input validation.
type Tool struct {
	Name        string                 `json:"name"`        // Unique tool identifier
	Description string                 `json:"description"` // Human-readable description
	InputSchema map[string]interface{} `json:"inputSchema"` // JSON Schema for arguments
}

// ListToolsRequest contains parameters for tools/list method.
// Supports pagination via cursor-based iteration.
type ListToolsRequest struct {
	Cursor string `json:"cursor,omitempty"` // Pagination cursor (empty for first page)
}

// ListToolsResult contains list of available tools.
// Includes nextCursor for fetching additional pages if needed.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`                // Available tools
	NextCursor string `json:"nextCursor,omitempty"` // Pagination cursor
}

// CallToolRequest contains parameters for tools/call method.
// Specifies which tool to execute and its input arguments.
type CallToolRequest struct {
	Name      string                 `json:"name"`                // Tool name to execute
	Arguments map[string]interface{} `json:"arguments,omitempty"` // Tool input arguments
}

// CallToolResult contains the result of a tool execution.
// Returns content (text output) and optional error flag.
type CallToolResult struct {
	Content []ToolContent `json:"content"`           // Tool output content
	IsError bool          `json:"isError,omitempty"` // True if tool execution failed
}

// ToolContent represents content returned by a tool.
// Currently only text content is supported.
type ToolContent struct {
	Type string `json:"type"` // Content type (e.g., "text")
	Text string `json:"text"` // Text content
}

// PromptOperationResult indicates success/failure of create/update/delete.
type PromptOperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
