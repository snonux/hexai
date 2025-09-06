package main

import (
    "bytes"
    "log"
    "os"
    "testing"
)

func TestMain_Version(t *testing.T) {
    oldArgs := os.Args
    defer func() { os.Args = oldArgs }()
    os.Args = []string{"hexai-lsp", "-version"}
    var buf bytes.Buffer
    old := log.Writer()
    log.SetOutput(&buf)
    defer log.SetOutput(old)
    main()
    if buf.Len() == 0 {
        t.Fatalf("expected version log")
    }
}

