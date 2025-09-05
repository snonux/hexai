# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

It has got improved capabilities for Go code understanding (for example, create unit tests from function), but other programming language work as well.
## Features

* LSP Code auto-completion
* LSP Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
* Support for OpenAI, GitHub Copilot, and Ollama

AI coded it under human orchestration and supervision following best practices with manual code reviews.

## Documentation

* [Configuration guide](docs/configuration.md)  
* [Usage examples](docs/usage-examples.md)
* [Source structure](docs/source-structure.md)

## Build and tasks

Hexai uses Mage for developer tasks. Install Mage, then run targets like build, dev, test, and install.

- Install Mage: `go install github.com/magefile/mage@latest`
- Build binaries: `mage build` (produces `hexai` and `hexai-lsp`)
- Dev build (+ tests, vet, lint): `mage dev`
- Run tests: `mage test`
- Install binaries to `GOPATH/bin`: `mage install`

Note: `mage lint` uses `golangci-lint`. Install via `mage devinstall` if needed.

## Install

Either use the Mage method as mentioned above, or install directly with:

- CLI: `go install codeberg.org/snonux/hexai/cmd/hexai@latest`
- LSP: `go install codeberg.org/snonux/hexai/cmd/hexai-lsp@latest`
