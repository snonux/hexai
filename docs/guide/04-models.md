# 4. Models and providers

[← The command line](03-cli.md) · [Contents](index.md) · [Next: Completions in Helix →](05-helix.md)

A quick completion and a long explanation need different things from a
model. Hexai lets you configure each surface independently: `completion`,
`code_action`, `chat`, and `cli`.

![Comparing fixture models, then selecting one cached answer](images/04-models.gif)

The animation uses two local fixture models. In the examples below, replace
model names with models available through your providers.

## Choose a model per surface

Add entries to the configuration from chapter 2:

```toml
[models.completion]
provider = "openai"
model = "gpt-4.1-mini"
temperature = 0.0

[models.code_action]
provider = "openai"
model = "gpt-4.1"
temperature = 0.2

[models.chat]
provider = "ollama"
model = "minimax-m3:cloud"

[models.cli]
provider = "ollama"
model = "minimax-m3:cloud"
```

The provider's own section supplies its base URL and default settings.
A shorthand such as `[models]` with `cli = "minimax-m3:cloud"` selects only
the model and uses the general provider. Use the table form when choosing
a different provider or temperature. Do not define both forms for the same
surface in one TOML file.

Environment equivalents use `HEXAI_MODEL_COMPLETION`, `HEXAI_MODEL_CODE_ACTION`,
`HEXAI_MODEL_CHAT`, and `HEXAI_MODEL_CLI`. Replace `MODEL` with `PROVIDER` or
`TEMPERATURE` to override those fields instead. For example:

```sh
HEXAI_PROVIDER_CLI=openai HEXAI_MODEL_CLI=gpt-4.1 hexai 'Explain slices'
```

## Compare several models

Array tables configure several entries for the same surface. Replace an
existing single `[models.cli]` table with these blocks:

```toml
[[models.cli]]
provider = "openai"
model = "gpt-4.1-mini"

[[models.cli]]
provider = "ollama"
model = "minimax-m3:cloud"
```

```sh
hexai 'What does Add do?'
hexai -0 'What does Add do?'
hexai -1 'What does Add do?'
```

The first command calls every CLI entry concurrently and displays the replies
side by side. Flags `-0`, `-1`, and so on select entries by their zero-based
configuration order. Combine flags to choose a subset. Selecting one entry
prints a single response, suitable for a pipe; an identical rerun can use its
cached answer.

`[[models.completion]]` works similarly for LSP suggestions, labeling results
with `provider:model`. `[completion] completion_wait_all = true` waits for
all completion backends; `false` returns the first result.
Code actions use the **first entry only**, logging a warning about extras.
Treat chat as a single configured entry as well; comparison is supported for
the CLI and completions.

## Name endpoints

Profiles let you configure several endpoints of the same provider type:

```toml
[providers.ollama-cloud]
type = "ollama"
base_url = "https://ollama.com"
model = "minimax-m3:cloud"

[providers.ollama-local]
type = "ollama"
base_url = "http://127.0.0.1:11434"
model = "your-local-model"

[[models.cli]]
provider = "ollama-cloud"
fallback_provider = "ollama-local"
fallback_model = "your-local-model"
```

Copy all three blocks together, replacing any earlier CLI entry for this
example. Substitute an installed local model tag in both local-model fields.
The `type` selects the provider implementation; the profile name is how a
surface refers to the endpoint. Credentials use the normal variables for
that provider type. Local Ollama needs no key.

## Recover from a provider outage

`fallback_provider` supplies a second target for one surface entry.
`fallback_model` is optional; when omitted, the fallback profile's model is
used. The fallback runs **after** an eligible primary failure, such as quota
exhaustion, rate limiting, a server error, or a transport failure.

Fallback is sequential and produces one result for that entry. It does not
add another comparison column. Invalid input, authentication failures, and
user cancellation do not trigger fallback. Once streaming has emitted output,
Hexai avoids joining a second model's output onto the partial first answer.
Successful usage is recorded under the provider and model that answered.

![The primary fixture endpoint returns 503 and the backup answers](images/04-fallback.gif)

To try recovery yourself, configure an unreachable primary local endpoint
and a working fallback, then send a new uncached CLI prompt. A cached success
will bypass the provider call. The complete cloud/local example is also in
[config.toml.example](../../config.toml.example).
