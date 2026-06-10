package lsp

import (
	"bytes"
	"io"
	"log"
	"testing"
)

func TestDetectAndHandleChat_NoDoubleAnswer(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	initServerDefaults(s)
	s.llmClient = fakeLLM{resp: "IGNORED"}
	uri := "file:///x.go"
	// Question line with trigger, followed by an existing answer line starting with '>'
	s.setDocument(uri, "What?>\n> already answered\n")
	s.chatSvc().detectAndHandleChat(uri)
	if out.Len() != 0 {
		t.Fatalf("expected no applyEdit request when answer exists; got %d bytes", out.Len())
	}
}
