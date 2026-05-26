package askcli

import (
	"context"
	"io"
)

type commandHandler func(d *Dispatcher, ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)

type simpleCommand func(d *Dispatcher, ctx context.Context, args []string, stdout, stderr io.Writer) (int, error)

func wrapSimpleCommand(handler simpleCommand) commandHandler {
	return func(d *Dispatcher, ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return handler(d, ctx, args, stdout, stderr)
	}
}

type commandEntry struct {
	name                string
	description         string
	handler             commandHandler
	includeInCompletion bool
	singleSelector      bool
	readOnly            bool
}

type commandTable struct {
	entries []commandEntry
	lookup  map[string]int
}

func newCommandTable(entries []commandEntry) commandTable {
	lookup := make(map[string]int, len(entries))
	for i := range entries {
		entry := entries[i]
		lookup[entry.name] = i
	}
	return commandTable{entries: entries, lookup: lookup}
}

func (t commandTable) get(name string) (*commandEntry, bool) {
	idx, ok := t.lookup[name]
	if !ok {
		return nil, false
	}
	return &t.entries[idx], true
}

func (t commandTable) rootCompletionEntries() []commandEntry {
	var entries []commandEntry
	for _, entry := range t.entries {
		if entry.includeInCompletion {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (t commandTable) singleSelectorNames() []string {
	var names []string
	for _, entry := range t.entries {
		if entry.singleSelector {
			names = append(names, entry.name)
		}
	}
	return names
}

func (t *commandTable) add(entry commandEntry) {
	t.entries = append(t.entries, entry)
	t.lookup[entry.name] = len(t.entries) - 1
}

var commandRegistry = newCommandTable([]commandEntry{
	{
		name:                "add",
		description:         "Create a new task",
		handler:             wrapSimpleCommand((*Dispatcher).handleAdd),
		includeInCompletion: true,
	},
	{
		name:                "list",
		description:         "List active tasks",
		handler:             wrapSimpleCommand((*Dispatcher).handleList),
		includeInCompletion: true,
		readOnly:            true,
	},
	{
		name:                "all",
		description:         "List all tasks",
		handler:             wrapSimpleCommand((*Dispatcher).handleAll),
		includeInCompletion: true,
		readOnly:            true,
	},
	{
		name:                "ready",
		description:         "List READY tasks",
		handler:             wrapSimpleCommand((*Dispatcher).handleReady),
		includeInCompletion: true,
		readOnly:            true,
	},
	{
		name:                "info",
		description:         "Show task details",
		handler:             wrapSimpleCommand((*Dispatcher).handleInfo),
		includeInCompletion: true,
		singleSelector:      true,
		readOnly:            true,
	},
	{
		name:                "annotate",
		description:         "Add an annotation",
		handler:             wrapSimpleCommand((*Dispatcher).handleAnnotate),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "start",
		description:         "Start a task",
		handler:             wrapSimpleCommand((*Dispatcher).handleStart),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "stop",
		description:         "Stop a task",
		handler:             wrapSimpleCommand((*Dispatcher).handleStop),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "done",
		description:         "Mark a task complete",
		handler:             wrapSimpleCommand((*Dispatcher).handleDone),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "priority",
		description:         "Set priority",
		handler:             wrapSimpleCommand((*Dispatcher).handlePriority),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "tag",
		description:         "Add or remove a tag",
		handler:             wrapSimpleCommand((*Dispatcher).handleTag),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "modify",
		description:         "Modify task fields",
		handler:             wrapSimpleCommand((*Dispatcher).handleModify),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "denotate",
		description:         "Remove an annotation",
		handler:             wrapSimpleCommand((*Dispatcher).handleDenotate),
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "delete",
		description:         "Delete a task",
		handler:             (*Dispatcher).handleDelete,
		includeInCompletion: true,
		singleSelector:      true,
	},
	{
		name:                "dep",
		description:         "Manage dependencies",
		handler:             wrapSimpleCommand((*Dispatcher).handleDep),
		includeInCompletion: true,
	},
	{
		name:                "urgency",
		description:         "List tasks sorted by urgency",
		handler:             wrapSimpleCommand((*Dispatcher).handleUrgency),
		includeInCompletion: true,
		readOnly:            true,
	},
})

func init() {
	commandRegistry.add(commandEntry{
		name:                "watch",
		description:         "Repeatedly run a subcommand and redraw when output changes",
		handler:             wrapSimpleCommand((*Dispatcher).handleWatch),
		includeInCompletion: true,
	})
	commandRegistry.add(commandEntry{
		name:                "fish",
		description:         "Emit Fish shell completion script",
		handler:             wrapSimpleCommand((*Dispatcher).handleFish),
		includeInCompletion: true,
	})
	commandRegistry.add(commandEntry{
		name:        "help",
		description: "Show help",
		readOnly:    true,
		handler: func(d *Dispatcher, ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			_ = ctx
			_ = args
			_ = stdin
			_ = stderr
			return d.help(stdout)
		},
		includeInCompletion: true,
	})
	commandRegistry.add(commandEntry{
		name:                "complete-uuids",
		description:         "Emit task selector list",
		handler:             wrapSimpleCommand((*Dispatcher).handleCompleteUUIDs),
		includeInCompletion: false,
	})
	commandRegistry.add(commandEntry{
		name:                "complete-aliases",
		description:         "Emit short task selector list for Fish completion",
		handler:             wrapSimpleCommand((*Dispatcher).handleCompleteAliases),
		includeInCompletion: false,
	})
}
