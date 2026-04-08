# Fish Completion

The `do` task-management CLI embeds its Fish completion script in the binary and prints it with `do fish`.

It completes the top-level `do` subcommands and the nested `do dep` operations.
It also completes task selectors for UUID-taking commands by reading pending tasks through `do complete-aliases`, which uses the local alias cache for stable short IDs.
The `do complete-uuids` command still emits both alias and UUID lines for scripts and tests that need the full selector list.
Selector suggestions stop once a command has consumed its selector argument, and `do dep add` / `do dep rm` suggest selectors for both task positions.
When typing `do add depends:...`, Fish also completes the comma-separated dependency selector list inside the `depends:` modifier.
The script preserves the global `--json` flag.

Load it into the current Fish session:

```sh
do fish | source
```

If you installed with `mage install` and `~/go/bin` is not on your `PATH` yet, use:

```sh
~/go/bin/do fish | source
```

To enable it automatically for new Fish sessions, add this to your Fish config or a file in `~/.config/fish/conf.d/`:

```fish
set -l do_bin ~/go/bin/do

if test -x $do_bin
    $do_bin fish | source
end
```

No external `do.fish` file is required.
