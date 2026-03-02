package hexaiaction

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

// Covers the early error path in Run when no API key is available for the default provider.
func TestRun_MissingAPIKey(t *testing.T) {
	// Ensure no provider API keys in env
	for _, k := range []string{"HEXAI_OPENAI_API_KEY", "OPENAI_API_KEY"} {
		t.Setenv(k, "")
	}
	// Provide minimal stdin to get past empty input check (if reached)
	in := bytes.NewBufferString("some selection text")
	var out bytes.Buffer
	var errBuf bytes.Buffer
	// Expect an error due to missing OPENAI_API_KEY (default provider is openai)
	if err := Run(context.Background(), in, &out, &errBuf); err == nil {
		t.Fatal("expected error when API key is missing")
	}
	_ = os.Stderr
}

type stubChatDoer struct {
	calls int
	msgs  [][]llm.Message
}

func (s *stubChatDoer) Chat(ctx context.Context, msgs []llm.Message, opts ...llm.RequestOption) (string, error) {
	s.calls++
	s.msgs = append(s.msgs, msgs)
	return "ok", nil
}

func (s *stubChatDoer) DefaultModel() string { return "stub" }

func TestHandleDiagnosticsActionInvokesLLM(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "0")
	parts := InputParts{Diagnostics: []string{"warn1"}, Selection: "code"}
	client := &stubChatDoer{}
	cfg := appconfig.Load(nil)
	if _, err := handleDiagnosticsAction(context.Background(), parts, cfg, client); err != nil {
		t.Fatalf("handleDiagnosticsAction: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected 1 chat call, got %d", client.calls)
	}
	found := false
	for _, msg := range client.msgs[0] {
		if msg.Role == "user" && strings.Contains(msg.Content, "warn1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected diagnostics content in message: %#v", client.msgs[0])
	}
}

func TestHandleSimplifyActionPassesSelection(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "0")
	parts := InputParts{Selection: "value := 1"}
	client := &stubChatDoer{}
	cfg := appconfig.Load(nil)
	if _, err := handleSimplifyAction(context.Background(), parts, cfg, client); err != nil {
		t.Fatalf("handleSimplifyAction: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected single chat invocation, got %d", client.calls)
	}
	seen := false
	for _, msg := range client.msgs[0] {
		if msg.Role == "user" && strings.Contains(msg.Content, "value := 1") {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("expected selection echoed in prompt: %#v", client.msgs[0])
	}
}

func TestHandleCustomActionUsesProvidedCustom(t *testing.T) {
	t.Setenv("HEXAI_TMUX_STATUS", "0")
	sel := appconfig.CustomAction{ID: "custom", Title: "Do", Instruction: "do it"}
	parts := InputParts{Selection: "text"}
	client := &stubChatDoer{}
	cfg := appconfig.Load(nil)
	if _, err := handleCustomAction(context.Background(), parts, cfg, client, &sel); err != nil {
		t.Fatalf("handleCustomAction: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected custom action to invoke chat, got %d calls", client.calls)
	}
}
