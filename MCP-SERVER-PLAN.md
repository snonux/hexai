# Plan: hexai-mcp-server - MCP Server for Prompts and Runbooks

## Context

This change adds a new MCP (Model Context Protocol) server to hexai for managing prompts and runbooks. Currently, hexai has prompts hardcoded in the configuration file, making it difficult to share, version, and dynamically manage reusable prompts. An MCP server provides a standardized protocol for AI agents (like Claude Code CLI, Cursor) to discover and use prompts/runbooks from hexai.

**Why this is needed:**
- Centralized prompt management: Store, update, search, and retrieve prompts
- Agent-agnostic: Works with any MCP-compatible agent (Claude Code, Cursor, etc.)
- Follows MCP specification: Industry-standard protocol for prompt/tool/resource discovery
- Reuses hexai patterns: Leverages existing LSP server architecture and JSONL storage

**Intended outcome:**
- New `hexai-mcp-server` binary that implements MCP protocol over stdio
- File-based prompt storage using JSONL format (similar to tmux-edit history)
- Easy installation for agents via config files
- 80%+ unit test coverage with testable architecture

---

## Architecture Overview

### Command Structure
Following hexai's multi-binary pattern:

```
cmd/hexai-mcp-server/
└── main.go                      # Flag parsing + delegation (~60 lines)
                                 # Flags: --log, --config, --prompts-dir, --version

internal/hexaimcp/
├── run.go                       # Main orchestrator (~150 lines)
├── run_test.go                  # Unit tests
└── testhelpers_test.go         # Test helpers and mocks

internal/mcp/
├── server.go                    # MCP server loop + dispatch (~400 lines)
├── server_test.go              # Server tests
├── transport.go                 # JSON-RPC transport (~70 lines, reuses LSP pattern)
├── transport_test.go           # Transport tests
├── types.go                     # MCP protocol types (~200 lines)
├── handlers.go                  # Protocol handlers (~300 lines)
├── handlers_test.go            # Handler tests
└── testhelpers_test.go         # Test mocks and helpers

internal/promptstore/
├── store.go                     # Prompt storage interface + impl (~200 lines)
├── store_test.go               # Store tests (90%+ coverage)
├── types.go                     # Prompt data models (~100 lines)
├── jsonl.go                     # JSONL read/write (~150 lines)
├── jsonl_test.go               # JSONL tests
└── builtin.go                   # Built-in prompts (~100 lines)
```

---

## Storage Design: JSONL-Based

**Default Storage Location:**
```
~/.local/share/hexai/prompts/     (XDG_DATA_HOME)
├── default.jsonl                  # System/built-in prompts
└── user.jsonl                     # User-created prompts
```

**Configurable via:**
1. **Command-line flag**: `--prompts-dir /path/to/prompts`
2. **Config file**: `mcp_prompts_dir = "/path/to/prompts"` in config.toml
3. **Environment variable**: `HEXAI_MCP_PROMPTS_DIR=/path/to/prompts`
4. **Default**: `$XDG_DATA_HOME/hexai/prompts/` or `~/.local/share/hexai/prompts/`

**Precedence order** (highest to lowest):
1. Command-line flag
2. Environment variable
3. Config file
4. Default XDG location

**Rationale:**
- Matches existing pattern in `internal/tmuxedit/history.go`
- Human-readable and editable
- Git-friendly for version control
- No database dependencies
- Atomic append operations
- Easy backup and sync
- Allows project-specific prompt collections
- Enables shared/network storage if needed

**JSONL Format (one prompt per line):**
```json
{"name":"code_review","title":"Request Code Review","description":"Analyzes code quality","arguments":[{"name":"code","description":"Code to review","required":true}],"messages":[{"role":"user","content":{"type":"text","text":"Review: {{code}}"}}],"tags":["development","review"],"created":"2026-02-10T12:00:00Z","updated":"2026-02-10T12:00:00Z"}
```

---

## Data Model

### Core Types

```go
// internal/promptstore/types.go

// Prompt represents a reusable prompt template with arguments
type Prompt struct {
    Name        string           `json:"name"`        // Unique identifier (alphanumeric + underscores)
    Title       string           `json:"title"`       // Display name
    Description string           `json:"description"` // Human-readable description
    Arguments   []PromptArgument `json:"arguments"`   // Template variables
    Messages    []PromptMessage  `json:"messages"`    // Conversation messages
    Tags        []string         `json:"tags"`        // Categorization tags
    Created     time.Time        `json:"created"`     // Creation timestamp
    Updated     time.Time        `json:"updated"`     // Last update timestamp
}

// PromptArgument defines a template variable
type PromptArgument struct {
    Name        string `json:"name"`        // Variable name (used in {{name}})
    Description string `json:"description"` // Human-readable description
    Required    bool   `json:"required"`    // Whether argument is required
}

// PromptMessage represents a conversation message
type PromptMessage struct {
    Role    string         `json:"role"`    // "user" or "assistant"
    Content MessageContent `json:"content"` // Message content
}

// MessageContent contains the actual message data
type MessageContent struct {
    Type string `json:"type"` // "text", "image", "resource"
    Text string `json:"text,omitempty"`
}
```

