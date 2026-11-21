package askcli

import (
	"context"
	"io"
	"strings"
)

// needsHumanTag marks a task that is parked until a human answers a question.
// Parked tasks stay pending (their dependents remain blocked) but are hidden
// from "ask ready", so an agent working through the list moves on to the next
// task instead of stopping the whole run at the first blocker.
const needsHumanTag = "needshuman"

// needsHumanPrefix starts the annotation that records the open question, so
// humans and agents can find it with one search.
const needsHumanPrefix = "Needs human:"

// handlePark implements "ask park <id> <question>": stop the task if it was
// started, tag it, and record the question.
func (d *Dispatcher) handlePark(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	question := ""
	if len(args) >= 3 {
		question = strings.TrimSpace(strings.Join(args[2:], " "))
	}
	if question == "" {
		_, _ = io.WriteString(stderr, "error: ask park requires an ID or UUID and the question for the human, e.g. ask park <id> \"which port should the service use?\"\n")
		return 1, nil
	}
	if !strings.HasPrefix(question, needsHumanPrefix) {
		question = needsHumanPrefix + " " + question
	}
	return d.runTaskSteps(ctx, args[1], stdout, stderr, []taskStep{
		{optional: true, args: func(resolved resolvedTaskSelector) []string {
			return []string{"uuid:" + resolved.UUID, "stop"}
		}},
		{args: func(resolved resolvedTaskSelector) []string {
			return []string{"uuid:" + resolved.UUID, "modify", "+" + needsHumanTag}
		}},
		{args: func(resolved resolvedTaskSelector) []string {
			return []string{"uuid:" + resolved.UUID, "annotate", question}
		}},
	})
}

// handleUnpark implements "ask unpark <id>": the task becomes selectable by
// "ask ready" again. The question annotation is kept as history.
func (d *Dispatcher) handleUnpark(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: ask unpark requires an ID or UUID argument\n")
		return 1, nil
	}
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "modify", "-" + needsHumanTag}
	})
}

// handleParked lists the pending tasks that wait for a human answer.
func (d *Dispatcher) handleParked(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, []string{"status:pending", "+" + needsHumanTag}, args[1:], stdout, stderr)
}
