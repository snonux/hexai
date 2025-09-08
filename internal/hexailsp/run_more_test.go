package hexailsp

import (
	"bytes"
	"io"
	"log"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/lsp"
)

type recRunner struct{ ran bool }

func (r *recRunner) Run() error { r.ran = true; return nil }

func TestRunWithFactory_BuildsOptionsAndClient(t *testing.T) {
	var captured lsp.ServerOptions
	factory := func(r io.Reader, w io.Writer, logger *log.Logger, opts lsp.ServerOptions) ServerRunner {
		captured = opts
		return &recRunner{}
	}
	var in, out bytes.Buffer
	logger := log.New(&out, "", 0)
	cfg := appconfig.Load(logger)
	// Use ollama to avoid API keys
	cfg.Provider = "ollama"
	cfg.MaxTokens = 123
	cfg.PromptCodeActionRewriteSystem = "RSYS"
	cfg.PromptCodeActionRewriteUser = "RUSER"
	if err := RunWithFactory("", &in, &out, logger, cfg, nil, factory); err != nil {
		t.Fatalf("RunWithFactory error: %v", err)
	}
	if captured.MaxTokens != 123 {
		t.Fatalf("opts not applied: %+v", captured)
	}
	if captured.PromptRewriteSystem != "RSYS" || captured.PromptRewriteUser != "RUSER" {
		t.Fatalf("prompts not mapped: %+v", captured)
	}
	if captured.Client == nil {
		t.Fatalf("expected client to be constructed")
	}
}
