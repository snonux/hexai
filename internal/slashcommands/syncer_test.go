package slashcommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/promptstore"
)

func TestNewSyncer_Disabled(t *testing.T) {
	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{MCPSlashCommandSync: false},
	}

	syncer, err := NewSyncer(cfg)
	if err != nil {
		t.Fatalf("NewSyncer() with disabled sync failed: %v", err)
	}

	if syncer.enabled {
		t.Error("NewSyncer() should create disabled syncer when MCPSlashCommandSync is false")
	}
}

func TestNewSyncer_NoDirectory(t *testing.T) {
	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  "",
		},
	}

	_, err := NewSyncer(cfg)
	if err == nil {
		t.Error("NewSyncer() should fail when directory is not configured")
	}
}

func TestNewSyncer_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-commands")

	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  testDir,
		},
	}

	syncer, err := NewSyncer(cfg)
	if err != nil {
		t.Fatalf("NewSyncer() failed: %v", err)
	}

	if !syncer.enabled {
		t.Error("NewSyncer() should create enabled syncer")
	}

	// Verify directory was created
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("NewSyncer() should create commands directory")
	}
}

func TestNewSyncer_ExpandsHomeDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	home := os.Getenv("HOME")

	// Set temporary HOME for test
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", home)

	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{
			MCPSlashCommandSync: true,
			MCPSlashCommandDir:  "~/test-commands",
		},
	}

	syncer, err := NewSyncer(cfg)
	if err != nil {
		t.Fatalf("NewSyncer() failed: %v", err)
	}

	expectedDir := filepath.Join(tmpDir, "test-commands")
	if syncer.commandsDir != expectedDir {
		t.Errorf("NewSyncer() commandsDir = %q, want %q", syncer.commandsDir, expectedDir)
	}
}

func TestSync_Disabled(t *testing.T) {
	syncer := &Syncer{enabled: false}

	prompt := &promptstore.Prompt{Name: "test"}
	err := syncer.Sync(prompt, OpCreate)
	if err != nil {
		t.Errorf("Sync() with disabled syncer should not error, got: %v", err)
	}
}

func TestSync_Create(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	prompt := &promptstore.Prompt{
		Name:        "test-prompt",
		Title:       "Test Prompt",
		Description: "A test prompt",
		Arguments: []promptstore.PromptArgument{
			{Name: "arg1", Description: "First argument", Required: true},
		},
		Messages: []promptstore.PromptMessage{
			{Role: "user", Content: promptstore.MessageContent{Type: "text", Text: "Hello {{arg1}}"}},
		},
		Tags:    []string{"test", "example"},
		Created: time.Now(),
		Updated: time.Now(),
	}

	err := syncer.Sync(prompt, OpCreate)
	if err != nil {
		t.Fatalf("Sync() failed: %v", err)
	}

	// Verify file was created
	filename := filepath.Join(tmpDir, "hexai-test-prompt.md")
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read synced file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# Test Prompt") {
		t.Error("Synced file should contain prompt title")
	}
	if !strings.Contains(contentStr, "A test prompt") {
		t.Error("Synced file should contain description")
	}
	if !strings.Contains(contentStr, "Hello {{arg1}}") {
		t.Error("Synced file should contain message template")
	}
	if !strings.Contains(contentStr, "test, example") {
		t.Error("Synced file should contain tags")
	}
}

func TestSync_Update(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	// Create initial prompt
	prompt := &promptstore.Prompt{
		Name:     "test-prompt",
		Title:    "Original Title",
		Messages: []promptstore.PromptMessage{{Role: "user", Content: promptstore.MessageContent{Text: "Original"}}},
	}

	if err := syncer.Sync(prompt, OpCreate); err != nil {
		t.Fatalf("Initial Sync() failed: %v", err)
	}

	// Update prompt
	prompt.Title = "Updated Title"
	prompt.Messages[0].Content.Text = "Updated"

	if err := syncer.Sync(prompt, OpUpdate); err != nil {
		t.Fatalf("Update Sync() failed: %v", err)
	}

	// Verify file was updated
	filename := filepath.Join(tmpDir, "hexai-test-prompt.md")
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Updated Title") {
		t.Error("Updated file should contain new title")
	}
	if strings.Contains(contentStr, "Original Title") {
		t.Error("Updated file should not contain old title")
	}
}

