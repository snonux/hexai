# Hexai configuration

This page explains where the config lives and how to choose a style; the authoritative list of options and comments lives in the example file.

Config file

- Location: `$XDG_CONFIG_HOME/hexai/config.toml` (usually `~/.config/hexai/config.toml`).
- Style: sectioned tables only — see [config.toml.example](../config.toml.example) for a complete, commented reference.

Environment overrides

- All options can be overridden by environment variables prefixed with `HEXAI_`.
- Env values take precedence over the config file.
- Examples:
  - `HEXAI_PROVIDER`, `HEXAI_MAX_TOKENS`, `HEXAI_CONTEXT_MODE`, `HEXAI_CONTEXT_WINDOW_LINES`, `HEXAI_MAX_CONTEXT_TOKENS`, `HEXAI_LOG_PREVIEW_LIMIT`
  - `HEXAI_CODING_TEMPERATURE`
  - `HEXAI_COMPLETION_DEBOUNCE_MS`, `HEXAI_COMPLETION_THROTTLE_MS`
  - `HEXAI_TRIGGER_CHARACTERS` (comma-separated, e.g., `".,:,_ , "`)
  - `HEXAI_INLINE_OPEN`, `HEXAI_INLINE_CLOSE`
  - `HEXAI_CHAT_SUFFIX`, `HEXAI_CHAT_PREFIXES` (comma-separated)
  - `HEXAI_OPENAI_MODEL`, `HEXAI_OPENAI_BASE_URL`, `HEXAI_OPENAI_TEMPERATURE`
  - `HEXAI_COPILOT_MODEL`, `HEXAI_COPILOT_BASE_URL`, `HEXAI_COPILOT_TEMPERATURE`
  - `HEXAI_OLLAMA_MODEL`, `HEXAI_OLLAMA_BASE_URL`, `HEXAI_OLLAMA_TEMPERATURE`
  - Per-surface overrides: `HEXAI_MODEL_COMPLETION`, `HEXAI_MODEL_CODE_ACTION`, `HEXAI_MODEL_CHAT`, `HEXAI_MODEL_CLI`
  - Per-surface temperatures: `HEXAI_TEMPERATURE_COMPLETION`, `HEXAI_TEMPERATURE_CODE_ACTION`, `HEXAI_TEMPERATURE_CHAT`, `HEXAI_TEMPERATURE_CLI`

Per-surface models

- Use the `[models]` table in `config.toml` to tailor individual entry points (completion, code actions, chat, CLI) without changing the global provider default.
- Each key accepts either a string (shortcut) or a table with `model` / `temperature` fields, e.g.:

  ```toml
  [models]
  completion = "gpt-4.1-mini"

  [models.code_action]
  model = "gpt-4o"
  provider = "copilot"
  temperature = 0.4

  [models.cli]
  model = "gpt-4.1"
  provider = "openai"
  ```

- When a per-surface value is omitted, Hexai falls back to the provider’s configured default. Temperatures inherit from `coding_temperature` unless explicitly set, and OpenAI `gpt-5*` models automatically raise an unspecified coding temperature to `1.0` for exploratory behavior. Provider overrides support `"openai"`, `"copilot"`, or `"ollama"` and read the matching credential variables.

Runtime reloads


- The Hexai LSP can reload `config.toml` without restarting the editor session.
- Type `/reload>` in an inline chat prompt to reapply file changes; environment overrides are ignored during this reload so the file becomes authoritative.
- The client echoes a summary of the detected differences and logs the same details.

API keys:

- OpenAI: prefer `HEXAI_OPENAI_API_KEY`, falling back to `OPENAI_API_KEY`.
- Copilot: prefer `HEXAI_COPILOT_API_KEY`, falling back to `COPILOT_API_KEY`.

Selecting a provider

- Sectioned: set `[provider] name = "openai" | "copilot" | "ollama"`.
- If omitted, Hexai defaults to `openai`.

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

- All prompts can be customized under `[prompts.code_action]` in `config.toml`. In addition to `rewrite_*`, `diagnostics_*`, `document_*`, and `go_test_*`, the following templates control the “Simplify and improve” action:
  - `simplify_system`
  - `simplify_user` (uses `{{selection}}`)
