package askcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRenderTaskList_JSONOutput(t *testing.T) {
	loadAliases := func(tasks []TaskExport) (map[string]string, error) {
		return map[string]string{"uuid-json": "sq"}, nil
	}

	tasks := []TaskExport{{
		UUID:        "uuid-json",
		Description: "JSON task",
		Status:      "pending",
		Priority:    "H",
		Tags:        []string{"cli"},
		Urgency:     12.5,
	}}
	var stdout, stderr bytes.Buffer
	code, err := renderTaskListWithAliasLoader(tasks, &stdout, &stderr, true, loadAliases)
	if err != nil {
		t.Fatalf("renderTaskList returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("renderTaskList code = %d, want 0", code)
	}
	var parsed []taskExportWithID
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	want := []taskExportWithID{{
		ID:         "sq",
		TaskExport: tasks[0],
	}}
	if !reflect.DeepEqual(parsed, want) {
		t.Fatalf("rendered tasks = %#v, want %#v", parsed, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr = %q", stderr.String())
	}
}

func TestRenderTaskList_TextOutputUsesAliasLoader(t *testing.T) {
	loadAliases := func(tasks []TaskExport) (map[string]string, error) {
		if len(tasks) != 1 || tasks[0].UUID != "uuid-text" {
			t.Fatalf("unexpected tasks passed to loader: %#v", tasks)
		}
		return map[string]string{"uuid-text": "2"}, nil
	}

	var stdout, stderr bytes.Buffer
	code, err := renderTaskListWithAliasLoader([]TaskExport{{UUID: "uuid-text", Description: "Text task", Priority: "L"}}, &stdout, &stderr, false, loadAliases)
	if err != nil {
		t.Fatalf("renderTaskList returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("renderTaskList code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "2") || strings.Contains(output, "uuid-text") {
		t.Fatalf("expected alias in output, got %q", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr = %q", stderr.String())
	}
}

func TestRenderTaskList_AliasLoaderError(t *testing.T) {
	loadAliases := func([]TaskExport) (map[string]string, error) {
		return nil, fmt.Errorf("boom")
	}
	var stdout, stderr bytes.Buffer
	code, err := renderTaskListWithAliasLoader([]TaskExport{{UUID: "uuid-error"}}, &stdout, &stderr, false, loadAliases)
	if err != nil {
		t.Fatalf("renderTaskList returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("renderTaskList code = %d, want 1 on alias error", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on alias error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "failed to load task aliases") {
		t.Fatalf("unexpected stderr message: %q", stderr.String())
	}
}
