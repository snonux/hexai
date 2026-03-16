package testutil

import (
	"strings"
	"testing"
)

func TestFixtures_ZeroCovTargets(t *testing.T) {
	if MarkdownCodeFence() == "" {
		t.Fatal("MarkdownCodeFence empty")
	}
	if MalformedJSON() == "" {
		t.Fatal("MalformedJSON empty")
	}
}

func TestMultilineDocBlock(t *testing.T) {
	got := MultilineDocBlock()
	if !strings.Contains(got, "\n") {
		t.Fatal("expected multi-line string")
	}
	if !strings.HasPrefix(got, "//") {
		t.Fatal("expected comment prefix")
	}
	if !strings.Contains(got, "sum") {
		t.Fatal("expected documentation about sum")
	}
}

func TestMultilineChatReply(t *testing.T) {
	got := MultilineChatReply()
	if !strings.Contains(got, "\n") {
		t.Fatal("expected multi-line string")
	}
	if !strings.Contains(got, "Hello") {
		t.Fatal("expected greeting in reply")
	}
}

func TestMultilineFunctionSuggestion(t *testing.T) {
	got := MultilineFunctionSuggestion()
	if !strings.Contains(got, "\n") {
		t.Fatal("expected multi-line string")
	}
	if !strings.Contains(got, "context.Context") {
		t.Fatal("expected context parameter")
	}
	if !strings.Contains(got, "return") {
		t.Fatal("expected return statement")
	}
}
