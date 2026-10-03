package askcli

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestHandleList_Success(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	deps := testTaskAliasCacheDeps(dir, &now)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "0", CreatedAt: now},
			{UUID: "uuid-2", Alias: "1", CreatedAt: now},
		},
	}, deps)

	jsonData := `[{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"H","tags":["cli"],"start":"2026-03-26T10:00:00Z","urgency":15.0,"depends":[]},{"uuid":"uuid-2","description":"Task 2","status":"completed","priority":"M","tags":["agent"],"urgency":10.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				_, _ = io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	d.aliasCache = deps
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"list"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "ID") || strings.Contains(output, "UUID") {
		t.Fatalf("output should use ID column: %s", output)
	}
	if !strings.Contains(output, "0") || !strings.Contains(output, "1") || strings.Contains(output, "uuid-1") || strings.Contains(output, "uuid-2") {
		t.Fatalf("output missing aliases or leaking UUIDs: %s", output)
	}
	if !strings.Contains(output, "Started") || !strings.Contains(output, "yes") || !strings.Contains(output, "no") {
		t.Fatalf("output missing explicit started state: %s", output)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if lines[len(lines)-1] != "2 tasks listed" {
		t.Fatalf("last line = %q, want %q", lines[len(lines)-1], "2 tasks listed")
	}
}

func TestHandleList_SortedByPriority(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	deps := testTaskAliasCacheDeps(dir, &now)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "0", CreatedAt: now},
			{UUID: "uuid-2", Alias: "1", CreatedAt: now},
		},
	}, deps)

	jsonData := `[{"uuid":"uuid-2","description":"Task 2","status":"pending","priority":"M","tags":[],"urgency":10.0,"depends":[]},{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"H","tags":[],"urgency":5.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				_, _ = io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	d.aliasCache = deps
	var stdout bytes.Buffer
	d.Dispatch(context.Background(), []string{"list"}, nil, &stdout, &bytes.Buffer{})
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	taskLine1 := lines[2]
	if !strings.Contains(taskLine1, "0") || strings.Contains(taskLine1, "uuid-1") {
		t.Fatalf("first task should be H priority alias 0, got: %s", taskLine1)
	}
}

func TestHandleList_EmptyList(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				_, _ = io.WriteString(stdout, "[]")
				return 0, nil
			}
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"list"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code = %d, want 0 for empty list", code)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("empty list should still print header, separator, and count: %q", stdout.String())
	}
	if lines[len(lines)-1] != "0 tasks listed" {
		t.Fatalf("last line = %q, want %q", lines[len(lines)-1], "0 tasks listed")
	}
}

func TestHandleAll_Success(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	deps := testTaskAliasCacheDeps(dir, &now)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "0", CreatedAt: now},
		},
	}, deps)

	jsonData := `[{"uuid":"uuid-1","description":"Done task","status":"completed","priority":"M","tags":[],"urgency":0.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				_, _ = io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	d.aliasCache = deps
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"all"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("all code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "0") || strings.Contains(stdout.String(), "uuid-1") {
		t.Fatalf("output should show alias only: %s", stdout.String())
	}
}

func TestHandleReady_Success(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	deps := testTaskAliasCacheDeps(dir, &now)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-ready", Alias: "0", CreatedAt: now},
		},
	}, deps)

	jsonData := `[{"uuid":"uuid-ready","description":"Ready task","status":"pending","priority":"H","tags":["READY"],"urgency":20.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				_, _ = io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	d.aliasCache = deps
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"ready"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ready code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "0") || strings.Contains(stdout.String(), "uuid-ready") {
		t.Fatalf("output should show alias only: %s", stdout.String())
	}
}

