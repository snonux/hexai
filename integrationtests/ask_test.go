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
	"strings"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/askcli"
)

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

func init() {
	repoRoot = findRepoRoot()
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
// ask add outputs the UUID directly (via rc.verbose=new-uuid), so no follow-up lookup is needed.
func createTask(ctx context.Context, desc string) (string, error) {
	stdout, stderr, code := runAsk(ctx, []string{"add", "+integrationtest", desc})
	if code != 0 {
		return "", fmt.Errorf("create task failed (code %d): stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	uuid := strings.TrimSpace(stdout.String())
	if uuid == "" {
		return "", fmt.Errorf("could not extract UUID from ask add output: %s", stdout.String())
	}
	return uuid, nil
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
	UUID        string
	Description string
	Status      string
	Priority    string
	Tags        []string
	Start       string
}

var (
	uuidFieldRx     = regexp.MustCompile(`UUID:\s+(.+)`)
	descFieldRx     = regexp.MustCompile(`Description:\s+(.+)`)
	statusFieldRx   = regexp.MustCompile(`Status:\s+(.+)`)
	priorityFieldRx = regexp.MustCompile(`Priority:\s+(.+)`)
	tagsFieldRx     = regexp.MustCompile(`Tags:\s+(.+)`)
	startFieldRx    = regexp.MustCompile(`Started:\s+(.+)`)
	uuidFormatRx    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func parseTaskInfoText(output string, uuid string) taskInfo {
	ti := taskInfo{UUID: uuid}
	if m := uuidFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.UUID = strings.TrimSpace(m[1])
	}
	if m := descFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Description = strings.TrimSpace(m[1])
	}
	if m := statusFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Status = strings.TrimSpace(m[1])
	}
	if m := priorityFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Priority = strings.TrimSpace(m[1])
	}
	if m := tagsFieldRx.FindStringSubmatch(output); len(m) > 1 {
		tagStr := strings.TrimSpace(m[1])
		ti.Tags = strings.Split(tagStr, ", ")
	}
	if m := startFieldRx.FindStringSubmatch(output); len(m) > 1 {
		ti.Start = strings.TrimSpace(m[1])
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

func TestMain(m *testing.M) {
	if repoRoot == "" {
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

// TestAddReturnsUUID verifies that ask add outputs a UUID, never a numeric task ID.
func TestAddReturnsUUID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, _, code := runAsk(ctx, []string{"add", "+integrationtest", "uuid format check"})
	if code != 0 {
		t.Fatalf("ask add failed with code %d", code)
	}
	uuid := strings.TrimSpace(stdout.String())
	defer deleteTask(ctx, uuid)

	if !uuidFormatRx.MatchString(uuid) {
		t.Errorf("ask add output %q is not a valid UUID", uuid)
	}
}

func TestList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uuid, err := createTask(ctx, "integration test task for list")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	defer deleteTask(ctx, uuid)

	stdout, _, code := runAsk(ctx, []string{"list"})
	if code != 0 {
		t.Fatalf("list failed with code %d: %s", code, stdout.String())
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
	if !strings.Contains(ti.Description, "integration test task for info") {
		t.Errorf("info description mismatch: %s", ti.Description)
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
	if ti.Start == "" {
		t.Errorf("task start field is empty after start")
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
	if ti.Start != "" {
		t.Errorf("task start field is not empty after stop: %s", ti.Start)
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
	if !strings.Contains(stdout.String(), uuid1) {
		t.Errorf("dep list output does not contain target uuid: %s", stdout.String())
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
