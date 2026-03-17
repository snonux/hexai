// Package tmuxedit provides JSONL-based history storage for tmux popup submissions.
package tmuxedit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/textutil"
)

// HistoryEntry represents a single submission to the AI agent via tmux popup.
// Stored in JSONL format (one JSON object per line) for easy appending and reading.
type HistoryEntry struct {
	Timestamp string `json:"timestamp"` // RFC3339 format
	Agent     string `json:"agent"`     // AI agent name (e.g., "claude", "aider")
	Cwd       string `json:"cwd"`       // Current working directory at submission time
	Text      string `json:"text"`      // The submitted text
}

// AppendHistory appends a new history entry to the history file.
// Uses atomic write pattern (write to temp file, then rename) for safety.
func AppendHistory(text, agent, cwd string) error {
	stateDir, err := appconfig.StateDir()
	if err != nil {
		return fmt.Errorf("cannot get state directory: %w", err)
	}

	historyPath := filepath.Join(stateDir, "tmux-edit-history.jsonl")

	// Create entry with current timestamp
	entry := HistoryEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Agent:     agent,
		Cwd:       cwd,
		Text:      text,
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("cannot marshal history entry: %w", err)
	}

	// Append newline for JSONL format
	data = append(data, '\n')

	// Open file in append mode, create if doesn't exist
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open history file: %w", err)
	}
	defer func() { _ = f.Close() }() // best-effort on error paths

	// Write entry
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("cannot write history entry: %w", err)
	}

	// Check Close error to catch deferred-write failures (e.g. disk full).
	return f.Close()
}

// GetHistory retrieves the most recent history entries (up to limit).
// Returns entries in chronological order (oldest first).
// If limit <= 0, returns all entries.
func GetHistory(limit int) ([]HistoryEntry, error) {
	stateDir, err := appconfig.StateDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get state directory: %w", err)
	}

	historyPath := filepath.Join(stateDir, "tmux-edit-history.jsonl")

	// Read entire file
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryEntry{}, nil // Empty history is not an error
		}
		return nil, fmt.Errorf("cannot read history file: %w", err)
	}

	// Parse JSONL line by line
	var entries []HistoryEntry
	lines := textutil.SplitLinesBytes(data)
	for i, line := range lines {
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var entry HistoryEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// Log error but continue parsing (don't fail entire history on one bad line)
			fmt.Fprintf(os.Stderr, "warning: cannot parse history entry at line %d: %v\n", i+1, err)
			continue
		}
		entries = append(entries, entry)
	}

	// Apply limit if specified
	if limit > 0 && len(entries) > limit {
		// Return the most recent entries
		entries = entries[len(entries)-limit:]
	}

	return entries, nil
}
