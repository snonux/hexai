// Summary: LSP protocol types used by the server (requests, responses, params, capabilities).
package lsp

import "encoding/json"

// Request represents a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RespError      `json:"error,omitempty"`
}

// RespError represents the error object in a JSON-RPC 2.0 response.
type RespError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// InitializeResult is the response payload for the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

// ServerInfo describes the LSP server's name and version.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// ServerCapabilities declares the features the server supports.
type ServerCapabilities struct {
	TextDocumentSync   any                `json:"textDocumentSync,omitempty"`
	CompletionProvider *CompletionOptions `json:"completionProvider,omitempty"`
	// bool | CodeActionOptions
	CodeActionProvider any `json:"codeActionProvider,omitempty"`
}

// CompletionOptions configures the server's completion provider capabilities.
type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// CompletionList contains a set of completion items returned by the server.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionItem represents a single completion suggestion.
type CompletionItem struct {
	Label               string     `json:"label"`
	Kind                int        `json:"kind,omitempty"`
	Detail              string     `json:"detail,omitempty"`
	InsertText          string     `json:"insertText,omitempty"`
	InsertTextFormat    int        `json:"insertTextFormat,omitempty"`
	FilterText          string     `json:"filterText,omitempty"`
	TextEdit            *TextEdit  `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit `json:"additionalTextEdits,omitempty"`
	SortText            string     `json:"sortText,omitempty"`
	Documentation       string     `json:"documentation,omitempty"`
}

// CodeActionOptions configures the server's code action provider capabilities.
type CodeActionOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// TextDocumentItem represents the content of an opened text document.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId,omitempty"`
	Version    int    `json:"version,omitempty"`
	Text       string `json:"text"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a text document.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version,omitempty"`
}

// TextDocumentIdentifier identifies a text document by its URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// DidOpenTextDocumentParams is the parameter for textDocument/didOpen notifications.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentContentChangeEvent describes a change to a text document's content.
type TextDocumentContentChangeEvent struct {
	Range       any    `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// DidChangeTextDocumentParams is the parameter for textDocument/didChange notifications.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams is the parameter for textDocument/didClose notifications.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// Position represents a zero-based line and character offset in a document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// CompletionParams is the parameter for textDocument/completion requests.
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      any                    `json:"context,omitempty"`
}

// CodeActionParams is the parameter for textDocument/codeAction requests.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      json.RawMessage        `json:"context,omitempty"`
}

// WorkspaceEdit represents changes to many resources managed in the workspace.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []any                 `json:"documentChanges,omitempty"`
}

// ApplyWorkspaceEditParams is the client request payload for workspace/applyEdit.
type ApplyWorkspaceEditParams struct {
	Label string        `json:"label,omitempty"`
	Edit  WorkspaceEdit `json:"edit"`
}

// CodeAction represents an action the server can perform on the user's behalf.
type CodeAction struct {
	Title   string          `json:"title"`
	Kind    string          `json:"kind,omitempty"`
	Edit    *WorkspaceEdit  `json:"edit,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Command *Command        `json:"command,omitempty"`
}

// TextDocumentEdit describes edits to a single versioned text document.
type TextDocumentEdit struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                      `json:"edits"`
}

// CreateFile is a workspace edit operation that creates a new file.
type CreateFile struct {
	Kind string `json:"kind"`
	URI  string `json:"uri"`
}

// Command represents an LSP command that can be executed by the client.
type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

// ExecuteCommandParams is the parameter for workspace/executeCommand requests.
type ExecuteCommandParams struct {
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

// Diagnostic represents a compiler diagnostic such as an error or warning.
type Diagnostic struct {
	Range    Range       `json:"range"`
	Message  string      `json:"message"`
	Severity int         `json:"severity,omitempty"`
	Code     interface{} `json:"code,omitempty"`
	Source   string      `json:"source,omitempty"`
}

// CodeActionContext carries diagnostics associated with a code action request.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Range defines a text range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextEdit represents a textual edit applicable to a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}
