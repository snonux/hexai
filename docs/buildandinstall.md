## Build and tasks

Hexai uses Mage for developer tasks. Install Mage, then run targets like build, dev, test, and install.

- Install Mage: `go install github.com/magefile/mage@latest`
- Build binaries: `mage build` (produces `ask`, `hexai`, `hexai-lsp-server`, `hexai-tmux-action`, and `hexai-tmux-edit`)
- Dev build (+ tests, vet, lint): `mage dev`
- Run tests: `mage test`
- Run tests with coverage: `go test ./... -cover`
- Full cross-package coverage and HTML report: `mage coverage` (writes `docs/coverage.html`)
- In restricted sandboxes/CI (no sockets), skip network-based tests:
  - `HEXAI_TEST_SKIP_NET=1 go test ./... -cover`
- Install binaries to `GOPATH/bin`: `mage install`
- Load Fish completions in the current shell after install: `~/go/bin/ask fish | source` (or add a `conf.d` snippet; see `docs/fish-completion.md`)

Note: `mage lint` uses `golangci-lint`. Install via `mage devinstall` if needed.

## Install

Either use the Mage method as mentioned above, or install directly with:

- Task CLI (`ask`, agent-scoped Taskwarrior wrapper): `go install codeberg.org/snonux/hexai/cmd/ask@latest`
- CLI: `go install codeberg.org/snonux/hexai/cmd/hexai@latest`
- LSP: `go install codeberg.org/snonux/hexai/cmd/hexai-lsp-server@latest`
- Action runner: `go install codeberg.org/snonux/hexai/cmd/hexai-tmux-action@latest`
- Tmux popup editor: `go install codeberg.org/snonux/hexai/cmd/hexai-tmux-edit@latest`
