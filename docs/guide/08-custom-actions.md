# 8. Your own actions and prompts

[← Code actions](07-actions.md) · [Contents](index.md) · [Next: Tmux status →](09-tmux.md)

Save a prompt as an action when you use it repeatedly. The same custom
action appears in LSP code actions and in the tmux runner, so the instruction
can live in your configuration instead of in every selection.

![A custom two-item menu running a reusable compact-function action](images/08-custom-actions.gif)

## Add a reusable instruction

Add this to global `config.toml` or project `.hexaiconfig.toml`:

```toml
[[prompts.code_action.custom]]
id = "compact"
title = "Compact function"
kind = "refactor.rewrite"
scope = "selection"
hotkey = "e"
instruction = "Simplify the selection. Return only code."
```

`id` identifies the action; `title` is the menu label. The optional `kind`
sets the LSP action kind (default `refactor`). `scope` defaults to `selection`;
use `diagnostics` when the action needs diagnostics overlapping the selection.
The hotkey is optional and selects the action in the TUI submenu.

Reload the LSP config, select a function, and open Space-a to choose the
action. In the default tmux menu, press `a` for Custom actions, then `e`.
`[tmux] custom_menu_hotkey = "x"` changes the submenu key.

## Write the full prompt

Use `system` and `user` instead of `instruction` when the action needs a
specialized template:

```toml
[[prompts.code_action.custom]]
id = "fix-lints"
title = "Fix selected lints"
kind = "quickfix"
scope = "diagnostics"
hotkey = "l"
system = "You fix selected code precisely. Return only corrected code."
user = "Diagnostics:\n{{diagnostics}}\n\nSelection:\n{{selection}}"
```

`{{selection}}` is the selected text; `{{diagnostics}}` supplies diagnostics
for a diagnostics-scoped action. Use either `instruction` or `user`, not
both. If you supply `user` without `system`, Hexai uses its rewrite system
prompt. These templates use literal placeholder replacement, not arbitrary
Go template expressions.

IDs and nonempty custom hotkeys must be unique (case-insensitively). Actions
need an ID, a title, and either an instruction or a user template. Invalid
scope values and conflicting or multicharacter menu keys fail validation;
check the logs when an action does not appear.

## Replace the tmux menu

Once you define `[[tmux_action.menu]]`, those entries replace the default
main menu. For the two-item menu in the recording, add these blocks alongside
the `compact` action above:

```toml
[[tmux_action.menu]]
kind = "custom"
custom_id = "compact"
hotkey = "e"

[[tmux_action.menu]]
kind = "skip"
hotkey = "s"
```

`custom_id` must match a defined custom action. Built-in menu kinds are
`rewrite`, `simplify`, `document`, `gotest`, `fix_typos`, `custom_prompt`,
and `skip`. A `custom` entry places a reusable action directly in the main
menu. You can reorder entries, override `title` and `hotkey`, or omit actions
you do not use. Include Skip for a visible way to leave the selection alone.

## Change built-in prompts

Override only the prompts you want to change. Omitted keys keep their
built-in text:

```toml
[prompts.code_action]
document_system = "Add short Go documentation comments. Preserve behavior. Return code only."
document_user = "Document this selection:\n{{selection}}"

[prompts.chat]
system = "You are a concise coding assistant. Include a small example when useful."

[prompts.cli]
default_system = "Answer in at most three sentences."
explain_system = "Explain with a small example and its tradeoffs."
```

Action prompt prefixes are `rewrite`, `diagnostics`, `document`, `go_test`,
`simplify`, and `fix_typos`, each with `_system` and `_user` keys. Rewrite
also uses `{{instruction}}`; Go tests use `{{function}}`.

`[prompts.completion]` controls `system_general`, `system_params`,
`system_inline`, `user_general`, `user_params`, and `additional_context`.
Completion placeholders include `{{file}}`, `{{function}}`, `{{above}}`,
`{{current}}`, `{{below}}`, `{{char}}`, and `{{context}}`.
`[prompts.provider_native] completion` is the provider-native template,
using `{{path}}` and `{{before}}`.

See [config.toml.example](../../config.toml.example) for the full default
prompts before replacing them. Removing a prose or code-only instruction
changes what the model is asked to return, which can affect editor edits.
