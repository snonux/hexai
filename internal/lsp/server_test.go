package lsp

import (
	"context"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/runtimeconfig"
)

func TestPromptSetUsesConfigStoreSnapshot(t *testing.T) {
	s := newTestServer()
	initial := appconfig.App{MaxTokens: 77}
	store := runtimeconfig.New(initial)
	s.configStore = store

	got := s.promptSet()
	if got.MaxTokens != 77 {
		t.Fatalf("expected initial snapshot, got %+v", got)
	}

	updated := initial
	updated.MaxTokens = 42
	store.Set(updated)

	got = s.promptSet()
	if got.MaxTokens != 42 {
		t.Fatalf("expected updated snapshot, got %+v", got)
	}
}

func TestChatConfigRespectsExplicitEmptySuffix(t *testing.T) {
	s := newTestServer()
	cfg := s.cfg
	cfg.ChatSuffix = ""
	cfg.ChatPrefixes = []string{"#"}
	s.cfg = cfg

	suffix, prefixes, suffixChar := s.chatConfig()
	if suffix != "" {
		t.Fatalf("expected explicit empty suffix, got %q", suffix)
	}
	if len(prefixes) == 0 || prefixes[0] != "#" {
		t.Fatalf("expected custom prefixes, got %v", prefixes)
	}
	if suffixChar != '>' {
		t.Fatalf("expected default suffix char fallback, got %q", suffixChar)
	}
}

func TestChatConfigTrimsWhitespaceSuffix(t *testing.T) {
	s := newTestServer()
	cfg := s.cfg
	cfg.ChatSuffix = "  >>  "
	s.cfg = cfg

	suffix, _, suffixChar := s.chatConfig()
	if suffix != ">>" {
		t.Fatalf("expected trimmed suffix '>>', got %q", suffix)
	}
	if suffixChar != '>' {
		t.Fatalf("expected suffixChar to use trimmed value, got %q", suffixChar)
	}
}

type stubLLMClient struct{}

func (stubLLMClient) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "", nil
}
func (stubLLMClient) Name() string         { return "stub" }
func (stubLLMClient) DefaultModel() string { return "stub-model" }

func TestServerApplyOptions(t *testing.T) {
	s := newTestServer()
	client := stubLLMClient{}
	cfg := appconfig.App{MaxTokens: 88}
	opts := ServerOptions{Config: &cfg, Client: client}
	s.ApplyOptions(opts)
	if s.currentLLMClient() != client {
		t.Fatalf("expected client to be replaced")
	}
	if got := s.currentConfig().MaxTokens; got != 88 {
		t.Fatalf("expected config to update, got %d", got)
	}
}
