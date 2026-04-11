# Fish Completion

The `ask` task-management CLI embeds its Fish completion script in the binary and prints it with `ask fish`.

`mage install` and `go install …/cmd/ask` only place the `ask` binary on your `PATH` (under `GOPATH/bin` or `~/go/bin`). They do **not** install anything under `fish/completions/`. You load completions yourself as below.

If you used an older Hexai release whose `mage install` wrote `ask.fish` into your Fish completions directory, remove that file so Fish does not keep sourcing a stale script:  
`~/.config/fish/completions/ask.fish` or `$XDG_CONFIG_HOME/fish/completions/ask.fish`.

It completes the top-level `ask` subcommands and the nested `ask dep` operations.
It also suggests the global task prefixes `na`, `no-agent`, and `proj:`.
It also completes task selectors for UUID-taking commands by reading pending tasks through `ask complete-aliases`, which uses the local alias cache for stable short IDs.
The `ask complete-uuids` command still emits both alias and UUID lines for scripts and tests that need the full selector list.
Selector suggestions stop once a command has consumed its selector argument, and `ask dep add` / `ask dep rm` suggest selectors for both task positions.
When typing `ask add depends:...`, Fish also completes the comma-separated dependency selector list inside the `depends:` modifier.
The script preserves the global `--json` flag.

Load it into the current Fish session:

```sh
ask fish | source
```

If `ask` is not on your `PATH` yet, call it by full path (for example `~/go/bin/ask` when `GOPATH` is unset):

```sh
~/go/bin/ask fish | source
```

To enable it automatically for new Fish sessions, add this to your Fish config or a file in `$XDG_CONFIG_HOME/fish/conf.d/` (often `~/.config/fish/conf.d/`):

```fish
set -l ask_bin ~/go/bin/ask

if test -x $ask_bin
    $ask_bin fish | source
end
```

Do not rely on a static file under `fish/completions/`; either run `ask fish | source` when you want completions in the current session, or keep the `conf.d` snippet so each new session sources the script from the installed binary (stays in sync when you upgrade `ask`).
