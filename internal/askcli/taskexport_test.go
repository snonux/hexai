package askcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

// mustParseTaskExport is a test-only helper that panics on parse failure.
func mustParseTaskExport(data []byte) []TaskExport {
	var tasks []TaskExport
	if err := json.Unmarshal(data, &tasks); err != nil {
		panic(fmt.Sprintf("failed to parse task export JSON: %v", err))
	}
	return tasks
}

func TestParseTaskExport_ValidJSON(t *testing.T) {
	data := `[{"uuid":"abc123","description":"Test task","status":"pending","priority":"M","tags":["cli"],"urgency":10.5,"depends":[]}]`
	tasks, err := ParseTaskExport(strings.NewReader(data))
	if err != nil {
		t.Fatalf("ParseTaskExport returned error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].UUID != "abc123" {
		t.Fatalf("tasks[0].UUID = %q, want %q", tasks[0].UUID, "abc123")
	}
	if tasks[0].Urgency != 10.5 {
		t.Fatalf("tasks[0].Urgency = %f, want %f", tasks[0].Urgency, 10.5)
	}
}

func TestParseTaskExport_InvalidJSON(t *testing.T) {
	_, err := ParseTaskExport(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestMustParseTaskExportHelper_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("mustParseTaskExport should panic on invalid JSON")
		}
	}()
	mustParseTaskExport([]byte("not json"))
}

func TestMustParseTaskExportHelper_ValidJSON(t *testing.T) {
	data := []byte(`[{"uuid":"xyz789","description":"Another task","status":"completed","priority":"H","tags":["agent"],"urgency":15.0,"depends":["dep1"]}]`)
	tasks := mustParseTaskExport(data)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].UUID != "xyz789" {
		t.Fatalf("tasks[0].UUID = %q, want %q", tasks[0].UUID, "xyz789")
	}
}

func TestTaskExport_JSONRoundTrip(t *testing.T) {
	original := TaskExport{
		UUID:        "test-uuid",
		Description: "Test description",
		Status:      "pending",
		Priority:    "H",
		Tags:        []string{"cli", "agent"},
		Urgency:     12.5,
		Depends:     []string{"dep1", "dep2"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	var parsed TaskExport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if parsed.UUID != original.UUID {
		t.Fatalf("parsed.UUID = %q, want %q", parsed.UUID, original.UUID)
	}
	if parsed.Priority != original.Priority {
		t.Fatalf("parsed.Priority = %q, want %q", parsed.Priority, original.Priority)
	}
	if len(parsed.Depends) != len(original.Depends) {
		t.Fatalf("len(parsed.Depends) = %d, want %d", len(parsed.Depends), len(original.Depends))
	}
}

func TestParseTaskExport_EmptyInput(t *testing.T) {
	_, err := ParseTaskExport(strings.NewReader("[]"))
	if err != nil {
		t.Fatalf("ParseTaskExport returned error for empty array: %v", err)
	}
}

func TestParseTaskExport_MultipleTasks(t *testing.T) {
	data := `[{"uuid":"a1","description":"Task 1","status":"pending","priority":"H","tags":[],"urgency":10,"depends":[]},{"uuid":"b2","description":"Task 2","status":"completed","priority":"M","tags":["cli"],"urgency":5,"depends":["a1"]}]`
	tasks, err := ParseTaskExport(strings.NewReader(data))
	if err != nil {
		t.Fatalf("ParseTaskExport returned error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].UUID != "a1" || tasks[1].UUID != "b2" {
		t.Fatalf("unexpected task order or UUIDs: %v", tasks)
	}
}

func TestParseTaskExport_ReadError(t *testing.T) {
	_, err := ParseTaskExport(&errReader{})
	if err == nil {
		t.Fatal("expected error for read failure, got nil")
	}
}

type errReader struct{}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}
