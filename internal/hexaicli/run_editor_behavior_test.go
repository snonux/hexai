package hexaicli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llm"
)

// fake client that returns a fixed response
type okClient struct{}

func (okClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "OK", nil
}
func (okClient) Name() string         { return "prov" }
func (okClient) DefaultModel() string { return "m" }

// Ensure that when stdin has content and args are empty, Run does not open the editor.
func TestRun_DoesNotOpenEditorWhenStdinPresent(t *testing.T) {
	// Guard: make editor invocation fatal if called
	oldRunEd := editor.RunEditor
	defer func() { editor.RunEditor = oldRunEd }()
	editor.RunEditor = func(_ context.Context, _ string, _ string) error {
		t.Fatalf("editor should not be invoked when stdin has content")
		return nil
	}

	// Stub client constructor to avoid hitting real providers
	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()
	newClientFromApp = func(_ appconfig.App) (llm.Client, error) { return okClient{}, nil }

	var out, errb bytes.Buffer
	restore, f := setStdin(t, "from-stdin")
	defer restore()
	if err := Run(context.Background(), nil, f, &out, &errb); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "OK") {
		t.Fatalf("expected OK output, got %q", out.String())
	}
}
