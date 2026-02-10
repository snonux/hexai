# MCP Server Setup Guide

## What is MCP?

Model Context Protocol (MCP) is a standardized protocol for AI agents to discover and use prompts, tools, and resources from external servers. The `hexai-mcp-server` provides a prompt management system that works with any MCP-compatible agent like Claude Code CLI, Cursor, or other AI coding assistants.

## Why Use hexai-mcp-server?

- **Centralized Prompt Management**: Store reusable prompts in one place instead of scattered across config files
- **Agent-Agnostic**: Works with any MCP-compatible agent (Claude Code, Cursor, etc.)
- **Git-Friendly**: JSONL storage format is human-readable and version control friendly
- **Built-in Prompts**: Comes with useful prompts for code review, testing, documentation, etc.
- **Easy Sharing**: Share prompt collections with your team via git repositories

## Installation

The `hexai-mcp-server` binary is installed automatically when you build/install hexai:

```bash
cd ~/git/hexai
go install ./cmd/hexai-mcp-server
```

Verify installation:

```bash
hexai-mcp-server --version
# Output: 0.19.0
```

The binary will be installed to `~/go/bin/hexai-mcp-server` (or wherever your `GOBIN` points).

## Configuring Claude Code CLI

Claude Code CLI discovers MCP servers via `~/.config/claude/mcp.json`. Create or edit this file:

### Option 1: Full Path (Recommended)

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

Replace `/home/paul` with your actual home directory path.

### Option 2: Using $PATH

If `~/go/bin` is in your PATH:

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

### Option 3: Custom Configuration

```json
{
  "mcpServers": {
    "hexai-prompts": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": [
        "--config", "/home/paul/.config/hexai/config.toml",
        "--log", "/tmp/hexai-mcp-server.log"
      ],
      "env": {
        "HEXAI_MCP_PROMPTS_DIR": "/home/paul/Dropbox/hexai-prompts"
      }
    }
  }
}
```

**Configuration Options:**
- `--config`: Path to hexai config file (optional)
- `--log`: Path to log file (default: `~/.local/state/hexai/hexai-mcp-server.log`)
- `--prompts-dir`: Directory for prompt storage (optional)
- `--version`: Print version and exit

**Environment Variables:**
- `HEXAI_MCP_PROMPTS_DIR`: Override prompts directory

## Configuring Cursor

Cursor uses `~/.cursor/mcp.json` for MCP server configuration:

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

After configuring, restart Cursor to load the MCP server.

## Configuring Other MCP Clients

Any MCP-compatible client can use hexai-mcp-server. The general pattern is:

1. Find the client's MCP configuration file (usually in `~/.config/<client>/mcp.json`)
2. Add an entry with the command path to `hexai-mcp-server`
3. Restart the client

## Prompts Directory

By default, prompts are stored in `~/.local/share/hexai/prompts/` (XDG_DATA_HOME). You can customize this location using:

1. **Command-line flag**: `--prompts-dir /path/to/prompts`
2. **Environment variable**: `HEXAI_MCP_PROMPTS_DIR=/path/to/prompts`
3. **Config file**: Add `prompts_dir = "/path/to/prompts"` to `[mcp]` section in `config.toml`
4. **Default**: `$XDG_DATA_HOME/hexai/prompts/` or `~/.local/share/hexai/prompts/`

**Precedence order** (highest to lowest):
1. Command-line flag (`--prompts-dir`)
2. Environment variable (`HEXAI_MCP_PROMPTS_DIR`)
3. Config file (`[mcp] prompts_dir`)
4. Default XDG location

### Custom Prompts Directory Example

To use a git-versioned prompt collection:

```bash
# Clone your team's prompt repository
git clone https://github.com/myteam/hexai-prompts ~/hexai-prompts

# Configure hexai to use it
export HEXAI_MCP_PROMPTS_DIR=~/hexai-prompts

# Or add to config.toml
echo '[mcp]' >> ~/.config/hexai/config.toml
echo 'prompts_dir = "~/hexai-prompts"' >> ~/.config/hexai/config.toml
```

## Testing the Connection

After configuration, test that the MCP server is accessible:

### Method 1: Check Logs

Run the server manually to see if it starts:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | \
  hexai-mcp-server --log /tmp/test-mcp.log

# Check log
cat /tmp/test-mcp.log
```

### Method 2: Use Your Agent

In Claude Code CLI or Cursor, try using a prompt:
- In Claude Code: Type a message and see if hexai prompts appear in suggestions
- In Cursor: Open command palette and search for "hexai" or "prompt"

## Troubleshooting

### Binary Not Found

**Error**: `command not found: hexai-mcp-server`

**Solution**:
1. Use full path: `/home/paul/go/bin/hexai-mcp-server`
2. Add `~/go/bin` to PATH: `export PATH="$HOME/go/bin:$PATH"`
3. Verify installation: `ls -la ~/go/bin/hexai-mcp-server`

### Permission Denied

**Error**: `permission denied: /home/paul/go/bin/hexai-mcp-server`

**Solution**:
```bash
chmod +x ~/go/bin/hexai-mcp-server
```

### Server Not Responding

**Check the log file**:
```bash
tail -f ~/.local/state/hexai/hexai-mcp-server.log
```

Common issues:
- Prompts directory doesn't exist or isn't writable
- Config file has invalid TOML syntax
- Another process is using stdio

### Prompts Not Appearing

1. Verify prompts directory exists:
   ```bash
   ls -la ~/.local/share/hexai/prompts/
   # Should show default.jsonl and possibly user.jsonl
   ```

2. Check default.jsonl has content:
   ```bash
   wc -l ~/.local/share/hexai/prompts/default.jsonl
   # Should show 7 or more lines
   ```

3. Verify JSON format:
   ```bash
   jq -s '.' ~/.local/share/hexai/prompts/default.jsonl
   # Should parse successfully
   ```

### Client Configuration Issues

**Claude Code CLI**:
- Configuration file: `~/.config/claude/mcp.json`
- Restart required after config changes

**Cursor**:
- Configuration file: `~/.cursor/mcp.json`
- Restart Cursor after config changes
- Check Cursor's developer console for errors

## Advanced Configuration

### Multiple Prompt Collections

You can run multiple instances of hexai-mcp-server with different prompt directories:

```json
{
  "mcpServers": {
    "hexai-general": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": ["--prompts-dir", "/home/paul/prompts/general"],
      "env": {}
    },
    "hexai-go": {
      "command": "/home/paul/go/bin/hexai-mcp-server",
      "args": ["--prompts-dir", "/home/paul/prompts/go"],
      "env": {}
    }
  }
}
```

### Shared Team Prompts

Store prompts in a git repository and share with your team:

```bash
# On developer machine
cd ~/hexai-prompts
git add user.jsonl
git commit -m "Add new prompt: optimize_sql"
git push

# On team member's machine
cd ~/hexai-prompts
git pull
# Prompts automatically available
```

## Next Steps

- [Creating Custom Prompts](mcp-prompts.md)
- [hexai Configuration Guide](configuration.md)
