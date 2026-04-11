package askcli

import (
	"context"
	"fmt"
	"io"
	"os"
)

func (d *Dispatcher) handleFish(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	_ = ctx
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: ask fish")
		return 1, nil
	}
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "ask"
	}
	if _, err := io.WriteString(stdout, FishCompletionFor(binaryPath)); err != nil {
		return 1, err
	}
	return 0, nil
}
