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

- Hexai can update a tmux user option during LLM activity. Add this to your `~/.tmux.conf` to display it:

  - `set -g status-right '#{@hexai_status} #[fg=colour8]| %H:%M'`

- Status content is updated best‑effort at key moments:
  - CLI: start (⏳ provider:model) and completion with compact stats (↑sent, ↓recv, rpm, reqs)
  - LSP: initial heartbeat on client initialize, and periodic compact stats (provider, model, rpm, reqs, bytes)
  - TUI action runner: ready (provider:model) and completion with compact stats

- Toggle via environment:
  - Enable (default): unset or `HEXAI_TMUX_STATUS=1`
  - Disable: `HEXAI_TMUX_STATUS=0`

- Theme and colors:
  - Preset: `HEXAI_TMUX_STATUS_THEME=white-on-purple` (white fg on purple/magenta bg)
  - Explicit: `HEXAI_TMUX_STATUS_FG=<color>`, `HEXAI_TMUX_STATUS_BG=<color>` (e.g., `white`, `magenta`, `colour5`)
  - Colors use tmux’s `fg`/`bg` names; both methods wrap the entire `@hexai_status` segment.
  - If truncated, increase width: `set -g status-right-length 120`

Code action prompts

- All prompts can be customized under `[prompts.code_action]` in `config.toml`. In addition to `rewrite_*`, `diagnostics_*`, `document_*`, and `go_test_*`, the following templates control the “Simplify and improve” action:
  - `simplify_system`
  - `simplify_user` (uses `{{selection}}`)
