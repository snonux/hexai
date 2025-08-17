// Summary: Hexai CLI entrypoint; parses flags and delegates to internal/hexaicli.
// Not yet reviewed by a human
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"hexai/internal"
	"hexai/internal/hexaicli"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Fprintln(os.Stdout, internal.Version)
		return
	}

	if err := hexaicli.Run(context.Background(), flag.Args(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
