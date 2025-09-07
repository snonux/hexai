Comprehensive plan: integrate Helix + tmux flow into hexai-action

Summary of current setup
- Helix keybinding pipes selection to `ai`, which dispatches to `hx.hexai-action-prompt` for hexai-action mode.
- `hx.hexai-action-prompt` writes stdin to `~/.hx-action-input`, opens a tmux split-pane, runs `hexai-action -infile <input> -outfile <reply>.tmp`, then atomically renames to `<reply>` and prints the reply back to Helix.
- This works but requires shell scripts and out-of-band temp files.

Goal
- Helix should call hexai-action directly: `:pipe hexai-action`.
- hexai-action itself handles: reading stdin; presenting TUI in a tmux pane (when needed); executing the chosen action; printing the result to stdout for Helix to apply.
- Consolidate all tmux logic in a reusable `internal/tmux` package for future features.

Proposed CLI/UX
- Default (auto):
  - If running in an interactive TTY, run TUI inline and print the result to stdout.
  - If stdin/stdout are pipes (Helix `:pipe`) and a tmux session is available, spawn a temporary tmux pane to render the TUI, then return the final output on stdout.
  - If tmux is not available and no TTY is present, fall back to a sensible non-interactive mode: echo input or use instruction-based rewrite if an inline instruction is detected.
- Flags (public):
  - `-tmux` (bool, default: auto): force enabling tmux-pane mode.
  - `-no-tmux` (bool): force disabling tmux-pane mode even if available.
  - `-infile`, `-outfile`: keep for compatibility/testing, but not needed for Helix.
- Flags (internal/private; hidden in help):
  - `-ui-child` (bool): internal child mode that assumes `-infile` and `-outfile` and runs the interactive TUI on the attached TTY, writing final output to outfile.
  - `-tmux-target` (string, optional): tmux target pane/window (advanced users).
  - `-tmux-split` (enum: `v|h`, default `v`): split orientation.

High-level design
1) IO orchestration (parent process)
   - Determine whether to show TUI inline or via tmux based on TTY detection and `-tmux`/`-no-tmux`.
   - Inline path: run current `hexaiaction.Run(ctx, in, out, err)` (unchanged behavior) and exit.
   - Tmux path: write stdin to a secure temp file, spawn a tmux split-pane that executes a `hexai-action -ui-child -infile <in> -outfile <out>.tmp`, wait for completion (by process exit and/or file rename), then print `<out>` to stdout.

2) Child/TUI execution (`-ui-child`)
   - Read from `-infile`, parse input (diagnostics + selection), construct LLM client, show Bubble Tea menu, run selected action, write result to `-outfile.tmp`, fsync, rename to `-outfile`.
   - On error, write a human-readable message to stderr and a minimal fallback to outfile (e.g., echo selection) to avoid blocking Helix.

3) Tmux integration package (`internal/tmux`)
   - Responsibilities:
     - Detect availability: binary present and inside a tmux session (`$TMUX` set) or a viable target.
     - Run commands in a new split pane and return control immediately or after completion.
     - Small helpers for file-based rendezvous (optional), e.g., `WaitForFile(path, timeout)`.
   - Minimal API (initial):
     - `func Available() bool`
     - `type SplitOpts struct { Target string; Vertical bool; Percent int }`
     - `func SplitRun(opts SplitOpts, argv []string) error` — runs `tmux split-window ... <argv>` and returns once tmux has launched the child process.
     - `func HasBinary() bool` and `func InSession() bool` (if we want finer checks).
   - Implementation details:
     - Shell out to `tmux` (no lib dep). Build command like: `tmux split-window -v -p 33 "<cmd>"`.
     - Quote/escape argv safely. Prefer `exec.Command` for the child in a shell wrapper, or join argv for `tmux`’s command string.
     - Avoid writing to `~/.hx-*`; use `os.CreateTemp("", "hexai-action-*" )` under `$TMPDIR`.

4) hexaiaction refactor (internal package)
   - Separate concerns to keep functions small/testable:
     - New function `ChooseAction(ctx, stdin, stderr) (ActionKind, InputParts, error)` that parses input and (conditionally) runs TUI.
     - Existing action runners remain unchanged (rewrite, document, diagnostics, gotest).
     - Keep `Run` as a thin orchestrator that assumes interactive mode; the parent (cmd) decides inline vs tmux/child.
   - Ensure unit-testable seams: parse, instruction extraction, action execution already have tests; add tests for new branching logic where possible without tmux.

