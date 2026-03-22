package askcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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

func MustParseTaskExport(data []byte) []TaskExport {
	var tasks []TaskExport
	if err := json.Unmarshal(data, &tasks); err != nil {
		panic(fmt.Sprintf("failed to parse task export JSON: %v", err))
	}
	return tasks
}

func ExtractUUIDFromOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Created task ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return strings.TrimSuffix(parts[2], ".")
			}
		}
	}
	fields := strings.Fields(output)
	for i, f := range fields {
		if f == "uuid" && i+1 < len(fields) {
			return fields[i+1]
		}
		if strings.HasPrefix(f, "Created task") {
			parts := strings.Split(f, " ")
			if len(parts) >= 2 {
				return strings.TrimSuffix(parts[1], ".")
			}
		}
	}
	return strings.TrimSpace(output)
}
