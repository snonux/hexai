# Hexai configuration

This document covers all configuration options for Hexai, including the config file,
environment overrides, provider selection, and temperature behavior.

## Config file

The config file is optional. 

- Location: `$XDG_CONFIG_HOME/hexai/config.json` (usually `~/.config/hexai/config.json`).
- Example:

```json
{
  "max_tokens": 4000,
  "context_mode": "always-full",
  "context_window_lines": 120,
  "max_context_tokens": 4000,
  "log_preview_limit": 100,
  "completion_debounce_ms": 200,
  "completion_throttle_ms": 0,
  "no_disk_io": true,
  "trigger_characters": [".", ":", "/", "_", " " ],
  "inline_open": ">",
  "inline_close": ">",
  "chat_suffix": ">",
  "chat_prefixes": ["?", "!", ":", ";"],
  "coding_temperature": 0.2,
  "provider": "ollama",
  "copilot_model": "gpt-4o-mini",
  "copilot_base_url": "https://api.githubcopilot.com",
  "copilot_temperature": 0.2,
  "openai_model": "gpt-4.1",
  "openai_base_url": "https://api.openai.com/v1",
  "openai_temperature": 0.2,
  "ollama_model": "qwen3-coder:30b-a3b-q4_K_M",
  "ollama_base_url": "http://localhost:11434",
  "ollama_temperature": 0.2
}
```

Key fields:

- max_tokens: upper bound for a single LLM response.
- context_mode: `minimal` | `window` | `file-on-new-func` | `always-full`.
- context_window_lines: line count for `window` mode.
- max_context_tokens: hard cap for sent context tokens.
- log_preview_limit: max characters of context preview logged.
- completion_debounce_ms: minimum idle time before sending completion requests.
- completion_throttle_ms: minimum spacing between completion requests (0 disables).
- manual_invoke_min_prefix: minimum typed identifier chars required for manual invoke to proceed without structural triggers (0 allows always).
- no_disk_io: avoid reading files from disk when building context.
- trigger_characters: LSP completion trigger characters.
- inline_open / inline_close: characters that bracket inline prompts (default `>`/`>`). Inline prompts support `>text>` and a double-open variant `>>text>`. Single-character markers are required.
- chat_suffix / chat_prefixes: in-editor chat triggers (default suffix `>` and prefixes `["?","!",":",";"]`). A line ending with one of these prefixes immediately followed by the suffix triggers a chat reply (e.g., `What?>`). Prefixes must be single characters.
- coding_temperature: optional override for LSP calls.
- provider: `openai` | `copilot` | `ollama`.

### Trigger customization

Defaults use `>` for inline prompts and chat suffix. You can change them, e.g.:

```json
{
  "inline_open": "<",
  "inline_close": ">",
  "chat_suffix": "/",
  "chat_prefixes": ["?", "!"],
  "trigger_characters": [".", ":", "/", "_", " "]
}
```

Notes:
- `inline_open`/`inline_close` must be single characters; `>>text>` is the double‑open variant.
- `chat_prefixes` items must be single characters.

## Environment overrides

- All config-file options can be overridden by environment variables prefixed with `HEXAI_`.
- Env values take precedence over `config.json`.
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

## Selecting a provider

- Set `provider` in the config to `openai`, `copilot`, or `ollama`.
- If omitted, Hexai defaults to `openai`.

### OpenAI configuration

- Required: `HEXAI_OPENAI_API_KEY` (or `OPENAI_API_KEY`).
- Options:
  - `openai_model` — model name (default: `gpt-4.1`).
  - `openai_base_url` — API base (default: `https://api.openai.com/v1`).
  - `openai_temperature` — default temperature (coding-friendly `0.2`).

### GitHub Copilot configuration

- Required: `COPILOT_API_KEY`.
- Options:
  - `copilot_model` — model name (default: `gpt-4o-mini`).
  - `copilot_base_url` — API base (default: `https://api.githubcopilot.com`).
  - `copilot_temperature` — default temperature (coding-friendly `0.2`).

### Ollama configuration

- Options:
  - `ollama_model` — model name/tag (default: `qwen3-coder:30b-a3b-q4_K_M`).
  - `ollama_base_url` — base URL (default: `http://localhost:11434`).
  - `ollama_temperature` — default temperature (coding-friendly `0.2`).

Notes:

- Ensure the model is available locally (e.g., `ollama pull qwen3-coder:30b-a3b-q4_K_M`).
- Alternatively, run Ollama in OpenAI‑compatible mode and use the OpenAI provider with
  `openai_base_url` pointed at your local endpoint.

## LSP completion tuning

- Debounce: `completion_debounce_ms` waits until there has been no recent input for at least this many milliseconds before sending a completion request. Recommended 150–300 ms to balance responsiveness and API usage.
- Throttle: `completion_throttle_ms` enforces a minimum spacing between completion requests, across both chat and provider-native paths. Set to 0 to disable. Recommended 300–600 ms if you still see excessive requests with just debounce.
- Manual invoke prefix: `manual_invoke_min_prefix` requires this many identifier characters before a manual completion (TriggerKind=1) proceeds without other triggers. Use 0 to always allow manual invoke.

Environment variables mirror these settings: `HEXAI_COMPLETION_DEBOUNCE_MS`, `HEXAI_COMPLETION_THROTTLE_MS`, `HEXAI_MANUAL_INVOKE_MIN_PREFIX`.

## Temperature behavior

- What it is: controls randomness/creativity of outputs.
- Default for coding: `0.2` for all providers unless overridden.
- Per-provider overrides: `openai_temperature`, `copilot_temperature`, `ollama_temperature`.

Recommended ranges:

- 0.0–0.3: deterministic and precise; best for refactors, tests, and bug fixes.
- 0.4–0.7: balanced; general Q&A and writing.
- 0.8–1.2+: creative; brainstorming; may increase tangents.

Guidance:

- Lower temperature increases consistency, but can be terse or repetitive.
- Higher temperature increases diversity, but can wander or introduce mistakes.
