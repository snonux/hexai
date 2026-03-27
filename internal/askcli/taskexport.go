package askcli

import (
	"encoding/json"
	"fmt"
	"io"
)

// TaskExport mirrors the JSON structure returned by Taskwarrior export commands.
type TaskExport struct {
	UUID        string   `json:"uuid"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags"`
	Start       string   `json:"start,omitempty"`
	Urgency     float64  `json:"urgency"`
	Depends     []string `json:"depends"`
	Annotations []struct {
		Description string `json:"description"`
		Entry       string `json:"entry"`
	} `json:"annotations"`
}

// ParseTaskExport decodes Taskwarrior JSON from the supplied reader.
func ParseTaskExport(r io.Reader) ([]TaskExport, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read task export data: %w", err)
	}
	var tasks []TaskExport
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task export JSON: %w", err)
	}
	return tasks, nil
}