func TestDelete_Disabled(t *testing.T) {
	syncer := &Syncer{enabled: false}

	err := syncer.Delete("test")
	if err != nil {
		t.Errorf("Delete() with disabled syncer should not error, got: %v", err)
	}
}

func TestDelete_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	// Create a test file
	filename := filepath.Join(tmpDir, "hexai-test-prompt.md")
	if err := os.WriteFile(filename, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete it
	if err := syncer.Delete("test-prompt"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Error("Delete() should remove the file")
	}
}

func TestDelete_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	// Delete non-existent file (should be idempotent)
	err := syncer.Delete("non-existent")
	if err != nil {
		t.Errorf("Delete() of non-existent file should not error, got: %v", err)
	}
}

func TestSyncAll(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	// Create a mock store with test prompts
	storeDir := t.TempDir()
	store, err := promptstore.NewJSONLStore(storeDir)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	// Add test prompts
	prompts := []*promptstore.Prompt{
		{Name: "prompt1", Title: "Prompt 1", Messages: []promptstore.PromptMessage{{Role: "user", Content: promptstore.MessageContent{Text: "Test 1"}}}},
		{Name: "prompt2", Title: "Prompt 2", Messages: []promptstore.PromptMessage{{Role: "user", Content: promptstore.MessageContent{Text: "Test 2"}}}},
		{Name: "prompt3", Title: "Prompt 3", Messages: []promptstore.PromptMessage{{Role: "user", Content: promptstore.MessageContent{Text: "Test 3"}}}},
	}

	for _, p := range prompts {
		if err := store.Create(p); err != nil {
			t.Fatalf("Failed to create test prompt: %v", err)
		}
	}

	// Sync all
	if err := syncer.SyncAll(store); err != nil {
		t.Fatalf("SyncAll() failed: %v", err)
	}

	// Verify all files were created
	for _, p := range prompts {
		filename := filepath.Join(tmpDir, MakeFilename(p.Name))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("SyncAll() should create file for prompt %q", p.Name)
		}
	}
}

func TestSyncAll_Disabled(t *testing.T) {
	syncer := &Syncer{enabled: false}

	err := syncer.SyncAll(nil)
	if err == nil {
		t.Error("SyncAll() with disabled syncer should return error")
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	filename := filepath.Join(tmpDir, "test-atomic.md")
	content := []byte("test content")

	err := syncer.atomicWrite(filename, content)
	if err != nil {
		t.Fatalf("atomicWrite() failed: %v", err)
	}

	// Verify file was created with correct content
	actual, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(actual) != string(content) {
		t.Errorf("atomicWrite() content = %q, want %q", actual, content)
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Errorf("atomicWrite() left temp file: %s", entry.Name())
		}
	}
}

func TestConcurrentSync(t *testing.T) {
	tmpDir := t.TempDir()

	syncer := &Syncer{
		commandsDir: tmpDir,
		enabled:     true,
	}

	// Sync multiple prompts concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			prompt := &promptstore.Prompt{
				Name:     "prompt-" + string(rune('0'+n)),
				Title:    "Concurrent Test",
				Messages: []promptstore.PromptMessage{{Role: "user", Content: promptstore.MessageContent{Text: "Test"}}},
			}
			if err := syncer.Sync(prompt, OpCreate); err != nil {
				t.Errorf("Concurrent Sync() failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all files were created
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(entries) != 10 {
		t.Errorf("Concurrent sync created %d files, want 10", len(entries))
	}
}
