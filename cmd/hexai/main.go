// Package main is the Hexai CLI entrypoint; parses flags and delegates to internal/hexaicli.
package main

import "os"

func main() {
	if exitCode := newAppRunner().run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); exitCode != 0 {
		os.Exit(exitCode)
	}
}
