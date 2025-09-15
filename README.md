# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

It has got improved capabilities for Go code understanding (for example, create unit tests from function), but other programming language work as well.

## Features

* LSP Code auto-completion
* LSP Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
* TUI code-action runner (`hexai-tmux-action`) with Bubble Tea
  - Includes a “Custom prompt” action (hotkey `p`) that opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary Markdown file.
* Support for OpenAI, GitHub Copilot, and Ollama

## Documentation

* [Configuration guide](docs/configuration.md)
* [Build and install guide](docs/buildandinstall.md)
* [Usage examples](docs/usage.md)
* [Helix + tmux quickstart](docs/tmux.md)

## Hexai CLI

- When invoked without arguments, `hexai` opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary `.md` file to capture a prompt, and combines it with any piped stdin.
- With arguments, behavior is unchanged (no editor spawn).

## Tmux Status Line

See the [tmux integration guide](docs/tmux.md) for details on configuring the status line.
