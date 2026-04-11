//go:build integration

package integrationtests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/askcli"
)

func scopedAskArgs(scopePrefix string, args ...string) []string {
	if strings.TrimSpace(scopePrefix) == "" {
		return append([]string(nil), args...)
	}
	scoped := []string{scopePrefix}
	return append(scoped, args...)
}

func createTaskInScope(ctx context.Context, scopePrefix, desc string) (taskInfo, error) {
	stdout, stderr, code := runAsk(ctx, scopedAskArgs(scopePrefix, "add", "+integrationtest", desc))
	if code != 0 {
		return taskInfo{}, fmt.Errorf("create task failed (code %d): stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	id := extractTaskIDFromAddOutput(stdout.String())
	if id == "" {
		return taskInfo{}, fmt.Errorf("could not extract task ID from ask add output: %s", stdout.String())
	}

	info, ok := getTaskInfoInScope(ctx, scopePrefix, id)
	if !ok {
		return taskInfo{}, fmt.Errorf("could not resolve task ID %q after ask %s add", id, scopePrefix)
	}
	if info.UUID == "" {
		return taskInfo{}, fmt.Errorf("ask %s info %q did not return a UUID", scopePrefix, id)
	}
	return info, nil
}

func getTaskInfoInScope(ctx context.Context, scopePrefix, selector string) (taskInfo, bool) {
	stdout, _, code := runAsk(ctx, scopedAskArgs(scopePrefix, "info", selector))
	if code != 0 {
		return taskInfo{}, false
	}
	return parseTaskInfoText(stdout.String(), selector), true
}

func exportTaskByUUID(ctx context.Context, uuid string) (askcli.TaskExport, error) {
	stdout, stderr, code := runTask(ctx, []string{"uuid:" + uuid, "export"})
	if code != 0 {
		return askcli.TaskExport{}, fmt.Errorf("task export failed (code %d): stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	var tasks []askcli.TaskExport
	if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil {
		return askcli.TaskExport{}, fmt.Errorf("parse task export: %w", err)
	}
	if len(tasks) != 1 {
		return askcli.TaskExport{}, fmt.Errorf("expected 1 task, got %d", len(tasks))
	}
	return tasks[0], nil
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

// hasSelectorLine reports whether any line in output starts with want followed
// by a tab or end-of-line. This handles the "selector\tdescription" format
// emitted by complete-uuids / complete-aliases (tab-separated selector lines).
func hasSelectorLine(output, want string) bool {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		// Accept exact match (no description) or tab-prefixed description.
		if line == want || strings.HasPrefix(line, want+"\t") {
			return true
		}
	}
	return false
}

func TestNoAgentAddOmitsAgentTag(t *testing.T) {
	for _, prefix := range []string{"na", "no-agent"} {
		t.Run(prefix, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			desc := fmt.Sprintf("integration test non-agent add %s %d", prefix, time.Now().UnixNano())
			info, err := createTaskInScope(ctx, prefix, desc)
			if err != nil {
				t.Fatalf("failed to create no-agent task: %v", err)
			}
			defer deleteTask(ctx, info.UUID)

			task, err := exportTaskByUUID(ctx, info.UUID)
			if err != nil {
				t.Fatalf("failed to export task: %v", err)
			}
			if task.Description != desc {
				t.Fatalf("description = %q, want %q", task.Description, desc)
			}
			if hasTag(task.Tags, "agent") {
				t.Fatalf("tags = %v, task should not have agent tag", task.Tags)
			}
			if !hasTag(task.Tags, "integrationtest") {
				t.Fatalf("tags = %v, task should keep explicit integrationtest tag", task.Tags)
			}
			if info.ID == "" {
				t.Fatal("expected alias ID for no-agent task")
			}
		})
	}
}

func TestNoAgentListSeparatesScopedTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	agentDesc := fmt.Sprintf("integration test scoped agent list %d", time.Now().UnixNano())
	noAgentDesc := fmt.Sprintf("integration test scoped no-agent list %d", time.Now().UnixNano())

	agentUUID, err := createTask(ctx, agentDesc)
	if err != nil {
		t.Fatalf("failed to create agent task: %v", err)
	}
	defer deleteTask(ctx, agentUUID)

	noAgentInfo, err := createTaskInScope(ctx, "na", noAgentDesc)
	if err != nil {
		t.Fatalf("failed to create no-agent task: %v", err)
	}
	defer deleteTask(ctx, noAgentInfo.UUID)

	stdout, stderr, code := runAsk(ctx, []string{"list"})
	if code != 0 {
		t.Fatalf("ask list failed with code %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), agentDesc) {
		t.Fatalf("ask list should contain agent task %q: %s", agentDesc, stdout.String())
	}
	if strings.Contains(stdout.String(), noAgentDesc) {
		t.Fatalf("ask list should not contain no-agent task %q: %s", noAgentDesc, stdout.String())
	}

	for _, prefix := range []string{"na", "no-agent"} {
		t.Run(prefix, func(t *testing.T) {
			scopedStdout, scopedStderr, scopedCode := runAsk(ctx, []string{prefix, "list"})
			if scopedCode != 0 {
				t.Fatalf("ask %s list failed with code %d: stdout=%s stderr=%s", prefix, scopedCode, scopedStdout.String(), scopedStderr.String())
			}
			if !strings.Contains(scopedStdout.String(), noAgentDesc) {
				t.Fatalf("ask %s list should contain no-agent task %q: %s", prefix, noAgentDesc, scopedStdout.String())
			}
			if strings.Contains(scopedStdout.String(), agentDesc) {
				t.Fatalf("ask %s list should not contain agent task %q: %s", prefix, agentDesc, scopedStdout.String())
			}
		})
	}
}

func TestNoAgentSelectorCommandsUseScopedTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	desc := fmt.Sprintf("integration test scoped info %d", time.Now().UnixNano())
	info, err := createTaskInScope(ctx, "na", desc)
	if err != nil {
		t.Fatalf("failed to create no-agent task: %v", err)
	}
	defer deleteTask(ctx, info.UUID)

	_, stderr, code := runAsk(ctx, []string{"info", info.ID})
	if code == 0 {
		t.Fatalf("ask info %s unexpectedly succeeded outside no-agent scope", info.ID)
	}
	if !strings.Contains(stderr.String(), "current scope") {
		t.Fatalf("stderr = %q, want current-scope guidance", stderr.String())
	}

	for _, prefix := range []string{"na", "no-agent"} {
		t.Run(prefix, func(t *testing.T) {
			stdout, scopedStderr, scopedCode := runAsk(ctx, []string{prefix, "info", info.ID})
			if scopedCode != 0 {
				t.Fatalf("ask %s info failed with code %d: stdout=%s stderr=%s", prefix, scopedCode, stdout.String(), scopedStderr.String())
			}
			if !strings.Contains(stdout.String(), "UUID:        "+info.UUID) {
				t.Fatalf("ask %s info output missing UUID %q: %s", prefix, info.UUID, stdout.String())
			}
		})
	}

	stdout, stderr, code := runAsk(ctx, []string{"na", "done", info.ID})
	if code != 0 {
		t.Fatalf("ask na done failed with code %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	task, err := exportTaskByUUID(ctx, info.UUID)
	if err != nil {
		t.Fatalf("failed to export completed no-agent task: %v", err)
	}
	if task.Status != "completed" {
		t.Fatalf("status = %q, want completed", task.Status)
	}
}

func TestNoAgentCompleteUUIDsUsesScopedTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	agentUUID, err := createTask(ctx, fmt.Sprintf("integration test complete uuids agent %d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("failed to create agent task: %v", err)
	}
	defer deleteTask(ctx, agentUUID)
	agentAlias := mustTaskAlias(t, ctx, agentUUID)

	noAgentInfo, err := createTaskInScope(ctx, "na", fmt.Sprintf("integration test complete uuids no-agent %d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("failed to create no-agent task: %v", err)
	}
	defer deleteTask(ctx, noAgentInfo.UUID)

	defaultStdout, defaultStderr, defaultCode := runAsk(ctx, []string{"complete-uuids"})
	if defaultCode != 0 {
		t.Fatalf("ask complete-uuids failed with code %d: stdout=%s stderr=%s", defaultCode, defaultStdout.String(), defaultStderr.String())
	}
	if !hasSelectorLine(defaultStdout.String(), agentAlias) || !hasSelectorLine(defaultStdout.String(), agentUUID) {
		t.Fatalf("default complete-uuids should contain agent selectors: %s", defaultStdout.String())
	}
	if hasSelectorLine(defaultStdout.String(), noAgentInfo.ID) || hasSelectorLine(defaultStdout.String(), noAgentInfo.UUID) {
		t.Fatalf("default complete-uuids should not contain no-agent selectors: %s", defaultStdout.String())
	}

	for _, prefix := range []string{"na", "no-agent"} {
		t.Run(prefix, func(t *testing.T) {
			stdout, stderr, code := runAsk(ctx, []string{prefix, "complete-uuids"})
			if code != 0 {
				t.Fatalf("ask %s complete-uuids failed with code %d: stdout=%s stderr=%s", prefix, code, stdout.String(), stderr.String())
			}
			if !hasSelectorLine(stdout.String(), noAgentInfo.ID) || !hasSelectorLine(stdout.String(), noAgentInfo.UUID) {
				t.Fatalf("ask %s complete-uuids should contain no-agent selectors: %s", prefix, stdout.String())
			}
			if hasSelectorLine(stdout.String(), agentAlias) || hasSelectorLine(stdout.String(), agentUUID) {
				t.Fatalf("ask %s complete-uuids should not contain agent selectors: %s", prefix, stdout.String())
			}
		})
	}
}
