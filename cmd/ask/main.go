package main

import (
	"os"

	"github.com/snonux/hexai/internal/askcmd"
)

func main() {
	os.Exit(askcmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
