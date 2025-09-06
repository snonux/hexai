# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI addition for your Helix Editor (https://helix-editor.com) .. Other editors should work but weren't tested.

It has got improved capabilities for Go code understanding (for example, create unit tests from function), but other programming language work as well.
## Features

* LSP Code auto-completion
* LSP Code actions
* LSP in-editor chat with the LLM
* Stand-alone command line tool for LLM interaction
* TUI code-action runner (`hexai-action`) with Bubble Tea
* Support for OpenAI, GitHub Copilot, and Ollama

## Documentation

* [Configuration guide](docs/configuration.md)  
* [Usage examples](docs/usage.md)
* [Source structure](docs/source-structure.md)

## Build and tasks

Hexai uses Mage for developer tasks. Install Mage, then run targets like build, dev, test, and install.

- Install Mage: `go install github.com/magefile/mage@latest`
- Build binaries: `mage build` (produces `hexai`, `hexai-lsp`, and `hexai-action`)
- Dev build (+ tests, vet, lint): `mage dev`
- Run tests: `mage test`
- Run tests with coverage: `go test ./... -cover`
- In restricted sandboxes/CI (no sockets), skip network-based tests:
  - `HEXAI_TEST_SKIP_NET=1 go test ./... -cover`
- Install binaries to `GOPATH/bin`: `mage install`

Note: `mage lint` uses `golangci-lint`. Install via `mage devinstall` if needed.

## Install

Either use the Mage method as mentioned above, or install directly with:

- CLI: `go install codeberg.org/snonux/hexai/cmd/hexai@latest`
- LSP: `go install codeberg.org/snonux/hexai/cmd/hexai-lsp@latest`

For `hexai-action`, use Mage or a local build:

- Build locally: `go build -o hexai-action cmd/internal/hexai-action/main.go`
- Or via Mage: `mage buildHexaiAction` (or `mage build`)
- Install: `mage install` (copies `hexai-action` to `GOPATH/bin` together with other binaries)

## Hexai Action (TUI)

`hexai-action` is a small TUI to run Hexai code actions from stdin. It loads the same `config.toml` as `hexai` and `hexai-lsp` (XDG path: `~/.config/hexai/config.toml`), and respects the same environment overrides.

- Pipe code (and optionally diagnostics) into the tool.
- Select an action with arrow keys, vi keys (`j/k`, `g/G`), Enter, or hotkeys `[s] [r] [d] [c] [t]`.
- The tool prints the transformed text to stdout.

Input formats

- Rewrite: include an inline instruction near the top of the selection using one of:
  - `;do something;`
  - `/* do something */`
  - `<!-- do something -->`
  - `// do something` (or `#`, `--`)

- Diagnostics (optional block):
  - Begin with a header line `Diagnostics:` (case-insensitive), one diagnostic per line, blank line, then the code selection.

Examples

- Rewrite selection:

```
;replace fmt.Println with log.Println;
package main

import "fmt"

func main() { fmt.Println("hi") }
```

- Diagnostics + selection:

```
Diagnostics:
missing return at end of function
use of undefined: foo

func f() int {
    foo()
}
```

Run:

- `cat input.go | ./hexai-action`
- or `./hexai-action < input.go`
- or with files: `./hexai-action --infile input.go --outfile output.go`

Flags

- `--infile`  Read input from the given file instead of stdin.
- `--outfile` Write output to the given file instead of stdout (truncates/creates).
