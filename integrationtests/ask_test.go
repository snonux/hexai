//go:build integration

package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/askcli"
)

// repoRoot is set in TestMain before any test runs.
var repoRoot string

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func askBinaryPath() string {
	return filepath.Join(repoRoot, "cmd", "ask", "ask")
}

func runAsk(ctx context.Context, args []string) (stdout, stderr bytes.Buffer, exitCode int) {
	cmd := exec.CommandContext(ctx, askBinaryPath(), args...)
	cmd.Dir = repoRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return bytes.Buffer{}, stderr, -1
	}
	return stdout, stderr, ee.ExitCode()
}

// runAskWithStdin runs ask with the given stdin. Only use this for commands
// that actually forward stdin to taskwarrior (currently only: delete).
func runAskWithStdin(ctx context.Context, args []string, stdin string) (stdout, stderr bytes.Buffer, exitCode int) {
	cmd := exec.CommandContext(ctx, askBinaryPath(), args...)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return bytes.Buffer{}, stderr, -1
	}
	return stdout, stderr, ee.ExitCode()
}

func runTask(ctx context.Context, args []string) (stdout, stderr bytes.Buffer, exitCode int) {
	cmd := exec.CommandContext(ctx, "task", args...)
	cmd.Dir = repoRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return bytes.Buffer{}, stderr, -1
	}
	return stdout, stderr, ee.ExitCode()
}

func runTaskWithStdin(ctx context.Context, args []string, stdin string) (stdout, stderr bytes.Buffer, exitCode int) {
	cmd := exec.CommandContext(ctx, "task", args...)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return bytes.Buffer{}, stderr, -1
	}
	return stdout, stderr, ee.ExitCode()
}

// createTask creates a new task via ask add and returns its UUID.
// ask add prints a human-facing created-task message, so we resolve the created UUID via ask info.
func createTask(ctx context.Context, desc string) (string, error) {
	stdout, stderr, code := runAsk(ctx, []string{"add", "+integrationtest", desc})
	if code != 0 {
		return "", fmt.Errorf("create task failed (code %d): stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	id := extractTaskIDFromAddOutput(stdout.String())
	if id == "" {
		return "", fmt.Errorf("could not extract task ID from ask add output: %s", stdout.String())
	}
	info, ok := getTaskInfoFast(ctx, id)
	if !ok {
		return "", fmt.Errorf("could not resolve task ID %q after ask add", id)
	}
	if info.UUID == "" {
		return "", fmt.Errorf("ask info %q did not return a UUID", id)
	}
	return info.UUID, nil
}

func extractTaskIDFromAddOutput(output string) string {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "created task ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "created task "))
		}
	}
	return strings.TrimSpace(output)
}

func deleteTask(ctx context.Context, uuid string) {
	runTaskWithStdin(ctx, []string{"uuid:" + uuid, "delete"}, "yes\n")
}

func listTasksWithTag(ctx context.Context, tag string) []askcli.TaskExport {
	stdout, _, _ := runTask(ctx, []string{"export", "project:hexai", "+agent"})
	var tasks []askcli.TaskExport
	if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil {
		return nil
	}
	var filtered []askcli.TaskExport
	for _, t := range tasks {
		if t.Status == "deleted" || t.Status == "completed" {
			continue
		}
		for _, t2 := range t.Tags {
			if t2 == tag {
				filtered = append(filtered, t)
				break
			}
		}
	}
	return filtered
}

type taskInfo struct {
	ID          string
	UUID        string
	Description string
	Status      string
	Started     string
	StartTime   string
	Priority    string
	Depends     []string
	Tags        []string
}

