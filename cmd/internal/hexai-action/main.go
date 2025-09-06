package main

import (
	"context"
	"fmt"
	"os"

	"codeberg.org/snonux/hexai/internal/hexaiaction"
)

func main() {
	if err := hexaiaction.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
