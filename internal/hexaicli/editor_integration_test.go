package hexaicli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llm"
)

type cliFake struct{}

func (cliFake) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "OUT", nil
}
func (cliFake) Name() string         { return "fake" }
func (cliFake) DefaultModel() string { return "m" }
func (cliFake) CodeCompletion(context.Context, string, string, int, string, float64) ([]string, error) {
	return nil, nil
}

func TestRun_NoArgs_OpensEditor(t *testing.T) {
	// Seam: fake client and editor
	oldNew := newClientFromApp
	newClientFromApp = func(_ appconfig.App) (llm.Client, error) { return cliFake{}, nil }
	t.Cleanup(func() { newClientFromApp = oldNew })
	oldRun := editor.RunEditor
	editor.RunEditor = func(_ context.Context, _ string, path string) error {
		return os.WriteFile(path, []byte("PROMPT"), 0o600)
	}
	t.Cleanup(func() { editor.RunEditor = oldRun })
	t.Setenv("HEXAI_EDITOR", "dummy")

	// Provide stdin selection
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), nil, bytes.NewBufferString("SELECTION"), &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.String() == "" {
		t.Fatalf("expected some output")
	}
}

func TestRun_WithArgs_DoesNotOpenEditor(t *testing.T) {
	// Provide args; still use fake client
	oldNew := newClientFromApp
	newClientFromApp = func(_ appconfig.App) (llm.Client, error) { return cliFake{}, nil }
	t.Cleanup(func() { newClientFromApp = oldNew })
	// Stub editor and detect if called (should not be)
	called := false
	oldRun := editor.RunEditor
	editor.RunEditor = func(_ context.Context, _ string, _ string) error { called = true; return nil }
	t.Cleanup(func() { editor.RunEditor = oldRun })
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"ARG"}, bytes.NewBufferString("SEL"), &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called {
		t.Fatalf("editor should not be invoked when args provided")
	}
}
