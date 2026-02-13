# MCP Server API Reference

> **⚠️ DEPRECATION NOTICE**
>
> This MCP server is **EXPERIMENTAL** and **NOT ACTIVELY MAINTAINED**.
>
> The author currently manages prompts through slash commands and meta-commands
> in the hexai agent system, making this MCP server redundant for its original
> purpose. This code is kept for potential future enhancements (possibly with
> different functionality beyond prompt management), but no guarantees are made
> about stability or continued support.
>
> **This documentation is preserved for reference only.**

The hexai-mcp-server implements the Model Context Protocol with prompt management extensions.

## Protocol Version

**Version**: `2025-06-18`

## Server Capabilities

The server advertises these capabilities during initialization:

```json
{
  "capabilities": {
    "prompts": {
      "listChanged": false,
      "mutable": true
    }
  }
}
```

- `listChanged`: Server does not currently emit notifications when prompts change
- `mutable`: **Server supports create/update/delete operations**

## Standard MCP Methods

### initialize

Establishes connection and negotiates capabilities.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "capabilities": {},
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "prompts": {
        "listChanged": false,
        "mutable": true
      }
    },
    "serverInfo": {
      "name": "hexai-mcp-server",
      "version": "0.19.0"
    }
  }
}
```

### prompts/list

Lists all available prompts with pagination support.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "prompts/list",
  "params": {
    "cursor": ""
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "prompts": [
      {
        "name": "code_review",
        "title": "Request Code Review",
        "description": "Analyzes code quality, style, and suggests improvements",
        "arguments": [
          {
            "name": "code",
            "description": "The code to review",
            "required": true
          }
        ]
      }
    ],
    "nextCursor": ""
  }
}
```

### prompts/get

Retrieves a specific prompt with argument rendering.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "prompts/get",
  "params": {
    "name": "code_review",
    "arguments": {
      "code": "function hello() { console.log('world'); }"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "description": "Analyzes code quality, style, and suggests improvements",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Please review the following code for quality, style, and potential issues:\n\nfunction hello() { console.log('world'); }"
        }
      }
    ]
  }
}
```

## Management Methods (Extension)

These methods are **hexai-mcp-server extensions** that allow prompt management through the MCP protocol.

### prompts/create

Creates a new custom prompt.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "prompts/create",
  "params": {
    "name": "api_design",
    "title": "Design REST API",
    "description": "Creates RESTful API endpoint specification",
    "arguments": [
      {
        "name": "resource",
        "description": "Resource name (e.g., users, posts)",
        "required": true
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Design a REST API for: {{resource}}"
        }
      }
    ],
    "tags": ["api", "rest", "design"]
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "success": true,
    "message": "Created prompt: api_design"
  }
}
```

**Required Fields:**
- `name`: Unique identifier (alphanumeric + underscores)
- `title`: Display name
- `messages`: At least one message

**Optional Fields:**
- `description`: Human-readable description
- `arguments`: Template variables
- `tags`: Categorization tags

### prompts/update

Updates an existing custom prompt.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "prompts/update",
  "params": {
    "name": "api_design",
    "title": "Design RESTful API (Updated)",
    "description": "Creates comprehensive RESTful API specification with best practices"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "success": true,
    "message": "Updated prompt: api_design"
  }
}
```

**Required Fields:**
- `name`: Prompt identifier

**Optional Fields** (only provided fields are updated):
- `title`: New title
- `description`: New description
- `arguments`: New arguments (replaces entire list)
- `messages`: New messages (replaces entire list)
- `tags`: New tags (replaces entire list)

### prompts/delete

Deletes a custom prompt.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "prompts/delete",
  "params": {
    "name": "old_prompt"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "success": true,
    "message": "Deleted prompt: old_prompt"
  }
}
```

**Note**: Can only delete custom prompts from `user.jsonl`, not built-in prompts from `default.jsonl`.

## Error Responses

All methods return standard JSON-RPC 2.0 errors:

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "error": {
    "code": -32602,
    "message": "Prompt name is required"
  }
}
```

### Error Codes

| Code | Meaning | Example |
|------|---------|---------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid request | Malformed request structure |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Missing required field |
| -32603 | Internal error | Database/storage error |

## Template Syntax

Prompts support template variables using `{{variable}}` syntax:

```json
{
  "text": "Review this {{language}} code:\n\n{{code}}"
}
```

When rendered with arguments `{"language": "Go", "code": "..."}`:
```
Review this Go code:

...
```

## Example: Full Workflow

```javascript
// 1. Initialize
send({
  jsonrpc: "2.0",
  id: 1,
  method: "initialize",
  params: {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "my-client", version: "1.0" }
  }
});

// 2. List prompts
send({
  jsonrpc: "2.0",
  id: 2,
  method: "prompts/list"
});

// 3. Create custom prompt
send({
  jsonrpc: "2.0",
  id: 3,
  method: "prompts/create",
  params: {
    name: "perf_analysis",
    title: "Performance Analysis",
    arguments: [{ name: "code", required: true }],
    messages: [
      {
        role: "user",
        content: { type: "text", text: "Analyze performance: {{code}}" }
      }
    ],
    tags: ["performance"]
  }
});

// 4. Use the prompt
send({
  jsonrpc: "2.0",
  id: 4,
  method: "prompts/get",
  params: {
    name: "perf_analysis",
    arguments: { code: "for i := 0; i < n; i++ { ... }" }
  }
});

// 5. Update prompt
send({
  jsonrpc: "2.0",
  id: 5,
  method: "prompts/update",
  params: {
    name: "perf_analysis",
    description: "Enhanced performance analysis with profiling suggestions"
  }
});

// 6. Delete prompt
send({
  jsonrpc: "2.0",
  id: 6,
  method: "prompts/delete",
  params: { name: "perf_analysis" }
});
```

## Client Implementation Notes

### Capability Detection

Check for `mutable: true` in server capabilities:

```javascript
const initResult = await initialize();
const canManage = initResult.capabilities.prompts?.mutable === true;

if (canManage) {
  // Show create/edit/delete UI
}
```

### Error Handling

Always check for errors in responses:

```javascript
const response = await request("prompts/create", params);
if (response.error) {
  console.error(`Error ${response.error.code}: ${response.error.message}`);
  return;
}

if (response.result.success) {
  console.log(response.result.message);
}
```

### Immediate Effect

Changes take effect immediately:
- Create/update/delete operations modify `user.jsonl`
- Next `prompts/list` or `prompts/get` reflects changes
- No server restart required

### Built-in vs Custom

- **Built-in prompts**: Cannot be modified or deleted
- **Custom prompts**: Stored in `user.jsonl`, fully manageable
- Attempting to delete/update built-in prompts returns an error

## Testing with curl

```bash
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | hexai-mcp-server

# Create prompt
cat <<EOF | hexai-mcp-server
Content-Length: 300

{"jsonrpc":"2.0","id":2,"method":"prompts/create","params":{"name":"test","title":"Test","messages":[{"role":"user","content":{"type":"text","text":"Test"}}]}}
EOF
```

## See Also

- [MCP Setup Guide](mcp-setup.md)
- [Creating Custom Prompts](mcp-prompts.md)
- [Managing Prompts](mcp-managing-prompts.md)
- [MCP Specification](https://modelcontextprotocol.io/)