func TestHandleCompleted_Success(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	deps := testTaskAliasCacheDeps(dir, &now)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-done", Alias: "0", CreatedAt: now},
		},
	}, deps)

	jsonData := `[{"uuid":"uuid-done","description":"Done task","status":"completed","priority":"M","tags":[],"urgency":0.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				_, _ = io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	d.aliasCache = deps
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"completed"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("completed code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "0") || strings.Contains(stdout.String(), "uuid-done") {
		t.Fatalf("output should show alias only: %s", stdout.String())
	}
}

func TestHandleList_PassesFilters(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"list", "+READY", "limit:5", "sort:priority-"}, nil, &stdout, &stderr)
	if len(capturedArgs) < 2 {
		t.Fatalf("expected export args, got %v", capturedArgs)
	}
	hasExport := false
	for _, arg := range capturedArgs {
		if arg == "export" {
			hasExport = true
			break
		}
	}
	if !hasExport {
		t.Fatalf("expected export in args, got %v", capturedArgs)
	}
}

func TestIsPassThroughFilter(t *testing.T) {
	cases := []struct {
		arg  string
		want bool
	}{
		{arg: "limit:5", want: true},
		{arg: "sort:priority-", want: true},
		{arg: "+READY", want: true},
		{arg: "+agent", want: true},
		{arg: "started", want: true},
		{arg: "end:today", want: true},
		{arg: "end:7.days", want: true},
		{arg: "end:2.hours.ago", want: true},
		{arg: "modified:7.days", want: true},
		{arg: "end.after:2026-08-22", want: true},
		{arg: "end.before:today", want: true},
		{arg: "end.by:2026-08-29", want: true},
		{arg: "due.after:2026-01-01", want: true},
		{arg: "status:pending", want: false},
		{arg: "foo:bar", want: false},
		{arg: "project:hexai", want: false},
		{arg: "end.unknownmod:today", want: false},
		{arg: "end", want: false},
		{arg: "", want: false},
	}
	for _, tc := range cases {
		if got := isPassThroughFilter(tc.arg); got != tc.want {
			t.Errorf("isPassThroughFilter(%q) = %v, want %v", tc.arg, got, tc.want)
		}
	}
}

func TestResolveSince(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) // a Saturday
	cases := []struct {
		value string
		want  string
		err   bool
	}{
		{value: "today", want: "end:today"},
		{value: "this.week", want: "end.after:2026-08-24"},
		{value: "week", want: "end.after:2026-08-24"},
		{value: "this.month", want: "end.after:2026-08-01"},
		{value: "month", want: "end.after:2026-08-01"},
		{value: "24.hours", want: "end.after:2026-08-28T12:00"},
		{value: "6.hours", want: "end.after:2026-08-29T06:00"},
		{value: "7.days", want: "end.after:2026-08-22T12:00"},
		{value: "7.days.ago", want: "end.after:2026-08-22T12:00"},
		{value: "2.weeks", want: "end.after:2026-08-15T12:00"},
		{value: "1.months", want: "end.after:2026-07-29T12:00"},
		{value: "foo", err: true},
		{value: "1.5.days", err: true},
		{value: "-3.days", err: true},
		{value: "10001.months", err: true},
		{value: "", err: true},
	}
	for _, tc := range cases {
		got, err := resolveSince(tc.value, now)
		if tc.err {
			if err == nil {
				t.Errorf("resolveSince(%q) expected error, got %q", tc.value, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveSince(%q) unexpected error: %v", tc.value, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveSince(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestHandleCompleted_PassesSinceFilter(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	d.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"completed", "since:7.days"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("completed code = %d, want 0", code)
	}
	if !containsArg(capturedArgs, "status:completed") {
		t.Fatalf("expected status:completed in args, got %v", capturedArgs)
	}
	if !containsArg(capturedArgs, "end.after:2026-08-22T12:00") {
		t.Fatalf("expected end.after:2026-08-22T12:00 in args, got %v", capturedArgs)
	}
}

func TestHandleCompleted_InvalidSince(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"completed", "since:foo"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1 for invalid since value", code)
	}
	if !strings.Contains(stderr.String(), "invalid since value") {
		t.Fatalf("expected invalid-since error, got %q", stderr.String())
	}
}

func TestHandleCompleted_DropsNonDateFilters(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"completed", "status:pending", "foo:bar", "end:7.days"}, nil, &stdout, &stderr)
	if containsArg(capturedArgs, "status:pending") {
		t.Fatalf("status:pending should not be forwarded, got %v", capturedArgs)
	}
	if containsArg(capturedArgs, "foo:bar") {
		t.Fatalf("foo:bar should not be forwarded, got %v", capturedArgs)
	}
	if !containsArg(capturedArgs, "end:7.days") {
		t.Fatalf("expected end:7.days in args, got %v", capturedArgs)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestResolveDueWithin(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) // a Saturday
	cases := []struct {
		value string
		want  string
		err   bool
	}{
		{value: "today", want: "due.before:2026-08-30"},
		{value: "this.week", want: "due.before:2026-08-31"},
		{value: "week", want: "due.before:2026-08-31"},
		{value: "this.month", want: "due.before:2026-09-01"},
		{value: "month", want: "due.before:2026-09-01"},
		{value: "6.hours", want: "due.by:2026-08-29T18:00"},
		{value: "7.days", want: "due.by:2026-09-05T12:00"},
		{value: "0.days", want: "due.by:2026-08-29T12:00"},
		{value: "2.weeks", want: "due.by:2026-09-12T12:00"},
		{value: "1.months", want: "due.by:2026-09-29T12:00"},
		{value: "7.days.ago", err: true},
		{value: "10000.days", want: "due.by:2054-01-14T12:00"},
		{value: "10001.days", err: true},
		{value: "9999999999999.hours", err: true},
		{value: "99999999999999999999.weeks", err: true},
		{value: "foo", err: true},
		{value: "1.5.days", err: true},
		{value: "-3.days", err: true},
		{value: "", err: true},
	}
	for _, tc := range cases {
		got, err := resolveDueWithin(tc.value, now)
		if tc.err {
			if err == nil {
				t.Errorf("resolveDueWithin(%q) expected error, got %q", tc.value, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveDueWithin(%q) unexpected error: %v", tc.value, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveDueWithin(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestResolveDateShortcut(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		arg  string
		want string
		ok   bool
		err  bool
	}{
		{arg: "since:7.days", want: "end.after:2026-08-22T12:00", ok: true},
		{arg: "due-within:7.days", want: "due.by:2026-09-05T12:00", ok: true},
		{arg: "due-window:7.days", want: "(due.none: or due.by:2026-09-05T12:00)", ok: true},
		{arg: "due-window:today", want: "(due.none: or due.before:2026-08-30)", ok: true},
		{arg: "due-within:bogus", ok: true, err: true},
		{arg: "due-window:", ok: true, err: true},
		{arg: "due:today", ok: false},
		{arg: "due.by:now+7days", ok: false},
		{arg: "+auto", ok: false},
	}
	for _, tc := range cases {
		got, ok, err := resolveDateShortcut(tc.arg, now)
		if ok != tc.ok || (err != nil) != tc.err {
			t.Errorf("resolveDateShortcut(%q) ok=%v err=%v, want ok=%v err=%v", tc.arg, ok, err, tc.ok, tc.err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveDateShortcut(%q) = %q, want %q", tc.arg, got, tc.want)
		}
	}
}

func TestHandleReady_PassesDueWindowFilter(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	d.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"ready", "+auto", "due-window:7.days"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ready code = %d, want 0 (stderr %q)", code, stderr.String())
	}
	for _, want := range []string{"+READY", "+auto", "(due.none: or due.by:2026-09-05T12:00)"} {
		if !containsArg(capturedArgs, want) {
			t.Fatalf("expected %q in args, got %v", want, capturedArgs)
		}
	}
}

func TestHandleList_InvalidDueWithin(t *testing.T) {
	called := false
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		called = true
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"list", "due-within:7.days.ago"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1 for invalid due value", code)
	}
	if called {
		t.Fatalf("taskwarrior must not run for an invalid due value")
	}
	if !strings.Contains(stderr.String(), "invalid due value") {
		t.Fatalf("expected invalid-due error, got %q", stderr.String())
	}
}

func TestAddMonthsClamped(t *testing.T) {
	loc := time.FixedZone("EEST", 3*3600)
	cases := []struct {
		from time.Time
		n    int
		want time.Time
	}{
		{time.Date(2026, 1, 31, 9, 30, 0, 0, loc), 1, time.Date(2026, 2, 28, 9, 30, 0, 0, loc)},
		{time.Date(2028, 1, 31, 9, 30, 0, 0, loc), 1, time.Date(2028, 2, 29, 9, 30, 0, 0, loc)},
		{time.Date(2026, 3, 31, 0, 0, 0, 0, loc), -1, time.Date(2026, 2, 28, 0, 0, 0, 0, loc)},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, loc), 2, time.Date(2027, 2, 28, 0, 0, 0, 0, loc)},
		{time.Date(2026, 8, 29, 12, 0, 0, 0, loc), -1, time.Date(2026, 7, 29, 12, 0, 0, 0, loc)},
		{time.Date(2026, 8, 15, 12, 0, 0, 0, loc), 0, time.Date(2026, 8, 15, 12, 0, 0, 0, loc)},
	}
	for _, tc := range cases {
		if got := addMonthsClamped(tc.from, tc.n); !got.Equal(tc.want) {
			t.Errorf("addMonthsClamped(%v, %d) = %v, want %v", tc.from, tc.n, got, tc.want)
		}
	}
}

func TestResolveDueWithin_MonthEndAndLocalTime(t *testing.T) {
	loc := time.FixedZone("EEST", 3*3600)
	now := time.Date(2026, 1, 31, 23, 30, 0, 0, loc)
	if got, _ := resolveDueWithin("1.months", now); got != "due.by:2026-02-28T23:30" {
		t.Fatalf("1.months from Jan 31 = %q, want due.by:2026-02-28T23:30", got)
	}
	// Boundaries are formatted in now's (local) zone, as taskwarrior expects.
	if got, _ := resolveDueWithin("today", now); got != "due.before:2026-02-01" {
		t.Fatalf("today = %q, want due.before:2026-02-01", got)
	}
	if got, _ := resolveSince("1.months", time.Date(2026, 3, 31, 12, 0, 0, 0, loc)); got != "end.after:2026-02-28T12:00" {
		t.Fatalf("since 1.months from Mar 31 = %q, want end.after:2026-02-28T12:00", got)
	}
}

func TestHandleList_TranslatesStarted(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	if code, _ := d.Dispatch(context.Background(), []string{"list", "started", "startedx"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("list code = %d, want 0 (stderr %q)", code, stderr.String())
	}
	// started becomes +ACTIVE; the unknown startedx is dropped.
	want := []string{"status:pending", "+ACTIVE", "export"}
	if !slices.Equal(capturedArgs, want) {
		t.Fatalf("args = %v, want %v", capturedArgs, want)
	}
}
