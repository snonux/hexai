# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor.

At the moment this project is only in the proof of concept phase.

## LLM provider

Hexai exposes a simple LLM provider interface. It supports OpenAI and a local
Ollama server. Provider selection and models are configured via environment
variables.

### Selecting a provider

- Set `HEXAI_LLM_PROVIDER` to `openai` or `ollama` to force a provider.
- If not set, Hexai auto‑detects:
  - Uses OpenAI when `OPENAI_API_KEY` is present.
  - Uses Ollama when any `OLLAMA_*` variables are present.
  - Otherwise, Hexai falls back to a basic, local completion.

### OpenAI configuration

- Required: `OPENAI_API_KEY` — your OpenAI API key.
- Optional: `OPENAI_MODEL` — model name (default: `gpt-4o-mini`).
- Optional: `OPENAI_BASE_URL` — override the API base (e.g., a compatible endpoint).

### Ollama configuration (local)

- Optional: `OLLAMA_MODEL` — model name/tag (default: `qwen2.5-coder:latest`).
- Optional: `OLLAMA_BASE_URL` or `OLLAMA_HOST` — base URL to Ollama
  (default: `http://localhost:11434`).

Notes:
- For Ollama, ensure the model is available locally (e.g., `ollama pull qwen2.5-coder:latest`).
- If you run Ollama in OpenAI‑compatible mode, you may alternatively use the
  OpenAI provider with `OPENAI_BASE_URL` pointing to your local endpoint.

## CLI usage and configuration

- Run LSP server over stdio:
  - `hexai`

- Completion settings:
  - `-max-tokens`: maximum tokens for LLM completions. If the flag isn’t provided, `HEXAI_MAX_TOKENS` is used when set.
  - `-context-mode`: how much additional context to include with completion prompts (If the flag isn’t provided, `HEXAI_CONTEXT_MODE` is used when set). One of:
    - `minimal`: no extra context
    - `window`: include a sliding window around the cursor
    - `file-on-new-func`ude the full file only when defining a new function (cursor before the opening `{`)
    - `always-full`: always include the full file (may be slower/costly)
  - `-context-window-lines`: line count for the sliding window when `context-mode=window`.
  - `-max-context-tokens`: budget for additional context tokens. If the flag isn’t provided, `HEXAI_MAX_CONTEXT_TOKENS` is used when set.
  - `-provider`: LLM provider override: `openai` or `ollama` (overrides `HEXAI_LLM_PROVIDER`).

Notes:
- Token estimation for truncation uses a simple 4 chars/token heuristic.
- Full-file context is only included by default when defining a new function to balance quality, latency, and cost.

### Flags quick reference

| Flag                    | Env override               | Description                                        |
|-------------------------|----------------------------|----------------------------------------------------|
| `-stdio`                | —                          | Run as LSP over stdio (only supported mode).       |
| `-log`                  | —                          | Path to log file (optional).                       |
| `-max-tokens`           | `HEXAI_MAX_TOKENS`         | Max tokens for LLM completions.                    |
| `-context-mode`         | `HEXAI_CONTEXT_MODE`       | `minimal` `window` `file-on-new-func` `always-full` |
| `-context-window-lines` | `HEXAI_CONTEXT_WINDOW_LINES` | Lines around cursor when using `window` mode.      |
| `-max-context-tokens`   | `HEXAI_MAX_CONTEXT_TOKENS` | Token budget for additional context.               |
| `-log-preview-limit`    | `HEXAI_LOG_PREVIEW_LIMIT`  | Limit characters shown in LLM preview logs.        |
| `-no-disk-io`           | `HEXAI_NO_DISK_IO`         | Disallow any disk reads for context.               |
| `-provider`             | `HEXAI_LLM_PROVIDER`       | Force LLM provider: `openai` or `ollama`.          |

### Environment quick reference (providers)

- `HEXAI_LLM_PROVIDER`: `openai` | `ollama` (optional; otherwise auto‑detect).
- OpenAI: `OPENAI_API_KEY` (required), `OPENAI_MODEL`, `OPENAI_BASE_URL`.
- Ollama: `OLLAMA_MODEL`, `OLLAMA_BASE_URL` or `OLLAMA_HOST`.
