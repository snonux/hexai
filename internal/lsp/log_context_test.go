package lsp

import (
    "io"
    "log"
    "testing"
)

func TestLogCompletionContext(t *testing.T) {
    s := newTestServer()
    s.logger = log.New(io.Discard, "", 0)
    p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x"}, Position: Position{Line:1, Character:2}}
    s.logCompletionContext(p, "a", "b", "c", "f")
}

