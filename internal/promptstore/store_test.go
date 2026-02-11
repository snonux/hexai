// Summary: Tests for prompt store operations
package promptstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONLStore_Get(t *testing.T) {
	t.Run("get existing prompt", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewJSONLStore(tmpDir)
		if err != nil {
			t.Fatalf("NewJSONLStore() error = %v", err)
		}

		// Create a test prompt first
		testPrompt := &Prompt{
			Name:        "test_prompt",
			Title:       "Test Prompt",
			Description: "A test prompt",
			Messages:    []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Test"}}},
			Created:     time.Now(),
			Updated:     time.Now(),
		}
		if err := store.Create(testPrompt); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Now get it
		got, err := store.Get("test_prompt")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Name != "test_prompt" {
			t.Errorf("Get() name = %v, want test_prompt", got.Name)
		}
		if got.Title != "Test Prompt" {
			t.Errorf("Get() title = %v, want Test Prompt", got.Title)
		}
	})

	t.Run("prompt not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewJSONLStore(tmpDir)
		if err != nil {
			t.Fatalf("NewJSONLStore() error = %v", err)
		}

		_, err = store.Get("nonexistent")
		if err == nil {
			t.Error("Get() expected error for nonexistent prompt, got nil")
		}
	})
}

func TestJSONLStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Create test prompts
	for i := 0; i < 7; i++ {
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
	}

	// List all prompts
	prompts, cursor, err := store.List("", 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have all prompts (7 user + 2 built-ins)
	if len(prompts) != 9 {
		t.Errorf("List() got %d prompts, want 9 (7 user + 2 built-ins)", len(prompts))
	}

	// No cursor for full list
	if cursor != "" {
		t.Errorf("List() cursor = %v, want empty", cursor)
	}

	// Test pagination
	prompts1, cursor1, err := store.List("", 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(prompts1) != 3 {
		t.Errorf("List() got %d prompts, want 3", len(prompts1))
	}
	if cursor1 == "" {
		t.Error("List() expected cursor, got empty")
	}

	// Get next page
	prompts2, cursor2, err := store.List(cursor1, 3)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(prompts2) != 3 {
		t.Errorf("List() got %d prompts on page 2, want 3", len(prompts2))
	}
	if cursor2 == "" {
		t.Error("List() expected cursor2, got empty")
	}
}

func TestJSONLStore_Create(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	now := time.Now()
	prompt := &Prompt{
		Name:        "test_prompt",
		Title:       "Test Prompt",
		Description: "A test prompt",
		Arguments: []PromptArgument{
			{Name: "input", Description: "Test input", Required: true},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Test: {{input}}",
				},
			},
		},
		Tags:    []string{"test"},
		Created: now,
		Updated: now,
	}

	// Create prompt
	if err := store.Create(prompt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify it exists
	got, err := store.Get("test_prompt")
	if err != nil {
		t.Fatalf("Get() after Create() error = %v", err)
	}
	if got.Name != prompt.Name {
		t.Errorf("Get() name = %v, want %v", got.Name, prompt.Name)
	}

	// Try to create duplicate
	err = store.Create(prompt)
	if err == nil {
		t.Error("Create() duplicate should fail")
	}
}

func TestJSONLStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	now := time.Now()
	prompt := &Prompt{
		Name:        "test_update",
		Title:       "Original Title",
		Description: "Original description",
		Messages:    []PromptMessage{},
		Created:     now,
		Updated:     now,
	}

	// Create initial prompt
	if err := store.Create(prompt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update prompt
	prompt.Title = "Updated Title"
	prompt.Description = "Updated description"
	if err := store.Update(prompt); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	got, err := store.Get("test_update")
	if err != nil {
		t.Fatalf("Get() after Update() error = %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("Get() title = %v, want Updated Title", got.Title)
	}
}

func TestJSONLStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	now := time.Now()
	prompt := &Prompt{
		Name:        "test_delete",
		Title:       "Delete Me",
		Description: "Will be deleted",
		Messages:    []PromptMessage{},
		Created:     now,
		Updated:     now,
	}

	// Create prompt
	if err := store.Create(prompt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete prompt
	if err := store.Delete("test_delete"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err = store.Get("test_delete")
	if err == nil {
		t.Error("Get() after Delete() should fail")
	}
}

func TestJSONLStore_SearchByTags(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Create test prompts with tags
	prompt1 := &Prompt{
		Name:     "test1",
		Title:    "Test 1",
		Tags:     []string{"development", "review"},
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Test 1"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	prompt2 := &Prompt{
		Name:     "test2",
		Title:    "Test 2",
		Tags:     []string{"development"},
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Test 2"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	prompt3 := &Prompt{
		Name:     "test3",
		Title:    "Test 3",
		Tags:     []string{"testing"},
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Test 3"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	if err := store.Create(prompt1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Create(prompt2); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Create(prompt3); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Search for development tag
	results, err := store.SearchByTags([]string{"development"})
	if err != nil {
		t.Fatalf("SearchByTags() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("SearchByTags() got %d results, want 2", len(results))
	}

	// Search for multiple tags
	results, err = store.SearchByTags([]string{"development", "review"})
	if err != nil {
		t.Fatalf("SearchByTags() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("SearchByTags() got %d results, want 1", len(results))
	}
	if results[0].Name != "test1" {
		t.Errorf("SearchByTags() got prompt %s, want test1", results[0].Name)
	}
}

func TestJSONLStore_LoadAllPrompts_CodeAndFile(t *testing.T) {
	t.Run("loads from both code (built-ins) and user.jsonl", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewJSONLStore(tmpDir)
		if err != nil {
			t.Fatalf("NewJSONLStore() error = %v", err)
		}

		// Create a user prompt
		userPrompt := &Prompt{
			Name:     "user_test",
			Title:    "User Test",
			Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "User test"}}},
			Created:  time.Now(),
			Updated:  time.Now(),
		}
		if err := store.Create(userPrompt); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// List all prompts (should include both built-ins from code and user prompt)
		prompts, _, err := store.List("", 100)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		// Check we have at least the built-ins (save_prompt, update_prompt) + user prompt
		if len(prompts) < 3 {
			t.Errorf("List() got %d prompts, want at least 3 (2 built-ins + 1 user)", len(prompts))
		}

		// Verify built-in prompts are present
		hasBuiltIn := false
		hasUser := false
		for _, p := range prompts {
			if p.Name == "save_prompt" || p.Name == "update_prompt" {
				hasBuiltIn = true
			}
			if p.Name == "user_test" {
				hasUser = true
			}
		}

		if !hasBuiltIn {
			t.Error("List() missing built-in prompts (save_prompt or update_prompt)")
		}
		if !hasUser {
			t.Error("List() missing user prompt (user_test)")
		}
	})

	t.Run("built-ins take precedence over user prompts with same name", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewJSONLStore(tmpDir)
		if err != nil {
			t.Fatalf("NewJSONLStore() error = %v", err)
		}

		// Get the built-in save_prompt (from code)
		builtIn, err := store.Get("save_prompt")
		if err != nil {
			t.Fatalf("Get(save_prompt) error = %v", err)
		}
		originalTitle := builtIn.Title

		// Manually add a conflicting prompt to user.jsonl (bypass protection)
		// This simulates a conflict scenario
		jStore := store.(*JSONLStore)
		conflictPrompt := &Prompt{
			Name:     "save_prompt",
			Title:    "Conflicting Title",
			Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Conflict"}}},
			Created:  time.Now(),
			Updated:  time.Now(),
		}
		// Write directly to user.jsonl without using Create (which has protection)
		userPrompts := []Prompt{*conflictPrompt}
		if err := jStore.writePromptsToFile("user.jsonl", userPrompts); err != nil {
			t.Fatalf("writePromptsToFile() error = %v", err)
		}

		// Get save_prompt again - should return built-in from code, not user version
		result, err := store.Get("save_prompt")
		if err != nil {
			t.Fatalf("Get(save_prompt) after conflict error = %v", err)
		}

		if result.Title != originalTitle {
			t.Errorf("Get(save_prompt) title = %v, want %v (built-in should take precedence)", result.Title, originalTitle)
		}
	})
}

