package askcli

import (
	"context"
	"fmt"
	"io"
)

// Runner performs CLI work that would otherwise be handled by ask itself.
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

	if len(args) == 0 {
		return d.handleList(ctx, []string{"list"}, stdout, stderr)
	}
	subcommand := args[0]
	switch subcommand {
	case "info":
		return d.handleInfo(ctx, args, stdout, stderr)
	case "add":
		return d.handleAdd(ctx, args, stdout, stderr)
	case "list":
		return d.handleList(ctx, args, stdout, stderr)
	case "all":
		return d.handleAll(ctx, args, stdout, stderr)
	case "ready":
		return d.handleReady(ctx, args, stdout, stderr)
	case "dep":
		return d.handleDep(ctx, args, stdout, stderr)
	case "urgency":
		return d.handleUrgency(ctx, stdout, stderr)
	case "annotate":
		return d.handleAnnotate(ctx, args, stdout, stderr)
	case "start":
		return d.handleStart(ctx, args, stdout, stderr)
	case "stop":
		return d.handleStop(ctx, args, stdout, stderr)
	case "done":
		return d.handleDone(ctx, args, stdout, stderr)
	case "priority":
		return d.handlePriority(ctx, args, stdout, stderr)
	case "tag":
		return d.handleTag(ctx, args, stdout, stderr)
	case "modify":
		return d.handleModify(ctx, args, stdout, stderr)
	case "denotate":
		return d.handleDenotate(ctx, args, stdout, stderr)
	case "delete":
		return d.handleDelete(ctx, args, stdin, stdout, stderr)
	case "fish":
		return d.handleFish(args, stdout, stderr)
	case "help":
		return d.help(stdout)
	case "complete-uuids":
		return d.handleCompleteUUIDs(ctx, stdout, stderr)
	default:
		return d.unknownCommand(stderr, subcommand)
	}
}

func (d *Dispatcher) help(w io.Writer) (int, error) {
	_, _ = io.WriteString(w, "ask - task management CLI\n")
	_, _ = io.WriteString(w, "\nSubcommands:\n")
	_, _ = io.WriteString(w, "  ask add [mods...] [depends:<id|uuid>,...] <description...> Create a new task and print its ID\n")
	_, _ = io.WriteString(w, "  ask list [filters]           List active tasks (default)\n")
	_, _ = io.WriteString(w, "  ask ready                   List READY tasks (not blocked)\n")
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
