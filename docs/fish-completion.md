# Fish Completion

The `ask` task-management CLI embeds its Fish completion script in the binary and prints it with `ask fish`.

It completes the top-level `ask` subcommands and the nested `ask dep` operations.
It also completes task selectors for UUID-taking commands by reading pending tasks through `ask complete-uuids`, which uses the local alias cache for stable short IDs.
Fish suggests each task's alias ID first and also keeps the raw UUID available as a fallback selector.
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
