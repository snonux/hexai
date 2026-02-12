# Custom Code Actions Design

This document proposes how Hexai can support user-defined code actions via the config file, and surface them in both hexai-lsp-server and hexai-tmux-action.

## Goals

- Users define additional code-action prompts in `config.toml` as an array of tables.
- These actions appear alongside built-ins in hexai-lsp’s Code Actions.
- The same actions are available in hexai-tmux-action via a dedicated “Custom actions…” submenu.
- Provide a configurable hotkey to open the custom actions submenu in tmux.
- Validate and fail fast on duplicates with clear error messages.

## Configuration Schema (TOML)

```
[prompts.code_action]
# existing prompt overrides (rewrite_*, diagnostics_*, document_*, go_test_*, simplify_*)

[[prompts.code_action.custom]]
id = "extract-function"                # required, unique slug (case-insensitive)
title = "Extract function"             # required, shown in LSP + tmux
kind = "refactor.extract"              # optional, LSP CodeAction.kind (default: "refactor")
scope = "selection"                    # optional: selection | diagnostics (default: selection)
hotkey = "e"                           # optional, single character for tmux submenu
# Option A: fixed instruction using the global rewrite templates
instruction = "Extract the selected code into a new function named 'extracted' and replace with a call. Return only code, no backticks."

[[prompts.code_action.custom]]
id = "fix-lints"
title = "Fix linters"
kind = "quickfix"
scope = "diagnostics"
hotkey = "l"
# Option B: fully custom system+user templates for this action
system = "You are a precise code fixer. Only change selected code."
user = "Diagnostics to resolve (selection only):\n{{diagnostics}}\n\nSelected code:\n{{selection}}"

[tmux]
# Hotkey to open the custom actions submenu in hexai-tmux-action
custom_menu_hotkey = "a"               # optional, single character; must not clash with built-ins
```

Notes:
- Available template variables: `{{selection}}` always; `{{diagnostics}}` when scope=diagnostics; LSP can also provide `{{uri}}` and `{{file_name}}` later if needed.
- If `user` is set, Hexai uses that and optional `system`. If `instruction` is set (and `user` is not), Hexai uses the global rewrite `system`/`user` templates with the fixed instruction.

## App Model Additions

- Add to `internal/appconfig`:
  - In `App`:
    - `CustomActions []CustomAction`
    - `TmuxCustomMenuHotkey string`
  - Type `CustomAction`:
    - `ID string`
    - `Title string`
    - `Kind string` (optional)
    - `Scope string` ("selection" | "diagnostics")
    - `Hotkey string` (optional, 1 char)
    - `Instruction string` (optional)
    - `System string` (optional)
    - `User string` (optional)
  - TOML mapping:
    - `[[prompts.code_action.custom]]` → slice of `CustomAction`
    - `[tmux] custom_menu_hotkey`

### Validation

Implement `func (a App) Validate() error` and call it on startup in hexai-lsp-server and hexai-tmux-action. Fail fast with a descriptive error if any rule is violated:
- Duplicate `id` among custom actions (case-insensitive): "config: duplicate custom action id: <id>"
- Duplicate custom action `hotkey` (case-insensitive, non-empty): "config: duplicate custom action hotkey: <hotkey>"
- `tmux.custom_menu_hotkey` collides with built-in tmux top-level hotkeys (`r,i,c,t,p,s`) or is not a single character: "config: invalid tmux.custom_menu_hotkey: <value>"
- Custom action `hotkey` collides with another custom action hotkey: as above.
- Missing `title` or `id`: "config: custom action missing required field <field>"
- Both `instruction` and `user` set (ambiguous): "config: custom action <id> must set either instruction or user, not both"
- Neither `instruction` nor `user` set: "config: custom action <id> requires instruction or user"
- Invalid `scope` value: "config: custom action <id> has invalid scope: <value>"

If validation fails:
- hexai-lsp-server: log the error and exit the server (do not serve requests).
- hexai-tmux-action: print the error on stderr and exit non-zero.

## LSP Integration

Listing (textDocument/codeAction):
- For each custom action in `App.CustomActions`:
  - scope=selection: include only when `sel` (selected text) is non-empty.
  - scope=diagnostics: include only when there are diagnostics in the range.
  - Build `CodeAction{ Title: "Hexai: "+Title, Kind: KindOrDefault, Data: payload }` where `payload` includes:
    - `Type: "custom"`, `ID`, `URI`, `Range`, `Selection`, and `Diagnostics` if present.

Resolution (codeAction/resolve):
- On `Type == "custom"`, look up the action by `ID` in `App.CustomActions`.
- Construct LLM messages:
  - If `User` set: `system = action.System or s.promptRewriteSystem`; `user = render(action.User, vars)`.
  - Else (`Instruction` set): use LSP’s global rewrite templates: `system = s.promptRewriteSystem`; `user = render(s.promptRewriteUser, {instruction, selection})`.
- Perform `chatWithStats` with existing request options.
- Strip fences and return a `WorkspaceEdit` replacing the selection range.

## hexai-tmux-action Integration

TUI changes:
- Add a top-level item: "Custom actions…" with hotkey from `[tmux].custom_menu_hotkey` (default "a"). Selecting it opens a submenu listing all `App.CustomActions` as items (Title + optional per-item hotkey).
- Built-in items remain as-is (Rewrite, Simplify, Document, Generate Go test(s), Custom prompt, Skip).

Execution:
- When a custom action is selected, run a new `runCustom(ctx, cfg, client, action, parts)`:
  - If `action.User` set: render with available vars and call `runOnceWithOpts`.
  - Else: call `runRewrite` with `action.Instruction`.
  - Return output to stdout (same as other actions).

Validation behavior:
- Before showing the TUI, validate config. If invalid, print the error and exit non-zero.
- Disallow `tmux.custom_menu_hotkey` collisions with built-ins; disallow duplicate custom `hotkey`s; report conflicts clearly.

## Error Messages (examples)

- "config: duplicate custom action id: extract-function"
- "config: custom action fix-lints requires instruction or user"
- "config: duplicate custom action hotkey: e"
- "config: invalid tmux.custom_menu_hotkey: r (clashes with built-in)"

## Backwards Compatibility

- Existing config remains valid; if no `[[prompts.code_action.custom]]` is defined, behavior is unchanged.
- The existing "Custom prompt" action (free-form editor input) remains and is separate from the “Custom actions…” submenu.

## Implementation Notes

- Keep template rendering consistent with existing helpers (`textutil.RenderTemplate`, `StripCodeFences`).
- Reuse request option construction (`llmRequestOpts` in LSP, `reqOptsFrom(cfg)` in hexaiaction).
- Keep validation logic centralized in appconfig so both binaries share identical rules.
- Tests:
  - appconfig: parsing + validation (ids, hotkeys, fields, scopes).
  - lsp: codeAction listing/resolve for selection and diagnostics scopes.
  - hexaiaction: TUI shows submenu; custom action execution path.

