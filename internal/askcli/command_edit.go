package askcli

import (
	"bytes"
	"context"
	"io"

	"codeberg.org/snonux/hexai/internal/editor"
)

// editorCapture opens the user's editor on a temporary file pre-filled with
// the given initial content and returns its trimmed contents after the editor
// exits. ctx is forwarded to the editor subprocess so it can be cancelled with
// the surrounding command.
func editorCapture(ctx context.Context, initial []byte) (string, error) {
	return editor.OpenTempAndEdit(ctx, initial)
}

// handleEdit opens the configured editor on a temporary file. With no selector
// it creates a new task from the resulting content. With a task ID/UUID selector
// it pre-fills the editor with the task's current description and updates it.
func (d *Dispatcher) handleEdit(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) >= 2 {
		return d.editTaskDescription(ctx, args[1], stdout, stderr)
	}
	description, err := d.capture(ctx, nil)
	if err != nil {
		writeInfoError(stderr, err)
		return 1, nil
	}
	if description == "" {
		_, _ = io.WriteString(stderr, "error: ask edit aborted: empty description\n")
		return 1, nil
	}
	return d.createTask(ctx, nil, description, nil, stdout, stderr)
}

// editTaskDescription resolves a task selector, opens the editor pre-filled with
// the task's current description, and modifies the task with the new content.
func (d *Dispatcher) editTaskDescription(ctx context.Context, selector string, stdout, stderr io.Writer) (int, error) {
	resolved, tasks, code, err := d.resolveTaskSelector(ctx, selector, stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}

	description, err := d.capture(ctx, []byte(tasks[0].Description))
	if err != nil {
		writeInfoError(stderr, err)
		return 1, nil
	}
	if description == "" {
		_, _ = io.WriteString(stderr, "error: ask edit aborted: empty description\n")
		return 1, nil
	}

	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "modify", description}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	_, _ = io.WriteString(stdout, FormatSuccess(displayResolvedTaskID(resolved)))
	return 0, nil
}
