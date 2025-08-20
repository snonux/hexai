# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor and also a simple command line tool to interact with LLMs in general.

It has been coded with AI and human review.

Hexai exposes a simple LLM provider interface. It supports OpenAI, GitHub Copilot, and a local Ollama server. Provider selection and models are configured via a JSON configuration file (overridable via environment variables).

## Configuration

See the full configuration guide in `docs/configuration.md`.

## Usage

### Hexai LSP server

- Run over stdio: `hexai-lsp`
- Flags: `-version`, `-log`
  
More in `docs/usage-examples.md`.

### Configure in Helix

See `docs/usage-examples.md#configure-in-helix` for a sample `languages.toml` snippet.

## In-editor chat and inline features

- In-editor chat: ask inline by ending a line with `..`, `??`, `!!`, `::`, or `;;`. Hexai inserts
  a `>`-prefixed answer below. See `docs/usage-examples.md#in-editor-chat`.
- Inline triggers: strict `;text;` instructions for selection-based actions. See
  `docs/usage-examples.md#inline-triggers`.


## Code actions

Overview and details in `docs/usage-examples.md#code-actions`.

## Hexai CLI tool

See `docs/usage-examples.md#cli-usage` and `docs/usage-examples.md#examples` for examples.

<!-- In-editor chat example moved to docs/usage-examples.md#in-editor-chat -->
