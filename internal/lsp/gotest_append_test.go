package lsp

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestResolveGoTest_AppendsToExisting(t *testing.T) {
    s := newTestServer()
    dir := t.TempDir()
    src := filepath.Join(dir, "m.go")
    uri := "file://" + src
    s.setDocument(uri, "package m\n\nfunc F(){}\n")
    // Create existing test file
    testPath := filepath.Join(dir, "m_test.go")
    if err := os.WriteFile(testPath, []byte("package m\n\nimport \"testing\"\n\n"), 0o644); err != nil { t.Fatal(err) }
    // LLM path to increase generateGoTestFunction coverage
    s.llmClient = fakeLLM{resp: "func TestF(t *testing.T) {}"}
    we, testURI, jump, ok := s.resolveGoTest(uri, Position{Line:2})
    if !ok || len(we.Changes) == 0 { t.Fatalf("expected append edit") }
    if !strings.HasSuffix(testURI, "_test.go") { t.Fatalf("unexpected uri: %s", testURI) }
    edits := we.Changes[testURI]
    if len(edits) != 1 || !strings.Contains(edits[0].NewText, "TestF") { t.Fatalf("expected append with TestF") }
    if jump.Start.Line < 0 { t.Fatalf("expected non-negative jump line") }
}

