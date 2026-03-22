package askcli

import (
	"context"
	"fmt"
	"io"
)

type Runner interface {
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
}

type Dispatcher struct {
	runner Runner
}

func NewDispatcher(runner Runner) *Dispatcher {
	if runner == nil {
		e := NewExecutor("ask")
		runner = &e
	}
	return &Dispatcher{runner: runner}
}

func (d Dispatcher) Dispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return d.help(stdout)
	}
	subcommand := args[0]
	switch subcommand {
	case "add", "list", "info", "annotate", "start", "stop", "done",
		"priority", "tag", "dep", "urgency", "modify", "denotate", "export":
		return d.runner.Run(ctx, args, stdin, stdout, stderr)
	case "delete":
		return d.handleDelete(ctx, args, stdout, stderr)
	default:
		return d.unknownCommand(stderr, subcommand)
	}
}

func (d Dispatcher) help(w io.Writer) (int, error) {
	io.WriteString(w, "ask - task management CLI\n")
	io.WriteString(w, "\nSubcommands:\n")
	io.WriteString(w, "  ask add \"description\"          Create a new task\n")
	io.WriteString(w, "  ask list [filters]           List tasks (UUID-only output)\n")
	io.WriteString(w, "  ask info <uuid>               Show task details\n")
	io.WriteString(w, "  ask annotate <uuid> \"note\"    Add annotation to task\n")
	io.WriteString(w, "  ask start <uuid>              Start working on task\n")
	io.WriteString(w, "  ask stop <uuid>               Stop working on task\n")
	io.WriteString(w, "  ask done <uuid>               Mark task complete\n")
	io.WriteString(w, "  ask priority <uuid> <P>        Set priority (H/M/L)\n")
	io.WriteString(w, "  ask tag <uuid> +/-<tag>       Add or remove tag\n")
	io.WriteString(w, "  ask dep add <uuid> <dep-uuid> Add dependency\n")
	io.WriteString(w, "  ask dep rm <uuid> <dep-uuid> Remove dependency\n")
	io.WriteString(w, "  ask dep list <uuid>           List dependencies\n")
	io.WriteString(w, "  ask urgency                   List tasks sorted by urgency\n")
	io.WriteString(w, "  ask modify <uuid> <args...>   Modify task fields\n")
	io.WriteString(w, "  ask denotate <uuid> \"text\"     Remove annotation\n")
	io.WriteString(w, "  ask delete <uuid>             Delete task\n")
	io.WriteString(w, "  ask export                    Raw JSON export\n")
	io.WriteString(w, "\nFilters:\n")
	io.WriteString(w, "  +READY +BLOCKED +<tag> started limit:N sort:priority-,urgency-\n")
	return 0, nil
}

func (d Dispatcher) unknownCommand(w io.Writer, subcommand string) (int, error) {
	fmt.Fprintf(w, "ask: unknown subcommand %q\n", subcommand)
	return 1, nil
}
