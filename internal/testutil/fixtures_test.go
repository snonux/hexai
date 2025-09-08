package testutil

import "testing"

func TestFixtures_ZeroCovTargets(t *testing.T) {
	if MarkdownCodeFence() == "" {
		t.Fatal("MarkdownCodeFence empty")
	}
	if MalformedJSON() == "" {
		t.Fatal("MalformedJSON empty")
	}
}
