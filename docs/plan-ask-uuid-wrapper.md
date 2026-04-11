# Plan: `ask` as UUID-only Taskwarrior Wrapper

The user-facing CLI binary is **`ask`** (it was briefly named `do`). This document uses `ask` throughout. The Go implementation package remains `internal/askcli` in the tree below.

## Goal

Rewrite the `ask` command from a thin pass-through proxy into a **subcommand-based CLI** that wraps Taskwarrior. The wrapper never exposes numeric task IDs to the caller — only UUIDs. Output is minimal and machine-friendly for coding agents.

The existing `project:<repo> +agent` auto-injection is preserved.

## Subcommands

| Subcommand | Example | Taskwarrior equivalent |
|---|---|---|
| `ask add "Implement X"` | create task | `task project:P +agent add "Implement X"` |
| `ask add priority:H +cli "Fix bug"` | create with priority & tag | same + `priority:H +cli` |
| `ask list` | list pending tasks | `task project:P +agent status:pending export` → reformat |
| `ask info <uuid>` | show one task | `task uuid:<uuid> export` → filtered fields |
| `ask annotate <uuid> "note"` | add annotation | `task uuid:<uuid> annotate "note"` |
| `ask start <uuid>` | start work | `task uuid:<uuid> start` |
| `ask stop <uuid>` | stop work | `task uuid:<uuid> stop` |
| `ask done <uuid>` | mark complete | `task uuid:<uuid> done` |
| `ask priority <uuid> H` | set priority | `task uuid:<uuid> modify priority:H` |
| `ask tag <uuid> +foo` | add tag | `task uuid:<uuid> modify +foo` |
| `ask tag <uuid> -foo` | remove tag | `task uuid:<uuid> modify -foo` |
| `ask dep add <uuid> <dep-uuid>` | add dependency | `task uuid:<uuid> modify depends:<dep-uuid>` |
| `ask dep rm <uuid> <dep-uuid>` | remove dependency | `task uuid:<uuid> modify depends:-<dep-uuid>` |
| `ask dep list <uuid>` | show dependencies | `task uuid:<uuid> export` → `depends` field |
| `ask urgency` | list by urgency | `task project:P +agent export` → sort by urgency |
| `ask modify <uuid> <args...>` | general modify | `task uuid:<uuid> modify <args...>` (priority, tags, depends, /old/new/) |
| `ask denotate <uuid> "text"` | remove annotation | `task uuid:<uuid> denotate "text"` |
| `ask delete <uuid>` | delete task | `task uuid:<uuid> delete` |
| `ask export` | raw JSON dump | `task project:P +agent export` → pass through |

### List filters, sort, and limit

`ask list` accepts optional filters, sort, and limit arguments:

| Example | Taskwarrior equivalent |
|---|---|
| `ask list` | `task project:P +agent status:pending export` (default sort: priority-, urgency-) |
| `ask list +READY` | `task project:P +agent +READY export` |
| `ask list +BLOCKED` | `task project:P +agent +BLOCKED export` |
| `ask list +frontend` | `task project:P +agent +frontend export` |
| `ask list started` | `task project:P +agent start.any: export` |
| `ask list limit:3` | show only first 3 results |
| `ask list +READY limit:1` | next ready task |

## Data Retrieval: `task export`

All read operations use `task <filter> export` which returns a JSON array. This avoids fragile text parsing. Write operations (`add`, `done`, `start`, `stop`, `annotate`, `modify`) call Taskwarrior directly and capture stdout to extract the created UUID.

### TaskExport struct

```go
type TaskExport struct {
    UUID        string   `json:"uuid"`
    Description string   `json:"description"`
    Status      string   `json:"status"`
    Priority    string   `json:"priority"`
    Tags        []string `json:"tags"`
    Start       string   `json:"start,omitempty"`
    Urgency     float64  `json:"urgency"`
    Depends     []string `json:"depends"`
    Annotations []struct {
        Description string `json:"description"`
        Entry       string `json:"entry"`
    } `json:"annotations"`
}
```

## Output Formatting

- **`list` / `urgency`**: Compact table — `UUID | Priority | Status | Tags | Description | Urgency`. No numeric ID column.
- **`info`**: UUID, description, status, priority, tags, annotations, dependencies (as UUIDs), urgency.
- **`add`**: Print only the UUID of the created task (parse from Taskwarrior stdout).
- **All other write commands**: Print success/failure + UUID. Suppress Taskwarrior decorative output.

## ID Rejection

If an argument looks like a bare numeric ID where a UUID is expected, reject with: `"use UUID, not numeric task ID"`.

## Package Layout

