# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

## Features

* LSP Code auto-completion
* LSP Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
* Support for OpenAI, GitHub Copilot, and Ollama

AI coded it under human supervision, and a human developer reviewed the code.

## Documentation

* [Configuration guide](docs/configuration.md)  
* [Usage examples](docs/usage-examples.md)

## Build and tasks

Hexai uses Mage for developer tasks. Install Mage, then run targets like build, dev, test, and install.

- Install Mage: `go install github.com/magefile/mage@latest`
- Build binaries: `mage build` (produces `hexai` and `hexai-lsp`)
- Dev build (+ tests, vet, lint): `mage dev`
- Run tests: `mage test`
- Install binaries to `GOPATH/bin`: `mage install`

Note: `mage lint` uses `golangci-lint`. Install via `mage devinstall` if needed.
