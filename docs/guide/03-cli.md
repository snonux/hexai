# 3. The command line

[← Configuration](02-configuration.md) · [Contents](index.md) · [Next: Models and providers →](04-models.md)

Use `hexai` whenever text is already in your shell: a question, a file,
or output from another command. It writes answers to stdout so you can pipe
or save them, while provider labels and request summaries go to stderr.

![Piping a function, saving an answer, and previewing streaming](images/03-cli.gif)

## Ask a question

With your provider configured:

```sh
hexai 'What is a slice in Go?'
hexai 'show a command to find Go files'
hexai 'explain when a slice allocates a new backing array'
```

The default system prompt asks for concise replies. A request for commands
asks the model to return commands only. Including `explain` switches to a
more detailed system prompt. These are model instructions, so review the
answer before using it.

Put flags before the prompt. `hexai --help` lists the numbered provider flags
for your current configuration, alongside `--config`, `--version`, and
`--tps-simulation`.

## Pipe a file or another command

```sh
cat selection.go | hexai
cat selection.go | hexai 'explain this function'
git diff | hexai 'summarize the changes in three bullets'
hexai 'show a command to find Go files' > command.txt
cat command.txt
```

When you provide stdin and arguments together, Hexai constructs the request
as `argument text`, a colon and blank line, then the stdin text. With no
arguments, piped stdin becomes the prompt by itself.

Only the text you pass is CLI input; `hexai` does not automatically attach
your repository. Editor context settings apply to the LSP workflow in
[chapter 5](05-helix.md).

## Write the prompt in your editor

Run `hexai` from a terminal with no arguments or piped input. It opens a
temporary Markdown file in `HEXAI_EDITOR`, falling back to `EDITOR`.
Write a prompt, save it, and close the editor to send it.

```sh
export HEXAI_EDITOR=hx
hexai
```

If no usable text comes back, the command reports an input error. Temporary
editor files use the system temporary directory and are cleaned up after use.
`hexai config` opens configuration instead of an LLM prompt.

## Reuse an answer

Successful answers are cached for 24 hours in Hexai's cache directory.
The key includes the input, provider, resolved model, and effective CLI
system prompt. Changing any of those yields a different entry.

```sh
hexai 'What does Add do?'
hexai 'What does Add do?'
```

The second run can reuse the first answer. For a comparison with several
models, select a single numbered entry on the rerun to get one clean answer
on stdout. [Chapter 4](04-models.md#compare-several-models) demonstrates this.
Provider labels and summaries stay on stderr even when a reply is cached.

## Preview output speed offline

TPS simulation streams supplied text without contacting an LLM. This is
useful for getting a feel for a model's response rate before running it.

```sh
hexai --tps-simulation 20
hexai --tps-simulation 12-18 'A short answer at a variable rate.'
cat selection.go | hexai --tps-simulation 20
```

Use a positive fixed rate, or a positive ordered range such as `12-18`.
Without supplied text, Hexai streams built-in placeholder text. It is a
speed preview, not a measurement of a provider's actual token throughput.

## Exit status

| Status | Meaning |
|--------|---------|
| `0` | Successful response, simulation, config editing, or version output |
| `1` | Runtime error, including missing input, provider failure, or invalid simulation rate |
| `2` | Flag parsing error, including an unknown flag (`--help` also currently returns `2`) |

If a shell pipeline needs to report errors from any stage, enable
`set -o pipefail`. [Troubleshooting](11-troubleshooting.md) covers missing
keys, unreachable endpoints, and finding the cache.