```
cmd/ask/main.go             — parse subcommand, dispatch to internal/askcli
internal/askcli/             — implementation package (CLI name: ask)
  ├── dispatch.go            — subcommand router (switch args[0])
  ├── taskexec.go            — wraps Taskwarrior execution (binary lookup, repo detection, run)
  ├── taskexport.go          — TaskExport struct + JSON parse helper
  ├── formatter.go           — shared UUID-only table/info formatting
  ├── command_add.go         — add logic + UUID extraction from stdout
  ├── command_list.go        — list via export + reformat
  ├── command_info.go        — info via export + field filter
  ├── command_annotate.go    — annotate
  ├── command_start.go       — start
  ├── command_stop.go        — stop
  ├── command_done.go        — done
  ├── command_priority.go    — set priority
  ├── command_tag.go         — add/remove tags
  ├── command_dep.go         — dep add/rm/list
  ├── command_urgency.go     — urgency-sorted list
  ├── command_modify.go      — general-purpose modify
  ├── command_denotate.go    — remove annotation
  ├── command_delete.go      — delete task
  └── command_export.go      — raw JSON export
```

Each `command_*.go` file gets a corresponding `command_*_test.go`.

## Changes to Existing Code

- **`cmd/ask/main.go`** — stops calling `taskproxy.Runner.Run` directly; delegates to `askcli.Dispatch()`.
- **`internal/taskproxy/`** — reused by `askcli/taskexec.go` for binary lookup (`findTaskBinary`) and repo root detection (`detectRepoRoot`). The `Runner.Run` pass-through method becomes unused and can be removed.

## Task Breakdown

1. Scaffold `internal/askcli/` — dispatch, taskexec, taskexport, formatter
2. Implement `ask add` (UUID extraction from Taskwarrior stdout)
3. Implement `ask list` (export → UUID-only table)
4. Implement `ask info <uuid>` (export → filtered fields)
5. Implement `ask annotate <uuid> "note"`
6. Implement `ask start <uuid>` / `ask stop <uuid>`
7. Implement `ask done <uuid>`
8. Implement `ask priority <uuid> <P>`
9. Implement `ask tag <uuid> +/-tag`
10. Implement `ask dep add/rm/list`
11. Implement `ask urgency`
12. Implement `ask modify <uuid> <args...>` (general-purpose modify)
13. Implement `ask denotate <uuid> "text"` (remove annotation)
14. Implement `ask delete <uuid>`
15. Implement `ask export` (raw JSON)
16. Add filter/sort/limit support to `ask list` (+READY, +BLOCKED, +tag, started, limit:N)
17. Wire `cmd/ask/main.go` to `askcli.Dispatch`, remove old pass-through
18. Update docs and README
19. Create `agent-task-management` skill (replacement for `taskwarrior-task-management`) — uses only `ask` subcommands, no Taskwarrior references
20. Update Pi coding agent: rename `taskwarrior-plan-mode` extension → `agent-plan-mode`, rewrite to use `ask` subcommands only
21. Audit `agent-task-management` skill and `agent-plan-mode` extension: ensure zero Taskwarrior leakage — agents must see `ask` as the native task system, not a wrapper

## Skill & Extension Migration

After the `ask` CLI is complete and documented, three follow-up tasks abstract away the Taskwarrior implementation detail:

### 19. `agent-task-management` skill

Create a new skill at `~/.agents/skills/agent-task-management/` by copying the structure from `taskwarrior-task-management` (SKILL.md + references/00-context.md through 5-review-overview-tasks.md). Rewrite all content to use `ask` subcommands (`ask add`, `ask list`, `ask info`, `ask start`, `ask stop`, `ask done`, `ask annotate`, `ask denotate`, `ask modify`, `ask priority`, `ask tag`, `ask dep`, `ask urgency`, `ask delete`, `ask export`). Remove all mentions of Taskwarrior, raw `task` command, numeric IDs, and `_uuid` lookups.

### 20. `agent-plan-mode` Pi extension

Rename `~/git/hyperstack/pi/agent/extensions/taskwarrior-plan-mode/` → `agent-plan-mode/`. Update `index.ts`, `utils.ts`, `README.md` to use new `ask` subcommands instead of raw pass-through calls. Rename `TaskwarriorTask` type, update state keys, command names, descriptions, and UI messages.

### 21. Taskwarrior leakage audit

Grep all files in the new skill and extension for `taskwarrior` (any casing), `task warrior`, raw `task ` command patterns, numeric task ID usage, `_uuid` lookups, and any hint that `ask` wraps another tool. **Agents must believe `ask` IS the task management system — not a proxy.** This prevents agents from attempting raw Taskwarrior commands or mentioning Taskwarrior in annotations/descriptions.
