package hexaiaction

import (
	"bytes"
	"context"
	"log"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type llmFake struct{}

func (llmFake) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "OK", nil
}
func (llmFake) Name() string         { return "fake" }
func (llmFake) DefaultModel() string { return "model" }

type recordingActionStatusSink struct {
	provider string
	model    string
}

func (s *recordingActionStatusSink) SetLLMStart(provider, model string) error {
	s.provider = provider
	s.model = model
	return nil
}

func TestRun_WithSeams_SkipAndRewrite(t *testing.T) {
	// Isolate from user config to avoid environment-dependent behavior/logging.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := NewRunner()
	runner.newClient = func(_ appconfig.App) (actionClient, error) { return llmFake{}, nil }
	// 1) Skip -> echoes selection
	runner.chooseAction = func(_ appconfig.App) (actionChoice, error) {
		return actionChoice{kind: ActionSkip}, nil
	}
	var out bytes.Buffer
	var errBuf bytes.Buffer
	in := bytes.NewBufferString("some code")
	if err := runner.Run(context.Background(), in, &out, &errBuf); err != nil {
		t.Fatalf("Run skip: %v", err)
	}
	if out.String() != "some code" {
		t.Fatalf("skip out: %q", out.String())
	}
	// 2) Rewrite -> requires inline instruction
	runner.chooseAction = func(_ appconfig.App) (actionChoice, error) {
		return actionChoice{kind: ActionRewrite}, nil
	}
	out.Reset()
	in = bytes.NewBufferString(";upper;\nhello")
	if err := runner.Run(context.Background(), in, &out, &errBuf); err != nil {
		t.Fatalf("Run rewrite: %v", err)
	}
	if out.String() == "" {
		t.Fatalf("expected non-empty rewrite output")
	}
}

func TestRun_WithInjectedConfigAndStatusSink(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	sink := &recordingActionStatusSink{}
	runner := NewRunner()
	runner.loadConfig = func(context.Context, *log.Logger) appconfig.App { return appconfig.Load(nil) }
	runner.newClient = func(_ appconfig.App) (actionClient, error) { return llmFake{}, nil }
	runner.chooseAction = func(_ appconfig.App) (actionChoice, error) {
		return actionChoice{kind: ActionSkip}, nil
	}
	runner.statusSink = sink

	var out bytes.Buffer
	if err := runner.Run(context.Background(), bytes.NewBufferString("selection"), &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.String() != "selection" {
		t.Fatalf("unexpected output %q", out.String())
	}
	if sink.provider != "fake" || sink.model == "" {
		t.Fatalf("unexpected status sink values: provider=%q model=%q", sink.provider, sink.model)
	}
}
