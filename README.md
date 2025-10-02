# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

It has got improved capabilities for Go code understanding (for example, create unit tests from function), but other programming language work as well.

## Features

* LSP AI Code auto-completion
* LSP AI Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
* Parallel completions and CLI responses from multiple providers/models for side-by-side comparison
* TUI AI code-action runner (`hexai-tmux-action`) with Bubble Tea
  - Includes a “Custom prompt” action (hotkey `p`) that opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary Markdown file.
* Support for OpenAI, OpenRouter, GitHub Copilot, and Ollama

## Documentation

* [Build and install guide](docs/buildandinstall.md)
* [Configuration guide](docs/configuration.md)
* [Usage examples](docs/usage.md)
* [Helix + tmux quickstart](docs/tmux.md)

## Tmux Status Line

See the [tmux integration guide](docs/tmux.md) for details on configuring the status line.
