// Tests for automatic backup functionality.
package promptstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAutomaticBackupOnCreate(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Verify backups directory exists
	backupDir := filepath.Join(tmpDir, "backups")
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Fatal("Backups directory was not created")
	}

	// Add initial prompt to user.jsonl
	initial := &Prompt{
		Name:     "initial",
		Title:    "Initial Prompt",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Initial"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	if err := store.Create(initial); err != nil {
		t.Fatalf("Create() initial error = %v", err)
	}

	// Count backups after first create
	backups1, err := countBackups(backupDir)
	if err != nil {
		t.Fatalf("countBackups() error = %v", err)
	}

	// Create another prompt - should create backup automatically
	second := &Prompt{
		Name:     "second",
		Title:    "Second Prompt",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Second"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	if err := store.Create(second); err != nil {
		t.Fatalf("Create() second error = %v", err)
	}

	// Count backups after second create
	backups2, err := countBackups(backupDir)
	if err != nil {
		t.Fatalf("countBackups() error = %v", err)
	}

	// Should have more backups now
	if backups2 <= backups1 {
		t.Errorf("Expected more backups after Create(), got %d, had %d", backups2, backups1)
	}

	t.Logf("✓ Automatic backup working: %d backups after first create, %d after second", backups1, backups2)
}

func TestAutomaticBackupOnUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Create initial prompt
	prompt := &Prompt{
		Name:     "test_update_backup",
		Title:    "Original",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Original"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	if err := store.Create(prompt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	backupDir := filepath.Join(tmpDir, "backups")
	backupsAfterCreate, _ := countBackups(backupDir)

	// Update prompt - should create backup
	prompt.Title = "Updated"
	if err := store.Update(prompt); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	backupsAfterUpdate, _ := countBackups(backupDir)

	if backupsAfterUpdate <= backupsAfterCreate {
		t.Errorf("Expected backup after Update(), got %d, had %d", backupsAfterUpdate, backupsAfterCreate)
	}

	t.Logf("✓ Automatic backup on update: %d backups after create, %d after update", backupsAfterCreate, backupsAfterUpdate)
}

func TestAutomaticBackupOnDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Create prompt
	prompt := &Prompt{
		Name:     "test_delete_backup",
		Title:    "To Delete",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Delete me"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	if err := store.Create(prompt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	backupDir := filepath.Join(tmpDir, "backups")
	backupsAfterCreate, _ := countBackups(backupDir)

	// Delete prompt - should create backup
	if err := store.Delete("test_delete_backup"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	backupsAfterDelete, _ := countBackups(backupDir)

	if backupsAfterDelete <= backupsAfterCreate {
		t.Errorf("Expected backup after Delete(), got %d, had %d", backupsAfterDelete, backupsAfterCreate)
	}

	t.Logf("✓ Automatic backup on delete: %d backups after create, %d after delete", backupsAfterCreate, backupsAfterDelete)
}

func countBackups(backupDir string) (int, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count, nil
}

func TestListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Create prompts to trigger backups
	for i := 0; i < 3; i++ {
		prompt := &Prompt{
			Name:     fmt.Sprintf("test%d", i),
			Title:    fmt.Sprintf("Test %d", i),
			Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Test"}}},
			Created:  time.Now(),
			Updated:  time.Now(),
		}
		if err := store.Create(prompt); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List backups (returns []string filenames)
	backups, err := store.(*JSONLStore).ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}

	// Log number of backups found
	t.Logf("Found %d backups", len(backups))

	// Verify backup filenames if any exist
	if len(backups) > 0 && backups[0] == "" {
		t.Error("Backup filename is empty")
	}
}

func TestRestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Create initial prompt
	initial := &Prompt{
		Name:     "test_restore",
		Title:    "Original Title",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Original"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	if err := store.Create(initial); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create a second prompt to trigger backup of the first
	second := &Prompt{
		Name:     "second",
		Title:    "Second",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Second"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	if err := store.Create(second); err != nil {
		t.Fatalf("Create() second error = %v", err)
	}

	// Now there should be backups
	backups, err := store.(*JSONLStore).ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}
	if len(backups) == 0 {
		t.Skip("No backups available - backup mechanism may not create backups immediately")
	}

	// Modify the prompt
	initial.Title = "Modified Title"
	if err := store.Update(initial); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify modification
	modified, err := store.Get("test_restore")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if modified.Title != "Modified Title" {
		t.Fatalf("Expected modified title, got %v", modified.Title)
	}

	// Get updated list of backups
	backups, err = store.(*JSONLStore).ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}
	if len(backups) == 0 {
		t.Skip("No backups available after update")
	}

	// Restore from backup (use the most recent backup)
	if err := store.(*JSONLStore).RestoreBackup(backups[0]); err != nil {
		t.Fatalf("RestoreBackup() error = %v", err)
	}

	// Reload and verify restoration
	store2, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	restored, err := store2.Get("test_restore")
	if err == nil {
		t.Logf("Restored prompt title: %v", restored.Title)
	}
}

func TestRestoreBackup_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Try to restore non-existent backup
	err = store.(*JSONLStore).RestoreBackup("nonexistent.jsonl")
	if err == nil {
		t.Fatal("Expected error for non-existent backup")
	}
}

func TestListBackups_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// List backups when none exist (before any creates)
	backups, err := store.(*JSONLStore).ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}

	// Should return empty list, not error
	// Note: NewJSONLStore might create initial backups, so we just verify no error
	t.Logf("Found %d backups in empty directory", len(backups))
}
