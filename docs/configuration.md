# Hexai configuration

This document covers all configuration options for Hexai, including the config file,
environment overrides, provider selection, and temperature behavior.

## Config file

- Location: `$XDG_CONFIG_HOME/hexai/config.json` (usually `~/.config/hexai/config.json`).
- Example:

```json
{
  "max_tokens": 4000,
  "context_mode": "always-full",
  "context_window_lines": 120,
  "max_context_tokens": 4000,
  "log_preview_limit": 100,
  "no_disk_io": true,
  "trigger_characters": [".", ":", "/", "_", " " ],
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
- no_disk_io: avoid reading files from disk when building context.
- trigger_characters: LSP completion trigger characters.
- coding_temperature: optional override for LSP calls.
- provider: `openai` | `copilot` | `ollama`.

## Environment overrides

- All config-file options can be overridden by environment variables prefixed with `HEXAI_`.
- Env values take precedence over `config.json`.
- Examples:
  - `HEXAI_PROVIDER`, `HEXAI_MAX_TOKENS`, `HEXAI_CONTEXT_MODE`, `HEXAI_CONTEXT_WINDOW_LINES`, `HEXAI_MAX_CONTEXT_TOKENS`, `HEXAI_LOG_PREVIEW_LIMIT`
  - `HEXAI_CODING_TEMPERATURE`
  - `HEXAI_TRIGGER_CHARACTERS` (comma-separated, e.g., `".,:,_ , "`)
  - `HEXAI_OPENAI_MODEL`, `HEXAI_OPENAI_BASE_URL`, `HEXAI_OPENAI_TEMPERATURE`
  - `HEXAI_COPILOT_MODEL`, `HEXAI_COPILOT_BASE_URL`, `HEXAI_COPILOT_TEMPERATURE`
  - `HEXAI_OLLAMA_MODEL`, `HEXAI_OLLAMA_BASE_URL`, `HEXAI_OLLAMA_TEMPERATURE`

API keys:

- OpenAI: prefer `HEXAI_OPENAI_API_KEY`, falling back to `OPENAI_API_KEY`.
- Copilot: prefer `HEXAI_COPILOT_API_KEY`, falling back to `COPILOT_API_KEY`.

## Selecting a provider

- Set `provider` in the config to `openai`, `copilot`, or `ollama`.
- If omitted, Hexai defaults to `openai`.

## OpenAI configuration

- Required: `HEXAI_OPENAI_API_KEY` (or `OPENAI_API_KEY`).
- Options:
  - `openai_model` — model name (default: `gpt-4.1`).
  - `openai_base_url` — API base (default: `https://api.openai.com/v1`).
  - `openai_temperature` — default temperature (coding-friendly `0.2`).

## GitHub Copilot configuration

- Required: `COPILOT_API_KEY`.
- Options:
  - `copilot_model` — model name (default: `gpt-4o-mini`).
  - `copilot_base_url` — API base (default: `https://api.githubcopilot.com`).
  - `copilot_temperature` — default temperature (coding-friendly `0.2`).

## Ollama configuration

- Options:
  - `ollama_model` — model name/tag (default: `qwen3-coder:30b-a3b-q4_K_M`).
  - `ollama_base_url` — base URL (default: `http://localhost:11434`).
  - `ollama_temperature` — default temperature (coding-friendly `0.2`).

Notes:

- Ensure the model is available locally (e.g., `ollama pull qwen3-coder:30b-a3b-q4_K_M`).
- Alternatively, run Ollama in OpenAI‑compatible mode and use the OpenAI provider with
  `openai_base_url` pointed at your local endpoint.

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
