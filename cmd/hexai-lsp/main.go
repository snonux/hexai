// Summary: Hexai LSP entrypoint; parses flags and delegates to internal/hexailsp.
package main

import (
	"flag"
	"log"
	"os"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/hexailsp"
)

func main() {
	logPath := flag.String("log", "/tmp/hexai-lsp.log", "path to log file (optional)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		log.Println(internal.Version)
		return
	}

	if err := hexailsp.Run(*logPath, os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
