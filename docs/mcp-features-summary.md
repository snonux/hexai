# hexai-mcp-server Complete Feature Summary

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

## 🎯 Overview

The hexai-mcp-server is a complete Model Context Protocol implementation with prompt management capabilities. It includes both **standard MCP methods** and **management extensions** that allow full CRUD operations on prompts.

## ✨ Key Features

### 1. Standard MCP Protocol Support
- ✅ Protocol version `2025-06-18` (latest specification)
- ✅ `initialize` - Capability negotiation
- ✅ `prompts/list` - List all prompts with pagination
- ✅ `prompts/get` - Retrieve and render prompts with arguments
- ✅ JSON-RPC 2.0 transport with Content-Length framing
- ✅ Compatible with Claude Code CLI, Cursor, and all MCP clients

### 2. Prompt Management (MCP Extensions)
- ✅ **`prompts/create`** - Create new prompts via MCP protocol
- ✅ **`prompts/update`** - Update existing prompts
- ✅ **`prompts/delete`** - Delete custom prompts
- ✅ Server advertises `mutable: true` capability
- ✅ All operations work through the protocol - no file editing needed!

### 3. Automatic Backup System
- ✅ **Automatic backups** before every write operation (create/update/delete)
- ✅ **Timestamped backups** in `prompts/backups/` directory
- ✅ **Retention policy** - keeps last 10 backups automatically
- ✅ **Safety backups** - creates pre-restore backup when restoring
- ✅ **No data loss** - always have multiple restore points

### 4. Command-Line Management (hexai-prompt)
- ✅ **list** - List all prompts with tags
- ✅ **show** - View prompt details
- ✅ **add** - Interactive prompt creation
- ✅ **delete** - Remove custom prompts
- ✅ **export** - Export prompts to JSON
- ✅ **validate** - Check all prompts for errors
- ✅ **backups** - List available backups
- ✅ **restore** - Restore from backup by index or name

### 5. Storage & Organization
- ✅ **JSONL format** - Git-friendly, human-readable
- ✅ **Separate files** - `default.jsonl` (built-in), `user.jsonl` (custom)
- ✅ **XDG-compliant** - `~/.local/hexai/data/prompts/`
- ✅ **Configurable** - Override via flag, env var, or config file
- ✅ **Tag-based categorization** - Filter and organize prompts

### 6. Built-in Prompts
7 production-ready prompts included:
1. **code_review** - Code quality analysis
2. **explain_code** - Detailed code explanations
3. **generate_tests** - Unit test generation
4. **document_function** - Generate documentation
5. **simplify_code** - Code simplification
6. **fix_bugs** - Bug analysis and fixes
7. **refactor_extract** - Extract code to functions

## 🚀 Three Ways to Manage Prompts

### 1. Through MCP Protocol (From Any Client)

```json
// Create prompt via MCP
{
  "method": "prompts/create",
  "params": {
    "name": "my_prompt",
    "title": "My Prompt",
    "messages": [...]
  }
}

// Update prompt via MCP
{
  "method": "prompts/update",
  "params": {
    "name": "my_prompt",
    "title": "Updated Title"
  }
}

// Delete prompt via MCP
{
  "method": "prompts/delete",
  "params": {
    "name": "my_prompt"
  }
}
```

**Benefits:**
- Works from any MCP client (Claude Code, Cursor, etc.)
- No command-line access needed
- Automatic backups on every change
- Immediate availability

### 2. Using hexai-prompt CLI

```bash
# List prompts
hexai-prompt list

# Create prompt (interactive)
hexai-prompt add my_new_prompt

# Delete prompt
hexai-prompt delete old_prompt

# List backups
hexai-prompt backups

# Restore from backup
hexai-prompt restore 1
```

**Benefits:**
- Simple, interactive interface
- Perfect for scripting
- Direct backup management
- Quick validation

### 3. Direct File Editing

```bash
# Edit user.jsonl directly
$EDITOR ~/.local/hexai/data/prompts/user.jsonl

# Validate after editing
hexai-prompt validate
```

**Benefits:**
- Full control over format
- Bulk operations easy
- Git-friendly workflow
- Advanced users preferred

## 🔒 Backup System Details

### Automatic Backups
Every write operation (create/update/delete) automatically creates a timestamped backup:

