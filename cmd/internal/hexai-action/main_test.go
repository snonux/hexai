package main

import (
    "io"
    "os"
    "path/filepath"
    "testing"
)

// TestOpenIO_InOutFiles verifies that openIO opens the specified files
// and that writing via the returned writer persists to disk.
func TestOpenIO_InOutFiles(t *testing.T) {
    dir := t.TempDir()
    inPath := filepath.Join(dir, "in.txt")
    outPath := filepath.Join(dir, "out.txt")

    // Prepare input file
    want := "hello world"
    if err := os.WriteFile(inPath, []byte(want), 0o600); err != nil {
        t.Fatalf("write infile: %v", err)
    }

    in, out, cin, cout, err := openIO(inPath, outPath)
    if err != nil {
        t.Fatalf("openIO: %v", err)
    }
    defer cin()
    defer cout()

    // Copy through to simulate main's behavior
    if _, err := io.Copy(out.(io.Writer), in); err != nil {
        t.Fatalf("copy: %v", err)
    }

    // Verify outfile content
    got, err := os.ReadFile(outPath)
    if err != nil {
        t.Fatalf("read outfile: %v", err)
    }
    if string(got) != want {
        t.Fatalf("mismatch: got %q want %q", string(got), want)
    }
}

