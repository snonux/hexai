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
	uuid := NormalizeUUID(args[1])
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"uuid:" + uuid, "delete"}, stdin, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}
