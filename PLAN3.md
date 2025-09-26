# Parallel Provider/Model Comparison Plan

Goal: allow configuring multiple provider:model pairs per surface so users can run a local and cloud LLM side-by-side for manual comparison. Completions should fan out and show one suggestion per configured entry; Hexai CLI should emit one response per entry; code actions remain single-provider.

## Phase 1 – Configuration & Schema
- [x] Audit existing per-surface config to support arrays of provider:model entries (preserving current single-entry behavior by default).
- [x] Design updated TOML and env schema (e.g., `[[models.completion]] provider="openai" model="gpt-4o"`).
- [x] Define merge, validation, and backward-compatibility rules (single entry auto-wraps into list).

## Phase 2 – Runtime Plumbing
- [x] Extend appconfig/runtime store to emit ordered slices for multi-entry surfaces, including diff output.
- [x] Update request-spec helpers to iterate across configured entries, building dedicated request specs (and caching clients per provider/model combo).
- [x] Ensure logging/stats capture provider/model context per entry.

## Phase 3 – Surface Implementations
- [x] Completion: fan out requests concurrently, gather one suggestion per entry, and surface them distinctly to the editor (labelled with provider/model).
- [x] CLI: run all configured providers in parallel and print separate responses per entry with stats.
- [x] Code actions: keep single-provider flow and warn/ignore additional `[[models.code_action]]` entries.
- [x] Add concurrency safeguards (debounce/throttle gate still respected before fan-out).

## Phase 4 – UX & Validation
- [x] Tests covering multi-entry parsing, diffing, and surface behavior (expanded CLI/LSP/appconfig suites).
- [x] Update docs and example TOML with array syntax and dual-provider guidance.
- [x] Capture lessons/issues in scratchpad for follow-up polishing.
