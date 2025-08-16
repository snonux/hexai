# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor.

At the moment this project is only in the proof of concept phase.

## LLM provider

Hexai exposes a simple LLM provider interface and uses OpenAI by default for
code completion when `OPENAI_API_KEY` is present in the environment.

- Required: set `OPENAI_API_KEY` to your OpenAI API key.
- Optional: set `OPENAI_MODEL` (default: `gpt-4o-mini`).
- Optional: set `OPENAI_BASE_URL` to point at a compatible endpoint.

If no key is configured, Hexai will fall back to a basic, local completion.

## CLI usage and configuration

- Run LSP server over stdio:
  - `hexai -stdio`

- Completion settings:
  - `-max-tokens`: maximum tokens for LLM completions (default `500`). If the flag isn’t provided, `HEXAI_MAX_TOKENS` is used when set.
  - `-context-mode`: how much additional context to include with completion prompts. One of:
    - `minimal`: no extra context
    - `window`: include a sliding window around the cursor
    - `file-on-new-func` (default): include the full file only when defining a new function (cursor before the opening `{`)
    - `always-full`: always include the full file (may be slower/costly)
  - `-context-window-lines`: line count for the sliding window when `context-mode=window` (default `120`).
  - `-max-context-tokens`: budget for additional context tokens (default `2000`). If the flag isn’t provided, `HEXAI_MAX_CONTEXT_TOKENS` is used when set.

Notes:
- Token estimation for truncation uses a simple 4 chars/token heuristic.
- Full-file context is only included by default when defining a new function to balance quality, latency, and cost.

### Flags quick reference

| Flag                    | Default            | Env override               | Description                                        |
|-------------------------|--------------------|----------------------------|----------------------------------------------------|
| `-stdio`                | `true`             | —                          | Run as LSP over stdio (only supported mode).       |
| `-log`                  | `/tmp/hexai.log`   | —                          | Path to log file (optional).                       |
| `-max-tokens`           | `500`              | `HEXAI_MAX_TOKENS`         | Max tokens for LLM completions.                    |
| `-context-mode`         | `file-on-new-func` | —                          | `minimal` `window` `file-on-new-func` `always-full` |
| `-context-window-lines` | `120`              | —                          | Lines around cursor when using `window` mode.      |
| `-max-context-tokens`   | `2000`             | `HEXAI_MAX_CONTEXT_TOKENS` | Token budget for additional context.               |
