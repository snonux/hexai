package main

import (
	"io"
	"os"

	"github.com/snonux/hexai/internal/askcmd"
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return askcmd.Run(append([]string{"na"}, args...), stdin, stdout, stderr)
}
