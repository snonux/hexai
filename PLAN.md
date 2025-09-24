# Runtime Model Configuration Plan

Implement a /reload> endpoint that reloads all the configuration from hexai.toml and updates the running application's state without requiring a restart.

## Progress
- [x] Phase 1 – Design approach for making settings dynamically changeable
- [x] Phase 2 – Implement dynamic configuration plumbing
- [x] Phase 3 – Expose `/reload>` command and emit change summary (file values override env on reload)

## Phase 1 Notes (in progress)
- Current config flow: each entry point calls `appconfig.Load(logger)` which merges defaults + `config.toml` + env overrides, then copies fields into long-lived structs (e.g. `lsp.Server`).
- LSP server captures many scalar copies (`maxTokens`, prompts, triggers, stats window), so runtime changes require a reapply step that updates these cached fields plus the `llm.Client` instance and stats window.
- Proposed shape: introduce a central runtime config manager wrapping `appconfig.App` with an `RWMutex`, diff helpers, and subscription callbacks. All components pull the latest snapshot or subscribe to updates instead of keeping independent copies.
- Reload path should reuse shared loader logic that can optionally skip env overrides so `/reload>` can make file values authoritative.
- Applying updates must be atomic per component (e.g. server lock + swap) and should emit a structured list of changed settings for user feedback.
- Manager responsibilities: (1) hold current snapshot, (2) surface `Subscribe(func(old, new appconfig.App))` for live update hooks, (3) expose `Reload(ctx)` that re-parses `config.toml`, produces diff keys, updates snapshot, and returns the changes for logging/UX.
- LSP integration: pass manager into `RunWithFactory`, add `Server.applyAppConfig(cfg appconfig.App)` guarded by lock, rebuild `llm.Client` when provider/model change, refresh prompts/triggers/debounce/temps, and re-run `initializeModelConfig`/stats window.
- Logging/stats: ensure updates propagate (e.g., call `stats.SetWindow` within manager or server update when `StatsWindowMinutes` changes) so runtime metrics align with new config.
- `/reload>` command: hook into existing chat command detection (extend `chatCommandResponse`) to invoke manager `Reload`, then write the diff summary back to the buffer as a synthetic assistant response.

## Phase 2 Notes (progress)
- Added `appconfig.LoadWithOptions` to support skipping env overrides and introduced a `runtimeconfig.Store` (subscribe + diff) as the runtime configuration backbone.
- `hexailsp.RunWithFactory` now builds a shared store, subscribes the LSP server to config updates, and rebuilds the LLM client + stats/log settings when config changes.
- Implemented CLI-style slash commands in the LSP (`/reload>` currently) so runtime reloads can be triggered without restarting; reload skips env overrides and reports diffed keys.
- Remaining work: surface change summaries inside the server logs, ensure other entry points (CLI/action) share the same store, and add verification around env override precedence.
