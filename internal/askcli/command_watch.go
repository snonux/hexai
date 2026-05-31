package askcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
)

const (
	watchInterval   = 2 * time.Second
	ansiClearScreen = "\033[2J\033[H"
)

type watchTicker interface {
	C() <-chan time.Time
	Stop()
}

type realWatchTicker struct {
	ticker *time.Ticker
}

func (t realWatchTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t realWatchTicker) Stop() {
	t.ticker.Stop()
}

var newWatchTicker = func(interval time.Duration) watchTicker {
	return realWatchTicker{ticker: time.NewTicker(interval)}
}

func (d *Dispatcher) handleWatch(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	watchArgs := args[1:]
	if len(watchArgs) == 0 {
		watchArgs = []string{"list"}
	}
	if !watchCommandAllowed(watchArgs) {
		_, _ = io.WriteString(stderr, "error: ask watch supports read-only subcommands: list, all, ready, completed, info, dep list, urgency, help\n")
		return 1, nil
	}

	ticker := newWatchTicker(watchInterval)
	defer ticker.Stop()

	var lastOutput []byte
	for {
		output, code, err := d.watchOutput(ctx, watchArgs)
		if err != nil {
			return code, err
		}
		if !bytes.Equal(output, lastOutput) {
			_, _ = io.WriteString(stdout, ansiClearScreen)
			_, _ = stdout.Write(output)
			lastOutput = bytes.Clone(output)
		}
		if code != 0 {
			return code, nil
		}

		select {
		case <-ctx.Done():
			return 0, nil
		case <-ticker.C():
		}
	}
}

func watchCommandAllowed(args []string) bool {
	entry, ok := commandRegistry.get(args[0])
	if !ok {
		return false
	}
	if entry.readOnly {
		return true
	}
	// dep is not read-only as a whole, but dep list is.
	if args[0] == "dep" {
		return len(args) >= 2 && args[1] == "list"
	}
	return false
}

func (d *Dispatcher) watchOutput(ctx context.Context, args []string) ([]byte, int, error) {
	var stdout, stderr bytes.Buffer
	code, err := d.dispatchCommand(ctx, append([]string(nil), args...), nil, &stdout, &stderr)
	if err != nil {
		return nil, code, fmt.Errorf("watch %s: %w", args[0], err)
	}
	// Combine stdout and stderr so warnings or errors emitted by the
	// watched subcommand are visible in the watched display.
	out := make([]byte, 0, stdout.Len()+stderr.Len())
	out = append(out, stdout.Bytes()...)
	out = append(out, stderr.Bytes()...)
	return out, code, nil
}