---

## MCP Protocol Implementation

### Transport Layer (Reuses LSP Pattern)

Based on `internal/lsp/transport.go`:

```go
// internal/mcp/transport.go

// readMessage reads a Content-Length framed JSON-RPC message
func (s *Server) readMessage() ([]byte, error) {
    // Parse Content-Length header
    // Read exact bytes from stream
    // Return raw JSON message
}

// writeMessage writes a JSON-RPC response with Content-Length framing
func (s *Server) writeMessage(v any) {
    // Marshal to JSON
    // Write "Content-Length: N\r\n\r\n"
    // Write JSON body
    // Thread-safe with mutex
}
```

### MCP Protocol Types

```go
// internal/mcp/types.go

// Request represents an MCP JSON-RPC 2.0 request
type Request struct {
    JSONRPC string          `json:"jsonrpc"` // Always "2.0"
    ID      any             `json:"id"`      // Request ID (string or number)
    Method  string          `json:"method"`  // Method name
    Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC 2.0 response
type Response struct {
    JSONRPC string      `json:"jsonrpc"` // Always "2.0"
    ID      any         `json:"id"`      // Matching request ID
    Result  any         `json:"result,omitempty"`
    Error   *RespError  `json:"error,omitempty"`
}

// InitializeRequest is the first message from client
type InitializeRequest struct {
    ProtocolVersion string       `json:"protocolVersion"` // "2024-11-05"
    Capabilities    Capabilities `json:"capabilities"`
    ClientInfo      ClientInfo   `json:"clientInfo"`
}

// Capabilities describes what the client/server supports
type Capabilities struct {
    Prompts   *PromptsCapability   `json:"prompts,omitempty"`
    Resources *ResourcesCapability `json:"resources,omitempty"`
    Tools     *ToolsCapability     `json:"tools,omitempty"`
}

// ListPromptsResult contains paginated prompts
type ListPromptsResult struct {
    Prompts    []PromptInfo `json:"prompts"`
    NextCursor string       `json:"nextCursor,omitempty"`
}

// GetPromptResult contains a rendered prompt
type GetPromptResult struct {
    Description string          `json:"description,omitempty"`
    Messages    []PromptMessage `json:"messages"`
}
```

### MCP Method Handlers

**Required Methods:**
1. `initialize` - Capability negotiation
2. `prompts/list` - List available prompts (with pagination)
3. `prompts/get` - Get specific prompt with arguments rendered
4. `notifications/initialized` - Client ready signal (no response)

**Handler Dispatch Pattern (from LSP):**
```go
// internal/mcp/server.go

func (s *Server) setupHandlers() {
    s.handlers = map[string]func(Request){
        "initialize":                s.handleInitialize,
        "prompts/list":              s.handlePromptsList,
        "prompts/get":               s.handlePromptsGet,
        "notifications/initialized": s.handleInitialized,
    }
}

func (s *Server) handleInitialize(req Request) {
    // Parse InitializeRequest
    // Validate protocol version
    // Return server capabilities
    // Set initialized flag
}

func (s *Server) handlePromptsList(req Request) {
    // Parse pagination params (cursor, limit)
    // Call promptStore.List(cursor, limit)
    // Return ListPromptsResult with nextCursor
}

func (s *Server) handlePromptsGet(req Request) {
    // Parse GetPromptRequest (name, arguments)
    // Call promptStore.Get(name)
    // Render template with arguments
    // Return GetPromptResult
}
```

---

## Testability Design

### Dependency Injection via Interfaces

```go
// internal/promptstore/store.go

// PromptStore defines the interface for prompt storage
// This allows easy mocking in tests
type PromptStore interface {
    List(cursor string, limit int) ([]Prompt, string, error)
    Get(name string) (*Prompt, error)
    Create(prompt *Prompt) error
    Update(prompt *Prompt) error
    Delete(name string) error
    Search(tags []string) ([]Prompt, error)
}

// JSONLStore is the file-based implementation
type JSONLStore struct {
    dataDir string
    mu      sync.RWMutex
    // Package variable for file operations (can be mocked)
    readFileFn  func(string) ([]byte, error)
    writeFileFn func(string, []byte, os.FileMode) error
}

// NewJSONLStore creates a new JSONL-based store
func NewJSONLStore(dataDir string) (PromptStore, error) {
    return &JSONLStore{
        dataDir:     dataDir,
        readFileFn:  os.ReadFile,
        writeFileFn: os.WriteFile,
    }, nil
}
```

