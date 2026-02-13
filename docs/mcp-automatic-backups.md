# MCP Server Automatic Backups

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

## ✅ Fully Automatic - No Manual Tools Required!

The hexai-mcp-server automatically creates backups **on every write operation**. You don't need any CLI tools - everything happens automatically when you use the MCP protocol.

## How It Works

### Architecture

```
MCP Client (Claude Code/Cursor)
    ↓
MCP Protocol (prompts/create, update, delete)
    ↓
MCP Server Handler
    ↓
PromptStore.Create/Update/Delete()
    ↓
[AUTOMATIC BACKUP] ← Happens here automatically!
    ↓
Write to user.jsonl
```

### Automatic Backup Triggers

Every operation through the MCP server automatically creates a backup:

1. **`prompts/create`**
   ```
   Client → MCP Server → Store.Create()
                         ↓
                    [AUTO BACKUP]
                         ↓
                    Save new prompt
   ```

2. **`prompts/update`**
   ```
   Client → MCP Server → Store.Update()
                         ↓
                    [AUTO BACKUP]
                         ↓
                    Save changes
   ```

3. **`prompts/delete`**
   ```
   Client → MCP Server → Store.Delete()
                         ↓
                    [AUTO BACKUP]
                         ↓
                    Remove prompt
   ```

## Backup Details

### Storage Location
```
~/.local/hexai/data/prompts/backups/
├── user.jsonl.20260210-190358
├── user.jsonl.20260210-192145
├── user.jsonl.20260210-193422
└── ... (up to 10 backups)
```

### Backup Format
- **Filename**: `user.jsonl.YYYYMMDD-HHMMSS`
- **Timestamp**: When backup was created
- **Content**: Complete copy of user.jsonl before change

### Retention Policy
- Keeps last **10 backups** automatically
- Oldest backups auto-deleted when limit exceeded
- No manual cleanup needed

## Usage (MCP Clients Only)

### From Claude Code CLI

```javascript
// Claude Code will automatically use these MCP methods:

// Create a prompt
{
  "method": "prompts/create",
  "params": {
    "name": "my_prompt",
    "title": "My Prompt",
    "messages": [...]
  }
}
// ✅ Backup created automatically before save!

// Update a prompt
{
  "method": "prompts/update",
  "params": {
    "name": "my_prompt",
    "title": "Updated Title"
  }
}
// ✅ Backup created automatically before update!

// Delete a prompt
{
  "method": "prompts/delete",
  "params": {
    "name": "old_prompt"
  }
}
// ✅ Backup created automatically before delete!
```

### From Cursor

Same automatic backups happen when using Cursor's MCP integration!

## Manual Recovery (If Needed)

In rare cases where you need to manually restore:

### List Backups
```bash
ls -lht ~/.local/hexai/data/prompts/backups/
```

Output:
```
-rw-r--r-- 1 paul paul 1.2K Feb 10 19:34 user.jsonl.20260210-193422
-rw-r--r-- 1 paul paul 1.1K Feb 10 19:21 user.jsonl.20260210-192145
-rw-r--r-- 1 paul paul 1.0K Feb 10 19:03 user.jsonl.20260210-190358
```

### Restore from Backup
```bash
# Copy backup to restore
cp ~/.local/hexai/data/prompts/backups/user.jsonl.20260210-193422 \
   ~/.local/hexai/data/prompts/user.jsonl

# Restart MCP client to reload
```

## Test Results

The automatic backup system is fully tested:

```
✓ TestAutomaticBackupOnCreate - Backup created on prompts/create
✓ TestAutomaticBackupOnUpdate - Backup created on prompts/update
✓ TestAutomaticBackupOnDelete - Backup created on prompts/delete
```

All tests pass with 71.7% coverage on the promptstore layer.

## Configuration

No configuration needed! Just set up your MCP client:

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

Then all backups happen automatically!

## Summary

✅ **Automatic** - Backups created on every MCP operation
✅ **Transparent** - Happens inside the store layer
✅ **No tools needed** - Everything through MCP protocol
✅ **Retention** - Keeps last 10 backups automatically
✅ **Timestamped** - Easy to identify when backup was made
✅ **Zero config** - Works out of the box
✅ **Tested** - Comprehensive unit tests

**You just use the MCP server through your client (Claude Code/Cursor) and backups happen automatically!** 🚀
