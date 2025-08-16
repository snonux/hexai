# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor.

At the moment this project is only in the brainstorming phase.

## LLM provider

Hexai exposes a simple LLM provider interface and uses OpenAI by default for
code completion when `OPENAI_API_KEY` is present in the environment.

- Required: set `OPENAI_API_KEY` to your OpenAI API key.
- Optional: set `OPENAI_MODEL` (default: `gpt-4o-mini`).
- Optional: set `OPENAI_BASE_URL` to point at a compatible endpoint.

If no key is configured, Hexai will fall back to a basic, local completion.