### Server Factory Pattern (from LSP)

```go
// internal/hexaimcp/run.go

// ServerRunner interface for dependency injection
type ServerRunner interface {
    Run() error
}

// ServerFactory creates a server (testable)
type ServerFactory func(
    r io.Reader,
    w io.Writer,
    logger *log.Logger,
    store promptstore.PromptStore,
) ServerRunner

// RunWithFactory allows test injection
func RunWithFactory(
    logPath string,
    configPath string,
    stdin io.Reader,
    stdout io.Writer,
    stderr io.Writer,
    factory ServerFactory,
) error {
    // Load config
    // Setup logger
    // Create prompt store
    // Call factory to create server
    // Run server
}

// Run is the main entry point (uses default factory)
func Run(logPath, configPath string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
    return RunWithFactory(logPath, configPath, stdin, stdout, stderr, defaultServerFactory)
}
```

### Mock Implementation for Tests

```go
// internal/mcp/testhelpers_test.go

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
    // Default implementation
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
```

### Table-Driven Tests

```go
// internal/promptstore/jsonl_test.go

func TestJSONLStore_Get(t *testing.T) {
    tests := []struct {
        name       string
        promptName string
        fileData   string
        wantErr    bool
        wantPrompt *Prompt
    }{
        {
            name:       "existing prompt",
            promptName: "test",
            fileData:   `{"name":"test","title":"Test","messages":[]}` + "\n",
            wantErr:    false,
            wantPrompt: &Prompt{Name: "test", Title: "Test"},
        },
        {
            name:       "prompt not found",
            promptName: "missing",
            fileData:   `{"name":"other","title":"Other","messages":[]}` + "\n",
            wantErr:    true,
            wantPrompt: nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            tmpDir := t.TempDir()
            setupTestFile(t, tmpDir, tt.fileData)
            store, _ := NewJSONLStore(tmpDir)

            // Execute
            got, err := store.Get(tt.promptName)

            // Assert
            if (err != nil) != tt.wantErr {
                t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !reflect.DeepEqual(got, tt.wantPrompt) {
                t.Errorf("Get() = %v, want %v", got, tt.wantPrompt)
            }
        })
    }
}
```

---

## Configuration Integration

### Config Extension

```go
// internal/appconfig/config.go

type App struct {
    // ... existing fields ...

    // MCP server settings
    MCPPromptsDir       string   `json:"mcp_prompts_dir,omitempty" toml:"mcp_prompts_dir,omitempty"`
    MCPIncludeBuiltin   bool     `json:"mcp_include_builtin" toml:"mcp_include_builtin"`
    MCPCategories       []string `json:"mcp_categories,omitempty" toml:"mcp_categories,omitempty"`
    MCPMaxPromptsPerPage int     `json:"mcp_max_prompts_per_page" toml:"mcp_max_prompts_per_page"`
}
```

### TOML Configuration

```toml
# ~/.config/hexai/config.toml

[mcp]
# Storage location for prompts
# Can be absolute path or relative to home directory
# Default: $XDG_DATA_HOME/hexai/prompts/ (usually ~/.local/share/hexai/prompts/)
# Examples:
#   prompts_dir = "/home/user/git/my-prompts"     # Absolute path
#   prompts_dir = "~/Dropbox/hexai-prompts"       # Home-relative
#   prompts_dir = ""                              # Use default
prompts_dir = ""

# Include built-in prompts (default: true)
include_builtin = true

# Filter by categories (empty = all)
categories = []

# Max prompts per list response (pagination)
max_prompts_per_page = 50
```

### Environment Variable Override

```bash
# Override prompts directory via environment variable
export HEXAI_MCP_PROMPTS_DIR="/path/to/custom/prompts"
hexai-mcp-server
```

### Command-line Flag

```bash
# Override prompts directory via command-line flag
hexai-mcp-server --prompts-dir /path/to/custom/prompts

# Use with config file
hexai-mcp-server --config ~/.config/hexai/config.toml --prompts-dir ~/my-prompts
```

---

## Prompt Discovery and Search Strategy

### How MCP Clients Find Prompts

According to the MCP specification, the `prompts/list` method returns a list of prompts with metadata. Clients use this to discover available prompts. Here are multiple strategies for search/discovery:

### 1. List-Based Discovery (MCP Standard)

**How it works:**
- Client calls `prompts/list` to get all available prompts
- Each prompt includes: `name`, `title`, `description`, `arguments`
- Client presents list to user for selection
- User selects prompt, client calls `prompts/get` with name

