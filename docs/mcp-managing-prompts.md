# Managing MCP Prompts

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

Quick reference for managing hexai MCP server prompts.

## 🚀 All Management Through MCP Protocol

All prompt management is done through the MCP server protocol. Use any MCP client (Claude Code, Cursor, etc.) to:

- **Create prompts**: `prompts/create` method
- **Update prompts**: `prompts/update` method
- **Delete prompts**: `prompts/delete` method
- **List prompts**: `prompts/list` method
- **Get prompts**: `prompts/get` method

**Automatic backups** are created on every operation!

## 📍 Prompt Locations

- **Default prompts**: `~/.local/hexai/data/prompts/default.jsonl` (built-in, auto-regenerated)
- **Custom prompts**: `~/.local/hexai/data/prompts/user.jsonl` (your prompts)
- **Backups**: `~/.local/hexai/data/prompts/backups/` (automatic backups)
- **Override directory**: Set `HEXAI_MCP_PROMPTS_DIR` environment variable

## 📝 Method 1: Using MCP Protocol (Recommended)

### List All Prompts

Use your MCP client (Claude Code, Cursor) to list prompts, or use the MCP protocol:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/list",
  "params": {}
}
```

### Create a New Prompt

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "prompts/create",
  "params": {
    "name": "performance_analysis",
    "title": "Performance Analysis",
    "description": "Analyzes code performance",
    "arguments": [
      {
        "name": "code",
        "description": "Code to analyze",
        "required": true
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Analyze performance:\n{{code}}"
        }
      }
    ],
    "tags": ["performance", "optimization"]
  }
}
```

### Update a Prompt

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "prompts/update",
  "params": {
    "name": "performance_analysis",
    "description": "Enhanced performance analysis with profiling"
  }
}
```

### Delete a Prompt

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "prompts/delete",
  "params": {
    "name": "old_prompt"
  }
}
```

**Note**: You can only delete custom prompts from `user.jsonl`, not built-in prompts.

## 📝 Method 2: Direct File Editing (Advanced)

### Edit user.jsonl
```bash
# Using your editor
$EDITOR ~/.local/hexai/data/prompts/user.jsonl

# Or with nano
nano ~/.local/hexai/data/prompts/user.jsonl
```

### Prompt Format (JSONL - one line per prompt)

```json
{"name":"prompt_name","title":"Display Title","description":"What it does","arguments":[{"name":"arg1","description":"Argument description","required":true}],"messages":[{"role":"user","content":{"type":"text","text":"Prompt text with {{arg1}}"}}],"tags":["tag1","tag2"],"created":"2026-02-10T18:00:00Z","updated":"2026-02-10T18:00:00Z"}
```

### Pretty Format (for editing)

```json
{
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
        "text": "Design REST API for: {{resource}}"
      }
    }
  ],
  "tags": ["api", "rest", "design"],
  "created": "2026-02-10T18:00:00Z",
  "updated": "2026-02-10T18:00:00Z"
}
```

**To add**: Minify with `jq -c` and append:
```bash
jq -c . my-prompt.json >> ~/.local/hexai/data/prompts/user.jsonl
```

## 📝 Method 3: Programmatic Access (Go)

```go
package main

import (
    "time"
    "codeberg.org/snonux/hexai/internal/promptstore"
)

func main() {
    dir := "/home/paul/.local/hexai/data/prompts"
    store, _ := promptstore.NewJSONLStore(dir)

    prompt := &promptstore.Prompt{
        Name:        "optimize_query",
        Title:       "Optimize Database Query",
        Description: "Analyzes and optimizes SQL queries",
        Arguments: []promptstore.PromptArgument{
            {
                Name:        "query",
                Description: "SQL query to optimize",
                Required:    true,
            },
        },
        Messages: []promptstore.PromptMessage{
            {
                Role: "user",
                Content: promptstore.MessageContent{
                    Type: "text",
                    Text: "Optimize this query: {{query}}",
                },
            },
        },
        Tags:    []string{"database", "sql", "optimization"},
        Created: time.Now(),
        Updated: time.Now(),
    }

    store.Create(prompt)
}
```

## 🔄 When Do Changes Take Effect?

### MCP Server Reloads
The MCP server reads prompts from disk on each request, so changes are **immediately available** without restarting the server!

### Client Caching
Some MCP clients (like Claude Code, Cursor) may cache the prompt list:
- **To refresh**: Restart the client
- **Or**: Wait for the client's cache timeout (usually a few minutes)

## 🎯 Common Tasks

### View Automatic Backups
```bash
ls -lht ~/.local/hexai/data/prompts/backups/
```

Output shows timestamped backups (most recent first):
```
-rw-r--r-- 1 paul paul 1.2K Feb 10 19:34 user.jsonl.20260210-193422
-rw-r--r-- 1 paul paul 1.1K Feb 10 19:21 user.jsonl.20260210-192145
```

### Restore from Backup (Manual)
```bash
# Copy backup to restore
cp ~/.local/hexai/data/prompts/backups/user.jsonl.20260210-193422 \
   ~/.local/hexai/data/prompts/user.jsonl
```

### Import a Prompt
```bash
# From a JSON file
jq -c . imported-prompt.json >> ~/.local/hexai/data/prompts/user.jsonl
hexai-prompt validate
```

### Share Prompts with Team
```bash
# Export specific prompts
hexai-prompt export my_team_prompt > team-prompts/prompt1.json
hexai-prompt export security_checklist > team-prompts/prompt2.json

# Commit to git
git add team-prompts/
git commit -m "Add team prompts"
git push
```

### Team Members Import
```bash
cd team-prompts/
for f in *.json; do
    jq -c . "$f" >> ~/.local/hexai/data/prompts/user.jsonl
done
```

### Update an Existing Prompt

**Method 1**: Use MCP protocol
```json
{
  "method": "prompts/update",
  "params": {
    "name": "my_prompt",
    "title": "Updated Title",
    "description": "Updated description"
  }
}
```

**Method 2**: Edit user.jsonl directly
```bash
$EDITOR ~/.local/hexai/data/prompts/user.jsonl
# Find the line with the prompt name and edit it
hexai-prompt validate
```

**Method 3**: Use Go program with Update
```go
store.Update(prompt)  // Updates existing prompt by name
```

### Count Prompts
```bash
wc -l ~/.local/hexai/data/prompts/default.jsonl  # Built-in
wc -l ~/.local/hexai/data/prompts/user.jsonl     # Custom
```

## 🛠️ Troubleshooting

### Invalid JSON
```bash
# Check specific file
jq empty ~/.local/hexai/data/prompts/user.jsonl
```

### Prompt Not Appearing
```bash
# Check MCP server logs
tail -f ~/.local/hexai/state/hexai-mcp-server.log

# Verify file exists
cat ~/.local/hexai/data/prompts/user.jsonl | jq .
```

### Duplicate Prompt Names
Prompt names must be unique. If you have duplicates:
```bash
# List all prompt names
cat ~/.local/hexai/data/prompts/user.jsonl | jq -r .name | sort | uniq -d

# Fix: Edit user.jsonl and rename or remove duplicates
```

### Reset to Defaults
```bash
# Backup first
cp ~/.local/hexai/data/prompts/default.jsonl ~/backup/

# Delete and let server recreate
rm ~/.local/hexai/data/prompts/default.jsonl

# Restart MCP server or wait for next request
```

## 📚 See Also

- [MCP Setup Guide](mcp-setup.md)
- [Creating Custom Prompts](mcp-prompts.md)
- [hexai Configuration](configuration.md)
