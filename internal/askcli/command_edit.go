package askcli

import (
	"context"
	"io"

	"codeberg.org/snonux/hexai/internal/editor"
)

// captureFromEditor opens the user's editor on a temporary file and returns its
// trimmed contents after the editor exits. It is a variable so tests can stub it.
var captureFromEditor = func() (string, error) {
	return editor.OpenTempAndEdit(nil)
}

// handleEdit opens the configured editor on a temporary file and creates a new
// task from the resulting (possibly multi-line) content.
func (d *Dispatcher) handleEdit(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	_ = args
	description, err := captureFromEditor()
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
