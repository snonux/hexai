# Changelog

All notable changes to this project are documented here.

## [0.0.3] - 2025-08-16

- Added: "Resolve diagnostics" code action that fixes only diagnostics overlapping the current selection; replaces exactly the selected range.
- Added: Configurable completion trigger characters via `trigger_characters` in config.
- Changed: Default trigger characters when unset now include `.` `:` `/` `_` `;` `?`.
- Refactor: `lsp.NewServer` now takes a `ServerOptions` struct instead of a long parameter list.
- Docs: Updated README and `config.json.example` with trigger configuration and new code action.
- Tests: Added unit tests for diagnostic filtering and range overlap helpers.

