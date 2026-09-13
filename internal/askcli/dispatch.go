package askcli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Runner performs CLI work that would otherwise be handled by the ask CLI itself.
//
// The interface is implemented by the executor that ultimately proxies commands to Taskwarrior.
type Runner interface {
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
}

// Dispatcher translates CLI arguments into concrete subcommands and presents the output.
//
// newTicker is the injected factory for the watch-loop ticker. Production code
// uses the real time.Ticker-backed factory installed by NewDispatcher; tests
// inject a fake ticker so they can drive the watch loop without real delays.
type Dispatcher struct {
	runner         Runner
	jsonOutput     bool
	newTicker      func(time.Duration) watchTicker
	capture        func(context.Context, []byte) (string, error)
	aliasCache     taskAliasCacheDeps
	findTaskBinary func() (string, error)
	runTaskCommand taskCommandRunner
	now            func() time.Time
}

// NewDispatcher creates a Dispatcher backed by the provided Runner or a default
// executor when nil. It wires the default real-ticker factory used by the
// `ask watch` loop.
func NewDispatcher(runner Runner) *Dispatcher {
	if runner == nil {
		e := NewExecutor("ask")
		runner = &e
	}
	return &Dispatcher{
		runner:         runner,
		newTicker:      newRealWatchTicker,
		capture:        editorCapture,
		aliasCache:     defaultTaskAliasCacheDeps(),
		findTaskBinary: findTaskBinary,
		runTaskCommand: runTaskCommand,
		now:            time.Now,
	}
}

func parseGlobalFlags(args []string) ([]string, bool) {
	var filtered []string
	var jsonOutput bool
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered, jsonOutput
}

// Dispatch parses CLI arguments, handles global flags, and routes the request to the matching subcommand.
func (d *Dispatcher) Dispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	args, jsonOutput := parseGlobalFlags(args)
	d.jsonOutput = jsonOutput
	scope, projectName, projectSet, args := parseTaskPrefixes(args)
	ctx = contextWithTaskScope(ctx, scope)
	if projectSet {
		ctx = contextWithTaskProject(ctx, projectName)
	}

	if len(args) == 0 {
		args = []string{"list"}
	}
	if args[0] == "--help" || args[0] == "-h" {
		return d.help(stdout)
	}
	return d.dispatchCommand(ctx, args, stdin, stdout, stderr)
}

func (d *Dispatcher) dispatchCommand(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	subcommand := args[0]
	entry, ok := commandRegistry.get(subcommand)
	if !ok {
		if code, refused := d.rejectImplicitAdd(ctx, args, stderr); refused {
			return code, nil
		}
		args = append([]string{"add"}, args...)
		subcommand = "add"
		entry, ok = commandRegistry.get(subcommand)
	}
	if !ok {
		return d.unknownCommand(stderr, subcommand)
	}
	return entry.handler(d, ctx, args, stdin, stdout, stderr)
}

// implicitAddInfoVerbs lists first words that are not ask subcommands but that
// callers commonly pass when they meant "ask info <id>". When such a word is
// followed by a task-like selector, the implicit-add fallback is refused so
// botched invocations cannot silently create junk tasks.
var implicitAddInfoVerbs = map[string]bool{
	"display": true,
	"get":     true,
	"open":    true,
	"print":   true,
	"show":    true,
	"view":    true,
}

// rejectImplicitAdd reports whether the implicit-add fallback for an unknown
// first word must be refused instead of creating a task. It refuses when the
// first word resolves to an existing task (e.g. "ask gd1 start" meant
// "ask start gd1") or when it is a misused info verb followed by a numeric
// taskwarrior ID or an existing task selector (e.g. "ask show 361").
// Refusing keeps agent syntax mistakes from landing in the task list as junk
// tasks while ordinary descriptions (e.g. "fix the bug") still fall through
// to the implicit add.
func (d *Dispatcher) rejectImplicitAdd(ctx context.Context, args []string, stderr io.Writer) (int, bool) {
	first := args[0]
	if implicitAddInfoVerbs[first] && len(args) >= 2 {
		second := args[1]
		if IsNumericID(second) {
			fmt.Fprintf(stderr, "error: ask has no %q subcommand; numeric Taskwarrior IDs are not accepted\nFind the alias with ask list, then use e.g. ask info <alias>.\nTo create a task with this description, use: ask add \"%s\"\n",
				first, strings.Join(args, " "))
			return 1, true
		}
		if looksLikeTaskAlias(second) {
			if resolved, tasks, _, err := d.resolveTaskSelector(ctx, second, io.Discard); err == nil && len(tasks) > 0 {
				id := displayResolvedTaskID(resolved)
				fmt.Fprintf(stderr, "error: ask has no %q subcommand; use ask info %s to show task %s %q\nTo create a task with this description, use: ask add \"%s\"\n",
					first, id, id, truncateDescription(tasks[0].Description, 60), strings.Join(args, " "))
				return 1, true
			}
		}
	}
	if !looksLikeTaskAlias(first) {
		return 0, false
	}
	resolved, tasks, _, err := d.resolveTaskSelector(ctx, first, io.Discard)
	if err != nil || len(tasks) == 0 {
		return 0, false
	}
	id := displayResolvedTaskID(resolved)
	fmt.Fprintf(stderr, "error: %q is not a subcommand; it is task %s %q\nDid you mean: ask start %s, ask info %s, ask done %s, or ask annotate %s \"note\"?\nTo create a task with this description, use: ask add \"%s\"\n",
		first, id, truncateDescription(tasks[0].Description, 60), id, id, id, id, strings.Join(args, " "))
	return 1, true
}

