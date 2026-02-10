# Hexai configuration

This page explains where the config lives and how to choose a style; the authoritative list of options and comments lives in the example file.

Global config file

- Location: `$XDG_CONFIG_HOME/hexai/config.toml` (usually `~/.config/hexai/config.toml`).
- Style: sectioned tables only — see [config.toml.example](../config.toml.example) for a complete, commented reference.

Per-project config file

- Place a `.hexaiconfig.toml` at the root of a git repository to selectively override the global config for that project.
- Uses the same TOML format as the global config file — only specify the settings you want to override.
- Hexai auto-detects the git repository root by walking up from the current working directory.
- Precedence (lowest to highest): built-in defaults → global config → per-project config → environment variables.

Environment overrides

- All options can be overridden by environment variables prefixed with `HEXAI_`.
- Env values always take precedence over both the global and per-project config files.
- Examples:
  - `HEXAI_PROVIDER`, `HEXAI_MAX_TOKENS`, `HEXAI_CONTEXT_MODE`, `HEXAI_CONTEXT_WINDOW_LINES`, `HEXAI_MAX_CONTEXT_TOKENS`, `HEXAI_LOG_PREVIEW_LIMIT`, `HEXAI_REQUEST_TIMEOUT`
  - `HEXAI_CODING_TEMPERATURE`
  - `HEXAI_COMPLETION_DEBOUNCE_MS`, `HEXAI_COMPLETION_THROTTLE_MS`
  - `HEXAI_TRIGGER_CHARACTERS` (comma-separated, e.g., `".,:,_ , "`)
  - `HEXAI_INLINE_OPEN`, `HEXAI_INLINE_CLOSE`
  - `HEXAI_CHAT_SUFFIX`, `HEXAI_CHAT_PREFIXES` (comma-separated)
  - `HEXAI_OPENAI_MODEL`, `HEXAI_OPENAI_BASE_URL`, `HEXAI_OPENAI_TEMPERATURE`
  - `HEXAI_OPENROUTER_MODEL`, `HEXAI_OPENROUTER_BASE_URL`, `HEXAI_OPENROUTER_TEMPERATURE`
  - `HEXAI_OLLAMA_MODEL`, `HEXAI_OLLAMA_BASE_URL`, `HEXAI_OLLAMA_TEMPERATURE`
  - Per-surface overrides: `HEXAI_MODEL_COMPLETION`, `HEXAI_MODEL_CODE_ACTION`, `HEXAI_MODEL_CHAT`, `HEXAI_MODEL_CLI`
  - Per-surface temperatures: `HEXAI_TEMPERATURE_COMPLETION`, `HEXAI_TEMPERATURE_CODE_ACTION`, `HEXAI_TEMPERATURE_CHAT`, `HEXAI_TEMPERATURE_CLI`

Per-surface models

- Use the `[models]` table in `config.toml` to tailor individual entry points (completion, code actions, chat, CLI) without changing the global provider default.
- All entry points accept `--config /path/to/config.toml` to point at an alternate file. Helix `/reload>` reuses the same path when active.
- Each key accepts either a string (shortcut) or one or more tables with `model` / `temperature` fields, e.g.:

  ```toml
  [models]
  completion = "gpt-4.1-mini"

  [models.code_action]
  model = "gpt-4o"
  provider = "openai"
  temperature = 0.4

  [models.cli]
  model = "gpt-4.1"
  provider = "openai"
  ```

- Repeating the table (`[[models.<surface>]]`) configures multiple provider/model pairs. Completion requests and the Hexai CLI fan out to every configured entry concurrently and label the responses with `provider:model`. Code actions continue to use the first entry only; any extra [[models.code_action]] tables are ignored at runtime and the loader logs a warning so you know an additional entry was skipped.

- When a per-surface value is omitted, Hexai falls back to the provider’s configured default. Temperatures inherit from `coding_temperature` unless explicitly set, and OpenAI `gpt-5*` models automatically raise an unspecified coding temperature to `1.0` for exploratory behavior. Provider overrides support `"openai"`, `"openrouter"`, `"anthropic"`, or `"ollama"` and read the matching credential variables.

Runtime reloads


