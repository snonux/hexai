package lsp

import (
	"context"
	"testing"

	"github.com/snonux/hexai/internal/llm"
)

type fakeCompleterOk struct{}

func (fakeCompleterOk) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", nil
}
func (fakeCompleterOk) Name() string         { return "prov" }
func (fakeCompleterOk) DefaultModel() string { return "m" }
func (fakeCompleterOk) CodeCompletion(context.Context, string, string, int, string, float64) ([]string, error) {
	return []string{"SUGG"}, nil
}

func TestProviderNativeCompletion_Success(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeCompleterOk{}
	spec := s.buildRequestSpec(surfaceCompletion)
	// current line with dot trigger; position after dot
	current := "fmt."
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x.go"}, Position: Position{Line: 0, Character: len(current)}}
	plan := completionPlan{current: current, params: p, funcCtx: "func f(){}", docStr: "doc", cacheKey: "k"}
	items, ok := s.completion.tryProviderNativeCompletion(context.Background(), plan, spec, s.llmClient, "0000")
	if !ok || len(items) == 0 {
		t.Fatalf("expected provider-native items")
	}
	if items[0].Label == "" || items[0].TextEdit == nil {
		t.Fatalf("unexpected completion item: %+v", items[0])
	}
}

type fakeCompleterIndent struct{}

func (fakeCompleterIndent) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", nil
}
func (fakeCompleterIndent) Name() string         { return "prov" }
func (fakeCompleterIndent) DefaultModel() string { return "m" }
func (fakeCompleterIndent) CodeCompletion(context.Context, string, string, int, string, float64) ([]string, error) {
	return []string{"a\nb"}, nil
}

func TestProviderNativeCompletion_IndentWithDoubleOpen(t *testing.T) {
	s := newTestServer()
	s.llmClient = fakeCompleterIndent{}
	spec := s.buildRequestSpec(surfaceCompletion)
	current := "  >>!do>" // leading indent + double-open marker
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x.go"}, Position: Position{Line: 0, Character: len(current)}}
	plan := completionPlan{current: current, params: p, funcCtx: "func f(){}", docStr: "doc", cacheKey: "k"}
	items, ok := s.completion.tryProviderNativeCompletion(context.Background(), plan, spec, s.llmClient, "0000")
	if !ok || len(items) == 0 {
		t.Fatalf("expected provider-native items")
	}
	if items[0].TextEdit == nil {
		t.Fatalf("expected text edit")
	}
	if got := items[0].TextEdit.NewText; len(got) < 2 || got[:2] != "  " {
		t.Fatalf("expected indentation applied, got %q", got)
	}
}

type fakeCompleterCapture struct{ lastPrompt string }

func (fakeCompleterCapture) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", nil
}
func (fakeCompleterCapture) Name() string         { return "prov" }
func (fakeCompleterCapture) DefaultModel() string { return "m" }
func (f *fakeCompleterCapture) CodeCompletion(_ context.Context, prompt string, suffix string, n int, language string, temperature float64) ([]string, error) {
	f.lastPrompt = prompt
	return []string{"SUG"}, nil
}

func TestProviderNativeCompletion_UsesPromptTemplate(t *testing.T) {
	s := newTestServer()
	cap := &fakeCompleterCapture{}
	s.llmClient = cap
	cfg := s.cfg
	cfg.PromptNativeCompletion = "NATIVE {{path}} {{before}}"
	s.cfg = cfg
	spec := s.buildRequestSpec(surfaceCompletion)
	uri := "file:///x.go"
	s.setDocument(uri, "AAA\nBBB\nCCC")
	current := "fmt."
	// Cursor at line 1, char 1 -> before should be "AAA\nB"
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 1, Character: 1}}
	plan := completionPlan{current: current, params: p, funcCtx: "func f(){}", docStr: "doc", cacheKey: "k"}
	if _, ok := s.completion.tryProviderNativeCompletion(context.Background(), plan, spec, s.llmClient, "0000"); !ok {
		t.Fatalf("expected provider-native path")
	}
	if cap.lastPrompt == "" {
		t.Fatalf("expected captured prompt")
	}
	if cap.lastPrompt != "NATIVE /x.go AAA\nB" {
		t.Fatalf("unexpected prompt: %q", cap.lastPrompt)
	}
}
