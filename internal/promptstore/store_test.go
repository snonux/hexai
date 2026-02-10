// Summary: Tests for prompt store operations
package promptstore

import (
	"fmt"
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

	// Should have all prompts
	if len(prompts) != 7 {
		t.Errorf("List() got %d prompts, want 7", len(prompts))
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
