package askcli

import (
	"context"
	"fmt"
	"io"
)

// Runner performs CLI work that would otherwise be handled by the do CLI itself.
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
		e := NewExecutor("do")
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
	subcommand := args[0]
	entry, ok := commandRegistry.get(subcommand)
	if !ok && scope != taskScopeAgent {
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
	_, _ = io.WriteString(w, "do - task management CLI\n")
	_, _ = io.WriteString(w, "\nProject prefixes:\n")
	_, _ = io.WriteString(w, "  do proj:<name> <subcommand...> Run a subcommand against an explicit project\n")
	_, _ = io.WriteString(w, "\nScope prefixes:\n")
	_, _ = io.WriteString(w, "  do na <subcommand...>         Run a subcommand against project tasks without +agent\n")
	_, _ = io.WriteString(w, "  do no-agent <subcommand...>   Alias for do na\n")
	_, _ = io.WriteString(w, "\nSubcommands:\n")
	_, _ = io.WriteString(w, "  do add [mods...] [depends:<id|uuid>,...] <description...> Create a new task and print created task <id>\n")
	_, _ = io.WriteString(w, "  do list [filters]           List active tasks (default)\n")
	_, _ = io.WriteString(w, "  do ready                   List READY tasks (not blocked)\n")
	_, _ = io.WriteString(w, "  do all [filters]            List all tasks including completed/deleted\n")
	_, _ = io.WriteString(w, "  do info [id|uuid]            Show task details or current started task\n")
	_, _ = io.WriteString(w, "  do annotate <id|uuid> \"note\" Add annotation to task\n")
	_, _ = io.WriteString(w, "  do start <id|uuid>           Start working on task\n")
	_, _ = io.WriteString(w, "  do stop <id|uuid>            Stop work on a task\n")
	_, _ = io.WriteString(w, "  do done <id|uuid>            Mark task complete\n")
	_, _ = io.WriteString(w, "  do priority <id|uuid> <P>    Set priority (H/M/L)\n")
	_, _ = io.WriteString(w, "  do tag <id|uuid> +/-<tag>    Add or remove tag\n")
	_, _ = io.WriteString(w, "  do dep add <id|uuid> <dep>   Add dependency\n")
	_, _ = io.WriteString(w, "  do dep rm <id|uuid> <dep>    Remove dependency\n")
	_, _ = io.WriteString(w, "  do dep list <id|uuid>        List dependencies\n")
	_, _ = io.WriteString(w, "  do urgency                   List tasks sorted by urgency\n")
	_, _ = io.WriteString(w, "  do modify <id|uuid> <args...> Modify task fields\n")
	_, _ = io.WriteString(w, "  do denotate <id|uuid> \"text\" Remove annotation\n")
	_, _ = io.WriteString(w, "  do delete <id|uuid>          Delete a task\n")
	_, _ = io.WriteString(w, "  do fish                      Emit Fish shell completion script\n")
	return 0, nil
}

func (d *Dispatcher) unknownCommand(w io.Writer, subcommand string) (int, error) {
	fmt.Fprintf(w, "do: unknown subcommand %q\n", subcommand)
	return 1, nil
}
