package askcli

import (
	"bytes"
	"context"
	"io"
)

func (d Dispatcher) handleDelete(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask delete requires a UUID argument\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "delete"}, stdin, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}