func (d *Dispatcher) help(w io.Writer) (int, error) {
	_, _ = io.WriteString(w, "ask - task management CLI\n")
	_, _ = io.WriteString(w, "\nProject prefixes:\n")
	_, _ = io.WriteString(w, "  ask proj:<name> <subcommand...> Run a subcommand against an explicit project\n")
	_, _ = io.WriteString(w, "\nScope prefixes:\n")
	_, _ = io.WriteString(w, "  ask na <subcommand...>         Run a subcommand against project tasks without +agent\n")
	_, _ = io.WriteString(w, "  ask no-agent <subcommand...>   Alias for ask na\n")
	_, _ = io.WriteString(w, "\nSubcommands:\n")
	_, _ = io.WriteString(w, "  ask add [mods...] [depends:<id|uuid>,...] <description...> Create a new task and print created task <id>\n")
	_, _ = io.WriteString(w, "  ask edit                     Open $EDITOR and create a task from its content\n")
	_, _ = io.WriteString(w, "  ask list [filters]           List active tasks (default)\n")
	_, _ = io.WriteString(w, "  ask ready                   List READY tasks (not blocked)\n")
	_, _ = io.WriteString(w, "  ask completed [filters]     List completed tasks\n")
	_, _ = io.WriteString(w, "  ask all [filters]            List all tasks including completed/deleted\n")
	_, _ = io.WriteString(w, "Filters: limit:<n>, sort:<key>, +<tag>, started, since:<value>, and date attrs\n")
	_, _ = io.WriteString(w, "  since:today|this.week|this.month|N.hours|N.days|N.weeks|N.months  (completed within the given period)\n")
	_, _ = io.WriteString(w, "  end:, modified:, created:, due:, scheduled:, waiting:, start:  (e.g. end:today, end.after:2026-08-22)\n")
	_, _ = io.WriteString(w, "  ask info [id|uuid]            Show task details or current started task\n")
	_, _ = io.WriteString(w, "  ask annotate <id|uuid> \"note\" Add annotation to task\n")
	_, _ = io.WriteString(w, "  ask start <id|uuid>           Start working on task\n")
	_, _ = io.WriteString(w, "  ask stop <id|uuid>            Stop work on a task\n")
	_, _ = io.WriteString(w, "  ask done <id|uuid>            Mark task complete\n")
	_, _ = io.WriteString(w, "  ask priority <id|uuid> <P>    Set priority (H/M/L)\n")
	_, _ = io.WriteString(w, "  ask tag <id|uuid> +/-<tag>    Add or remove tag\n")
	_, _ = io.WriteString(w, "  ask dep add <id|uuid> <dep>   Add dependency\n")
	_, _ = io.WriteString(w, "  ask dep rm <id|uuid> <dep>    Remove dependency\n")
	_, _ = io.WriteString(w, "  ask dep list <id|uuid>        List dependencies\n")
	_, _ = io.WriteString(w, "  ask urgency                   List tasks sorted by urgency\n")
	_, _ = io.WriteString(w, "  ask watch [subcommand...]     Re-run a read-only subcommand every 2s and redraw on changes\n")
	_, _ = io.WriteString(w, "  ask projects [+tag...]        List projects with pending, not-yet-started tasks (optional tag filters)\n")
	_, _ = io.WriteString(w, "  ask modify <id|uuid> <args...> Modify task fields\n")
	_, _ = io.WriteString(w, "  ask denotate <id|uuid> \"text\" Remove annotation\n")
	_, _ = io.WriteString(w, "  ask delete <id|uuid>          Delete a task\n")
	_, _ = io.WriteString(w, "  ask fish                      Emit Fish shell completion script\n")
	return 0, nil
}

func (d *Dispatcher) unknownCommand(w io.Writer, subcommand string) (int, error) {
	fmt.Fprintf(w, "ask: unknown subcommand %q\n", subcommand)
	return 1, nil
}
