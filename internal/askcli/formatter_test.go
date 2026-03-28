package askcli

import (
	"fmt"
	"strings"
	"testing"
)

func TestFormatTaskList(t *testing.T) {
	tasks := []TaskExport{
		{UUID: "uuid-1", Description: "Short task", Status: "pending", Priority: "H", Tags: []string{"cli"}, Urgency: 15.0},
		{UUID: "uuid-2", Description: strings.Repeat("a", 100), Status: "completed", Priority: "M", Tags: []string{"agent", "test"}, Urgency: 5.0},
		{UUID: "uuid-3", Description: "No tags task", Status: "waiting", Priority: "L", Tags: []string{}, Urgency: 8.0},
	}
	aliases := map[string]string{"uuid-1": "0", "uuid-2": "1", "uuid-3": "2"}
	output := FormatTaskList(tasks, aliases)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("FormatTaskList produced too few lines: %d", len(lines))
	}
	if !strings.Contains(lines[0], "ID") || !strings.Contains(lines[0], "Pri") {
		t.Fatalf("header missing ID or Pri column: %s", lines[0])
	}
	if strings.Contains(lines[0], "Priority") {
		t.Fatalf("header should use compact Pri label: %s", lines[0])
	}
	if !strings.Contains(lines[0], "Started") {
		t.Fatalf("header missing Started column: %s", lines[0])
	}
	if !strings.Contains(lines[2], "0") || strings.Contains(lines[2], "uuid-1") {
		t.Fatalf("first task line should show alias only: %s", lines[2])
	}
	if !strings.Contains(lines[3], strings.Repeat("a", 100)) {
		t.Fatalf("default formatting should keep the full description when width is unconstrained: %s", lines[3])
	}
}

func TestFormatTaskList_AlignsHeaderAndSeparator(t *testing.T) {
	tasks := []TaskExport{
		{
			UUID:        "uuid-short",
			Description: "Short task",
			Status:      "pending",
			Priority:    "H",
			Tags:        []string{"cli"},
			Urgency:     1.0,
		},
		{
			UUID:        "uuid-with-a-longer-value",
			Description: strings.Repeat("x", 60),
			Status:      "completed",
			Priority:    "M",
			Tags:        []string{"agent", "cli"},
			Urgency:     12.3,
		},
	}

	aliases := map[string]string{"uuid-short": "0", "uuid-with-a-longer-value": "00"}
	output := FormatTaskList(tasks, aliases)
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("FormatTaskList produced %d lines, want 4: %q", len(lines), output)
	}

	widths := taskListWidthsFor(tasks, aliases, 0)
	wantHeader := fmt.Sprintf("%-*s | %-*s | %-*s | %-*s | %-*s | %-*s | %-*s",
		widths.Urgency, "Urg",
		widths.Priority, "Pri",
		widths.ID, "ID",
		widths.Status, "Status",
		widths.Started, "Started",
		widths.Tags, "Tags",
		widths.Description, "Description",
	)
	if lines[0] != wantHeader {
		t.Fatalf("header = %q, want %q", lines[0], wantHeader)
	}
	if len(lines[1]) != len(wantHeader) {
		t.Fatalf("separator length = %d, want %d", len(lines[1]), len(wantHeader))
	}
}

func TestFormatTaskList_FallsBackToUUIDWithoutAlias(t *testing.T) {
	tasks := []TaskExport{{UUID: "uuid-1", Description: "Task", Status: "pending", Priority: "H", Urgency: 1.0}}

	output := FormatTaskList(tasks, nil)
	if !strings.Contains(output, "uuid-1") {
		t.Fatalf("FormatTaskList should fall back to UUID when alias is unavailable: %s", output)
	}
}

func TestFormatTaskListForWidth_UsesAvailableTerminalWidthForDescription(t *testing.T) {
	tasks := []TaskExport{
		{
			UUID:        "uuid-1",
			Description: strings.Repeat("x", 70),
			Status:      "pending",
			Priority:    "H",
			Tags:        []string{"cli"},
			Urgency:     1.0,
		},
	}

	aliases := map[string]string{"uuid-1": "0"}
	terminalWidth := 110
	output := FormatTaskListForWidth(tasks, aliases, terminalWidth)
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	widths := taskListWidthsFor(tasks, aliases, terminalWidth)
	if widths.Description <= 50 {
		t.Fatalf("description width = %d, want > 50 for this terminal width", widths.Description)
	}
	if got := len(lines[0]); got != taskListFixedWidth(widths)+widths.Description {
		t.Fatalf("header width = %d, want %d: %q", got, taskListFixedWidth(widths)+widths.Description, lines[0])
	}
	renderedDescription := strings.Split(lines[2], " | ")[6]
	if renderedDescription != strings.Repeat("x", widths.Description-3)+"..." {
		t.Fatalf("description should expand to the available terminal width: %s", lines[2])
	}
	if len(renderedDescription) != widths.Description {
		t.Fatalf("rendered description width = %d, want %d", len(renderedDescription), widths.Description)
	}
}

func TestFormatTaskListForWidth_TruncatesDescriptionWhenTerminalIsNarrow(t *testing.T) {
	tasks := []TaskExport{
		{
			UUID:        "uuid-1",
			Description: "abcdefghijklmnop",
			Status:      "pending",
			Priority:    "H",
			Tags:        []string{"cli"},
			Urgency:     1.0,
		},
	}

	aliases := map[string]string{"uuid-1": "0"}
	output := FormatTaskListForWidth(tasks, aliases, 40)
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if !strings.Contains(lines[2], "abcdefgh...") {
		t.Fatalf("description should truncate to fit a narrow terminal: %s", lines[2])
	}
	if strings.Contains(lines[2], "abcdefghijklmnop") {
		t.Fatalf("description should not print the full description in a narrow terminal: %s", lines[2])
	}
}

