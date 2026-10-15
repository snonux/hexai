# 6. Chat and inline prompts

[← Completions in Helix](05-helix.md) · [Contents](index.md) · [Next: Code actions →](07-actions.md)

Chat answers a question below the line you typed. Inline prompts replace a
marker with generated code. Both use the LSP setup from chapter 5.

![Asking about slices, then replacing an inline instruction](images/06-chat-inline.gif)

## Ask inside a file

Open a Markdown file in Helix and type:

```text
What is a slice in Go?>
```

The suffix `?>` triggers chat. The other built-in endings are `!>`, `:>`,
and `;>`. Once answered, Hexai removes just the final `>` from your question
and inserts the reply below it, with a blank line on either side:

```text
What is a slice in Go?

> A slice is a view of an array, with a length and capacity.

```

Every answer line starts with `>`, including multiline replies. Save the file
if you want to keep the exchange. The recording uses Markdown, but questions
can also appear in code comments:

```go
// Why does appending to this slice allocate?>
```

The answer is inserted as quoted text; in a source file you may need to
comment or remove it before compiling.

## Ask a follow-up

Below the answer, type `What is capacity?>`. Hexai includes up to the three
most recent question/answer pairs above the question, so a follow-up can
refer to the preceding exchange.

A question is sent once while its request is in flight. You can keep typing;
Hexai follows the question if earlier lines move. Editing the question itself
causes a stale answer to be dropped and the edited question to be handled
instead. Identical questions on separate lines are answered one after another.
If an answer is already present below a question, Hexai avoids answering it again.

When a request fails, Helix shows a Hexai warning. Edit the question line,
for example by removing and retyping the final `>`, to retry.

## Replace an inline instruction

Inside Go's `Add`, type this and request a completion if your editor has not
already requested one:

```go
return >!return the sum of a and b>
```

The prompt tag is replaced with the model's result:

```go
return a + b
```

Use `>>!instruction>` to replace the **entire current line** instead:

```text
>>!write a Go return statement that adds a and b>
```

The inline forms are strict: no space after `>!` and no space before the
closing `>`. Spaces inside the instruction are fine. A spaced form such as
`> instruction >` is ignored. Inline results are applied only while the tag
still matches; editing or deleting the tag drops the stale result.

## Slash commands

These commands run locally through the same chat mechanism:

| Type on a line | Effect |
|----------------|--------|
| `/help>` | Show available commands |
| `/reload>` | Reload configuration from disk |
| `/disable>` | Disable automatic completions for this LSP session |
| `/enable>` | Enable completions again |

The help replies also show forms such as `/reload?>`; both forms work.
Only a slash followed by a letter starts a command, so a `// question?>`
comment is treated as a question.

Reload reuses the selected config path and project config, but **ignores
environment overrides** during that reload. Restart the language server to
load environment overrides again. Disabling completions leaves chat and code
actions usable.

## Change the markers

```toml
[inline]
inline_open = ">!"
inline_close = ">"

[chat]
chat_suffix = ">"
chat_prefixes = ["?", "!", ":", ";"]
```

The closing marker, chat suffix, and individual chat prefixes are single
characters. Environment equivalents are `HEXAI_INLINE_OPEN`,
`HEXAI_INLINE_CLOSE`, `HEXAI_CHAT_SUFFIX`, and `HEXAI_CHAT_PREFIXES`
(comma-separated). Keep the defaults while learning the workflow.
