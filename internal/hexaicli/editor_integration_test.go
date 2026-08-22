package hexaicli

import (
	"bytes"
	"context"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
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
	runner := NewRunner()
	runner.newClient = func(_ appconfig.App) (llm.Client, error) { return cliFake{}, nil }
	runner.openEditor = func(context.Context, []byte) (string, error) { return "PROMPT", nil }

	// Provide stdin selection
	var stdout, stderr bytes.Buffer
	if err := runner.Run(context.Background(), nil, bytes.NewBufferString("SELECTION"), &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.String() == "" {
		t.Fatalf("expected some output")
	}
}

func TestRun_WithArgs_DoesNotOpenEditor(t *testing.T) {
	runner := NewRunner()
	runner.newClient = func(_ appconfig.App) (llm.Client, error) { return cliFake{}, nil }
	called := false
	runner.openEditor = func(context.Context, []byte) (string, error) {
		called = true
		return "", nil
	}
	var stdout, stderr bytes.Buffer
	if err := runner.Run(context.Background(), []string{"ARG"}, bytes.NewBufferString("SEL"), &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called {
		t.Fatalf("editor should not be invoked when args provided")
	}
}
