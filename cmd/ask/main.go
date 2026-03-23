package main

import (
	"context"
	"fmt"
	"os"

	"codeberg.org/snonux/hexai/internal/askcli"
)

func main() {
	d := askcli.NewDispatcher(nil)
	code, err := d.Dispatch(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		// Print the internal error so callers get a useful diagnostic message.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
	if code != 0 {
		os.Exit(code)
	}
}