**Implementation:**
```go
// internal/mcp/handlers.go

type PromptInfo struct {
    Name        string           `json:"name"`        // Unique ID
    Title       string           `json:"title"`       // Display name
    Description string           `json:"description"` // Human-readable
    Arguments   []PromptArgument `json:"arguments"`   // Template vars
}

func (s *Server) handlePromptsList(req Request) {
    // Return all prompts with metadata
    // Client does filtering client-side
}
```

**Pros:**
- Simple, follows MCP spec exactly
- No server-side search complexity
- Client can implement their own filtering
- Fast for small prompt collections (<1000)

**Cons:**
- Doesn't scale to thousands of prompts
- No fuzzy matching
- Limited by client UI capabilities

### 2. Tag-Based Filtering (Recommended)

**How it works:**
- Each prompt has tags: `["development", "review", "testing"]`
- Client filters by tags before presenting to user
- Hierarchical categories possible: `"language/go"`, `"domain/web"`

**Extended PromptInfo:**
```go
type PromptInfo struct {
    Name        string           `json:"name"`
    Title       string           `json:"title"`
    Description string           `json:"description"`
    Arguments   []PromptArgument `json:"arguments"`
    Tags        []string         `json:"tags"`  // NEW: categorization
}
```

**Search in PromptStore:**
```go
// internal/promptstore/store.go

type PromptStore interface {
    List(cursor string, limit int) ([]Prompt, string, error)
    Get(name string) (*Prompt, error)

    // NEW: Search by tags (AND logic: all tags must match)
    SearchByTags(tags []string) ([]Prompt, error)

    // NEW: Search by any tag (OR logic: any tag matches)
    SearchByAnyTag(tags []string) ([]Prompt, error)
}
```

**Example tags:**
- `"language"`: go, python, rust, javascript
- `"domain"`: web, cli, api, database
- `"task"`: review, test, document, refactor, debug
- `"difficulty"`: beginner, intermediate, advanced

**Pros:**
- Simple to implement
- Fast filtering (index tags)
- Human-organized categories
- Supports hierarchical organization

**Cons:**
- Requires manual tagging
- No fuzzy matching
- Limited to predefined categories

### 3. Full-Text Search (Enhanced)

**How it works:**
- Search across: title, description, message content, argument descriptions
- Case-insensitive substring matching
- Rank results by relevance

**Extended PromptStore:**
```go
// internal/promptstore/store.go

type SearchOptions struct {
    Query      string   // Search query
    Fields     []string // Which fields to search: "title", "description", "content"
    Tags       []string // Filter by tags
    Limit      int      // Max results
    MinScore   float64  // Minimum relevance score (0-1)
}

type SearchResult struct {
    Prompt *Prompt
    Score  float64  // Relevance score (0-1)
    Matches []Match // Where query was found
}

type Match struct {
    Field   string // "title", "description", "content"
    Context string // Surrounding text
}

func (s *JSONLStore) Search(opts SearchOptions) ([]SearchResult, error) {
    // 1. Load all prompts
    // 2. For each prompt, score against query
    // 3. Sort by score descending
    // 4. Return top results
}
```

**Scoring algorithm (simple):**
```go
func scorePrompt(prompt *Prompt, query string) float64 {
    query = strings.ToLower(query)
    score := 0.0

    // Title match (highest weight)
    if strings.Contains(strings.ToLower(prompt.Title), query) {
        score += 10.0
    }

    // Name match
    if strings.Contains(strings.ToLower(prompt.Name), query) {
        score += 8.0
    }

    // Description match
    if strings.Contains(strings.ToLower(prompt.Description), query) {
        score += 5.0
    }

    // Tag match
    for _, tag := range prompt.Tags {
        if strings.Contains(strings.ToLower(tag), query) {
            score += 3.0
        }
    }

    // Message content match (lowest weight)
    for _, msg := range prompt.Messages {
        if strings.Contains(strings.ToLower(msg.Content.Text), query) {
            score += 1.0
        }
    }

    return score
}
```

**Pros:**
- Natural language queries
- Searches all content
- Relevance ranking
- No manual tagging required

**Cons:**
- Slower for large collections
- No fuzzy matching (yet)
- Simple scoring may not be accurate

### 4. Fuzzy Search (Advanced, Future)

**How it works:**
- Levenshtein distance for typo tolerance
- Phonetic matching (Soundex, Metaphone)
- Substring + fuzzy combined