var (
	idFieldRx        = regexp.MustCompile(`ID:\s+(.+)`)
	uuidFieldRx      = regexp.MustCompile(`UUID:\s+(.+)`)
	descFieldRx      = regexp.MustCompile(`Description:\s+(.+)`)
	statusFieldRx    = regexp.MustCompile(`Status:\s+(.+)`)
	startedFieldRx   = regexp.MustCompile(`Started:\s+(.+)`)
	startTimeFieldRx = regexp.MustCompile(`Start time:\s+(.+)`)
	priorityFieldRx  = regexp.MustCompile(`Priority:\s+(.+)`)
	dependsFieldRx   = regexp.MustCompile(`Depends:\s+(.+)`)
	tagsFieldRx      = regexp.MustCompile(`Tags:\s+(.+)`)
	uuidFormatRx     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func parseTaskInfoText(output string, uuid string) taskInfo {
	ti := taskInfo{UUID: uuid}
	if m := idFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.ID = strings.TrimSpace(m[1])
	}
	if m := uuidFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.UUID = strings.TrimSpace(m[1])
	}
	if m := descFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Description = strings.TrimSpace(m[1])
	}
	if m := statusFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Status = strings.TrimSpace(m[1])
	}
	if m := startedFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Started = strings.TrimSpace(m[1])
	}
	if m := startTimeFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.StartTime = strings.TrimSpace(m[1])
	}
	if m := priorityFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Priority = strings.TrimSpace(m[1])
	}
	if m := dependsFieldRx.FindStringSubmatch(output); len(m) > 1 {
		depStr := strings.TrimSpace(m[1])
		if depStr != "" {
			ti.Depends = strings.Split(depStr, ", ")
		}
	}
	if m := tagsFieldRx.FindStringSubmatch(output); len(m) > 1 {
		tagStr := strings.TrimSpace(m[1])
		ti.Tags = strings.Split(tagStr, ", ")
	}
	return ti
}

func getTaskInfoFast(ctx context.Context, uuid string) (taskInfo, bool) {
	stdout, _, code := runAsk(ctx, []string{"info", uuid})
	if code != 0 {
		return taskInfo{}, false
	}
	return parseTaskInfoText(stdout.String(), uuid), true
}

// getTaskInfoRaw returns the raw text output of ask info for a given UUID.
func getTaskInfoRaw(ctx context.Context, uuid string) (string, bool) {
	stdout, _, code := runAsk(ctx, []string{"info", uuid})
	if code != 0 {
		return "", false
	}
	return stdout.String(), true
}

func mustTaskAlias(t *testing.T, ctx context.Context, uuid string) string {
	t.Helper()

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("failed to get task info for %s", uuid)
	}
	if ti.ID == "" {
		t.Fatalf("task info for %s did not include an alias ID", uuid)
	}
	return ti.ID
}

func aliasCachePath(t *testing.T, cacheRoot string) string {
	t.Helper()
	return filepath.Join(cacheRoot, "hexai", "ask", "task-aliases-v1.json")
}

func TestMain(m *testing.M) {
	repoRoot = findRepoRoot()
	if repoRoot == "" {
		fmt.Fprintln(os.Stderr, "integration tests: cannot find repo root (go.mod or .git)")
		os.Exit(1)
	}
	// Always rebuild the binary so tests reflect the current source.
	askBin := askBinaryPath()
	cmd := exec.Command("go", "build", "-o", askBin, "./cmd/ask/")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build ask binary: %v\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestAdd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for add")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	tasks := listTasksWithTag(ctx, "integrationtest")
	found := false
	for _, task := range tasks {
		if task.UUID == uuid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("task %s not found in export", uuid)
	}
}

// TestAddReturnsAlias verifies that ask add outputs the human-facing alias ID in its creation message.
func TestAddReturnsAlias(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, _, code := runAsk(ctx, []string{"add", "+integrationtest", "uuid format check"})
	if code != 0 {
		t.Fatalf("ask add failed with code %d", code)
	}
	rawOutput := strings.TrimSpace(stdout.String())
	id := extractTaskIDFromAddOutput(rawOutput)
	info, ok := getTaskInfoFast(ctx, id)
	if !ok {
		t.Fatalf("ask info %q failed after add", id)
	}
	defer deleteTask(ctx, info.UUID)

	if id == "" {
		t.Fatal("ask add returned an empty task ID")
	}
	if rawOutput != "created task "+id {
		t.Fatalf("ask add output = %q, want %q", rawOutput, "created task "+id)
	}
	if uuidFormatRx.MatchString(id) {
		t.Fatalf("ask add output %q leaked a UUID, want alias ID", id)
	}
	if info.ID != id {
		t.Fatalf("ask info ID = %q, want %q", info.ID, id)
	}
	if !uuidFormatRx.MatchString(info.UUID) {
		t.Fatalf("ask info UUID = %q, want valid UUID", info.UUID)
	}
}

