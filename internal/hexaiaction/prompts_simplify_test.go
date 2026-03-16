package hexaiaction

import (
	"context"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type simplifyClient struct{}

func (simplifyClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "OUT", nil
}
func (simplifyClient) DefaultModel() string { return "m" }

func TestRunSimplify_Smoke(t *testing.T) {
	cfg := appconfig.Load(nil)
	out, err := runSimplify(context.Background(), &cfg, simplifyClient{}, "code")
	if err != nil {
		t.Fatalf("runSimplify: %v", err)
	}
	if out == "" {
		t.Fatalf("expected output")
	}
}
