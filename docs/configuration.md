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
- Flat: set `provider = "openai" | "copilot" | "ollama"`.
- If omitted, Hexai defaults to `openai`.

Provider-specific options

- See [config.toml.example](../config.toml.example) for the per-provider tables and defaults.

Notes:

- Ensure the model is available locally (e.g., `ollama pull qwen3-coder:30b-a3b-q4_K_M`).
- Alternatively, run Ollama in OpenAI‑compatible mode and use the OpenAI provider with
  `openai_base_url` pointed at your local endpoint.

LSP completion tuning

- See the [completion] section in [config.toml.example](../config.toml.example).

Temperature behavior

- Defaults and recommended ranges are commented inline in [config.toml.example](../config.toml.example) under [general] and provider tables.
