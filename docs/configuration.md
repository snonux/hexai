# Hexai configuration

This page explains where the config lives and how to choose a style; the authoritative list of options and comments lives in the example file.

Global config file

- Location: `$XDG_CONFIG_HOME/hexai/config.toml` (usually `~/.config/hexai/config.toml`).
- Edit from a terminal: `hexai config` opens that file in `$HEXAI_EDITOR` or `$EDITOR` (or `hexai --config /path/to.toml config` for a specific file).
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
  - `HEXAI_YOUSEARCH_RESEARCH_EFFORT` (`lite` | `standard` | `deep` | `exhaustive`)
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

- When a per-surface value is omitted, Hexai falls back to the provider’s configured default. Temperatures inherit from `coding_temperature` unless explicitly set, and OpenAI `gpt-5*` models automatically raise an unspecified coding temperature to `1.0` for exploratory behavior. Provider overrides support `"openai"`, `"openrouter"`, `"anthropic"`, `"ollama"`, or `"yousearch"` and read the matching credential variables.

Runtime reloads


- The Hexai LSP can reload `config.toml` without restarting the editor session.
- Type `/reload>` in an inline chat prompt to reapply file changes; environment overrides are ignored during this reload so the file becomes authoritative.
- Type `/disable>` to temporarily pause auto-completions (chat prompts and actions keep working) and `/enable>` to resume them without restarting the session.
- The client echoes a summary of the detected differences and logs the same details.

API keys:

- Ollama Cloud (recommended default): prefer `HEXAI_OLLAMA_API_KEY`, falling back to `OLLAMA_API_KEY`. The key is optional — leave it unset to talk to a local Ollama server.
- OpenAI: prefer `HEXAI_OPENAI_API_KEY`, falling back to `OPENAI_API_KEY`.
- OpenRouter: prefer `HEXAI_OPENROUTER_API_KEY`, falling back to `OPENROUTER_API_KEY`.
- Anthropic: prefer `HEXAI_ANTHROPIC_API_KEY`, falling back to `ANTHROPIC_API_KEY`.
- You.com (YouSearch): prefer `HEXAI_YOUSEARCH_API_KEY`, falling back to `YOU_API_KEY`.

Selecting a provider

- Sectioned: set `[provider] name = "ollama" | "openai" | "openrouter" | "anthropic" | "yousearch"`.
- If omitted, Hexai defaults to `ollama` (Ollama Cloud at `https://ollama.com` with model `kimi-k2.6`).
- Selecting `openrouter` uses https://openrouter.ai/api/v1 by default and automatically sends the required `HTTP-Referer` (`https://github.com/snonux/hexai`) and `X-Title` (`Hexai`) headers. Override the base URL via `[openrouter]` or environment variables when needed.

Notes on Ollama:

- The default Ollama base URL is `https://ollama.com` (Ollama Cloud). Set `HEXAI_OLLAMA_API_KEY` (or `OLLAMA_API_KEY`) to authenticate.
- To use a local Ollama server instead, set `[ollama] base_url = "http://localhost:11434"` and pick a locally pulled model (e.g. `ollama pull qwen3-coder:30b-a3b-q4_K_M`). No API key is required when targeting localhost.
- Alternatively, run Ollama in OpenAI‑compatible mode and use the OpenAI provider with `openai_base_url` pointed at your local endpoint.

Notes on YouSearch (You.com Research API):

- Calls `https://api.you.com/v1/research` with the last user message as the research query; system messages are ignored because the Research API has its own reasoning pipeline.
- Configure the research effort under `[yousearch]` (or via `HEXAI_YOUSEARCH_RESEARCH_EFFORT`): `lite`, `standard` (default), `deep`, or `exhaustive`.
- Sources returned by the API are appended to the response as a `**Sources:**` markdown section.

Hexai Action (TUI) configuration

This is mostly useful when Helix runs in a [tmux](https://tmux.github.io/) session!

- Helix integration (recommended): bind a key to pipe the current selection to `hexai-tmux-action` and replace it with the output.
  - Example: `C-a = ":pipe hexai-tmux-action"`
- Default behavior:
  - When invoked via Helix `:pipe`, `hexai-tmux-action` opens a tmux popup to render the menu and returns the result on stdout for Helix to apply.
- Flags:
  - `--infile`  Read input from the given file instead of stdin.
  - `--outfile` Write output to the given file instead of stdout (truncates/creates).
  - `--tmux-target` tmux target pane (advanced).
  - `--tmux-popup-width` popup width (default: `60%`).
  - `--tmux-popup-height` popup height (default: `50%`).
  - `--ui-child` internal; used by the parent process when spawning inside tmux.

Configurable menu

By default `hexai-tmux-action` shows all built-in actions. Define `[[tmux_action.menu]]` in `config.toml` to fully replace the menu — reorder, remove, rename, rebind hotkeys, or embed custom actions directly in the main menu instead of the submenu:

```toml
[[tmux_action.menu]]
kind   = "rewrite"
hotkey = "r"

[[tmux_action.menu]]
kind   = "fix_typos"
hotkey = "f"
title  = "Proofread"   # optional title override

[[tmux_action.menu]]
kind      = "custom"
custom_id = "extract-function"   # references [[prompts.code_action.custom]]
hotkey    = "e"

[[tmux_action.menu]]
kind   = "skip"
hotkey = "s"
```

Valid built-in kinds: `rewrite`, `simplify`, `document`, `gotest`, `fix_typos`, `custom_prompt`, `skip`.

Editor integration

- Hexai tries to launch your preferred editor when needed (e.g., TUI “Custom prompt”, CLI with no args).
- Editor resolution: `HEXAI_EDITOR`, falling back to `EDITOR` when unset.
- Invocation form: `EDITOR /tmp/hexai-XXXX.md` (a temporary Markdown file).

Tmux status line

See the [tmux integration guide](docs/tmux.md) for details on configuring the status line.

Code action prompts

All prompts used by `hexai-tmux-action` (and the LSP code actions) can be overridden under `[prompts.code_action]` in `config.toml`. Each action has a `*_system` and a `*_user` template. Specifying a value completely replaces the built-in default; omitting it leaves the default in place.

| Key prefix | Action |
|---|---|
| `rewrite_*` | Rewrite selection |
| `diagnostics_*` | Resolve diagnostics |
| `document_*` | Document code |
| `go_test_*` | Generate Go unit test(s) |
| `simplify_*` | Simplify and improve |
| `fix_typos_*` | Fix typos and improve grammar and clarity |

User templates support `{{selection}}` (always available) and `{{diagnostics}}` (diagnostics scope). See [config.toml.example](../config.toml.example) for the full defaults.
