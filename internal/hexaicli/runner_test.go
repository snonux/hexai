package hexaicli

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/stats"
)

type recordingCLIStatusSink struct {
	startProvider string
	startModel    string
	globalCalls   int
}

func (s *recordingCLIStatusSink) SetLLMStart(provider, model string) error {
	s.startProvider = provider
	s.startModel = model
	return nil
}

func (s *recordingCLIStatusSink) SetGlobal(stats.Snapshot, string, string, float64, int64) error {
	s.globalCalls++
	return nil
}

func TestRunner_UsesInjectedDependencies(t *testing.T) {
	sink := &recordingCLIStatusSink{}
	runner := &Runner{
		loadConfig: func(context.Context, *log.Logger) appconfig.App {
			return appconfig.App{
				CoreConfig:   appconfig.CoreConfig{Provider: "openai"},
				PromptConfig: appconfig.PromptConfig{PromptCLIDefaultSystem: "SYS"},
			}
		},
		openEditor: func([]byte) (string, error) { return "PROMPT", nil },
		newClient: func(appconfig.App) (client llm.Client, err error) {
			return &fakeClient{name: "fake", model: "m", resp: "OUT"}, nil
		},
		statusSink: sink,
	}

	var stdout, stderr bytes.Buffer
	if err := runner.Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.String() != "OUT" {
		t.Fatalf("stdout = %q, want OUT", stdout.String())
	}
	if sink.startProvider != "fake" || sink.startModel == "" {
		t.Fatalf("unexpected start status: provider=%q model=%q", sink.startProvider, sink.startModel)
	}
	if sink.globalCalls != 1 {
		t.Fatalf("expected one global status update, got %d", sink.globalCalls)
	}
}
