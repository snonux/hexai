package askcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// completionDescriptionMaxLen is the maximum number of characters of a task
// description included in fish shell completion suggestions.
const completionDescriptionMaxLen = 60

func (d *Dispatcher) handleCompleteUUIDs(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.completeTaskSelectors(ctx, args, stdout, stderr, taskCompletionItems)
}

func (d *Dispatcher) handleCompleteAliases(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.completeTaskSelectors(ctx, args, stdout, stderr, taskCompletionAliasItems)
}

// taskCompletionLinesFn builds tab-separated "selector\tdescription" lines for
// completion output (full list vs alias-only for Fish).
type taskCompletionLinesFn func(tasks []TaskExport, aliases map[string]string) []string

func (d *Dispatcher) completeTaskSelectors(ctx context.Context, args []string, stdout, stderr io.Writer, lines taskCompletionLinesFn) (int, error) {
	_ = args
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"status:pending", "export"}, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
		return 1, nil
	}
	aliases, err := ensureTaskAliases(tasks)
	if err != nil {
		fmt.Fprintf(stderr, "warning: failed to update task alias cache: %v\n", err)
		aliases = nil
	}
	// Each line is "selector\tdescription" so fish shell can show the task
	// summary alongside the ID in the autocompletion menu.
	for _, item := range lines(tasks, aliases) {
		_, _ = io.WriteString(stdout, item+"\n")
	}
	return 0, nil
}

// taskCompletionItems returns tab-separated "selector\tdescription" strings
// for each task, placing the short alias before the UUID when available.
// Fish shell interprets the tab-separated format to display descriptions.
func taskCompletionItems(tasks []TaskExport, aliases map[string]string) []string {
	items := make([]string, 0, len(tasks)*2)
	for _, task := range tasks {
		if task.UUID == "" {
			continue
		}
		desc := truncateDescription(task.Description, completionDescriptionMaxLen)
		if alias := displayTaskAlias(task.UUID, aliases); alias != "" && alias != task.UUID {
			items = append(items, alias+"\t"+desc)
		}
		items = append(items, task.UUID+"\t"+desc)
	}
	return items
}

// taskCompletionAliasItems returns tab-separated "selector\tdescription" lines
// only for short alias IDs (never the raw UUID). Used by Fish completion via
// `complete-aliases`; non-shell callers still use complete-uuids for full
// selector lists.
func taskCompletionAliasItems(tasks []TaskExport, aliases map[string]string) []string {
	items := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.UUID == "" {
			continue
		}
		desc := truncateDescription(task.Description, completionDescriptionMaxLen)
		if alias := displayTaskAlias(task.UUID, aliases); alias != "" && alias != task.UUID {
			items = append(items, alias+"\t"+desc)
		}
	}
	return items
}

// truncateDescription collapses any embedded newlines to keep the completion
// item on a single line, then shortens s to at most maxLen characters,
// appending "…" when the string is cut, so the completion hint fits on one line.
func truncateDescription(s string, maxLen int) string {
	s = oneLineDescription(s)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// taskCompletionSelectors returns plain selector strings (no descriptions) for
// use in contexts that do not support tab-separated fish completion items.
func taskCompletionSelectors(tasks []TaskExport, aliases map[string]string) []string {
	selectors := make([]string, 0, len(tasks)*2)
	for _, task := range tasks {
		if task.UUID == "" {
			continue
		}
		if alias := displayTaskAlias(task.UUID, aliases); alias != "" && alias != task.UUID {
			selectors = append(selectors, alias)
		}
		selectors = append(selectors, task.UUID)
	}
	return selectors
}

// completionItemSelector extracts the selector (before the tab) from a
// tab-separated completion item returned by taskCompletionItems.
func completionItemSelector(item string) string {
	if idx := strings.IndexByte(item, '\t'); idx >= 0 {
		return item[:idx]
	}
	return item
}