func TestAddWithDependsModifier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	dep1UUID, err := createTask(ctx, "integration test add depends target one")
	if err != nil {
		t.Fatalf("failed to create first dependency task: %v", err)
	}
	defer deleteTask(ctx, dep1UUID)

	dep2UUID, err := createTask(ctx, "integration test add depends target two")
	if err != nil {
		t.Fatalf("failed to create second dependency task: %v", err)
	}
	defer deleteTask(ctx, dep2UUID)

	dep1Alias := mustTaskAlias(t, ctx, dep1UUID)
	dep2Alias := mustTaskAlias(t, ctx, dep2UUID)

	stdout, stderr, code := runAsk(ctx, []string{
		"add",
		"+integrationtest",
		"depends:" + dep1Alias + "," + dep2Alias,
		"integration",
		"test",
		"task",
		"with",
		"inline",
		"depends",
	})
	if code != 0 {
		t.Fatalf("ask add with depends modifier failed with code %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	id := extractTaskIDFromAddOutput(stdout.String())
	info, ok := getTaskInfoFast(ctx, id)
	if !ok {
		t.Fatalf("ask info %q failed after add", id)
	}
	defer deleteTask(ctx, info.UUID)

	raw, ok := getTaskInfoRaw(ctx, info.UUID)
	if !ok {
		t.Fatalf("raw info for created task %s failed", info.UUID)
	}
	if !strings.Contains(raw, dep1Alias+" ("+dep1UUID+")") || !strings.Contains(raw, dep2Alias+" ("+dep2UUID+")") {
		t.Fatalf("created task info missing formatted dependencies: %s", raw)
	}
}

func TestList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cacheRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	uuid, err := createTask(ctx, "integration test task for list")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"list"})
	if code != 0 {
		t.Fatalf("list failed with code %d: %s", code, stdout.String())
	}
	alias := mustTaskAlias(t, ctx, uuid)
	if !strings.Contains(stdout.String(), alias) {
		t.Errorf("list output does not contain expected alias %q", alias)
	}
	if strings.Contains(stdout.String(), uuid) {
		t.Errorf("list output should not contain raw UUID %s", uuid)
	}
	if !strings.Contains(stdout.String(), "integration test task for list") {
		t.Errorf("list output does not contain expected task description")
	}
}

func TestAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for all")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"all"})
	if code != 0 {
		t.Fatalf("all failed with code %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "integration test task for all") {
		t.Errorf("all output does not contain expected task description")
	}
}

func TestReady(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for ready")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"ready"})
	if code != 0 {
		t.Fatalf("ready failed with code %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "integration test task for ready") {
		t.Errorf("ready output does not contain expected task description")
	}
}

func TestInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	uuid, err := createTask(ctx, "integration test task for info")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("info failed or returned no output")
	}
	if ti.UUID != uuid {
		t.Errorf("info uuid mismatch: got %s, want %s", ti.UUID, uuid)
	}
	if ti.ID == "" {
		t.Errorf("info output missing alias ID")
	}
	if !strings.Contains(ti.Description, "integration test task for info") {
		t.Errorf("info description mismatch: %s", ti.Description)
	}

	aliasOutput, ok := getTaskInfoRaw(ctx, ti.ID)
	if !ok {
		t.Fatalf("info by alias failed")
	}
	if !strings.Contains(aliasOutput, "ID:          "+ti.ID) {
		t.Errorf("info by alias output missing alias line: %s", aliasOutput)
	}
	if !strings.Contains(aliasOutput, "UUID:        "+uuid) {
		t.Errorf("info by alias output missing uuid line: %s", aliasOutput)
	}
}