**Example library:**
```go
import "github.com/lithammer/fuzzysearch/fuzzy"

func fuzzySearchPrompts(query string, prompts []Prompt) []SearchResult {
    var results []SearchResult
    for _, p := range prompts {
        // Fuzzy match against title
        if fuzzy.MatchFold(query, p.Title) {
            distance := levenshtein.Distance(query, p.Title)
            score := 1.0 - (float64(distance) / float64(len(p.Title)))
            results = append(results, SearchResult{
                Prompt: &p,
                Score:  score,
            })
        }
    }
    return results
}
```

**Pros:**
- Typo-tolerant
- Natural for users
- "code revie" matches "code review"

**Cons:**
- More complex implementation
- Slower performance
- May match unwanted results

### 5. MCP Best Practices (from Specification)

According to MCP docs and existing servers:

**Standard approach:**
1. `prompts/list` returns ALL prompts with full metadata
2. Client does filtering/search client-side
3. Keep prompt count reasonable (<100 per server)
4. Use clear, descriptive names and titles

**For large collections (>100 prompts):**
1. Use pagination via `cursor` parameter
2. Consider namespace prefixes: `go/test`, `python/review`
3. Group related prompts into separate MCP servers
4. Example: `hexai-mcp-server-go` vs `hexai-mcp-server-python`

**Metadata best practices:**
- **name**: slug-style, unique (`code_review`, `test_generator`)
- **title**: Human-readable ("Request Code Review", "Generate Unit Tests")
- **description**: 1-2 sentences explaining what it does
- **arguments**: Clear names and descriptions

### Recommended Implementation (Phases)

**Phase 1 (MVP):** List-based + Tags
- `prompts/list` returns all prompts with tags
- Client-side filtering by tags
- Simple, follows spec, works for <100 prompts

**Phase 2:** Full-text search
- Add `Search(query string)` method to PromptStore
- Search title, description, tags
- Relevance scoring

**Phase 3:** Advanced features
- Fuzzy matching
- Multi-language support
- Cached search index

### Example Prompt Metadata Structure

```json
{
  "name": "code_review_detailed",
  "title": "Detailed Code Review",
  "description": "Comprehensive code review covering style, performance, security, and best practices",
  "tags": ["review", "quality", "security", "performance"],
  "arguments": [
    {
      "name": "code",
      "description": "The code to review",
      "required": true
    },
    {
      "name": "focus",
      "description": "Specific aspect to focus on (security, performance, style, all)",
      "required": false
    }
  ]
}
```

### Client-Side Search Example

Most MCP clients will implement their own search UI:

**Claude Code CLI example:**
```
User types: /prompt code review
-> Claude Code searches locally cached prompts
-> Shows: "code_review", "code_review_detailed", "review_api"
-> User selects one
-> Claude Code calls prompts/get
```

**Cursor example:**
```
User opens command palette, types "review"
-> Cursor filters MCP prompts by title/description
-> Shows matching prompts in dropdown
-> User selects, Cursor calls prompts/get
```

### Recommendation for hexai-mcp-server

**Start with:** Tag-based categorization (Phase 1)
- Simple to implement
- Fast performance
- Good for initial prompt collections
- Extensible to full-text search later

**Tag structure:**
```go
var builtinPrompts = []Prompt{
    {
        Name:  "code_review",
        Title: "Request Code Review",
        Tags:  []string{"development", "review", "quality", "go"},
        // ...
    },
    {
        Name:  "generate_tests",
        Title: "Generate Unit Tests",
        Tags:  []string{"development", "testing", "go", "tdd"},
        // ...
    },
}
```

**Implementation:**
1. Include `tags` in PromptInfo returned by `prompts/list`
2. Client does tag filtering
3. Add `SearchByTags()` method for server-side filtering (optional)
4. Future: Add full-text search when needed

---

## Built-in Prompts

Initial set of useful prompts:

1. **code_review** - Review code quality and suggest improvements
2. **explain_code** - Explain what code does in detail
3. **generate_tests** - Generate unit tests for a function/class
4. **document_function** - Generate documentation/docstrings
5. **simplify_code** - Simplify complex code while preserving behavior
6. **fix_bugs** - Analyze and suggest bug fixes
7. **refactor_extract** - Extract code into a separate function

```go
// internal/promptstore/builtin.go

var builtinPrompts = []Prompt{
    {
        Name:        "code_review",
        Title:       "Request Code Review",
        Description: "Analyzes code quality, style, and suggests improvements",
        Arguments: []PromptArgument{
            {Name: "code", Description: "The code to review", Required: true},
        },
        Messages: []PromptMessage{
            {
                Role: "user",
                Content: MessageContent{
                    Type: "text",
                    Text: "Please review the following code for quality, style, and potential issues:\n\n{{code}}",
                },
            },
        },
        Tags: []string{"development", "review", "quality"},
    },
    // ... more built-in prompts
}
```

---

## Agent Installation

### Claude Code CLI

Edit `~/.config/claude/mcp.json`:

