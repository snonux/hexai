package askcli

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

func FormatTaskList(tasks []TaskExport, aliases map[string]string) string {
	widths := taskListWidthsFor(tasks, aliases)
	var b strings.Builder
	writeTaskListHeader(&b, widths)
	writeTaskListSeparator(&b, widths)
	for _, t := range tasks {
		writeTaskListRow(&b, widths, t, aliases)
	}
	return b.String()
}

type taskListWidths struct {
	Urgency     int
	Priority    int
	ID          int
	Status      int
	Started     int
	Tags        int
	Description int
}

func taskListWidthsFor(tasks []TaskExport, aliases map[string]string) taskListWidths {
	widths := taskListWidths{
		Urgency:     len("Urgency"),
		Priority:    len("Prio"),
		ID:          len("ID"),
		Status:      len("Status"),
		Started:     len("Started"),
		Tags:        len("Tags"),
		Description: len("Description"),
	}
	for _, t := range tasks {
		widths.Urgency = maxInt(widths.Urgency, len(fmt.Sprintf("%.1f", t.Urgency)))
		widths.Priority = maxInt(widths.Priority, len(t.Priority))
		widths.ID = maxInt(widths.ID, len(displayTaskAlias(t.UUID, aliases)))
		widths.Status = maxInt(widths.Status, len(t.Status))
		widths.Started = maxInt(widths.Started, len(formatTaskStarted(t)))
		widths.Tags = maxInt(widths.Tags, len(formatTaskTags(t.Tags)))
		widths.Description = maxInt(widths.Description, len(formatTaskDescription(t.Description)))
	}
	return widths
}

func writeTaskListHeader(b *strings.Builder, widths taskListWidths) {
	fmt.Fprintf(b, "%-*s | %-*s | %-*s | %-*s | %-*s | %-*s | %-*s\n",
		widths.Urgency, "Urgency",
		widths.Priority, "Prio",
		widths.ID, "ID",
		widths.Status, "Status",
		widths.Started, "Started",
		widths.Tags, "Tags",
		widths.Description, "Description",
	)
}

func writeTaskListSeparator(b *strings.Builder, widths taskListWidths) {
	total := widths.Urgency + widths.Priority + widths.ID + widths.Status + widths.Started + widths.Tags + widths.Description + 18
	io.WriteString(b, strings.Repeat("-", total)+"\n")
}

func writeTaskListRow(b *strings.Builder, widths taskListWidths, t TaskExport, aliases map[string]string) {
	fmt.Fprintf(b, "%-*s | %-*s | %-*s | %-*s | %-*s | %-*s | %-*s\n",
		widths.Urgency, fmt.Sprintf("%.1f", t.Urgency),
		widths.Priority, t.Priority,
		widths.ID, displayTaskAlias(t.UUID, aliases),
		widths.Status, t.Status,
		widths.Started, formatTaskStarted(t),
		widths.Tags, formatTaskTags(t.Tags),
		widths.Description, formatTaskDescription(t.Description),
	)
}

func formatTaskTags(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	return strings.Join(tags, ",")
}

func formatTaskDescription(desc string) string {
	if len(desc) > 50 {
		return desc[:47] + "..."
	}
	return desc
}

func formatTaskStarted(t TaskExport) string {
	if t.Start == "" {
		return "no"
	}
	return "yes"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func FormatTaskInfo(t TaskExport, alias string, dependencyAliases map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ID:          %s\n", alias)
	fmt.Fprintf(&b, "UUID:        %s\n", t.UUID)
	fmt.Fprintf(&b, "Description: %s\n", t.Description)
	fmt.Fprintf(&b, "Status:      %s\n", t.Status)
	fmt.Fprintf(&b, "Started:     %s\n", formatTaskStarted(t))
	fmt.Fprintf(&b, "Priority:    %s\n", t.Priority)
	fmt.Fprintf(&b, "Urgency:     %.1f\n", t.Urgency)
	if len(t.Tags) > 0 {
		fmt.Fprintf(&b, "Tags:        %s\n", strings.Join(t.Tags, ", "))
	}
	if t.Start != "" {
		fmt.Fprintf(&b, "Start time:  %s\n", t.Start)
	}
	if len(t.Depends) > 0 {
		fmt.Fprintf(&b, "Depends:     %s\n", formatTaskDependencies(t.Depends, dependencyAliases))
	}
	if len(t.Annotations) > 0 {
		io.WriteString(&b, "Annotations:\n")
		for _, a := range t.Annotations {
			fmt.Fprintf(&b, "  - %s (added %s)\n", a.Description, a.Entry)
		}
	}
	return b.String()
}

func FormatSuccess(alias string) string {
	return fmt.Sprintf("ok %s\n", alias)
}

func FormatError(err error, taskID string) string {
	if taskID != "" {
		return fmt.Sprintf("error %s: %v\n", taskID, err)
	}
	return fmt.Sprintf("error: %v\n", err)
}

func displayResolvedTaskID(resolved resolvedTaskSelector) string {
	return displayTaskAlias(resolved.UUID, map[string]string{resolved.UUID: resolved.Alias})
}

func displayTaskAlias(uuid string, aliases map[string]string) string {
	if alias := strings.TrimSpace(aliases[uuid]); alias != "" {
		return alias
	}
	return uuid
}

func formatTaskDependencies(depends []string, aliases map[string]string) string {
	items := make([]string, 0, len(depends))
	for _, uuid := range depends {
		items = append(items, formatTaskReference(uuid, aliases))
	}
	slices.Sort(items)
	return strings.Join(items, ", ")
}

func formatTaskReference(uuid string, aliases map[string]string) string {
	alias := strings.TrimSpace(aliases[uuid])
	if alias == "" || alias == uuid {
		return uuid
	}
	return fmt.Sprintf("%s (%s)", alias, uuid)
}

// NormalizeUUID strips any leading "uuid:" prefix so callers can accept
// both "uuid:<value>" and bare UUID strings interchangeably. The returned
// value is always a plain UUID ready to be prefixed again when building
// taskwarrior filter arguments.
func NormalizeUUID(s string) string {
	return strings.TrimPrefix(s, "uuid:")
}

func IsNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func RejectNumericID() string {
	return "error: use UUID, not numeric task ID\n"
}
