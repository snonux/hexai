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
* [Usage examples](docs/usage.md)
* [Helix + tmux quickstart](docs/helix-tmux-quickstart.md)

## Build and tasks

Hexai uses Mage for developer tasks. Install Mage, then run targets like build, dev, test, and install.

- Install Mage: `go install github.com/magefile/mage@latest`
- Build binaries: `mage build` (produces `hexai`, `hexai-lsp`, and `hexai-tmux-action`)
- Dev build (+ tests, vet, lint): `mage dev`
- Run tests: `mage test`
- Run tests with coverage: `go test ./... -cover`
- Full cross-package coverage and HTML report: `mage coverage` (writes `docs/coverage.html`)
- In restricted sandboxes/CI (no sockets), skip network-based tests:
  - `HEXAI_TEST_SKIP_NET=1 go test ./... -cover`
- Install binaries to `GOPATH/bin`: `mage install`

Note: `mage lint` uses `golangci-lint`. Install via `mage devinstall` if needed.

## Install

Either use the Mage method as mentioned above, or install directly with:

- CLI: `go install codeberg.org/snonux/hexai/cmd/hexai@latest`
- LSP: `go install codeberg.org/snonux/hexai/cmd/hexai-lsp@latest`
- Action runner: `go install codeberg.org/snonux/hexai/cmd/hexai-tmux-action@latest`

## Hexai CLI

- When invoked without arguments, `hexai` opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary `.md` file to capture a prompt, and combines it with any piped stdin.
- With arguments, behavior is unchanged (no editor spawn).

## Tmux Status Line

Hexai can surface live progress in tmux's status line via a user option. Add this to your `~/.tmux.conf`:

```
set -g status-right '#{@hexai_status} #[fg=colour8]| %H:%M'
```

- CLI updates `@hexai_status` at start (⏳ provider:model) and on completion with compact stats (↑sent, ↓recv, rpm, reqs).
- LSP emits an initial heartbeat on client initialize and periodic compact stats (provider, model, rpm, reqs, bytes).
- The TUI action runner sets a ready heartbeat and a completion heartbeat with stats.
- Toggle with `HEXAI_TMUX_STATUS=0` to disable (enabled by default).

The status segment supports simple theming:

- Preset themes:
  - `HEXAI_TMUX_STATUS_THEME=white-on-purple` (white fg on purple/magenta bg)
  - `HEXAI_TMUX_STATUS_THEME=black-on-yellow` (black fg on yellow bg)
- Explicit colors: set any tmux color names or 256-color codes
  - `HEXAI_TMUX_STATUS_FG=white`
  - `HEXAI_TMUX_STATUS_BG=magenta` (or `colour5`, etc.)
- If the segment is truncated, widen it: `set -g status-right-length 120`
