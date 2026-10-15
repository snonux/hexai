# 11. Files and troubleshooting

[← Project tasks](10-tasks.md) · [Contents](index.md)

Start with the tool that fails: a plain CLI request isolates provider
configuration, Helix health checks isolate editor startup, and `ask info`
isolates a task's scope or selector.

![An invalid speed setting, its exit status, and a corrected simulation](images/11-troubleshooting.gif)

## File locations

Hexai uses these locations. Some fallbacks differ from conventional XDG
paths, so check the exact suffix when setting a base directory:

| Purpose | Default | With an XDG override |
|---------|---------|----------------------|
| Global configuration | `~/.config/hexai/config.toml` | `$XDG_CONFIG_HOME/hexai/config.toml` |
| Project configuration | `.hexaiconfig.toml` at git root | Same filename and location |
| Cache and usage | `~/.local/hexai/cache/` | `$XDG_CACHE_HOME/hexai/` |
| LSP log/state | `~/.local/hexai/state/` | `$XDG_STATE_HOME/state/` |
| Temporary editor/popup files | System temporary directory, usually `/tmp` | The OS temporary directory setting |

The cache contains `stats.json`, `stats.lock`, CLI response entries, and
`ask`'s task alias cache. Cached usage can be regenerated; deleting the alias
cache can change the aliases you have been using. Task records themselves
live in **Taskwarrior's** data directory, not Hexai's cache. Keep Taskwarrior
backups separately from cached data.

The maintained tools do not require a prompt database. The experimental MCP
component's data files are described in its archival docs.

## A provider does not answer

1. Run `hexai 'What is a slice in Go?'` from the shell used to start Helix.
2. Check the global provider, its endpoint and model, any project overrides,
   and environment overrides. An explicit `--config` still allows project
   and environment overrides.
3. Check that the matching provider key is available to the process. tmux and
   an already-running editor can have an older environment than your shell.
4. For local Ollama, check that the server is reachable and the exact model
   tag is installed. Set the completion model as well as the general model.
5. For a timeout, reduce context size or set `[general] request_timeout`
   (seconds). Verify authentication errors directly; they do not trigger fallback.

Do not print credentials into shared logs or recordings. The recording runner
uses a local fixture key with no external provider access.

## Helix shows no completion or reply

Run `hx --health go` and check that `hexai-lsp-server` is found. Confirm the
file's language entry includes `hexai`. Start a new language server after
changing Helix's server definition. For a manual completion, use insert-mode
Ctrl-x, then check your minimum prefix, ignored-file patterns, and whether
`/disable>` was used.

For chat, include the closing `>` after a valid prefix, for example `?>`.
For inline prompts, use the strict `>!instruction>` form and request a
completion. If a line already has a quoted reply, Hexai avoids answering
again. After a failure, edit the prompt line to retry.

`/reload>` reloads disk settings and ignores environment overrides for that
reload. Restart the server to restore environment-derived settings.

## Find the LSP log

Normally the log is `~/.local/hexai/state/hexai-lsp-server.log`. You can
choose an explicit path in Helix's language-server args:

```toml
[language-server.hexai]
command = "hexai-lsp-server"
args = ["--log", "/tmp/hexai-lsp-server.log"]
```

Inspect it for startup, config parsing/validation, missing provider keys,
request failures, or ignored paths. `[logging] log_preview_limit` controls
preview length; the environment equivalent is `HEXAI_LOG_PREVIEW_LIMIT`.
Logs can contain prompt or code previews, so inspect them before sharing.

## The action popup fails

Run Helix inside tmux and confirm `tmux` is available. First try
`cat notes.txt | hexai-tmux-action` directly in a tmux shell. Use
`--tmux-popup-width` and `--tmux-popup-height` to give the menu enough room.
A custom prompt needs a working `HEXAI_EDITOR` or `EDITOR`.

For a missing action, check its ID, instruction/user template, scope, and
hotkeys. A configured `[[tmux_action.menu]]` replaces the built-in menu;
include the actions you want. Child file flags and alternate popup config
behavior are explained in [chapter 7](07-actions.md#files-diagnostics-and-popup-options).

## ask cannot find a task

Check the repository and current subdirectory. A task in a parent or sibling
project is outside a subdirectory's scope. Try the root directory, an explicit
`proj:<name>`, or `ask na list` for tasks without the agent tag. Widen the due
date window when a future task is hidden. Copy a current alias from the
listing instead of using a Taskwarrior numeric ID. `ask info` without a selector
needs an unambiguous started task; supply its alias if several are started.

## Development checks

From a checkout:

```sh
mage test       # full tests with the race detector
mage coverage   # cross-package coverage and docs/coverage.html
mage fmt        # gofumpt formatting
mage vet
mage lint       # needs golangci-lint
mage dev        # tests, vet, lint, and race builds
```

`mage devinstall` installs developer tooling. For environments that forbid
network listeners, `HEXAI_TEST_SKIP_NET=1 go test ./... -cover` skips tests
that honor that switch; it is a reduced check, not the full suite.

Tests can pick up your installed Hexai configuration and alias cache. For
an independent check, run with temporary `XDG_CONFIG_HOME` and
`XDG_CACHE_HOME` directories, clearing inherited `HEXAI_*` overrides as
needed. `mage coverage` expects at least 80% total coverage and writes
ignored generated reports under `docs/`. No generated coverage files need
to be committed.

To update the animations, follow the [recording instructions](recordings/readme.md).
