# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

It has got improved capabilities for Go code understanding (for example, create unit tests from function), but other programming language work as well.

## Features

* LSP AI Code auto-completion
* LSP AI Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
  - `hexai config` opens the global `config.toml` in `$HEXAI_EDITOR` or `$EDITOR` (supports `--config` for an alternate file)
  - Includes `--tps-simulation` to preview how fast a model would feel by streaming placeholder text or piped stdin at a chosen token-per-second rate
* Task management CLI for agent-managed project work
  - Entrypoint: `ask` (the binary was briefly named `do`; use `ask` in scripts and documentation)
  - Auto-scopes to `project:<repo> +agent` (derived from git repo root)
  - Override the project explicitly with `ask proj:<name> <subcommand...>`
  - Prefixes can be combined, for example `ask proj:<name> na <subcommand...>`
  - Never exposes numeric task IDs; human-facing output uses stable alias IDs
  - `ask info` hides raw UUIDs unless `HEXAI_DEBUG` is set
  - Machine-friendly output with suppressed decorative text
  - Subcommands: `ask add`, `ask list`, `ask info`, `ask annotate`, `ask start`, `ask stop`, `ask done`, `ask priority`, `ask tag`, `ask dep`, `ask urgency`, `ask watch`, `ask projects`, `ask modify`, `ask denotate`, `ask delete`, `ask fish`, `ask help`
  - Fish shell completions: run `ask fish | source` in a session, or use a `conf.d` snippet (see [Fish shell completion](docs/fish-completion.md)); installs do not write files under `fish/completions/`
* Parallel completions and CLI responses from multiple providers/models for side-by-side comparison
* **MCP server for prompt/runbook management** (`hexai-mcp-server`) - **⚠️ DEPRECATED/EXPERIMENTAL**
  - Create, update, delete, and retrieve prompts via MCP protocol
  - Automatic backups on every change (keeps last 10)
  - Compatible with Claude Code CLI, Cursor, and other MCP clients
  - File-based storage with JSONL format (git-friendly)
  - Built-in meta-prompts for interactive prompt creation and management
* TUI AI code-action runner (`hexai-tmux-action`) with Bubble Tea
  - Opens as a tmux popup when invoked from Helix `:pipe`
  - Built-in actions: Rewrite (`r`), Simplify (`i`), Document (`c`), Go tests (`t`), Fix typos (`f`), Custom prompt (`p`), Skip (`s`)
  - Fully configurable menu via `[[tmux_action.menu]]` — reorder, remove, rename, rebind hotkeys, embed custom actions directly in main menu
  - All action prompts overridable via `[prompts.code_action]` in `config.toml`
  - Custom prompt action opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary Markdown file
* Support for Ollama (local + Ollama Cloud), OpenAI, OpenRouter, Anthropic, and You.com (YouSearch Research API) — Ollama Cloud (`kimi-k2.6` at `https://ollama.com`) is the default

> **Note on hexai-mcp-server:** This component is currently experimental and not actively maintained. The author manages prompts through slash commands and meta-commands in the hexai agent system, making the MCP server redundant for its original purpose. The code is preserved for potential future enhancements with different functionality beyond prompt management. See the [MCP documentation](docs/mcp-setup.md) for reference only.

## Documentation

* [Build and install guide](docs/buildandinstall.md)
* [Configuration guide](docs/configuration.md)
* [Usage examples](docs/usage.md)
* [Helix + tmux quickstart](docs/tmux.md)
* [Fish shell completion](docs/fish-completion.md)
* [MCP server setup guide](docs/mcp-setup.md) *(deprecated - reference only)*
* [Creating custom prompts](docs/mcp-prompts.md) *(deprecated - reference only)*

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
  - `hexai-lsp-server.log` - LSP server debug logs
  - `hexai-mcp-server.log` - MCP server debug logs
- **Data:** `~/.local/hexai/data/` (or `$XDG_DATA_HOME/`)
  - `prompts/user.jsonl` - User-created custom prompts (built-in prompts are compiled into the binary)
- **Temporary Files:** `/tmp/` (OS temp directory)
  - `hexai-*.md` - Temporary editor workspaces (auto-deleted)