func TestInfoShowsAllDependencies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	dependency1, err := createTask(ctx, "integration test info dependency one")
	if err != nil {
		t.Fatalf("failed to create first dependency task: %v", err)
	}
	defer deleteTask(ctx, dependency1)

	dependency2, err := createTask(ctx, "integration test info dependency two")
	if err != nil {
		t.Fatalf("failed to create second dependency task: %v", err)
	}
	defer deleteTask(ctx, dependency2)

	dependent, err := createTask(ctx, "integration test task for info dependencies")
	if err != nil {
		t.Fatalf("failed to create dependent task: %v", err)
	}
	defer deleteTask(ctx, dependent)

	if stdout, stderr, code := runAsk(ctx, []string{"dep", "add", dependent, dependency2}); code != 0 {
		t.Fatalf("dep add for second dependency failed with code %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout, stderr, code := runAsk(ctx, []string{"dep", "add", dependent, dependency1}); code != 0 {
		t.Fatalf("dep add for first dependency failed with code %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	ti, ok := getTaskInfoFast(ctx, dependent)
	if !ok {
		t.Fatalf("info failed for task with dependencies")
	}
	if len(ti.Depends) != 2 {
		t.Fatalf("info dependencies count = %d, want 2: %+v", len(ti.Depends), ti.Depends)
	}

	alias1 := mustTaskAlias(t, ctx, dependency1)
	alias2 := mustTaskAlias(t, ctx, dependency2)
	wantDepends := []string{
		alias1 + " (" + dependency1 + ")",
		alias2 + " (" + dependency2 + ")",
	}
	slices.Sort(wantDepends)
	if !slices.Equal(ti.Depends, wantDepends) {
		t.Fatalf("info dependencies = %+v, want %+v", ti.Depends, wantDepends)
	}
}

func TestAnnotate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for annotate")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	note := "this is a test annotation"
	stdout, _, code := runAsk(ctx, []string{"annotate", uuid, note})
	if code != 0 {
		t.Fatalf("annotate failed with code %d: %s", code, stdout.String())
	}

	raw, ok := getTaskInfoRaw(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after annotate")
	}
	if !strings.Contains(raw, note) {
		t.Errorf("annotation text %q not found in task info output:\n%s", note, raw)
	}
}

func TestStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for start")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"start", uuid})
	if code != 0 {
		t.Fatalf("start failed with code %d: %s", code, stdout.String())
	}

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after start")
	}
	if ti.Started != "yes" {
		t.Errorf("task started state = %q, want yes", ti.Started)
	}
	if ti.StartTime == "" {
		t.Errorf("task start time is empty after start")
	}
}

func TestStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for stop")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	runAsk(ctx, []string{"start", uuid})

	stdout, _, code := runAsk(ctx, []string{"stop", uuid})
	if code != 0 {
		t.Fatalf("stop failed with code %d: %s", code, stdout.String())
	}

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after stop")
	}
	if ti.Started != "no" {
		t.Errorf("task started state = %q, want no", ti.Started)
	}
	if ti.StartTime != "" {
		t.Errorf("task start time should be empty after stop: %s", ti.StartTime)
	}
}

func TestDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for done")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	stdout, _, code := runAsk(ctx, []string{"done", uuid})
	if code != 0 {
		t.Fatalf("done failed with code %d: %s", code, stdout.String())
	}

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after done")
	}
	if strings.ToLower(ti.Status) != "completed" {
		t.Errorf("task status = %s, want completed", ti.Status)
	}

	deleteTask(ctx, uuid)
}

func TestPriority(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for priority")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"priority", uuid, "H"})
	if code != 0 {
		t.Fatalf("priority failed with code %d: %s", code, stdout.String())
	}

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after priority")
	}
	if ti.Priority != "H" {
		t.Errorf("task priority = %s, want H", ti.Priority)
	}
}

func TestTag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for tag")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"tag", uuid, "+cli"})
	if code != 0 {
		t.Fatalf("tag add failed with code %d: %s", code, stdout.String())
	}

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after tag")
	}
	found := false
	for _, tg := range ti.Tags {
		if tg == "cli" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tag cli not found on task: %+v", ti.Tags)
	}

	runAsk(ctx, []string{"tag", uuid, "-cli"})

	ti2, _ := getTaskInfoFast(ctx, uuid)
	for _, tg := range ti2.Tags {
		if tg == "cli" {
			t.Errorf("tag cli should have been removed")
			break
		}
	}
}

func TestDepAdd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid1, err := createTask(ctx, "integration test dep target")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid1)

	uuid2, err := createTask(ctx, "integration test dep dependent")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid2)

	stdout, _, code := runAsk(ctx, []string{"dep", "add", uuid2, uuid1})
	if code != 0 {
		t.Fatalf("dep add failed with code %d: %s", code, stdout.String())
	}

	tasks := listTasksWithTag(ctx, "integrationtest")
	for _, task := range tasks {
		if task.UUID == uuid2 {
			found := false
			for _, dep := range task.Depends {
				if dep == uuid1 {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("dependency %s not found on task", uuid1)
			}
			break
		}
	}
}

