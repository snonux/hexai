package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGoTest_CreatesTestFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.go")
	if err := os.WriteFile(src, []byte("package x\n\nfunc Sum(a,b int) int { return a+b }\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := &Server{} // minimal server with nil llmClient to trigger stub
	initServerDefaults(s)
	uri := "file://" + src
	we, jumpURI, jumpRange, ok := s.resolveGoTest(uri, Position{Line: 2})
	if !ok || jumpURI == "" || jumpRange.Start.Line < 0 {
		t.Fatalf("resolveGoTest failed: ok=%v uri=%q range=%v", ok, jumpURI, jumpRange)
	}
	// Expect documentChanges to include a create and an edit
	if len(we.DocumentChanges) == 0 && len(we.Changes) == 0 {
		t.Fatalf("expected edits to create or append test file: %+v", we)
	}
}