**Basic configuration (uses default prompts directory):**
```json
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": [],
      "env": {}
    }
  }
}
```

**With custom prompts directory:**
```json
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": ["--prompts-dir", "/home/paul/Dropbox/hexai-prompts"],
      "env": {}
    }
  }
}
```

**With environment variable:**
```json
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": [],
      "env": {
        "HEXAI_MCP_PROMPTS_DIR": "/home/paul/git/team-prompts"
      }
    }
  }
}
```

**Alternative (if binary is in PATH):**
```json
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "hexai-mcp-server",
      "args": [],
      "env": {}
    }
  }
}
```

### Cursor Agent

Edit `~/.cursor/mcp.json`:

**Basic configuration:**
```json
{
  "mcpServers": {
    "hexai": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": [],
      "env": {}
    }
  }
}
```

**With custom config and prompts directory:**
```json
{
  "mcpServers": {
    "hexai": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": [
        "--config", "/home/paul/.config/hexai/config.toml",
        "--prompts-dir", "/home/paul/git/shared-prompts"
      ],
      "env": {}
    }
  }
}
```

**Project-specific prompts via environment variable:**
```json
{
  "mcpServers": {
    "hexai-project": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": [],
      "env": {
        "HEXAI_MCP_PROMPTS_DIR": "${workspaceFolder}/.hexai/prompts"
      }
    }
  }
}
```

**Note:**
- Some MCP clients may not expand `~` in the command path. Use full absolute path (`/home/paul/go/bin/...`) or ensure the binary is in `$PATH`.
- `${workspaceFolder}` is a Cursor variable that expands to the current project directory

---

## Template Rendering

```go
// internal/mcp/handlers.go

// renderPrompt substitutes template arguments
func renderPrompt(prompt *promptstore.Prompt, args map[string]string) ([]promptstore.PromptMessage, error) {
    // Validate required arguments
    for _, arg := range prompt.Arguments {
        if arg.Required {
            if _, ok := args[arg.Name]; !ok {
                return nil, fmt.Errorf("missing required argument: %s", arg.Name)
            }
        }
    }

    // Render each message
    var rendered []promptstore.PromptMessage
    for _, msg := range prompt.Messages {
        text := msg.Content.Text

        // Simple template substitution: {{arg}} -> value
        for key, val := range args {
            placeholder := "{{" + key + "}}"
            text = strings.ReplaceAll(text, placeholder, val)
        }

        rendered = append(rendered, promptstore.PromptMessage{
            Role: msg.Role,
            Content: promptstore.MessageContent{
                Type: "text",
                Text: text,
            },
        })
    }

    return rendered, nil
}
```

---

## Error Handling

### MCP Error Codes (JSON-RPC 2.0)

```go
// internal/mcp/types.go

const (
    ErrCodeParseError     = -32700 // Invalid JSON
    ErrCodeInvalidRequest = -32600 // Invalid Request structure
    ErrCodeMethodNotFound = -32601 // Method doesn't exist
    ErrCodeInvalidParams  = -32602 // Invalid parameters
    ErrCodeInternalError  = -32603 // Server internal error
)

// RespError represents a JSON-RPC error
type RespError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}
```

### Error Mapping

- Prompt not found → `-32602` Invalid params
- Storage I/O error → `-32603` Internal error
- Invalid JSONL → Log warning, skip entry
- Missing required argument → `-32602` Invalid params

---

## Critical Files to Modify/Create

### Files to Read/Reference
- `/home/paul/git/hexai/internal/lsp/transport.go` - Transport pattern
- `/home/paul/git/hexai/internal/lsp/server.go` - Server dispatch pattern
- `/home/paul/git/hexai/internal/tmuxedit/history.go` - JSONL storage pattern
- `/home/paul/git/hexai/cmd/hexai-lsp/main.go` - Command entry point
- `/home/paul/git/hexai/internal/appconfig/config.go` - Config extension pattern

### Files to Create
- `cmd/hexai-mcp-server/main.go`
- `internal/hexaimcp/run.go`
- `internal/hexaimcp/run_test.go`
- `internal/hexaimcp/testhelpers_test.go`
- `internal/mcp/server.go`
- `internal/mcp/server_test.go`
- `internal/mcp/transport.go`
- `internal/mcp/transport_test.go`
- `internal/mcp/types.go`
- `internal/mcp/handlers.go`
- `internal/mcp/handlers_test.go`
- `internal/mcp/testhelpers_test.go`
- `internal/promptstore/store.go`
- `internal/promptstore/store_test.go`
- `internal/promptstore/types.go`
- `internal/promptstore/jsonl.go`
- `internal/promptstore/jsonl_test.go`
- `internal/promptstore/builtin.go`

