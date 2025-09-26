package hexaiaction

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type simpleDoer struct{ s string }

func (d simpleDoer) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return d.s, nil
}
func (d simpleDoer) DefaultModel() string { return "m" }

func ptrFloat(v float64) *float64 {
	x := v
	return &x
}

func TestRunOnce_StripsFences(t *testing.T) {
	got, err := runOnce(context.Background(), simpleDoer{"```\nok\n```"}, "SYS", "USER")
	if err != nil {
		t.Fatalf("runOnce: %v", err)
	}
	if strings.TrimSpace(got) != "ok" {
		t.Fatalf("got %q", got)
	}
}

func TestReqOptsFrom_Override(t *testing.T) {
	cfg := appconfig.App{
		MaxTokens:         123,
		Provider:          "openai",
		CopilotModel:      "gpt-4o",
		CodeActionConfigs: []appconfig.SurfaceConfig{{Provider: "copilot", Model: "override", Temperature: ptrFloat(0.6)}},
	}
	req := reqOptsFrom(cfg)
	if req.model != "override" {
		t.Fatalf("expected override model, got %q", req.model)
	}
	var opts llm.Options
	for _, o := range req.options {
		o(&opts)
	}
	if opts.MaxTokens != 123 || opts.Model != "override" || opts.Temperature != 0.6 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestReqOptsFrom_Gpt5Temp(t *testing.T) {
	cfg := appconfig.App{Provider: "openai", CodingTemperature: ptrFloat(0.2), OpenAIModel: "gpt-5.0"}
	req := reqOptsFrom(cfg)
	var opts llm.Options
	for _, o := range req.options {
		o(&opts)
	}
	if opts.Temperature != 1.0 {
		t.Fatalf("expected gpt-5 temp adjustment to 1.0, got %v", opts.Temperature)
	}
}