func TestJSONLStore_BuiltInProtection_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Get the built-in save_prompt
	builtIn, err := store.Get("save_prompt")
	if err != nil {
		t.Fatalf("Get(save_prompt) error = %v", err)
	}

	// Try to update it
	builtIn.Title = "Modified Title"
	err = store.Update(builtIn)

	// Should fail with clear error message
	if err == nil {
		t.Fatal("Update() on built-in prompt should fail")
	}

	expectedMsg := "cannot update built-in prompt: save_prompt"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Update() error = %v, want error containing %q", err, expectedMsg)
	}
}

func TestJSONLStore_BuiltInProtection_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Verify save_prompt exists
	_, err = store.Get("save_prompt")
	if err != nil {
		t.Fatalf("Get(save_prompt) error = %v (built-in should exist)", err)
	}

	// Try to delete it
	err = store.Delete("save_prompt")

	// Should fail with clear error message
	if err == nil {
		t.Fatal("Delete() on built-in prompt should fail")
	}

	expectedMsg := "cannot delete built-in prompt: save_prompt"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Delete() error = %v, want error containing %q", err, expectedMsg)
	}

	// Verify it still exists
	_, err = store.Get("save_prompt")
	if err != nil {
		t.Error("Get(save_prompt) after failed delete should still work")
	}
}

func TestJSONLStore_Create_NameConflictWithBuiltIn(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Try to create a prompt with the same name as a built-in
	conflictPrompt := &Prompt{
		Name:     "save_prompt",
		Title:    "My Custom Save",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "Custom"}}},
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	err = store.Create(conflictPrompt)

	// Should fail with clear error message
	if err == nil {
		t.Fatal("Create() with built-in name should fail")
	}

	expectedMsg := "prompt already exists: save_prompt"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Create() error = %v, want error containing %q", err, expectedMsg)
	}
}

func TestJSONLStore_BuiltInPromptsLoadedFromCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store (built-ins loaded from code, not from file)
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Verify save_prompt exists (loaded from code)
	savePrompt, err := store.Get("save_prompt")
	if err != nil {
		t.Fatalf("Get(save_prompt) error = %v (should be loaded from code)", err)
	}
	if savePrompt.Name != "save_prompt" {
		t.Errorf("Get(save_prompt) name = %v, want save_prompt", savePrompt.Name)
	}

	// Verify update_prompt exists (loaded from code)
	updatePrompt, err := store.Get("update_prompt")
	if err != nil {
		t.Fatalf("Get(update_prompt) error = %v (should be loaded from code)", err)
	}
	if updatePrompt.Name != "update_prompt" {
		t.Errorf("Get(update_prompt) name = %v, want update_prompt", updatePrompt.Name)
	}

	// Verify both have correct tags
	if !containsTag(savePrompt.Tags, "meta") {
		t.Error("save_prompt should have 'meta' tag")
	}
	if !containsTag(updatePrompt.Tags, "meta") {
		t.Error("update_prompt should have 'meta' tag")
	}

	// Verify no default.jsonl file was created
	defaultPath := filepath.Join(tmpDir, "default.jsonl")
	if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
		t.Error("default.jsonl should not be created (built-ins loaded from code)")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to check if a slice contains a tag
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
