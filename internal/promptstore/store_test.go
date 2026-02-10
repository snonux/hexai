// Summary: Tests for prompt store operations
package promptstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONLStore_Get(t *testing.T) {
	tests := []struct {
		name       string
		promptName string
		want       *Prompt
		wantErr    bool
	}{
		{
			name:       "get built-in prompt",
			promptName: "code_review",
			want: &Prompt{
				Name:        "code_review",
				Title:       "Request Code Review",
				Description: "Analyzes code quality, style, and suggests improvements",
			},
			wantErr: false,
		},
		{
			name:       "prompt not found",
			promptName: "nonexistent",
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store, err := NewJSONLStore(tmpDir)
			if err != nil {
				t.Fatalf("NewJSONLStore() error = %v", err)
			}

			got, err := store.Get(tt.promptName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Name != tt.want.Name {
					t.Errorf("Get() name = %v, want %v", got.Name, tt.want.Name)
				}
				if got.Title != tt.want.Title {
					t.Errorf("Get() title = %v, want %v", got.Title, tt.want.Title)
				}
			}
		})
	}
}

func TestJSONLStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// List all prompts
	prompts, cursor, err := store.List("", 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have built-in prompts
	if len(prompts) < 5 {
		t.Errorf("List() got %d prompts, want at least 5", len(prompts))
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
	if len(prompts2) == 0 {
		t.Error("List() second page empty")
	}
	if cursor2 == "" && len(prompts) > 6 {
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

	// Search for development tag
	results, err := store.SearchByTags([]string{"development"})
	if err != nil {
		t.Fatalf("SearchByTags() error = %v", err)
	}

	if len(results) < 3 {
		t.Errorf("SearchByTags() got %d results, want at least 3", len(results))
	}

	// Search for multiple tags
	results, err = store.SearchByTags([]string{"development", "review"})
	if err != nil {
		t.Fatalf("SearchByTags() error = %v", err)
	}

	// Should find code_review prompt
	found := false
	for _, p := range results {
		if p.Name == "code_review" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SearchByTags() should find code_review prompt")
	}
}

func TestBuiltinPrompts(t *testing.T) {
	prompts := GetBuiltinPrompts()

	if len(prompts) < 5 {
		t.Errorf("GetBuiltinPrompts() got %d prompts, want at least 5", len(prompts))
	}

	// Verify each prompt has required fields
	for _, p := range prompts {
		if p.Name == "" {
			t.Error("Prompt missing name")
		}
		if p.Title == "" {
			t.Errorf("Prompt %s missing title", p.Name)
		}
		if len(p.Messages) == 0 {
			t.Errorf("Prompt %s has no messages", p.Name)
		}
	}
}

func TestJSONLStore_DefaultFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := NewJSONLStore(tmpDir)
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}

	// Verify default.jsonl was created
	defaultPath := filepath.Join(tmpDir, "default.jsonl")
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		t.Error("default.jsonl was not created")
	}

	// Verify it contains prompts
	data, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("default.jsonl is empty")
	}
}