func TestDepList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	uuid1, err := createTask(ctx, "integration test dep list target")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid1)

	uuid2, err := createTask(ctx, "integration test dep list dependent")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid2)

	runAsk(ctx, []string{"dep", "add", uuid2, uuid1})

	stdout, _, code := runAsk(ctx, []string{"dep", "list", uuid2})
	if code != 0 {
		t.Fatalf("dep list failed with code %d: %s", code, stdout.String())
	}
	alias1 := mustTaskAlias(t, ctx, uuid1)
	if !strings.Contains(stdout.String(), alias1) {
		t.Errorf("dep list output does not contain target alias %q: %s", alias1, stdout.String())
	}
	if strings.Contains(stdout.String(), uuid1) {
		t.Errorf("dep list output should not contain raw target uuid %s: %s", uuid1, stdout.String())
	}
}

func TestDepRm(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid1, err := createTask(ctx, "integration test dep rm target")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid1)

	uuid2, err := createTask(ctx, "integration test dep rm dependent")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid2)

	runAsk(ctx, []string{"dep", "add", uuid2, uuid1})

	stdout, _, code := runAsk(ctx, []string{"dep", "rm", uuid2, uuid1})
	if code != 0 {
		t.Fatalf("dep rm failed with code %d: %s", code, stdout.String())
	}

	tasks := listTasksWithTag(ctx, "integrationtest")
	for _, task := range tasks {
		if task.UUID == uuid2 {
			for _, dep := range task.Depends {
				if dep == uuid1 {
					t.Errorf("dependency should have been removed")
					break
				}
			}
			break
		}
	}
}

func TestModify(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for modify")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"modify", uuid, "priority:H"})
	if code != 0 {
		t.Fatalf("modify failed with code %d: %s", code, stdout.String())
	}

	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok {
		t.Fatalf("could not get task info after modify")
	}
	if ti.Priority != "H" {
		t.Errorf("task priority = %s, want H", ti.Priority)
	}
}

func TestDenotate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for denotate")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	note := "annotation to remove"
	runAsk(ctx, []string{"annotate", uuid, note})

	// Verify the annotation is present before denotating.
	rawBefore, _ := getTaskInfoRaw(ctx, uuid)
	if !strings.Contains(rawBefore, note) {
		t.Fatalf("annotation %q not found before denotate", note)
	}

	_, _, code := runAsk(ctx, []string{"denotate", uuid, note})
	if code != 0 {
		t.Fatalf("denotate returned non-zero code: %d", code)
	}

	// Verify the annotation is gone after denotating.
	rawAfter, _ := getTaskInfoRaw(ctx, uuid)
	if strings.Contains(rawAfter, note) {
		t.Errorf("annotation %q still present after denotate", note)
	}
}

func TestDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for delete")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// delete forwards stdin to taskwarrior for confirmation.
	stdout, _, code := runAskWithStdin(ctx, []string{"delete", uuid}, "yes\n")
	if code != 0 {
		t.Fatalf("delete failed with code %d: %s", code, stdout.String())
	}

	tasks := listTasksWithTag(ctx, "integrationtest")
	for _, task := range tasks {
		if task.UUID == uuid {
			t.Errorf("task should have been deleted but still exists")
			break
		}
	}
}

func TestUrgency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for urgency")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"urgency"})
	if code != 0 {
		t.Fatalf("urgency failed with code %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "integration test task for urgency") {
		t.Errorf("urgency output does not contain expected task description")
	}
}

func TestDefaultCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test for default command")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{})
	if code != 0 {
		t.Fatalf("default command (list) failed with code %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "integration test for default command") {
		t.Errorf("default command output does not contain expected task description")
	}
}

