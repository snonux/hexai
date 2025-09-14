Hexai: Custom Code Actions — Implementation Plan

Status legend: [ ] pending, [x] done, [~] in progress

1) Baseline + Design setup
   - [x] Add design doc and sample config updates
   - [x] Establish baseline test/coverage command

2) App config: model + parsing + validation
   - [x] Add App fields, TOML parsing for [[prompts.code_action.custom]] and [tmux]
   - [x] Implement App.Validate() and wire into LSP + tmux startup
   - [x] Unit tests: parsing, validation, duplicate detection, hotkey conflicts

3) LSP: list/resolve custom actions
   - [x] Extend ServerOptions + Server to carry custom actions
   - [x] List custom actions (scope-aware) in codeAction
   - [x] Resolve custom actions (instruction or system+user)
   - [x] Unit tests: list includes, resolve edits for selection/diagnostics

4) Tmux action: submenu + execution
   - [x] Add configurable “Custom actions…” menu item with [tmux].custom_menu_hotkey
   - [x] Implement submenu to pick a custom action (with per-item hotkeys)
   - [x] Implement runCustom() execution path
   - [x] Unit tests: runCustom path
   - [x] Unit tests: submenu hotkeys and selection via seam

5) Polish
   - [x] Update docs and finalize examples
   - [x] All unit tests pass
   - [ ] gofumpt formatting on modified Go files
   - [ ] (Optional) Raise coverage further with additional tests

Notes:
- After each numbered section is completed, run tests and verify coverage >= 85% before moving to the next.