---

## Documentation Updates

### README.md Updates

Add to the **Features** section (line 9):
```markdown
* MCP server for prompt/runbook management (`hexai-mcp-server`)
  - Store, update, search, and retrieve reusable prompts
  - Compatible with Claude Code CLI, Cursor, and other MCP clients
  - File-based storage with JSONL format
```

Add to **Documentation** section (line 24):
```markdown
* [MCP server setup guide](docs/mcp-setup.md)
* [Creating custom prompts](docs/mcp-prompts.md)
```

Add to **File Locations** section (line 36):
```markdown
- **Data:** `~/.local/share/hexai/` (or `$XDG_DATA_HOME/hexai/`)
  - `prompts/default.jsonl` - Built-in prompts
  - `prompts/user.jsonl` - User-created prompts
```

### New Documentation Files

**docs/mcp-setup.md** (~150 lines):
- What is MCP and why use it
- Installation instructions for Claude Code CLI
  - Full path example: `~/go/bin/hexai-mcp-server`
  - PATH-based example
  - Environment variable configuration
- Installation instructions for Cursor
  - Full path example: `~/go/bin/hexai-mcp-server`
  - Config file location and format
- Installation instructions for other MCP clients
- Configuration options (args, env variables)
- Testing the connection
- Troubleshooting (binary not found, permission issues)

**docs/mcp-prompts.md** (~200 lines):
- Creating custom prompts
- Prompt structure and format
- Template arguments
- Built-in prompts reference
- Best practices
- Examples

### Update Existing Documentation

**docs/buildandinstall.md**:
- Add `hexai-mcp-server` to the list of binaries
- Update build instructions to include new command
- Update install instructions

**docs/configuration.md**:
- Add `[mcp]` section documentation
- Explain `prompts_dir`, `include_builtin`, etc.
- Add examples

---

## Implementation Phases

### Phase 1: Core Protocol (MVP)
1. Create `cmd/hexai-mcp-server/main.go` with flag parsing
   - Flags: `--log`, `--config`, `--prompts-dir`, `--version`
   - Resolve prompts directory from: flag → env → config → default
2. Implement `internal/mcp/transport.go` (reuse LSP pattern)
3. Implement `internal/mcp/server.go` with dispatch table
4. Implement `internal/mcp/types.go` with MCP protocol types
5. Implement `initialize` handshake handler
6. Basic `prompts/list` and `prompts/get` handlers (stub)
7. Unit tests for transport and server setup

### Phase 2: Storage Layer
1. Create `internal/promptstore/types.go` with data models
2. Implement `internal/promptstore/jsonl.go` (JSONL read/write)
3. Implement `internal/promptstore/store.go` (interface + impl)
4. Add built-in prompts in `internal/promptstore/builtin.go`
5. Unit tests for store operations (90%+ coverage)
6. Integration tests for JSONL persistence

### Phase 3: Integration
1. Wire up store to MCP handlers
2. Implement template rendering in `internal/mcp/handlers.go`
3. Add pagination support for `prompts/list`
4. Add config.toml support in `internal/appconfig/config.go`
5. Create `internal/hexaimcp/run.go` with factory pattern
6. Integration tests (full protocol flow)

### Phase 4: Documentation & Polish
1. Write `docs/mcp-setup.md`
2. Write `docs/mcp-prompts.md`
3. Update `README.md` with MCP features
4. Update `docs/buildandinstall.md`
5. Update `docs/configuration.md`
6. Add error messages and validation
7. Achieve 80%+ total test coverage
8. Run `gofumpt` on all new files

---

## Testing Strategy

### Unit Tests (by package)

**internal/mcp:**
- Transport: Message framing, JSON marshaling
- Server: Dispatch table, handler routing
- Handlers: Each handler in isolation with mock store
- Types: JSON serialization/deserialization
- Target: 85%+ coverage

**internal/promptstore:**
- JSONL: Read/write operations, error cases
- Store: CRUD operations with temp directories
- Builtin: Prompt validation
- Target: 90%+ coverage (critical data layer)

**internal/hexaimcp:**
- Run: Config loading, factory pattern
- Integration: Full server lifecycle
- Target: 80%+ coverage

### Integration Tests

```go
// internal/hexaimcp/run_test.go

func TestFullProtocolFlow(t *testing.T) {
    // Setup pipes for stdin/stdout
    inReader, inWriter := io.Pipe()
    outReader, outWriter := io.Pipe()

    // Start server in goroutine
    go func() {
        Run("", "", inReader, outWriter, io.Discard)
    }()

    // Send initialize request
    sendRequest(inWriter, Request{
        JSONRPC: "2.0",
        ID:      1,
        Method:  "initialize",
        Params:  // ...
    })

    // Read and validate response
    resp := readResponse(outReader)
    assert.Equal(t, "initialize", resp.Result.Capabilities)

    // Send prompts/list request
    // Validate response

    // Send prompts/get request
    // Validate rendered prompt
}
```

