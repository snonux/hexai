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
	output := FormatTaskList(tasks)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("FormatTaskList produced too few lines: %d", len(lines))
	}
	if !strings.Contains(lines[0], "UUID") || !strings.Contains(lines[0], "Priority") {
		t.Fatalf("header missing UUID or Priority column: %s", lines[0])
	}
	if !strings.Contains(lines[0], "Started") {
		t.Fatalf("header missing Started column: %s", lines[0])
	}
	if !strings.Contains(lines[2], "uuid-1") {
		t.Fatalf("first task line missing uuid-1: %s", lines[2])
	}
	if strings.Contains(lines[2], "...") {
		t.Fatalf("long description should be truncated with ...: %s", lines[2])
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

	output := FormatTaskList(tasks)
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("FormatTaskList produced %d lines, want 4: %q", len(lines), output)
	}

	widths := taskListWidthsFor(tasks)
	wantHeader := fmt.Sprintf("%-*s | %-*s | %-*s | %-*s | %-*s | %-*s | %-*s",
		widths.Urgency, "Urgency",
		widths.Priority, "Priority",
		widths.UUID, "UUID",
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
	output := FormatTaskInfo(task)
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
	if !strings.Contains(output, "dep-1") {
		t.Fatalf("FormatTaskInfo missing depends: %s", output)
	}
	if !strings.Contains(output, "First note") {
		t.Fatalf("FormatTaskInfo missing annotation: %s", output)
	}
}

func TestFormatSuccess(t *testing.T) {
	output := FormatSuccess("test-uuid")
	if !strings.Contains(output, "ok") || !strings.Contains(output, "test-uuid") {
		t.Fatalf("FormatSuccess = %q, want ok + uuid", output)
	}
}

func TestFormatError(t *testing.T) {
	err := &testError{msg: "something went wrong"}
	output := FormatError(err, "uuid-123")
	if !strings.Contains(output, "error") || !strings.Contains(output, "uuid-123") || !strings.Contains(output, "something went wrong") {
		t.Fatalf("FormatError = %q, want error + uuid + message", output)
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
	if !strings.Contains(output, "use UUID") {
		t.Fatalf("RejectNumericID = %q, want use UUID message", output)
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
	output := FormatTaskInfo(task)
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
