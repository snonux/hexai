# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor.

At the moment this project is only in the proof of PoC phase.

## LLM provider

Hexai exposes a simple LLM provider interface. It supports OpenAI and a local
Ollama server. Provider selection and models are configured via a JSON
configuration file.

### Selecting a provider

- Set `provider` in the config file to `openai` or `ollama`.
- If omitted, Hexai defaults to `openai`.

### OpenAI configuration

- Required: `OPENAI_API_KEY` — provided via environment variable only.
- In config file:
  - `openai_model` — model name (default: `gpt-4o-mini`).
  - `openai_base_url` — API base (default: `https://api.openai.com/v1`).

### Ollama configuration (local)

- In config file:
  - `ollama_model` — model name/tag (default: `qwen2.5-coder:latest`).
  - `ollama_base_url` — base URL to Ollama (default: `http://localhost:11434`).

Notes:
- For Ollama, ensure the model is available locally (e.g., `ollama pull qwen2.5-coder:latest`).
- If you run Ollama in OpenAI‑compatible mode, you may alternatively use the
  OpenAI provider with `openai_base_url` in the config pointing to your local endpoint.

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

| Flag       | Description                          |
|------------|--------------------------------------|
| `-log`     | Path to log file (optional).         |
| `-version` | Print version and exit.              |

Configuration is via a JSON file only. Environment variables are not used
except for `OPENAI_API_KEY`.

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
  "provider": "ollama", // or "openai"
  // OpenAI-only options
  "openai_model": "gpt-4.1",
  "openai_base_url": "https://api.openai.com/v1",
  // Ollama-only options
  "ollama_model": "qwen2.5-coder:latest",
  "ollama_base_url": "http://localhost:11434"
}
```

Minimal config (defaults to OpenAI):

```
{}
```

Ensure `OPENAI_API_KEY` is set in your environment.

### Environment

- Only `OPENAI_API_KEY` is read from the environment when `provider` is `openai`.
