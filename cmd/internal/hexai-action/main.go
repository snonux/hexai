package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"codeberg.org/snonux/hexai/internal/hexaiaction"
)

func main() {
	infile := flag.String("infile", "", "Read input from this file instead of stdin")
	outfile := flag.String("outfile", "", "Write output to this file instead of stdout")
	flag.Parse()

	in, out, closeIn, closeOut, err := openIO(*infile, *outfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeIn()
	defer closeOut()

	if err := hexaiaction.Run(context.Background(), in, out, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// openIO returns readers/writers for infile/outfile flags with deferred closers.
func openIO(infile, outfile string) (io.Reader, io.Writer, func(), func(), error) {
	in := io.Reader(os.Stdin)
	out := io.Writer(os.Stdout)
	closeIn := func() {}
	closeOut := func() {}

	if path := infile; path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, func() {}, func() {}, fmt.Errorf("hexai-action: cannot open infile: %w", err)
		}
		in = f
		closeIn = func() { _ = f.Close() }
	}
	if path := outfile; path != "" {
		f, err := os.Create(path)
		if err != nil {
			return nil, nil, func() {}, func() {}, fmt.Errorf("hexai-action: cannot open outfile: %w", err)
		}
		out = f
		closeOut = func() { _ = f.Close() }
	}
	return in, out, closeIn, closeOut, nil
}
