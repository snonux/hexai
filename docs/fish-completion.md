# Fish Completion

The `ask` task-management CLI embeds its Fish completion script in the binary and prints it with `ask fish`.

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

If you installed with `mage install` and `~/go/bin` is not on your `PATH` yet, use:

```sh
~/go/bin/ask fish | source
```

To enable it automatically for new Fish sessions, add this to your Fish config or a file in `~/.config/fish/conf.d/`:

```fish
set -l ask_bin ~/go/bin/ask

if test -x $ask_bin
    $ask_bin fish | source
end
```

No external `ask.fish` file is required.

If you installed with `mage install`, the installer also writes an autoloadable completion file to `~/.config/fish/completions/ask.fish` (or `$XDG_CONFIG_HOME/fish/completions/ask.fish`), so new Fish sessions should pick it up automatically.