- The Hexai LSP can reload `config.toml` without restarting the editor session.
- Type `/reload>` in an inline chat prompt to reapply file changes; environment overrides are ignored during this reload so the file becomes authoritative.
- Type `/disable>` to temporarily pause auto-completions (chat prompts and actions keep working) and `/enable>` to resume them without restarting the session.
- The client echoes a summary of the detected differences and logs the same details.

API keys:

- OpenAI: prefer `HEXAI_OPENAI_API_KEY`, falling back to `OPENAI_API_KEY`.
- OpenRouter: prefer `HEXAI_OPENROUTER_API_KEY`, falling back to `OPENROUTER_API_KEY`.

Selecting a provider

- Sectioned: set `[provider] name = "openai" | "openrouter" | "anthropic" | "ollama"`.
- If omitted, Hexai defaults to `openai`.
- Selecting `openrouter` uses https://openrouter.ai/api/v1 by default and automatically sends the required `HTTP-Referer` (`https://github.com/snonux/hexai`) and `X-Title` (`Hexai`) headers. Override the base URL via `[openrouter]` or environment variables when needed.

Notes on Ollama:

- Ensure the model is available locally (e.g., `ollama pull qwen3-coder:30b-a3b-q4_K_M`).
- Alternatively, run Ollama in OpenAI‑compatible mode and use the OpenAI provider with
  `openai_base_url` pointed at your local endpoint.

Hexai Action (TUI) configuration

This is mostly useful when Helix runs in a [tmux](https://tmux.github.io/) session!

- Helix integration (recommended): bind a key to pipe the current selection to `hexai-tmux-action` and replace it with the output.
  - Example: `C-a = ":pipe hexai-tmux-action"`
- Default behavior:
  - Inline TUI when run in a real terminal (TTY).
  - When invoked via Helix `:pipe`, `hexai-tmux-action` opens a split pane to render the menu and returns the result on stdout for Helix to apply.
  - If no TTY and no tmux are available, it falls back to echoing the input.
- Flags:
  - `--infile`  Read input from the given file instead of stdin.
  - `--outfile` Write output to the given file instead of stdout (truncates/creates).
  - `--tmux-target` tmux target pane/window (advanced).
  - `--tmux-split v|h` split orientation (default: `v`).
  - `--tmux-percent N` split size percentage (default: `33`).
  - `--ui-child` internal; used by the parent process when spawning inside tmux.

Editor integration

- Hexai tries to launch your preferred editor when needed (e.g., TUI “Custom prompt”, CLI with no args).
- Editor resolution: `HEXAI_EDITOR`, falling back to `EDITOR` when unset.
- Invocation form: `EDITOR /tmp/hexai-XXXX.md` (a temporary Markdown file).

Tmux status line

See the [tmux integration guide](docs/tmux.md) for details on configuring the status line.

Code action prompts

- All prompts can be customized under `[prompts.code_action]` in `config.toml`. In addition to `rewrite_*`, `diagnostics_*`, `document_*`, and `go_test_*`, the following templates control the \u201cSimplify and improve\u201d action:
  - `simplify_system`
  - `simplify_user` (uses `{{selection}}`)

Hexai Tmux Edit (popup editor)

- `hexai-tmux-edit` opens `$EDITOR` in a tmux popup for composing longer AI agent prompts.
- Configure popup dimensions and agent detection patterns in the `[tmux_edit]` section:

  ```toml
  [tmux_edit]
  popup_width = "80%"
  popup_height = "80%"
  # default_agent = "claude"   # force agent; skip auto-detect
  ```

- Override or add agent definitions with `[[tmux_edit.agents]]` (merged with built-in defaults by name):

  ```toml
  [[tmux_edit.agents]]
  name = "claude"
  display_name = "Claude Code"
  detect_pattern = "(?i)(claude|anthropic)"
  prompt_pattern = '(?m)>\s*(.+)$'
  clear_first = true
  clear_keys = "C-u"
  newline_keys = "S-Enter"
  submit_keys = "Enter"
  ```

- Built-in agents: `claude`, `cursor`, `amp`, `aider`. See [config.toml.example](../config.toml.example) for all fields.
- Tmux keybinding: `bind e run-shell -b "cd '#{pane_current_path}' && hexai-tmux-edit --pane '#{pane_id}'"`
