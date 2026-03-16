// Package promptstore provides a prompt storage interface and JSONL-based implementation.
package promptstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// PromptStore defines the interface for prompt storage operations.
// Allows easy mocking in tests.
type PromptStore interface {
	// List returns prompts with pagination support.
	// cursor is the pagination token (empty for first page).
	// limit is the max prompts to return per page.
	List(cursor string, limit int) ([]Prompt, string, error)

	// Get retrieves a prompt by name.
	Get(name string) (*Prompt, error)

	// Create adds a new prompt.
	Create(prompt *Prompt) error

	// Update modifies an existing prompt.
	Update(prompt *Prompt) error

	// Delete removes a prompt by name.
	Delete(name string) error

	// SearchByTags finds prompts matching all given tags (AND logic).
	SearchByTags(tags []string) ([]Prompt, error)
}

// JSONLStore is a file-based prompt store using JSONL format.
// Built-in prompts are loaded from code, user prompts are stored in user.jsonl.
// Automatically creates backups before any write operation.
type JSONLStore struct {
	dataDir string
	mu      sync.RWMutex

	// File operation functions (can be mocked for testing)
	readFileFn  func(string) ([]byte, error)
	writeFileFn func(string, []byte, os.FileMode) error

	// Backup settings
	maxBackups int // Maximum number of backups to keep (0 = unlimited)
}

// NewJSONLStore creates a new JSONL-based prompt store.
// dataDir should be an absolute path (e.g., ~/.local/share/hexai/prompts/).
func NewJSONLStore(dataDir string) (PromptStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create prompts directory: %w", err)
	}

	// Create backups subdirectory
	backupDir := filepath.Join(dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create backups directory: %w", err)
	}

	store := &JSONLStore{
		dataDir:     dataDir,
		readFileFn:  os.ReadFile,
		writeFileFn: os.WriteFile,
		maxBackups:  10, // Keep last 10 backups
	}

	return store, nil
}

// List returns prompts with pagination.
// Returns both built-in prompts (from code) and user prompts (from user.jsonl).
func (s *JSONLStore) List(cursor string, limit int) ([]Prompt, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100 // Default limit
	}

	// Load all prompts from both files
	allPrompts, err := s.loadAllPrompts()
	if err != nil {
		return nil, "", err
	}

	// Sort by name for consistent ordering
	sort.Slice(allPrompts, func(i, j int) bool {
		return allPrompts[i].Name < allPrompts[j].Name
	})

	// Handle pagination
	startIdx := 0
	if cursor != "" {
		// Simple cursor: index as string
		fmt.Sscanf(cursor, "%d", &startIdx)
	}

	if startIdx >= len(allPrompts) {
		return []Prompt{}, "", nil
	}

	endIdx := startIdx + limit
	if endIdx > len(allPrompts) {
		endIdx = len(allPrompts)
	}

	result := allPrompts[startIdx:endIdx]
	nextCursor := ""
	if endIdx < len(allPrompts) {
		nextCursor = fmt.Sprintf("%d", endIdx)
	}

	return result, nextCursor, nil
}

