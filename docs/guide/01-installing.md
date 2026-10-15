# 1. Installing

[Contents](index.md) · [Next: Your first configuration →](02-configuration.md)

Hexai ships four maintained programs. Install only the ones you need:

| Program | What it does | Extra runtime tools |
|---------|--------------|---------------------|
| `hexai` | Send prompts and piped text to an LLM | A provider; an editor for interactive input |
| `hexai-lsp-server` | Completions, chat, and code actions over stdio LSP | Helix or another LSP client; a provider |
| `hexai-tmux-action` | Apply actions to a piped selection through a popup | tmux; a provider for actions that call an LLM |
| `ask` | Manage tasks in the current git project | git and Taskwarrior (`task` on your PATH) |

![Inspecting the installed version and CLI flags](images/01-installing.gif)

## Install with Go

Use Go 1.26 or newer, matching this repository's `go.mod`.

```sh
go install github.com/snonux/hexai/cmd/hexai@latest
go install github.com/snonux/hexai/cmd/hexai-lsp-server@latest
go install github.com/snonux/hexai/cmd/hexai-tmux-action@latest
go install github.com/snonux/hexai/cmd/ask@latest
```

`go install` writes to `$GOBIN` when set, otherwise the `bin` directory in your
GOPATH (normally `~/go/bin`). Add that directory to PATH, including the PATH
seen by Helix and tmux. For a default Bash setup:

```sh
export PATH="$HOME/go/bin:$PATH"
hexai --version
hexai-lsp-server --version
hexai --help
ask help
```

Install Helix and tmux through your operating system's package manager if you
want the editor workflow. Install Taskwarrior for `ask`. Fish is optional and
only needed for [Fish completion](10-tasks.md#fish-completion).

## Build a checkout

```sh
git clone https://github.com/snonux/hexai.git
cd hexai
go install github.com/magefile/mage@latest
mage build
mage install
```

`mage build` creates `ask`, `hexai`, `hexai-lsp-server`, and
`hexai-tmux-action` at the repository root. `mage install` builds and copies
them to `$GOPATH/bin`, defaulting to `~/go/bin` when GOPATH is unset.
It does not write Fish completion files. To keep the build in your checkout,
call a binary with `./hexai` instead.

You can build the programs directly with Go, for example
`go build -o hexai ./cmd/hexai`. The [last chapter](11-troubleshooting.md#development-checks)
covers the test, formatting, and coverage commands.

## Make a place to experiment

```sh
mkdir hexai-demo
cd hexai-demo
git init
printf 'func Add(a, b int) int { return a + b }\n' > selection.go
```

This repository name will also become the default `ask` project name.
Next, [choose a provider](02-configuration.md) before sending your first prompt.
