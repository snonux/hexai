package hexaicli

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/stats"
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
		openEditor: func(context.Context, []byte) (string, error) { return "PROMPT", nil },
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

func TestRunner_ConfigSubcommand_OpensConfigFromContext(t *testing.T) {
	var gotPath string
	runner := NewRunner()
	runner.openConfigEditor = func(_ context.Context, path string) error {
		gotPath = path
		return nil
	}
	cfgFile := filepath.Join(t.TempDir(), "hexai", "config.toml")
	ctx := WithCLIConfigPath(context.Background(), cfgFile)
	if err := runner.Run(ctx, []string{"config"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotPath != cfgFile {
		t.Fatalf("opened %q, want %q", gotPath, cfgFile)
	}
}

func TestRunner_ConfigSubcommand_UsesXDGWhenNoOverride(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	var gotPath string
	runner := NewRunner()
	runner.openConfigEditor = func(_ context.Context, path string) error {
		gotPath = path
		return nil
	}
	want := filepath.Join(xdg, "hexai", "config.toml")
	if err := runner.Run(context.Background(), []string{"config"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotPath != want {
		t.Fatalf("opened %q, want %q", gotPath, want)
	}
}

func TestRunner_ConfigSubcommand_RejectsExtraArgs(t *testing.T) {
	runner := NewRunner()
	var stderr bytes.Buffer
	err := runner.Run(context.Background(), []string{"config", "nope"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("err = %v", err)
	}
}
