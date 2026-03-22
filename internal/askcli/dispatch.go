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
	case "help":
		return d.help(stdout)
	default:
		return d.unknownCommand(stderr, subcommand)
	}
}

func (d Dispatcher) help(w io.Writer) (int, error) {
	io.WriteString(w, "ask - task management CLI\n")
	io.WriteString(w, "\nSubcommands:\n")
	io.WriteString(w, "  ask add \"description\"          Create a new task\n")
	io.WriteString(w, "  ask list [filters]           List active tasks (default)\n")
	io.WriteString(w, "  ask ready                   List READY tasks (not blocked)\n")
	io.WriteString(w, "  ask all [filters]            List all tasks including completed/deleted\n")
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
	return 0, nil
}

func (d Dispatcher) unknownCommand(w io.Writer, subcommand string) (int, error) {
	fmt.Fprintf(w, "ask: unknown subcommand %q\n", subcommand)
	return 1, nil
}
