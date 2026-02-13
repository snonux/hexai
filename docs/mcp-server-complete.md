# hexai-mcp-server: Complete Solution

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

## Overview

The hexai-mcp-server is a **fully self-contained MCP server** that manages prompts entirely through the protocol. No additional tools needed!

## ✨ Key Features

### 1. Full MCP Protocol Support
- ✅ Standard methods: `initialize`, `prompts/list`, `prompts/get`
- ✅ Management methods: `prompts/create`, `prompts/update`, `prompts/delete`
- ✅ Protocol version: `2025-06-18` (latest)
- ✅ Advertises `mutable: true` capability

### 2. Automatic Backups
- ✅ **Every write operation** creates a timestamped backup
- ✅ Backups created **before** changes (can always rollback)
- ✅ Keeps last **10 backups** automatically
- ✅ Stored in `~/.local/hexai/data/prompts/backups/`
- ✅ **Zero configuration** - works out of the box

### 3. No External Tools Required
- ✅ Everything through MCP protocol
- ✅ Works with any MCP client (Claude Code, Cursor, etc.)
- ✅ No CLI tools to install or maintain
- ✅ Simple and clean architecture

## 🚀 Usage

### Setup (One Time)

Configure your MCP client:

**Claude Code** (`~/.config/claude/mcp.json`):
```json
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "/home/paul/go/bin/hexai-mcp-server"
    }
  }
}
```

**Cursor** (`~/.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "hexai": {
      "command": "/home/paul/go/bin/hexai-mcp-server"
    }
  }
}
```

### Daily Use

Everything through your MCP client!

#### Create a Prompt
```json
{
  "method": "prompts/create",
  "params": {
    "name": "optimize_code",
    "title": "Optimize Code",
    "description": "Analyzes and optimizes code for performance",
    "arguments": [
      {"name": "code", "description": "Code to optimize", "required": true}
    ],
    "messages": [
      {
        "role": "user",
        "content": {"type": "text", "text": "Optimize: {{code}}"}
      }
    ],
    "tags": ["optimization", "performance"]
  }
}
```
→ **Automatic backup created!**

#### Update a Prompt
```json
{
  "method": "prompts/update",
  "params": {
    "name": "optimize_code",
    "description": "Enhanced optimization with profiling suggestions"
  }
}
```
→ **Automatic backup created!**

#### Delete a Prompt
```json
{
  "method": "prompts/delete",
  "params": {"name": "old_prompt"}
}
```
→ **Automatic backup created!**

#### List All Prompts
```json
{
  "method": "prompts/list",
  "params": {}
}
```

#### Use a Prompt
```json
{
  "method": "prompts/get",
  "params": {
    "name": "optimize_code",
    "arguments": {"code": "function slow() { ... }"}
  }
}
```

## 📦 Built-in Prompts

7 production-ready prompts included:
1. **code_review** - Code quality analysis
2. **explain_code** - Detailed explanations
3. **generate_tests** - Unit test generation
4. **document_function** - Documentation generation
5. **simplify_code** - Code simplification
6. **fix_bugs** - Bug analysis and fixes
7. **refactor_extract** - Extract to functions

## 🔒 Automatic Backup System

### How It Works

```
Client → MCP Server → Store Operation
                      ↓
                [CREATE BACKUP]
                      ↓
                Write to File
```

Every operation automatically:
1. Creates timestamped backup
2. Saves changes
3. Cleans old backups (keeps 10)

### Backup Files

```
~/.local/hexai/data/prompts/backups/
├── user.jsonl.20260210-193422  ← Most recent
├── user.jsonl.20260210-192145
├── user.jsonl.20260210-190358
└── ... (up to 10 backups)
```

### Manual Restore (If Needed)

```bash
# List backups
ls -lht ~/.local/hexai/data/prompts/backups/

# Restore from backup
cp ~/.local/hexai/data/prompts/backups/user.jsonl.20260210-193422 \
   ~/.local/hexai/data/prompts/user.jsonl

# Restart MCP client
```

## 📁 File Structure

```
~/.local/hexai/data/prompts/
├── default.jsonl           # Built-in prompts (7 prompts)
├── user.jsonl              # Your custom prompts
└── backups/                # Automatic backups (10 most recent)
    ├── user.jsonl.20260210-193422
    ├── user.jsonl.20260210-192145
    └── ...
```

## 🎯 Architecture

### Simple & Clean

```
┌─────────────────┐
│   MCP Client    │ (Claude Code, Cursor, etc.)
│  (any client)   │
└────────┬────────┘
         │ MCP Protocol
         │ (prompts/create, update, delete, list, get)
         ↓
┌─────────────────┐
│  MCP Server     │
│ hexai-mcp-server│
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│  PromptStore    │
│  (automatic     │
│   backups)      │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│   JSONL Files   │
│  + Backups      │
└─────────────────┘
```

### No External Dependencies

- ✅ Pure Go stdlib
- ✅ No database required
- ✅ No additional CLI tools
- ✅ Just the MCP server binary

## 📊 Test Coverage

```
✓ All MCP handlers tested (create/update/delete)
✓ Automatic backups tested (100% coverage)
✓ Store operations tested (71.7% coverage)
✓ MCP protocol tested (52.4% coverage)
```

## 🔧 Configuration

### Prompts Directory

**Default**: `~/.local/hexai/data/prompts/`

**Override** (priority order):
1. CLI flag: `--prompts-dir /path/to/prompts`
2. Env var: `HEXAI_MCP_PROMPTS_DIR=/path/to/prompts`
3. Config file: `[mcp] prompts_dir = "/path"`
4. Default XDG location

### Backup Settings

Hardcoded in `internal/promptstore/store.go`:
- `maxBackups = 10` (keeps last 10 backups)

To change, edit the NewJSONLStore function.

## 📚 Documentation

- **[MCP Setup](mcp-setup.md)** - Installation guide
- **[Creating Prompts](mcp-prompts.md)** - Prompt authoring
- **[Managing Prompts](mcp-managing-prompts.md)** - Management workflows
- **[API Reference](mcp-api.md)** - Complete protocol docs
- **[Automatic Backups](mcp-automatic-backups.md)** - Backup details

## ✅ Verification

Everything tested and working:

```bash
# Build
cd ~/git/hexai
go build ./cmd/hexai-mcp-server

# Test
go test ./internal/mcp/...         # MCP protocol
go test ./internal/promptstore/... # Storage + backups

# Install
go install ./cmd/hexai-mcp-server

# Verify
hexai-mcp-server --version
# Output: 0.19.0
```

## 🎉 Summary

**One binary. Complete solution.**

- ✅ MCP protocol with management methods
- ✅ Automatic backups on every change
- ✅ No external tools required
- ✅ Works with any MCP client
- ✅ Production-ready with tests
- ✅ Simple, clean architecture

**Just install the server, configure your MCP client, and everything works automatically!** 🚀