func TestFormatTaskInfo(t *testing.T) {
	task := TaskExport{
		UUID:        "test-uuid",
		Description: "Test description",
		Status:      "pending",
		Priority:    "H",
		Tags:        []string{"cli", "agent"},
		Start:       "2026-03-22T10:00:00Z",
		Urgency:     15.5,
		Depends:     []string{"dep-1", "dep-2"},
		Annotations: []struct {
			Description string `json:"description"`
			Entry       string `json:"entry"`
		}{
			{Description: "First note", Entry: "2026-03-22T11:00:00Z"},
		},
	}
	output := FormatTaskInfo(task, "0", map[string]string{"dep-1": "1", "dep-2": "2"})
	if !strings.Contains(output, "ID:          0") {
		t.Fatalf("FormatTaskInfo missing alias ID: %s", output)
	}
	if !strings.Contains(output, "test-uuid") {
		t.Fatalf("FormatTaskInfo missing UUID: %s", output)
	}
	if !strings.Contains(output, "H") {
		t.Fatalf("FormatTaskInfo missing priority H: %s", output)
	}
	if !strings.Contains(output, "Started:     yes") {
		t.Fatalf("FormatTaskInfo missing explicit started state: %s", output)
	}
	if !strings.Contains(output, "Start time:  2026-03-22T10:00:00Z") {
		t.Fatalf("FormatTaskInfo missing start timestamp: %s", output)
	}
	if !strings.Contains(output, "cli, agent") {
		t.Fatalf("FormatTaskInfo missing tags: %s", output)
	}
	if !strings.Contains(output, "1 (dep-1)") || !strings.Contains(output, "2 (dep-2)") {
		t.Fatalf("FormatTaskInfo missing formatted depends: %s", output)
	}
	if !strings.Contains(output, "First note") {
		t.Fatalf("FormatTaskInfo missing annotation: %s", output)
	}
}

func TestFormatSuccess(t *testing.T) {
	output := FormatSuccess("0")
	if !strings.Contains(output, "ok") || !strings.Contains(output, "0") {
		t.Fatalf("FormatSuccess = %q, want ok + alias", output)
	}
}

func TestFormatCreatedTask(t *testing.T) {
	output := FormatCreatedTask("sp")
	if output != "created task sp\n" {
		t.Fatalf("FormatCreatedTask = %q, want created task message", output)
	}
}

func TestFormatError(t *testing.T) {
	err := &testError{msg: "something went wrong"}
	output := FormatError(err, "0")
	if !strings.Contains(output, "error") || !strings.Contains(output, "0") || !strings.Contains(output, "something went wrong") {
		t.Fatalf("FormatError = %q, want error + alias + message", output)
	}
}

func TestFormatError_NoUUID(t *testing.T) {
	err := &testError{msg: "generic error"}
	output := FormatError(err, "")
	if !strings.Contains(output, "error") || !strings.Contains(output, "generic error") {
		t.Fatalf("FormatError = %q, want error + message", output)
	}
}

func TestIsNumericID(t *testing.T) {
	if !IsNumericID("123") {
		t.Error("IsNumericID(\"123\") = false, want true")
	}
	if !IsNumericID("0") {
		t.Error("IsNumericID(\"0\") = false, want true")
	}
	if IsNumericID("uuid-123") {
		t.Error("IsNumericID(\"uuid-123\") = true, want false")
	}
	if IsNumericID("abc") {
		t.Error("IsNumericID(\"abc\") = true, want false")
	}
	if IsNumericID("12a") {
		t.Error("IsNumericID(\"12a\") = true, want false")
	}
	if IsNumericID("") {
		t.Error("IsNumericID(\"\") = true, want false")
	}
}

func TestNormalizeUUID(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"fc390139-cc08-413f-a411-f2feae4875a3", "fc390139-cc08-413f-a411-f2feae4875a3"},
		{"uuid:fc390139-cc08-413f-a411-f2feae4875a3", "fc390139-cc08-413f-a411-f2feae4875a3"},
		{"fc390139", "fc390139"},
		{"uuid:fc390139", "fc390139"},
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizeUUID(c.input)
		if got != c.want {
			t.Errorf("NormalizeUUID(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestRejectNumericID(t *testing.T) {
	output := RejectNumericID()
	if !strings.Contains(output, "task alias ID or UUID") {
		t.Fatalf("RejectNumericID = %q, want alias-or-UUID message", output)
	}
}

func TestFormatTaskInfo_NoOptionalFields(t *testing.T) {
	task := TaskExport{
		UUID:        "simple-uuid",
		Description: "Simple task",
		Status:      "pending",
		Priority:    "M",
		Tags:        []string{},
		Urgency:     0,
	}
	output := FormatTaskInfo(task, "0", nil)
	if !strings.Contains(output, "simple-uuid") {
		t.Fatalf("FormatTaskInfo missing UUID: %s", output)
	}
	if !strings.Contains(output, "Started:     no") {
		t.Fatalf("FormatTaskInfo should show Started: no when not started: %s", output)
	}
	if strings.Contains(output, "Tags:") {
		t.Fatalf("FormatTaskInfo should not contain Tags: for empty tags: %s", output)
	}
	if strings.Contains(output, "Depends:") {
		t.Fatalf("FormatTaskInfo should not contain Depends: for empty depends: %s", output)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
