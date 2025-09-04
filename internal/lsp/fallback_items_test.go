package lsp

import "testing"

func TestFallbackCompletionItems(t *testing.T) {
    s := newTestServer()
    items := s.fallbackCompletionItems("doc")
    if len(items) != 1 || items[0].Label != "hexai-complete" || items[0].InsertText != "hexai" {
        t.Fatalf("unexpected fallback items: %+v", items)
    }
}

