# Per-Surface LLM Model Configuration Plan

Goal: allow users to configure distinct LLM models for (1) code completion, (2) code actions, (3) in-editor chat, and (4) the `hexai` CLI while keeping defaults sensible and maintaining backward compatibility. The new options must remain hot-reloadable via the existing runtime config store.

## Phase 1 – Configuration Design
- [x] Audit current config structures (`internal/appconfig`) and identify the model/temperature fields each surface consumes.
- [x] Propose TOML schema extensions (e.g., `[models] completion = "..."`) plus environment variable overrides.
- [x] Define precedence rules and fallback behavior when only a global model is provided.
- [x] Sketch migration approach (default legacy fields map to all surfaces).

## Phase 2 – Loader & Runtime Store Updates
- [x] Extend `appconfig` to parse per-surface model settings (and optional temperature overrides) with validation.
- [x] Update `runtimeconfig.Store` diff/flatten logic to include the new fields and guarantee reload propagation works without restart.
- [x] Ensure reload summaries list per-surface changes cleanly.
- [x] Add unit tests covering config parsing, env overrides, and diff output, plus runtime reload coverage.

## Phase 3 – Surface Wiring
- [x] Completion: adjust LSP completion code to pick the configured completion model, falling back to provider defaults.
- [x] Code actions: ensure code-action prompts and CLI action runner request the configured model.
- [x] In-editor chat: pass chat-specific model to chat requests and CLI chat command handling.
- [x] Hexai CLI: respect the CLI model when building `llm.Config` or request options.
- [x] Provide logging to confirm which model each surface uses for easier debugging.

## Phase 4 – Validation & Docs
- [x] Add integration/unit tests covering each surface model selection path.
- [x] Verify runtime reload switches models without restart (including diff output).
- [x] Update docs (`docs/configuration.md`, examples) with new keys and environment variables.
- [x] Announce in scratchpad or release notes placeholder for future update.
