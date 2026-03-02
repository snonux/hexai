// Summary: File syncer for exporting MCP prompts to slash command files.
package slashcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/promptstore"
)

// Operation represents the type of sync operation.
type Operation int

const (
	OpCreate Operation = iota
	OpUpdate
	OpDelete
)

// Syncer manages syncing MCP prompts to slash command files.
// Thread-safe with mutex protection for concurrent operations.
type Syncer struct {
	commandsDir string
	enabled     bool
	mu          sync.Mutex
}

// NewSyncer creates a new syncer and validates the commands directory.
// Returns error if directory cannot be created or is not writable.
func NewSyncer(cfg appconfig.App) (*Syncer, error) {
	if !cfg.MCPSlashCommandSync {
		return &Syncer{enabled: false}, nil
	}

	dir := cfg.MCPSlashCommandDir
	if dir == "" {
		return nil, fmt.Errorf("commands directory not configured")
	}

	// Expand home directory
	if len(dir) > 0 && dir[0] == '~' {
		home := os.Getenv("HOME")
		if home != "" {
			dir = filepath.Join(home, dir[1:])
		}
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create commands directory: %w", err)
	}

	return &Syncer{
		commandsDir: dir,
		enabled:     true,
	}, nil
}

// Sync writes a prompt to disk as hexai-{name}.md.
// Uses atomic write (temp file + rename) for consistency.
func (s *Syncer) Sync(prompt *promptstore.Prompt, op Operation) error {
	if !s.enabled {
		return nil // Silently skip if disabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filename := MakeFilename(prompt.Name)
	path := filepath.Join(s.commandsDir, filename)

	// Convert prompt to Markdown
	markdown := ConvertPromptToMarkdown(prompt)

	// Write atomically: temp file + rename
	return s.atomicWrite(path, []byte(markdown))
}

// SyncCreate syncs a newly created prompt.
func (s *Syncer) SyncCreate(prompt *promptstore.Prompt) error {
	return s.Sync(prompt, OpCreate)
}

// SyncUpdate syncs an updated prompt.
func (s *Syncer) SyncUpdate(prompt *promptstore.Prompt) error {
	return s.Sync(prompt, OpUpdate)
}

// Delete removes hexai-{name}.md file.
// Returns nil if file doesn't exist (idempotent).
func (s *Syncer) Delete(promptName string) error {
	if !s.enabled {
		return nil // Silently skip if disabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filename := MakeFilename(promptName)
	path := filepath.Join(s.commandsDir, filename)

	// Remove file (ignore if doesn't exist)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // File already gone, that's fine
	}
	return err
}

// SyncAll syncs all prompts from the store to slash commands.
// Used for backfilling existing prompts via --sync-all flag.
func (s *Syncer) SyncAll(store promptstore.PromptStore) error {
	if !s.enabled {
		return fmt.Errorf("syncer is disabled")
	}

	// List all prompts (no pagination, use large limit)
	prompts, _, err := store.List("", 1000)
	if err != nil {
		return fmt.Errorf("cannot list prompts: %w", err)
	}

	count := 0
	var errors []error

	for i := range prompts {
		if err := s.Sync(&prompts[i], OpCreate); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", prompts[i].Name, err))
		} else {
			count++
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("synced %d prompts with %d errors: %v", count, len(errors), errors)
	}

	return nil
}

// atomicWrite writes data to a file atomically using temp file + rename.
// This prevents partial writes if interrupted.
func (s *Syncer) atomicWrite(path string, data []byte) error {
	// Write to temp file in same directory
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	tmpFile = nil // Prevent deferred close

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
