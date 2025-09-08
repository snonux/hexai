package lsp

import (
	"bytes"
	"log"
	"testing"
)

func TestHandleInitialized_Logs(t *testing.T) {
	// Minimal server with a logger; just ensure it doesn't panic.
	var buf bytes.Buffer
	s := NewServer(bytes.NewBuffer(nil), &buf, log.New(&buf, "", 0), ServerOptions{})
	s.handleInitialized()
}
