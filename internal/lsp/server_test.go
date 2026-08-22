package lsp

import (
	"bytes"
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/runtimeconfig"
)

func TestPromptSetUsesConfigStoreSnapshot(t *testing.T) {
	s := newTestServer()
	initial := appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 77}}
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
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{MaxTokens: 88}}
	opts := ServerOptions{Config: &cfg, Client: client}
	s.ApplyOptions(opts)
	if s.currentLLMClient() != client {
		t.Fatalf("expected client to be replaced")
	}
	if got := s.currentConfig().MaxTokens; got != 88 {
		t.Fatalf("expected config to update, got %d", got)
	}
}

// TestRunReturnsOnEOF verifies the serve loop exits cleanly when the input
// stream reaches EOF, regardless of the parent context being active.
func TestRunReturnsOnEOF(t *testing.T) {
	srv := NewServer(bytes.NewReader(nil), &bytes.Buffer{}, log.New(io.Discard, "", 0), ServerOptions{})
	if err := srv.Run(context.Background()); err != nil {
		t.Fatalf("Run on EOF returned error: %v", err)
	}
}

// TestRunCancelsServerContextOnParentCancel verifies that cancelling the
// caller-provided context propagates into the server's own context (so
// in-flight LLM/network work is aborted) via watchParentContext.
func TestRunCancelsServerContextOnParentCancel(t *testing.T) {
	// A pipe whose write end is never written blocks the serve loop's read,
	// so Run only returns once the parent context cancellation unblocks it by
	// the input being closed.
	pr, pw := io.Pipe()
	srv := NewServer(pr, &bytes.Buffer{}, log.New(io.Discard, "", 0), ServerOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	// Cancel the parent: watchParentContext should cancel the server context.
	cancel()
	// Allow the watcher goroutine to observe cancellation.
	deadline := time.After(2 * time.Second)
	for srv.serverCtx.Err() == nil {
		select {
		case <-deadline:
			t.Fatal("server context was not cancelled after parent cancel")
		default:
		}
	}
	// Unblock the read so Run can return.
	_ = pw.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after input closed")
	}
}

// TestWatchParentContextNilIsNoOp verifies a nil/background context yields a
// no-op stop function and never cancels the server.
func TestWatchParentContextNilIsNoOp(t *testing.T) {
	srv := NewServer(bytes.NewReader(nil), &bytes.Buffer{}, log.New(io.Discard, "", 0), ServerOptions{})
	stop := srv.watchParentContext(nil)
	stop()
	if srv.serverCtx.Err() != nil {
		t.Fatalf("server context should remain active for nil parent ctx")
	}
}

func TestServerStoreAndTakePendingCompletion(t *testing.T) {
	s := newTestServer()
	items := []CompletionItem{{Label: "foo"}}
	s.storePendingCompletion("key", items)
	if len(s.completion.pendingCompletions) != 1 {
		t.Fatalf("expected pending map to be populated")
	}
	items[0].Label = "bar" // ensure copy stored
	got := s.takePendingCompletion("key")
	if len(got) != 1 || got[0].Label != "foo" {
		t.Fatalf("expected preserved copy of completion, got %+v", got)
	}
	if len(s.completion.pendingCompletions) != 0 {
		t.Fatalf("expected pending map to be cleared after take")
	}
}
