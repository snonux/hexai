package askcli

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

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

func TestMustParseTaskExport_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustParseTaskExport should panic on invalid JSON")
		}
	}()
	MustParseTaskExport([]byte("not json"))
}

func TestMustParseTaskExport_ValidJSON(t *testing.T) {
	data := []byte(`[{"uuid":"xyz789","description":"Another task","status":"completed","priority":"H","tags":["agent"],"urgency":15.0,"depends":["dep1"]}]`)
	tasks := MustParseTaskExport(data)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].UUID != "xyz789" {
		t.Fatalf("tasks[0].UUID = %q, want %q", tasks[0].UUID, "xyz789")
	}
}

func TestExtractUUIDFromOutput_CreatedTask(t *testing.T) {
	output := "Created task 123.\nUUID: abc-123-def"
	uuid := ExtractUUIDFromOutput(output)
	if uuid != "123" {
		t.Fatalf("ExtractUUIDFromOutput = %q, want %q", uuid, "123")
	}
}

func TestExtractUUIDFromOutput_UUIDField(t *testing.T) {
	output := "Some text\nuuid abc-123-def\nmore text"
	uuid := ExtractUUIDFromOutput(output)
	if uuid != "abc-123-def" {
		t.Fatalf("ExtractUUIDFromOutput = %q, want %q", uuid, "abc-123-def")
	}
}

func TestExtractUUIDFromOutput_PlainText(t *testing.T) {
	output := "abc-456-xyz"
	uuid := ExtractUUIDFromOutput(output)
	if uuid != output {
		t.Fatalf("ExtractUUIDFromOutput = %q, want %q", uuid, output)
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

func TestExtractUUIDFromOutput_NilOutput(t *testing.T) {
	uuid := ExtractUUIDFromOutput("")
	if uuid != "" {
		t.Fatalf("ExtractUUIDFromOutput = %q, want empty string", uuid)
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
