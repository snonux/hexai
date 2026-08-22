package lsp

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/ignore"
)

// newIgnoreTestServer creates a Server with an ignore checker configured
// from the given gitRoot and extra patterns.
func newIgnoreTestServer(gitRoot string, useGI bool, extra []string, notifyIgnored *bool) *Server {
	cfg := appconfig.App{
		CoreConfig: appconfig.CoreConfig{
			InlineOpen:   ">!",
			InlineClose:  ">",
			ChatSuffix:   ">",
			ChatPrefixes: []string{"?", "!", ":", ";"},
		},
		FeatureConfig: appconfig.FeatureConfig{IgnoreConfig: appconfig.IgnoreConfig{IgnoreLSPNotify: notifyIgnored}},
	}
	s := &Server{
		logger:              log.New(io.Discard, "", 0),
		docs:                make(map[string]*document),
		cfg:                 cfg,
		codeActionSubsystem: codeActionSubsystem{llmClientRegistry: llmClientRegistry{}},
		ignoreChecker:       ignore.New(gitRoot, useGI, extra),
	}
	s.chat = newChatService(s)
	s.completion = newCompletionService(s)
	return s
}

func boolPtr(b bool) *bool { return &b }

func TestHandleCompletion_IgnoredFile_WithNotify(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newIgnoreTestServer(dir, true, nil, boolPtr(true))

	uri := "file://" + filepath.Join(dir, "debug.log")
	ignored, _ := s.isFileIgnored(uri)
	if !ignored {
		t.Fatal("expected file to be ignored")
	}

	// Verify notify is enabled
	if !s.ignoreLSPNotifyEnabled() {
		t.Fatal("expected LSP notify enabled")
	}
}

func TestHandleCompletion_IgnoredFile_WithoutNotify(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newIgnoreTestServer(dir, true, nil, boolPtr(false))

	uri := "file://" + filepath.Join(dir, "debug.log")

	ignored, _ := s.isFileIgnored(uri)
	if !ignored {
		t.Fatal("expected file to be ignored")
	}
	if s.ignoreLSPNotifyEnabled() {
		t.Fatal("expected LSP notify disabled")
	}
}

func TestHandleCompletion_NonIgnoredFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newIgnoreTestServer(dir, true, nil, boolPtr(true))

	uri := "file://" + filepath.Join(dir, "main.go")

	ignored, _ := s.isFileIgnored(uri)
	if ignored {
		t.Fatal("main.go should not be ignored")
	}
}

func TestHandleCodeAction_IgnoredFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newIgnoreTestServer(dir, true, nil, nil)

	uri := "file://" + filepath.Join(dir, "app.log")

	ignored, reason := s.isFileIgnored(uri)
	if !ignored {
		t.Fatal("expected app.log to be ignored")
	}
	if reason != "matched .gitignore pattern" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestHandleDidOpen_IgnoredFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newIgnoreTestServer(dir, true, nil, nil)
	uri := "file://" + filepath.Join(dir, "app.log")

	// Simulate didOpen — document should be stored even if ignored
	s.setDocument(uri, "log content")
	d := s.getDocument(uri)
	if d == nil {
		t.Fatal("document should be stored even for ignored files")
	}

	ignored, _ := s.isFileIgnored(uri)
	if !ignored {
		t.Fatal("expected app.log to be ignored")
	}
}

func TestIsFileIgnored_NoChecker(t *testing.T) {
	s := &Server{
		logger:              log.New(io.Discard, "", 0),
		docs:                make(map[string]*document),
		codeActionSubsystem: codeActionSubsystem{llmClientRegistry: llmClientRegistry{}},
		// ignoreChecker is nil
	}

	ignored, reason := s.isFileIgnored("file:///some/file.log")
	if ignored {
		t.Fatal("nil checker should not ignore anything")
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}

func TestUriToPath(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///home/user/file.go", "/home/user/file.go"},
		{"file:///tmp/test.log", "/tmp/test.log"},
		{"", ""},
		{"https://example.com", ""},
		{"file:///path/with%20space/file.go", "/path/with space/file.go"},
	}
	for _, tc := range tests {
		got := uriToPath(tc.uri)
		if got != tc.want {
			t.Errorf("uriToPath(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

func TestIgnoreLSPNotifyEnabled_NilConfig(t *testing.T) {
	// When IgnoreLSPNotify is nil, defaults to true
	s := &Server{
		logger:              log.New(io.Discard, "", 0),
		docs:                make(map[string]*document),
		codeActionSubsystem: codeActionSubsystem{llmClientRegistry: llmClientRegistry{}},
		cfg:                 appconfig.App{},
	}
	if !s.ignoreLSPNotifyEnabled() {
		t.Error("expected notify enabled when config is nil (default)")
	}
}
