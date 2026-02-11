# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

It has got improved capabilities for Go code understanding (for example, create unit tests from function), but other programming language work as well.

## Features

* LSP AI Code auto-completion
* LSP AI Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
* Parallel completions and CLI responses from multiple providers/models for side-by-side comparison
* **MCP server for prompt/runbook management** (`hexai-mcp-server`)
  - Create, update, delete, and retrieve prompts via MCP protocol
  - Automatic backups on every change (keeps last 10)
  - Compatible with Claude Code CLI, Cursor, and other MCP clients
  - File-based storage with JSONL format (git-friendly)
  - Built-in prompts for code review, testing, documentation, etc.
* TUI AI code-action runner (`hexai-tmux-action`) with Bubble Tea
  - Includes a "Custom prompt" action (hotkey `p`) that opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary Markdown file.
* Tmux popup editor (`hexai-tmux-edit`) for composing longer AI agent prompts
  - Opens `$EDITOR` in a tmux popup, pre-filled with the current prompt text
  - Auto-detects Claude Code, Cursor, Amp, Aider (WIP), and other agents
  - Config-driven: add new agents via `[tmux_edit]` in config.toml
* Support for OpenAI, OpenRouter, Anthropic, and Ollama

## Documentation

* [Build and install guide](docs/buildandinstall.md)
* [Configuration guide](docs/configuration.md)
* [Usage examples](docs/usage.md)
* [Helix + tmux quickstart](docs/tmux.md)
* [MCP server setup guide](docs/mcp-setup.md)
* [Creating custom prompts](docs/mcp-prompts.md)

## Tmux Status Line

See the [tmux integration guide](docs/tmux.md) for details on configuring the status line.

## File Locations

hexai follows the XDG Base Directory Specification:

- **Configuration:** `~/.config/hexai/config.toml` (or `$XDG_CONFIG_HOME/hexai/`)
  - Global settings, provider credentials, and preferences
  - Project-specific: `.hexaiconfig.toml` at repository root
- **Cache:** `~/.local/hexai/cache/` (or `$XDG_CACHE_HOME/hexai/`)
  - `stats.json` - LLM usage tracking (regenerable)
  - `stats.lock` - File lock for stats access
- **State & Logs:** `~/.local/hexai/state/` (or `$XDG_STATE_HOME/state/`)
  - `tmux-edit-history.jsonl` - History of text submitted via tmux popup
  - `hexai-lsp.log` - LSP server debug logs
  - `hexai-tmux-edit.log` - Tmux edit debug logs
  - `hexai-mcp-server.log` - MCP server debug logs
- **Data:** `~/.local/hexai/data/` (or `$XDG_DATA_HOME/`)
  - `prompts/default.jsonl` - Built-in prompts for MCP server
  - `prompts/user.jsonl` - User-created custom prompts
- **Temporary Files:** `/tmp/` (OS temp directory)
  - `hexai-*.md` - Temporary editor workspaces (auto-deleted)