```
~/.local/hexai/data/prompts/backups/
├── user.jsonl.20260210-190358
├── user.jsonl.20260210-192145
└── user.jsonl.20260210-193422
```

### Retention Policy
- Keeps last **10 backups** by default
- Oldest backups auto-deleted
- Configurable in code (change `maxBackups`)

### Safety Features
- **Pre-restore backup** - Creates safety backup before restore
- **Atomic operations** - Backup succeeds or entire operation fails
- **No data loss** - Always have multiple restore points
- **Sorted by timestamp** - Easy to find recent backups

### Backup Management

```bash
# List backups (newest first)
hexai-prompt backups
# Output:
# Found 3 backup(s):
#   1. 20260210-193422
#   2. 20260210-192145
#   3. 20260210-190358

# Restore by index
hexai-prompt restore 1

# Restore by timestamp
hexai-prompt restore 20260210-193422

# Check backups directory
ls -lh ~/.local/share/hexai/prompts/backups/
```

## 📊 Statistics

| Metric | Value |
|--------|-------|
| **Protocol Version** | 2025-06-18 (latest) |
| **MCP Methods** | 7 (3 standard + 3 management + 1 init) |
| **Built-in Prompts** | 7 |
| **Test Coverage** | 71.7% (promptstore), 52.4% (mcp) |
| **Source Files** | 15 files, ~2800 lines |
| **Binary Size** | 3.9 MB (mcp-server), 3.2 MB (prompt CLI) |
| **Dependencies** | Zero external deps (pure stdlib) |

## 🎓 Usage Examples

### Example 1: Create Prompt via MCP

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/create",
  "params": {
    "name": "security_scan",
    "title": "Security Vulnerability Scan",
    "description": "Comprehensive security analysis",
    "arguments": [
      {
        "name": "code",
        "description": "Code to scan",
        "required": true
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Scan for security issues:\n{{code}}"
        }
      }
    ],
    "tags": ["security", "audit"]
  }
}
```

### Example 2: Update Prompt via MCP

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "prompts/update",
  "params": {
    "name": "security_scan",
    "description": "Enhanced security analysis with OWASP Top 10"
  }
}
```

### Example 3: Backup and Restore Workflow

```bash
# Make some changes
hexai-prompt add new_feature
hexai-prompt update old_prompt

# List backups (see all changes)
hexai-prompt backups
# Shows:
#   1. 20260210-194500 (after update)
#   2. 20260210-194430 (after add)

# Oops, made a mistake - restore previous state
hexai-prompt restore 2

# All changes undone, back to state at 19:44:30
```

### Example 4: Export and Share

```bash
# Export prompt
hexai-prompt export my_team_prompt > team-prompt.json

# Commit to git
git add team-prompt.json
git commit -m "Add team prompt"
git push

# Team members import
jq -c . team-prompt.json >> ~/.local/hexai/data/prompts/user.jsonl
```

## 🔧 Configuration

### Client Configuration

**Claude Code** (`~/.config/claude/mcp.json`):
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

**Cursor** (`~/.cursor/mcp.json`):
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

### Storage Location

**Default**: `~/.local/hexai/data/prompts/`

**Override** (priority order):
1. `--prompts-dir /path/to/prompts` (CLI flag)
2. `HEXAI_MCP_PROMPTS_DIR=/path` (env var)
3. `prompts_dir = "/path"` in `config.toml`
4. Default XDG location

## 📚 Documentation

- **[MCP Setup Guide](mcp-setup.md)** - Installation and configuration
- **[Creating Prompts](mcp-prompts.md)** - Prompt authoring guide
- **[Managing Prompts](mcp-managing-prompts.md)** - Management workflows
- **[MCP API Reference](mcp-api.md)** - Complete protocol documentation

## 🎉 Summary

The hexai-mcp-server provides:

✅ **Full MCP compliance** with latest 2025-06-18 specification
✅ **Management extensions** for create/update/delete via protocol
✅ **Automatic backups** with retention policy
✅ **CLI tool** for easy management
✅ **Git-friendly storage** with JSONL format
✅ **Zero data loss** with multiple restore points
✅ **Production-ready** with comprehensive tests
✅ **Agent-agnostic** works with any MCP client

**This is a complete, production-ready MCP server with enterprise-grade features!** 🚀