func TestHelp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, _, code := runAsk(ctx, []string{"help"})
	if code != 0 {
		t.Fatalf("help returned non-zero exit code %d", code)
	}
	out := stdout.String()
	for _, sub := range []string{"add", "list", "info", "start", "done", "delete", "annotate", "dep"} {
		if !strings.Contains(out, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestFish(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, code := runAsk(ctx, []string{"fish"})
	if code != 0 {
		t.Fatalf("fish returned non-zero exit code %d: stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, fragment := range []string{
		"# Source with: ask fish | source",
		"complete -c",
		"complete-uuids",
		"annotate",
		"delete",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("fish output missing %q", fragment)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("fish wrote unexpected stderr: %s", stderr.String())
	}
}

func TestFishRejectsExtraArgs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, code := runAsk(ctx, []string{"fish", "extra"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code for fish extra args, got 0")
	}
	if stdout.Len() != 0 {
		t.Errorf("fish with extra args wrote unexpected stdout: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: ask fish") {
		t.Errorf("fish with extra args stderr missing usage text: %s", stderr.String())
	}
}

func TestCompleteUUIDs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	uuid, err := createTask(ctx, "integration test task for complete-uuids")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, stderr, code := runAsk(ctx, []string{"complete-uuids"})
	if code != 0 {
		t.Fatalf("complete-uuids returned non-zero exit code %d: stderr=%s", code, stderr.String())
	}
	alias := mustTaskAlias(t, ctx, uuid)
	if !strings.Contains(stdout.String(), alias) {
		t.Errorf("complete-uuids output does not contain created task alias %s", alias)
	}
	if !strings.Contains(stdout.String(), uuid) {
		t.Errorf("complete-uuids output does not contain created task UUID %s", uuid)
	}
}

func TestAliasSelectorsAcrossUUIDCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	uuid, err := createTask(ctx, "integration test task for alias selectors")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	alias := mustTaskAlias(t, ctx, uuid)

	infoOut, ok := getTaskInfoRaw(ctx, alias)
	if !ok {
		t.Fatalf("info by alias failed")
	}
	if !strings.Contains(infoOut, "UUID:        "+uuid) {
		t.Fatalf("info by alias did not resolve the task: %s", infoOut)
	}

	note := "integration alias annotation"
	stdout, _, code := runAsk(ctx, []string{"annotate", alias, note})
	if code != 0 {
		t.Fatalf("annotate by alias failed with code %d: %s", code, stdout.String())
	}
	if strings.TrimSpace(stdout.String()) != "ok "+alias {
		t.Fatalf("annotate output = %q, want ok %s", stdout.String(), alias)
	}

	raw, ok := getTaskInfoRaw(ctx, uuid)
	if !ok || !strings.Contains(raw, note) {
		t.Fatalf("annotation %q not found after alias annotate: %s", note, raw)
	}

	note2 := "remove me via alias"
	if _, _, code = runAsk(ctx, []string{"annotate", alias, note2}); code != 0 {
		t.Fatalf("setup annotate for denotate failed with code %d", code)
	}
	stdout, _, code = runAsk(ctx, []string{"denotate", alias, note2})
	if code != 0 {
		t.Fatalf("denotate by alias failed with code %d: %s", code, stdout.String())
	}
	raw, ok = getTaskInfoRaw(ctx, uuid)
	if !ok || strings.Contains(raw, note2) {
		t.Fatalf("annotation %q still present after alias denotate: %s", note2, raw)
	}

	stdout, _, code = runAsk(ctx, []string{"start", alias})
	if code != 0 {
		t.Fatalf("start by alias failed with code %d: %s", code, stdout.String())
	}
	ti, ok := getTaskInfoFast(ctx, uuid)
	if !ok || ti.Started != "yes" {
		t.Fatalf("task not started after alias start: %+v", ti)
	}

	stdout, _, code = runAsk(ctx, []string{"stop", alias})
	if code != 0 {
		t.Fatalf("stop by alias failed with code %d: %s", code, stdout.String())
	}
	ti, ok = getTaskInfoFast(ctx, uuid)
	if !ok || ti.Started != "no" {
		t.Fatalf("task not stopped after alias stop: %+v", ti)
	}

	stdout, _, code = runAsk(ctx, []string{"priority", alias, "H"})
	if code != 0 {
		t.Fatalf("priority by alias failed with code %d: %s", code, stdout.String())
	}
	ti, ok = getTaskInfoFast(ctx, uuid)
	if !ok || ti.Priority != "H" {
		t.Fatalf("task priority not updated after alias priority: %+v", ti)
	}

	stdout, _, code = runAsk(ctx, []string{"modify", alias, "priority:L"})
	if code != 0 {
		t.Fatalf("modify by alias failed with code %d: %s", code, stdout.String())
	}
	ti, ok = getTaskInfoFast(ctx, uuid)
	if !ok || ti.Priority != "L" {
		t.Fatalf("task priority not updated after alias modify: %+v", ti)
	}

	stdout, _, code = runAsk(ctx, []string{"tag", alias, "+aliascheck"})
	if code != 0 {
		t.Fatalf("tag by alias failed with code %d: %s", code, stdout.String())
	}
	ti, ok = getTaskInfoFast(ctx, uuid)
	if !ok || !slices.Contains(ti.Tags, "aliascheck") {
		t.Fatalf("tag not added after alias tag: %+v", ti)
	}

	depUUID, err := createTask(ctx, "integration test task dependency alias target")
	if err != nil {
		t.Fatalf("failed to create dependency task: %v", err)
	}
	defer deleteTask(ctx, depUUID)

	depAlias := mustTaskAlias(t, ctx, depUUID)
	stdout, _, code = runAsk(ctx, []string{"dep", "add", alias, depAlias})
	if code != 0 {
		t.Fatalf("dep add by alias failed with code %d: %s", code, stdout.String())
	}

	stdout, _, code = runAsk(ctx, []string{"dep", "list", alias})
	if code != 0 {
		t.Fatalf("dep list by alias failed with code %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), depAlias) || strings.Contains(stdout.String(), depUUID) {
		t.Fatalf("dep list by alias output = %q, want alias %q without raw UUID", stdout.String(), depAlias)
	}

	stdout, _, code = runAsk(ctx, []string{"dep", "rm", alias, depAlias})
	if code != 0 {
		t.Fatalf("dep rm by alias failed with code %d: %s", code, stdout.String())
	}
	stdout, _, code = runAsk(ctx, []string{"dep", "list", alias})
	if code != 0 {
		t.Fatalf("dep list after rm failed with code %d: %s", code, stdout.String())
	}
	if strings.TrimSpace(stdout.String()) != "no dependencies" {
		t.Fatalf("dep list after rm = %q, want no dependencies", stdout.String())
	}

	doneUUID, err := createTask(ctx, "integration test alias done")
	if err != nil {
		t.Fatalf("failed to create done task: %v", err)
	}
	doneAlias := mustTaskAlias(t, ctx, doneUUID)
	stdout, _, code = runAsk(ctx, []string{"done", doneAlias})
	if code != 0 {
		t.Fatalf("done by alias failed with code %d: %s", code, stdout.String())
	}
	doneInfo, ok := getTaskInfoFast(ctx, doneUUID)
	if !ok || strings.ToLower(doneInfo.Status) != "completed" {
		t.Fatalf("done task not completed after alias done: %+v", doneInfo)
	}
	deleteTask(ctx, doneUUID)

	deleteUUID, err := createTask(ctx, "integration test alias delete")
	if err != nil {
		t.Fatalf("failed to create delete task: %v", err)
	}
	deleteAlias := mustTaskAlias(t, ctx, deleteUUID)
	stdout, _, code = runAskWithStdin(ctx, []string{"delete", deleteAlias}, "yes\n")
	if code != 0 {
		t.Fatalf("delete by alias failed with code %d: %s", code, stdout.String())
	}

	tasks := listTasksWithTag(ctx, "integrationtest")
	for _, task := range tasks {
		if task.UUID == deleteUUID {
			t.Fatalf("task %s still exists after alias delete", deleteUUID)
		}
	}
}

func TestAliasCachePrunesExpiredEntriesOlderThan120Days(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cacheRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	uuid, err := createTask(ctx, "integration test alias cache pruning")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	cachePath := aliasCachePath(t, cacheRoot)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", cachePath, err)
	}
	seed := `{
  "next_id": 37,
  "entries": [
    {
      "uuid": "expired-task",
      "alias": "z",
      "created_at": "2025-01-01T00:00:00Z",
      "last_accessed_at": "2025-01-01T00:00:00Z"
    }
  ]
}`
	if err := os.WriteFile(cachePath, []byte(seed), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", cachePath, err)
	}

	stdout, stderr, code := runAsk(ctx, []string{"info", uuid})
	if code != 0 {
		t.Fatalf("info failed with code %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "ID:          z\n") {
		t.Fatalf("info output still contains pruned alias z: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ID:          01\n") {
		t.Fatalf("info output did not allocate the next monotonic alias 01: %q", stdout.String())
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", cachePath, err)
	}
	if strings.Contains(string(data), "expired-task") {
		t.Fatalf("expired cache entry was not pruned: %s", string(data))
	}
	if !strings.Contains(string(data), `"next_id": 38`) {
		t.Fatalf("cache next_id was not advanced after pruning and allocation: %s", string(data))
	}
}

func TestUnknownCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, stderr, code := runAsk(ctx, []string{"notacommand"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code for unknown command, got 0")
	}
	if !strings.Contains(stderr.String(), "notacommand") {
		t.Errorf("error output does not mention unknown command: %s", stderr.String())
	}
}
