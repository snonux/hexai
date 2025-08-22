package lsp

import (
    "encoding/json"
    "testing"
)

func TestStripDuplicateGeneralPrefix_ExactOverlap(t *testing.T) {
    prefix := "func New "
    sugg := "func New() *CustData"
    got := stripDuplicateGeneralPrefix(prefix, sugg)
    // We expect the already typed prefix to be removed from the suggestion.
    if got == sugg {
        t.Fatalf("expected duplicate prefix to be stripped; got unchanged: %q", got)
    }
    if got != "() *CustData" {
        t.Fatalf("got %q want %q", got, "() *CustData")
    }
}

func TestStripDuplicateGeneralPrefix_TokenBoundarySuffix(t *testing.T) {
    prefix := "db."
    sugg := "db.Query()"
    got := stripDuplicateGeneralPrefix(prefix, sugg)
    if got != "Query()" {
        t.Fatalf("got %q want %q", got, "Query()")
    }
}

func TestStripDuplicateAssignmentPrefix_AssignAndWalrus(t *testing.T) {
    // walrus
    if out := stripDuplicateAssignmentPrefix("name := ", "name := compute()" ); out != "compute()" {
        t.Fatalf(":= expected compute(), got %q", out)
    }
    // equals
    if out := stripDuplicateAssignmentPrefix("x = ", "x = y+1" ); out != "y+1" {
        t.Fatalf("= expected y+1, got %q", out)
    }
}

func TestTryLLMCompletion_ManualInvokeAfterWhitespace_Allows(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_"}, compCache: make(map[string]string) }
    s.llmClient = fakeLLM{resp: "() *CustData"}
    line := "func fib(i int) " // cursor after space
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://x.go"} }
    // Simulate manual user invocation (TriggerKind=1)
    p.Context = json.RawMessage([]byte(`{"triggerKind":1}`))
    items, ok, busy := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if busy { t.Fatalf("unexpected busy=true") }
    if !ok { t.Fatalf("expected ok=true for manual invoke after whitespace") }
    if len(items) == 0 { t.Fatalf("expected at least one completion item") }
}

func TestTryLLMCompletion_InlineSemicolonPromptAlwaysTriggers(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_"}, compCache: make(map[string]string) }
    s.llmClient = fakeLLM{resp: "replacement"}
    line := "prefix ;do something; suffix"
    // No trigger char immediately before cursor; place cursor at end
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://inline.go"} }
    items, ok, busy := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if busy { t.Fatalf("unexpected busy=true") }
    if !ok || len(items) == 0 { t.Fatalf("expected completion to trigger on inline ;text; prompt") }
}

func TestTryLLMCompletion_DoubleSemicolonEmpty_DoesNotAutoTrigger(t *testing.T) {
    s := &Server{ maxTokens: 32, triggerChars: []string{".", ":", "/", "_"}, compCache: make(map[string]string) }
    fake := &countingLLM{}
    s.llmClient = fake
    line := ";;   " // empty content after ';;' should not force-trigger
    p := CompletionParams{ Position: Position{ Line: 0, Character: len(line) }, TextDocument: TextDocumentIdentifier{URI: "file://empty-inline.go"} }
    items, ok, _ := s.tryLLMCompletion(p, "", line, "", "", "", false, "")
    if !ok { t.Fatalf("expected ok=true for non-trigger path") }
    if len(items) != 0 { t.Fatalf("expected no items when inline ';;' is empty") }
    if fake.calls != 0 { t.Fatalf("LLM should not be called; calls=%d", fake.calls) }
}

func TestHasDoubleSemicolonTrigger_Variants(t *testing.T) {
    if hasDoubleSemicolonTrigger(";;") { t.Fatalf("bare ';;' should not trigger") }
    if hasDoubleSemicolonTrigger(";; ;") { t.Fatalf("';;' followed by space should not trigger") }
    if hasDoubleSemicolonTrigger(";;;") { t.Fatalf("';;;' should not trigger (no content)") }
    if !hasDoubleSemicolonTrigger(";;x;") { t.Fatalf("expected trigger for ';;x;' pattern") }
}
