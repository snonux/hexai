# Parallel Provider/Model Comparison Plan

Goal: allow configuring multiple provider:model pairs per surface so users can run a local and cloud LLM side-by-side for manual comparison. Completions should fan out and show one suggestion per configured entry; Hexai CLI should emit one response per entry; code actions remain single-provider.

## Phase 1 – Configuration & Schema
- [x] Audit existing per-surface config to support arrays of provider:model entries (preserving current single-entry behavior by default).
- [x] Design updated TOML and env schema (e.g., `[[models.completion]] provider="openai" model="gpt-4o"`).
- [x] Define merge, validation, and backward-compatibility rules (single entry auto-wraps into list).

## Phase 2 – Runtime Plumbing
- Extend appconfig/runtime store to emit ordered slices for multi-entry surfaces, including diff output.
- Update request-spec helpers to iterate across configured entries, building dedicated request specs (and caching clients per provider/model combo).
- Ensure logging/stats capture provider/model context per entry.

## Phase 3 – Surface Implementations
- Completion: fan out requests sequentially, gather one suggestion per entry, and surface them distinctly to the editor (label with provider/model).
- CLI: stream or print separate responses per entry, with clear headers and stats per run.
- Code actions: keep single-provider flow but ensure config ignores extra entries with validation warnings.
- Add reasonable concurrency limits / timeouts so multi-provider usage stays responsive.

## Phase 4 – UX & Validation
- Tests covering multi-entry parsing, diffing, and surface-specific behavior (mock providers to simulate dual responses).
- Update docs and example TOML with new array syntax, including env override strategy.
- Capture lessons/issues in scratchpad for follow-up polishing.
