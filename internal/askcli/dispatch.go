package askcli

import (
	"context"
	"fmt"
	"io"
)

// Runner performs CLI work that would otherwise be handled by the ask CLI itself.
//
// The interface is implemented by the executor that ultimately proxies commands to Taskwarrior.
type Runner interface {
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
}

// Dispatcher translates CLI arguments into concrete subcommands and presents the output.
type Dispatcher struct {
	runner     Runner
	jsonOutput bool
}

// NewDispatcher creates a Dispatcher backed by the provided Runner or a default executor when nil.
func NewDispatcher(runner Runner) *Dispatcher {
	if runner == nil {
		e := NewExecutor("ask")
		runner = &e
	}
	return &Dispatcher{runner: runner}
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
	return d.dispatchCommand(ctx, args, stdin, stdout, stderr)
}

func (d *Dispatcher) dispatchCommand(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	subcommand := args[0]
	entry, ok := commandRegistry.get(subcommand)
	if !ok {
		args = append([]string{"add"}, args...)
		subcommand = "add"
		entry, ok = commandRegistry.get(subcommand)
	}
	if !ok {
		return d.unknownCommand(stderr, subcommand)
	}
	return entry.handler(d, ctx, args, stdin, stdout, stderr)
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
	_, _ = io.WriteString(w, "  ask completed               List completed tasks\n")
	_, _ = io.WriteString(w, "  ask all [filters]            List all tasks including completed/deleted\n")
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
	_, _ = io.WriteString(w, "  ask projects                  List projects with pending, not-yet-started tasks\n")
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