5) Robustness and UX details
   - TTY detection: use `golang.org/x/term` or a small `isatty` helper to decide inline vs tmux.
   - Timeouts: child actions already use short timeouts; parent wait for outfile should have a reasonable deadline (e.g., 60s) to avoid hanging Helix.
   - Atomic writes: write to `outfile.tmp`, `Sync`, then `Rename` for a clear completion signal.
   - Cleanup: always remove temp files (defer and signal handling for SIGINT/SIGTERM).
   - Logging: log to stderr with clear `hexai-action` prefixes; keep stdout clean for Helix’s `:pipe`.

Helix configuration after change
- Replace the current keybinding pipeline with a single call:
  - `C-a = ":pipe hexai-action"`
- Optional: users can force tmux pane with `:pipe hexai-action -tmux` or disable with `:pipe hexai-action -no-tmux` if auto-detection does not fit their setup.

Migration plan
1) Implement tmux package and integrate auto-mode in `cmd/internal/hexai-action/main.go`.
2) Keep legacy flags (`-infile`, `-outfile`) for compatibility and tests.
3) Update README and docs to show new Helix keybinding and describe flags.
4) Mark shell scripts (`llminputs/ai`, `llminputs/hx.hexai-action-prompt`) as deprecated in repo notes; retain them temporarily.
5) After a stabilization period, remove the scripts (or move to `scripts/legacy/`).

Testing plan
- Unit tests:
  - `internal/tmux`: mock `exec.Command` via a small command-runner interface; verify command assembly and availability checks.
  - `cmd/internal/hexai-action`: tests for parent decision logic (TTY vs pipe; tmux available vs not) using injectable detectors.
  - hexaiaction: existing tests continue to pass; add tests for non-interactive fallback behavior when no TTY.
- Integration tests (manual or scripted):
  - Run under tmux: verify a pane opens, TUI choice is applied, stdout contains result.
  - Run outside tmux: verify inline TUI works in terminal; verify fallback behavior under `:pipe` without tmux.
 - Coverage target: ensure at least 85% unit test coverage for all new code paths added in this integration (verify with `go test -cover ./...` and per-package `-coverprofile`).

Edge cases and mitigations
- No tmux, no TTY: skip TUI; if inline instruction detected (`;...;`, `// ...`, etc.), run rewrite; else echo selection.
- tmux available but spawning fails: warn on stderr and fall back to non-interactive mode.
- Large inputs: spill to temp files; ensure temp dir has space; surface errors clearly.
- Windows: tmux mode auto-disables; inline mode continues to work.

Implementation steps (incremental)
1) Add `internal/tmux` with `Available`, `SplitRun`, and helpers.
2) Add TTY detection helper to `cmd/internal/hexai-action` and wire flags (`-tmux`, `-no-tmux`, `-ui-child`).
3) Parent flow: detect mode, manage temp files, spawn child via tmux when selected, wait/print result.
4) Child flow: reuse existing `hexaiaction.Run` to keep logic centralized; ensure outfile atomic write.
5) Docs: update README with new Helix config and flags.
6) Optional: add `-tmux-target`/`-tmux-split` for power users.

Notes on code organization
- Place all tmux-related code under `internal/tmux` and keep functions well under 50 lines.
- Keep command entrypoint (`cmd/internal/hexai-action/main.go`) small and focused on wiring/mode selection.
- Avoid duplication across `hexaiaction` and `tmux`; IO/file and action logic remain in `hexaiaction`.

Outcome
- One-step Helix integration (`:pipe hexai-action`).
- No helper scripts required; cross-platform friendly with graceful fallbacks.
- Reusable tmux utilities for future features.

Progress
- [x] Add `internal/tmux` with `Available`, `SplitRun`, quoting helpers.
- [x] Wire flags in `hexai-action`: `-tmux`, `-no-tmux`, `-ui-child`, `-tmux-target`, `-tmux-split`, `-tmux-percent`.
- [x] Parent tmux orchestration: write stdin to temp, split tmux, wait for outfile, print to stdout.
- [x] Child mode: atomic `outfile.tmp` write and rename, with error echo fallback.
- [x] Unit tests for `internal/tmux` and tmux decision logic in `hexai-action` (validate locally; target ≥85% coverage for new code).
- [x] Update README/docs for new Helix keybinding and flags.
- [ ] Delete legacy helper scripts (`llminputs/ai`, `llminputs/hx.hexai-action-prompt`) when ready; no deprecation notice.
- [x] Ran coverage locally. Notes:
  - `mage coverage` now passes (HTML at docs/coverage.html). Total cross-package coverage ≈ 84%.
  - New package `internal/tmux` is ≥85% covered. The `hexai-action` entrypoint package sits ~69% overall; newly added helper paths are covered (openIO, runChild, runInTmuxParent, echoThrough, waitForFile, etc.). The inline TTY UI path and `main()` remain intentionally untested.
