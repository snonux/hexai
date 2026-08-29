package askcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (d *Dispatcher) handleList(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, []string{"status:pending"}, args[1:], stdout, stderr)
}

func (d *Dispatcher) handleAll(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, nil, args[1:], stdout, stderr)
}

func (d *Dispatcher) handleReady(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, []string{"+READY"}, args[1:], stdout, stderr)
}

func (d *Dispatcher) handleCompleted(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, []string{"status:completed"}, args[1:], stdout, stderr)
}

// dateFilterAttrs are taskwarrior date attributes usable for time-range
// filtering. The end attribute is stamped when a task is completed.
var dateFilterAttrs = []string{"end", "modified", "created", "due", "scheduled", "waiting", "start"}

// dateFilterModifiers are the taskwarrior attribute modifiers allowed on date
// attributes, e.g. end.after:7.days.ago, end.before:today, end.by:2026-08-29.
var dateFilterModifiers = []string{"before", "after", "by", "is", "isnt", "none", "any"}

// isPassThroughFilter reports whether a user-supplied arg may be forwarded to
// taskwarrior as a filter modifier (limit:, sort:, +tag, started, and date
// attributes with optional before/after/by modifiers).
func isPassThroughFilter(arg string) bool {
	if strings.HasPrefix(arg, "limit:") || strings.HasPrefix(arg, "sort:") ||
		strings.HasPrefix(arg, "+") || arg == "started" {
		return true
	}
	for _, attr := range dateFilterAttrs {
		if strings.HasPrefix(arg, attr+":") {
			return true
		}
		for _, mod := range dateFilterModifiers {
			if strings.HasPrefix(arg, attr+"."+mod+":") {
				return true
			}
		}
	}
	return false
}

// handleListWithFilters is the shared implementation for list/all/ready.
// initialFilters seeds the taskwarrior filter; extraArgs are user-supplied
// filter modifiers (limit:, sort:, +tag, started, date attributes, since:).
func (d *Dispatcher) handleListWithFilters(ctx context.Context, initialFilters, extraArgs []string, stdout, stderr io.Writer) (int, error) {
	filterArgs := append([]string(nil), initialFilters...)
	for _, arg := range extraArgs {
		if value, ok := strings.CutPrefix(arg, "since:"); ok {
			filter, err := resolveSince(value, d.now())
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1, nil
			}
			filterArgs = append(filterArgs, filter)
			continue
		}
		if isPassThroughFilter(arg) {
			filterArgs = append(filterArgs, arg)
		}
	}
	filterArgs = append(filterArgs, "export")
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, filterArgs, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
		return 1, nil
	}
	sort.Slice(tasks, func(i, j int) bool {
		pi := priorityOrder(tasks[i].Priority)
		pj := priorityOrder(tasks[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return tasks[i].Urgency > tasks[j].Urgency
	})
	return renderTaskListWithAliasLoader(tasks, stdout, stderr, d.jsonOutput, d.aliasCache.withDefaults().ensureTaskAliases)
}

func priorityOrder(p string) int {
	switch p {
	case "H":
		return 1
	case "M":
		return 2
	case "L":
		return 3
	default:
		return 4
	}
}

// resolveSince converts a since:<value> spec into an absolute taskwarrior
// end.after: filter. taskwarrior 2.x relative date values (e.g. 7.days.ago)
// are unreliable in filters, so the lower boundary is computed here as an
// absolute timestamp from now. The end attribute is stamped on completion,
// so since: lists tasks completed at or after the boundary.
func resolveSince(value string, now time.Time) (string, error) {
	switch value {
	case "today":
		return "end:today", nil
	case "this.week", "week":
		return "end.after:" + startOfWeek(now).Format("2006-01-02"), nil
	case "this.month", "month":
		return "end.after:" + startOfMonth(now).Format("2006-01-02"), nil
	}
	if n, unit, ok := parseSinceDuration(value); ok {
		boundary := addSinceUnit(now, -n, unit)
		return "end.after:" + boundary.Format("2006-01-02T15:04"), nil
	}
	return "", fmt.Errorf("invalid since value %q (use today, this.week, this.month, or N.hours/N.days/N.weeks/N.months)", value)
}

// parseSinceDuration parses N.hours / N.days / N.weeks / N.months, optionally
// suffixed with .ago (e.g. 7.days.ago).
func parseSinceDuration(value string) (int, string, bool) {
	v := strings.TrimSuffix(value, ".ago")
	for _, unit := range []string{"hours", "days", "weeks", "months"} {
		suffix := "." + unit
		if strings.HasSuffix(v, suffix) {
			n, err := strconv.Atoi(strings.TrimSuffix(v, suffix))
			if err != nil || n < 0 {
				return 0, "", false
			}
			return n, unit, true
		}
	}
	return 0, "", false
}

// addSinceUnit returns t shifted by n units (n may be negative).
func addSinceUnit(t time.Time, n int, unit string) time.Time {
	switch unit {
	case "hours":
		return t.Add(time.Duration(n) * time.Hour)
	case "days":
		return t.AddDate(0, 0, n)
	case "weeks":
		return t.AddDate(0, 0, 7*n)
	case "months":
		return t.AddDate(0, n, 0)
	}
	return t
}

// startOfWeek returns midnight at the start of now's week (Monday).
func startOfWeek(now time.Time) time.Time {
	d := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
}

// startOfMonth returns midnight on the first day of now's month.
func startOfMonth(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}
