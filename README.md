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

- Flags (minimal):
  - `-version`: print the Hexai version and exit.
  - `-log`: path to log file (optional; default `/tmp/hexai.log`).

Notes:
- Token estimation for truncation uses a simple 4 chars/token heuristic.
- Full-file context is only included by default when defining a new function to balance quality, latency, and cost.

### Flags quick reference

| Flag                    | Env override               | Description                                        |
|-------------------------|----------------------------|----------------------------------------------------|
| `-log`      | —                 | Path to log file (optional).            |
| `-version`  | —                 | Print version and exit.                 |

Configuration is via JSON file and environment variables (env has precedence).

### JSON config file

- Location: `~/.config/hexai/config.json`
- Example:

```
{
  "max_tokens": 4000,
  "context_mode": "always-full", // minimal | window | file-on-new-func | always-full
  "context_window_lines": 120,
  "max_context_tokens": 4000,
  "log_preview_limit": 100,
  "no_disk_io": true,
  "provider": "ollama" // or "openai"
}
```

### Environment overrides (take precedence)

- `HEXAI_MAX_TOKENS`, `HEXAI_CONTEXT_MODE`, `HEXAI_CONTEXT_WINDOW_LINES`, `HEXAI_MAX_CONTEXT_TOKENS`
- `HEXAI_LOG_PREVIEW_LIMIT`, `HEXAI_NO_DISK_IO`
- `HEXAI_LLM_PROVIDER` (forces provider)

### Environment quick reference (providers)

- `HEXAI_LLM_PROVIDER`: `openai` | `ollama` (optional; otherwise auto‑detect).
- OpenAI: `OPENAI_API_KEY` (required), `OPENAI_MODEL`, `OPENAI_BASE_URL`.
- Ollama: `OLLAMA_MODEL`, `OLLAMA_BASE_URL` or `OLLAMA_HOST`.
