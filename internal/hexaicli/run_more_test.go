package hexaicli

import (
	"bytes"
	"context"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type streamClient struct{}

func (streamClient) Chat(ctx context.Context, msgs []llm.Message, opts ...llm.RequestOption) (string, error) {
	return "X", nil
}
func (streamClient) Name() string         { return "fake" }
func (streamClient) DefaultModel() string { return "m" }
func (streamClient) ChatStream(ctx context.Context, msgs []llm.Message, onDelta func(string), opts ...llm.RequestOption) error {
	onDelta("A")
	onDelta("B")
	return nil
}

func TestRunChat_Streaming(t *testing.T) {
	var out, errw bytes.Buffer
	input := "hello"
	msgs := []llm.Message{{Role: "user", Content: input}}
	req := requestArgs{model: "m"}
	if err := runChat(context.Background(), streamClient{}, req, msgs, input, &out, &errw); err != nil {
		t.Fatalf("runChat failed: %v", err)
	}
	if out.String() != "AB" {
		t.Fatalf("unexpected stream output: %q", out.String())
	}
}

func TestBuildMessagesFromConfig(t *testing.T) {
	cfg := appconfig.App{PromptConfig: appconfig.PromptConfig{PromptCLIDefaultSystem: "DEF", PromptCLIExplainSystem: "EXP"}}
	msgs := buildMessagesFromConfig(cfg, "tell me")
	if msgs[0].Content != "DEF" {
		t.Fatalf("default system wrong: %q", msgs[0].Content)
	}
	msgs = buildMessagesFromConfig(cfg, "please explain")
	if msgs[0].Content != "EXP" {
		t.Fatalf("explain system wrong: %q", msgs[0].Content)
	}
}