### Test Helpers

```go
// internal/mcp/testhelpers_test.go

// createTestServer creates a server with mock dependencies
func createTestServer(t *testing.T, store promptstore.PromptStore) *Server {
    t.Helper()
    inReader, _ := io.Pipe()
    _, outWriter := io.Pipe()
    logger := log.New(io.Discard, "", 0)
    return NewServer(inReader, outWriter, logger, store)
}

// sendTestRequest sends a request and returns the response
func sendTestRequest(t *testing.T, s *Server, method string, params any) Response {
    t.Helper()
    // Marshal params, send request, parse response
}
```

---

## Verification Steps

After implementation, verify the following:

### 1. Build and Install
```bash
cd ~/git/hexai
go build ./cmd/hexai-mcp-server
./hexai-mcp-server --version
go install ./cmd/hexai-mcp-server
which hexai-mcp-server
```

### 2. Test Coverage
```bash
mage coverage
# Verify overall coverage > 80%
# Check individual packages
go test -cover ./internal/mcp/...
go test -cover ./internal/promptstore/...
go test -cover ./internal/hexaimcp/...
```

### 3. Manual Protocol Test
```bash
# Start server
hexai-mcp-server --log /tmp/mcp.log

# In another terminal, send test messages
cat <<EOF | hexai-mcp-server
Content-Length: 123

{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
EOF
```

### 4. Agent Integration Test
```bash
# Configure Claude Code CLI
mkdir -p ~/.config/claude
cat > ~/.config/claude/mcp.json <<EOF
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "$HOME/go/bin/hexai-mcp-server",
      "args": [],
      "env": {}
    }
  }
}
EOF

# Restart Claude Code and verify prompts are available
claude --list-mcp-servers  # (if such command exists)
```

**For Cursor:**
```bash
# Configure Cursor Agent
mkdir -p ~/.cursor
cat > ~/.cursor/mcp.json <<EOF
{
  "mcpServers": {
    "hexai": {
      "command": "$HOME/go/bin/hexai-mcp-server",
      "args": [],
      "env": {}
    }
  }
}
EOF

# Restart Cursor and check MCP server status
```

### 5. Storage Verification
```bash
# Check data directory
ls -la ~/.local/share/hexai/prompts/

# View built-in prompts
cat ~/.local/share/hexai/prompts/default.jsonl | jq

# Test JSONL format
jq -s '.' ~/.local/share/hexai/prompts/default.jsonl
```

### 6. Code Quality
```bash
# Run formatter
gofumpt -l -w .

# Run tests
go test ./...

# Check for race conditions
go test -race ./internal/mcp/... ./internal/promptstore/...

# Verify no files > 1000 lines
find . -name "*.go" -type f -exec wc -l {} + | awk '$1 > 1000 {print}'
```

---

## Security Considerations

### Input Validation
- Prompt names: alphanumeric + underscores only (regex: `^[a-zA-Z0-9_]+$`)
- Limit prompt size: max 100KB per prompt
- Limit total prompts: max 1000 prompts
- Validate argument names against prompt schema

### Filesystem Safety
- All paths relative to `XDG_DATA_HOME/hexai/prompts/`
- Prevent path traversal attacks (no `..` in names)
- Atomic writes (temp file + rename)
- Read-only mode for clients (no write via MCP)

### Error Information Leakage
- Don't expose internal paths in error messages
- Sanitize error details sent to client
- Log detailed errors server-side only

---

## Future Extensions (Out of Scope)

- Resources support (runbooks, documentation files)
- Tools support (execute scripts, git operations)
- Web UI for prompt management
- Prompt sharing/import from URLs
- Versioning and history tracking
- Advanced search (full-text, regex)
- Prompt templates with includes/inheritance
- Multi-language prompt translations

---

## Summary

This implementation adds a production-ready MCP server to hexai that:

✅ Follows established hexai patterns (multi-binary, LSP-like architecture)
✅ Uses JSONL storage (matches tmux-edit history pattern)
✅ Provides testable architecture (interfaces, dependency injection)
✅ Achieves 80%+ test coverage (table-driven tests)
✅ Integrates seamlessly with existing config system
✅ Works with Claude Code CLI, Cursor, and other MCP clients
✅ Includes comprehensive documentation
✅ Maintains code quality standards (gofumpt, 50-line functions, 1000-line files)

The design leverages hexai's proven LSP server implementation while adapting it for the MCP protocol, resulting in a maintainable and extensible solution.
