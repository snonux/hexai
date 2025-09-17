## Global Hexai LLM Stats (Plan)

### Goals
- Unify LLM usage stats across all Hexai processes: `hexai-lsp`, `hexai` (CLI), and `hexai-tmux-action`).
- Persist stats on disk so concurrent processes contribute to a single, shared view.
- Show consistent stats in logs and in the tmux status line regardless of which binary triggered the last request.
- Track both per-provider:model and global totals; include request count and total bytes sent/received; compute RPM.
- Always display stats for a sliding recent window (default: last 1 hour).

### Non-Goals (for this iteration)
- No networked metrics backends, no long-term history beyond recent minutes needed to compute RPM.
- No user-facing commands to reset/export stats (can be a follow-up).

### Cache Location and Layout
- Directory: `XDG_CACHE_HOME/hexai` (fallback to `~/.cache/hexai` when `XDG_CACHE_HOME` is unset).
- File: `stats.json` (atomically written via temp file + rename).
- Schema (v1):
  {
    "version": 1,
    "updated_at": "RFC3339",
    "window_seconds": 3600,
    "events": [
      { "ts": "RFC3339Nano", "provider": "openai", "model": "gpt-4.1", "sent": 1234, "recv": 5678 }
    ]
  }
- Notes:
  - Array-like append-only event list with periodic compaction: on update, drop entries older than `window_seconds` (default 3600 seconds = 1h).
  - Aggregations (global totals, per provider/model, RPM) are computed on read from events within the current window only.
  - Keep file size bounded: compact (prune + optionally coalesce older sub-minute events into minute buckets) when length exceeds a threshold (e.g., 10k events) or on a time basis.

### Concurrency & File Locking
- Use advisory file locks for Unix-like systems.
  - Create/`open` lock file `stats.lock` in the same cache directory.
  - Apply `flock(LOCK_EX)` (via `syscall`/`golang.org/x/sys/unix`) around the read-modify-write cycle.
  - Ensure the lock file is held for the shortest duration (milliseconds).
- Atomic update:
  - Read existing `stats.json` (if missing, start with empty events and default window).
  - Append one event for the just-finished request; prune entries older than `window_seconds` (relative to now).
  - Write to `stats.json.tmp`, `fsync`, then `rename` to `stats.json`.
- Retry strategy:
  - Bounded retries with small backoff if lock acquisition or IO fails; log a single warning and continue without crashing.

### Package Design
- New package: `internal/stats`
  - `func Update(ctx, provider, model string, sentBytes, recvBytes int) error` (append event, prune old).
  - `func Snapshot(ctx context.Context) (S, error)` to read current state (aggregate from events within window).
  - `func RPM(s S) float64` computes requests/minute over the configured window.
  - `func SetWindow(d time.Duration)` and `func Window() time.Duration` to configure the window (default 1h; read from `config.toml`).
  - `func CacheDir() (string, error)` honoring XDG; `func Path() string` for `stats.json`.
  - Careful with allocations and zero/empty-state handling.
- Types:
  - `type Event struct { TS time.Time; Provider, Model string; Sent, Recv int64 }`
  - `type StatFile struct { Version int; UpdatedAt time.Time; WindowSeconds int; Events []Event }`
  - Aggregated snapshot (in-memory):
    - `type Counters struct { Reqs int64; Sent int64; Recv int64 }`
    - `type ProviderEntry struct { Totals Counters; Models map[string]Counters }`
    - `type Snapshot struct { Global Counters; Providers map[string]ProviderEntry; RPM float64; Window time.Duration }`

### Integration Points
- Common approach: update stats exactly where we already compute per-process counters.

1) LSP (`internal/lsp`)
- Hook at the end of:
  - `chatWithStats`: after successful Chat, call `stats.Update(provider, model, sentBytes, recvBytes)`.
  - Provider-native completion path: when we get suggestions, also update using `sentBytes` and received bytes of first suggestion (consistent with current local counters).
- After update, read a `Snapshot` (window-aware by design) and compute:
  - Per current provider:model totals (for context), and global totals over the last window (default 1h).
  - RPM computed from events in the current window.
