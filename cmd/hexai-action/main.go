package main

import (
    "context"
    "flag"
    "fmt"
    "os"

    "codeberg.org/snonux/hexai/internal/hexaiaction"
)

func main() {
    infile := flag.String("infile", "", "Read input from this file instead of stdin")
    outfile := flag.String("outfile", "", "Write output to this file instead of stdout")
    forceTmux := flag.Bool("tmux", false, "Force running the UI in a tmux split-pane (auto if not set)")
    noTmux := flag.Bool("no-tmux", false, "Disable tmux mode even if available")
    uiChild := flag.Bool("ui-child", false, "INTERNAL: run interactive UI and write to -outfile atomically")
    tmuxTarget := flag.String("tmux-target", "", "tmux split target (advanced)")
    tmuxSplit := flag.String("tmux-split", "v", "tmux split orientation: v or h")
    tmuxPercent := flag.Int("tmux-percent", 33, "tmux split size percentage (1-100)")
    flag.Parse()

    opts := hexaiaction.Options{
        Infile: *infile, Outfile: *outfile,
        ForceTmux: *forceTmux, NoTmux: *noTmux, UIChild: *uiChild,
        TmuxTarget: *tmuxTarget, TmuxSplit: *tmuxSplit, TmuxPercent: *tmuxPercent,
    }
    if err := hexaiaction.RunCommand(context.Background(), opts, os.Stdin, os.Stdout, os.Stderr); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

