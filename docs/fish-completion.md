# Fish Completion

Hexai ships a Fish completion file for the `ask` task-management CLI at [`assets/ask.fish`](../assets/ask.fish).

It completes the top-level `ask` subcommands and the nested `ask dep` operations.
It also completes task UUIDs for UUID-taking commands by reading the current project from `ask all --json` and filtering out completed and deleted tasks.
The script preserves the global `--json` flag.

Install it into Fish's completion directory:

```sh
install -Dm644 assets/ask.fish ~/.config/fish/completions/ask.fish
```

If you use a custom XDG config directory, copy it to:

```sh
$XDG_CONFIG_HOME/fish/completions/ask.fish
```

The completion file is static and ships with the repository, so it does not require a build step.