- Display:
  - Logs: extend existing LLM stats line to include Σ (global) view.
  - tmux: replace current status with a compact global view, e.g.:
    - `⏳ Σ reqs=123 rpm=4.2 ↑1.2MB ↓3.4MB | openai:gpt-4.1 reqs=80 rpm=3.1`.
    - Use `tmux.FormatLLMStatsStatusColoredGlobal(...)` (new) to render.

2) CLI (`cmd/hexai`/`internal/hexaicli`)
- Where Chat is invoked (current CLI flow calls LLM directly): wrap the LLM client or count bytes and call `stats.Update` after each request.
- Print a one-line summary to stderr (consistent with LSP logging format).

3) Tmux Action (`cmd/hexai-tmux-action` / `internal/hexaiaction`)
- In the code paths that call `client.Chat` (runOnce / runOnceWithOpts), after success call `stats.Update`.
- Update tmux status the same way as LSP by reusing the same formatter function in `internal/tmux`.

### Tmux Status API
- Extend `internal/tmux` with a new helper:
  - `func FormatGlobalStatsStatusColored(s stats.Snapshot, preferProvider, preferModel string) string` (include window indicator like `Σ@1h`).
  - Or a smaller data struct extracted from snapshot to avoid leaking types.
- Keep existing `FormatLLMStatsStatusColored` for backward compatibility; LSP/CLI/TUI all switch to the new global formatter.

### Logging
- Reuse existing logging but compute and append global counters:
  - `llm stats reqs=local avg_sent=... rpm_local=... | Σ reqs=... rpm=... sent_total=... recv_total=...`
- Keep logs short to avoid noise; gate with existing log level.

### Configuration
- New section in `config.toml`:
  - `[stats] window_minutes = 60` (default 60; min 1, max 1440)
  - All displays and RPM calculations operate over this sliding window.

### Error Handling
- Stats update failures must never fail the user-facing operation.
- Log at `info` once per process when disk write fails and then mute repeated errors for a cooldown period.

- Unit tests for `internal/stats`:
  - Cache dir resolution (XDG vs HOME).
  - Locking: concurrent goroutines updating stats in a temp XDG cache dir; assert totals match expected; ensure no partial writes.
  - Event pruning (older than window) and RPM calculation over the configured window.
  - JSON round-trip and version field.
- Integration tests (lightweight):
  - Override `XDG_CACHE_HOME` to a temp directory.
  - Simulate 2 processes: spawn subtests that call `stats.Update` interleaved; assert final snapshot.
  - LSP and hexaiaction: hook fakes that perform `Chat` and then verify `stats.Snapshot` reflects the calls.

### Migration / Backward Compatibility
- On first run, create cache dir and empty stats file lazily under lock.
- If file is invalid JSON or version mismatch, start from zero and overwrite.

### Rollout Plan
- [x] Scaffold `internal/stats` with types, JSON read/write, cache dir, and lock helpers (Unix).
- [x] Implement `Update()` with lock → read → mutate → write (atomic) and pruning.
- [x] Implement `Snapshot()` and helpers to compute aggregates and RPM over the configured window (pruning done; optional compaction TBD).
- [x] Add tmux formatter in `internal/tmux` to display global stats (compact view).
- [x] Integrate LSP: update stats in `chatWithStats` and provider-native path; use global snapshot for tmux status.
- [x] Integrate CLI and Tmux Action: update stats after each Chat; stderr/tmux show global view.
- [x] Add tests for `internal/stats` (window pruning, concurrency, XDG path).
- [x] Run mage Coverage and update docs/screenshots if needed.
- [x] Verify all LLM call paths contribute to the new stats mechanism.

### Estimation & Risks
- Est. 4–6 hours including tests and integration.
- Risk: file locking portability (Linux/macOS OK with flock). Mitigation: implement Unix only now; detect/disable gracefully elsewhere.
- Risk: tmux status width. Mitigation: show Σ-only by default and elide per-model when narrow (or truncate labels).