// Get retrieves a prompt by name.
func (s *JSONLStore) Get(name string) (*Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allPrompts, err := s.loadAllPrompts()
	if err != nil {
		return nil, err
	}

	for _, p := range allPrompts {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("prompt not found: %s", name)
}

// Create adds a new prompt to user.jsonl.
func (s *JSONLStore) Create(prompt *Prompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Backup before write
	if err := s.backupUserPrompts(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Check if prompt already exists in built-ins
	isBuiltIn, err := s.isBuiltInPrompt(prompt.Name)
	if err != nil {
		return fmt.Errorf("check built-in prompts: %w", err)
	}
	if isBuiltIn {
		return fmt.Errorf("prompt already exists: %s (choose a different name)", prompt.Name)
	}

	// Check if prompt already exists in user prompts
	userPrompts, err := s.loadPromptsFromFile("user.jsonl")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load user prompts: %w", err)
	}
	for _, p := range userPrompts {
		if p.Name == prompt.Name {
			return fmt.Errorf("prompt already exists: %s", prompt.Name)
		}
	}

	// Append to user.jsonl
	userPath := filepath.Join(s.dataDir, "user.jsonl")
	data, err := json.Marshal(prompt)
	if err != nil {
		return fmt.Errorf("marshal prompt: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(userPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open user.jsonl: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write user.jsonl: %w", err)
	}

	return nil
}

// Update modifies an existing prompt in user.jsonl.
// Note: This rewrites the entire user.jsonl file.
// Cannot update built-in prompts (returns error).
func (s *JSONLStore) Update(prompt *Prompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this is a built-in prompt (cannot be updated)
	isBuiltIn, err := s.isBuiltInPrompt(prompt.Name)
	if err != nil {
		return fmt.Errorf("check built-in prompts: %w", err)
	}
	if isBuiltIn {
		return fmt.Errorf("cannot update built-in prompt: %s (create a new prompt with a different name instead)", prompt.Name)
	}

	// Backup before write
	if err := s.backupUserPrompts(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Load user prompts
	userPrompts, err := s.loadPromptsFromFile("user.jsonl")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Find and update prompt
	found := false
	for i, p := range userPrompts {
		if p.Name == prompt.Name {
			userPrompts[i] = *prompt
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("prompt not found in user.jsonl: %s", prompt.Name)
	}

	// Rewrite user.jsonl
	return s.writePromptsToFile("user.jsonl", userPrompts)
}

// Delete removes a prompt from user.jsonl.
// Cannot delete built-in prompts (returns error).
func (s *JSONLStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this is a built-in prompt (cannot be deleted)
	isBuiltIn, err := s.isBuiltInPrompt(name)
	if err != nil {
		return fmt.Errorf("check built-in prompts: %w", err)
	}
	if isBuiltIn {
		return fmt.Errorf("cannot delete built-in prompt: %s", name)
	}

	// Backup before write
	if err := s.backupUserPrompts(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Load user prompts
	userPrompts, err := s.loadPromptsFromFile("user.jsonl")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Filter out deleted prompt
	var filtered []Prompt
	found := false
	for _, p := range userPrompts {
		if p.Name != name {
			filtered = append(filtered, p)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("prompt not found: %s", name)
	}

	// Rewrite user.jsonl
	return s.writePromptsToFile("user.jsonl", filtered)
}

// SearchByTags finds prompts matching all given tags.
func (s *JSONLStore) SearchByTags(tags []string) ([]Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allPrompts, err := s.loadAllPrompts()
	if err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return allPrompts, nil
	}

	var results []Prompt
	for _, p := range allPrompts {
		if s.hasAllTags(p.Tags, tags) {
			results = append(results, p)
		}
	}

	return results, nil
}

// hasAllTags checks if promptTags contains all searchTags.
func (s *JSONLStore) hasAllTags(promptTags, searchTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range promptTags {
		tagSet[t] = true
	}

	for _, t := range searchTags {
		if !tagSet[t] {
			return false
		}
	}
	return true
}

// loadAllPrompts loads prompts from both built-in code and user.jsonl.
// Built-in prompts (from code) take precedence in case of name conflicts.
// Logs a warning to stderr if a user prompt conflicts with a built-in.
func (s *JSONLStore) loadAllPrompts() ([]Prompt, error) {
	// Load built-in prompts directly from code (no file needed)
	builtInPrompts := DefaultPrompts()

	// Load user prompts from user.jsonl
	userPrompts, err := s.loadPromptsFromFile("user.jsonl")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Create a map of built-in prompt names for conflict detection
	builtInNames := make(map[string]bool)
	for _, p := range builtInPrompts {
		builtInNames[p.Name] = true
	}

	// Combine prompts, skipping user prompts that conflict with built-ins
	allPrompts := make([]Prompt, 0, len(builtInPrompts)+len(userPrompts))
	allPrompts = append(allPrompts, builtInPrompts...)

	for _, p := range userPrompts {
		if builtInNames[p.Name] {
			fmt.Fprintf(os.Stderr, "warning: skipping user prompt '%s' - conflicts with built-in\n", p.Name)
			continue
		}
		allPrompts = append(allPrompts, p)
	}

	return allPrompts, nil
}

// isBuiltInPrompt checks if a prompt with the given name exists in the built-in prompts.
// Returns true if the prompt is a built-in (read-only) prompt.
func (s *JSONLStore) isBuiltInPrompt(name string) (bool, error) {
	builtIns := DefaultPrompts()

	for _, p := range builtIns {
		if p.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// loadPromptsFromFile reads prompts from a JSONL file.
func (s *JSONLStore) loadPromptsFromFile(filename string) ([]Prompt, error) {
	path := filepath.Join(s.dataDir, filename)
	data, err := s.readFileFn(path)
	if err != nil {
		return nil, err
	}

	var prompts []Prompt
	lines := splitLines(data)
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		var p Prompt
		if err := json.Unmarshal(line, &p); err != nil {
			// Log error but continue parsing
			fmt.Fprintf(os.Stderr, "warning: cannot parse prompt at %s:%d: %v\n", filename, i+1, err)
			continue
		}
		prompts = append(prompts, p)
	}

	return prompts, nil
}

// writePromptsToFile writes prompts to a JSONL file.
func (s *JSONLStore) writePromptsToFile(filename string, prompts []Prompt) error {
	path := filepath.Join(s.dataDir, filename)

	var lines []byte
	for _, p := range prompts {
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal prompt: %w", err)
		}
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}

	if err := s.writeFileFn(path, lines, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}

	return nil
}

// splitLines splits data into lines (handles both \n and \r\n).
// Copied from tmuxedit/history.go pattern.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			end := i
			if end > start && data[end-1] == '\r' {
				end--
			}
			lines = append(lines, data[start:end])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// backupUserPrompts creates a timestamped backup of user.jsonl before any write operation.
// Automatically manages backup retention based on maxBackups setting.
func (s *JSONLStore) backupUserPrompts() error {
	userPath := filepath.Join(s.dataDir, "user.jsonl")

	// Check if user.jsonl exists
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	// Read current user.jsonl
	data, err := s.readFileFn(userPath)
	if err != nil {
		return fmt.Errorf("read user.jsonl: %w", err)
	}

	// Create backup with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(s.dataDir, "backups")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("user.jsonl.%s", timestamp))

	// Write backup
	if err := s.writeFileFn(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}

	// Clean old backups if maxBackups is set
	if s.maxBackups > 0 {
		if err := s.cleanOldBackups(); err != nil {
			// Log but don't fail - backup succeeded
			fmt.Fprintf(os.Stderr, "warning: failed to clean old backups: %v\n", err)
		}
	}

	return nil
}

// cleanOldBackups removes old backup files, keeping only the most recent maxBackups.
func (s *JSONLStore) cleanOldBackups() error {
	backupDir := filepath.Join(s.dataDir, "backups")

	// List all backups
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("read backup dir: %w", err)
	}

	// Filter backup files
	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "user.jsonl.") {
			backups = append(backups, entry.Name())
		}
	}

	// Sort backups (newest last due to timestamp format)
	sort.Strings(backups)

	// Remove old backups
	if len(backups) > s.maxBackups {
		toRemove := len(backups) - s.maxBackups
		for i := 0; i < toRemove; i++ {
			backupPath := filepath.Join(backupDir, backups[i])
			if err := os.Remove(backupPath); err != nil {
				return fmt.Errorf("remove backup %s: %w", backups[i], err)
			}
		}
	}

	return nil
}

// ListBackups returns a list of available backup files with timestamps.
func (s *JSONLStore) ListBackups() ([]string, error) {
	backupDir := filepath.Join(s.dataDir, "backups")

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "user.jsonl.") {
			backups = append(backups, entry.Name())
		}
	}

	// Sort newest first
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))

	return backups, nil
}

// RestoreBackup restores user.jsonl from a backup file.
func (s *JSONLStore) RestoreBackup(backupName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	backupPath := filepath.Join(s.dataDir, "backups", backupName)
	userPath := filepath.Join(s.dataDir, "user.jsonl")

	// Read backup
	data, err := s.readFileFn(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	// Create a safety backup of current state before restore
	if _, err := os.Stat(userPath); err == nil {
		timestamp := time.Now().Format("20060102-150405")
		safetyBackup := filepath.Join(s.dataDir, "backups", fmt.Sprintf("user.jsonl.%s.pre-restore", timestamp))
		currentData, _ := s.readFileFn(userPath)
		s.writeFileFn(safetyBackup, currentData, 0o644)
	}

	// Restore from backup
	if err := s.writeFileFn(userPath, data, 0o644); err != nil {
		return fmt.Errorf("write restored user.jsonl: %w", err)
	}

	return nil
}
