package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
)

func TestMain_Version(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"hexai", "-version"}
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	main()
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe: %v", err)
	}
	b, _ := io.ReadAll(r)
	if len(b) == 0 {
		t.Fatalf("expected version output")
	}
}

func TestSplitConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantRest []string
	}{
		{
			name:     "no args",
			args:     nil,
			wantPath: "",
			wantRest: []string{},
		},
		{
			name:     "no config flag",
			args:     []string{"-version", "hello"},
			wantPath: "",
			wantRest: []string{"-version", "hello"},
		},
		{
			name:     "--config with separate value",
			args:     []string{"--config", "/tmp/cfg.toml", "-version"},
			wantPath: "/tmp/cfg.toml",
			wantRest: []string{"-version"},
		},
		{
			name:     "-config with separate value",
			args:     []string{"-config", "/tmp/cfg.toml", "extra"},
			wantPath: "/tmp/cfg.toml",
			wantRest: []string{"extra"},
		},
		{
			name:     "--config= form",
			args:     []string{"--config=/tmp/cfg.toml", "extra"},
			wantPath: "/tmp/cfg.toml",
			wantRest: []string{"extra"},
		},
		{
			name:     "-config= form",
			args:     []string{"-config=/tmp/cfg.toml", "extra"},
			wantPath: "/tmp/cfg.toml",
			wantRest: []string{"extra"},
		},
		{
			name:     "--config as last arg without value",
			args:     []string{"extra", "--config"},
			wantPath: "",
			wantRest: []string{"extra"},
		},
		{
			name:     "-config as last arg without value",
			args:     []string{"-config"},
			wantPath: "",
			wantRest: []string{},
		},
		{
			name:     "path with whitespace is trimmed",
			args:     []string{"--config", "  /tmp/cfg.toml  "},
			wantPath: "/tmp/cfg.toml",
			wantRest: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotRest := splitConfigPath(tc.args)
			if gotPath != tc.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
			}
			if len(gotRest) != len(tc.wantRest) {
				t.Fatalf("rest len = %d, want %d; got %v", len(gotRest), len(tc.wantRest), gotRest)
			}
			for i := range gotRest {
				if gotRest[i] != tc.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q", i, gotRest[i], tc.wantRest[i])
				}
			}
		})
	}
}

func TestPickDefaultModel(t *testing.T) {
	cfg := appconfig.App{
		ProviderConfig: appconfig.ProviderConfig{
			OllamaModel:    "llama3",
			AnthropicModel: "claude-sonnet",
			OpenAIModel:    "gpt-4o",
		},
	}
	tests := []struct {
		provider string
		want     string
	}{
		{"ollama", "llama3"},
		{"Ollama", "llama3"},
		{"  OLLAMA  ", "llama3"},
		{"anthropic", "claude-sonnet"},
		{"Anthropic", "claude-sonnet"},
		{"openai", "gpt-4o"},
		{"unknown-provider", "gpt-4o"},
		{"", "gpt-4o"},
	}
	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			got := pickDefaultModel(cfg, tc.provider)
			if got != tc.want {
				t.Errorf("pickDefaultModel(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

func TestMain_TPSSimulation(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"hexai", "--tps-simulation=1000000", "simulated", "output"}
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	main()
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe: %v", err)
	}
	b, _ := io.ReadAll(r)
	if !strings.Contains(string(b), "simulated output") {
		t.Fatalf("expected simulation output, got %q", string(b))
	}
}
