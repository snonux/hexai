package askcli

import (
	"fmt"
	"io"
	"strings"
)

func FormatTaskList(tasks []TaskExport) string {
	widths := taskListWidthsFor(tasks)
	var b strings.Builder
	writeTaskListHeader(&b, widths)
	writeTaskListSeparator(&b, widths)
	for _, t := range tasks {
		writeTaskListRow(&b, widths, t)
	}
	return b.String()
}

type taskListWidths struct {
	Urgency     int
	Priority    int
	UUID        int
	Status      int
	Tags        int
	Description int
}

func taskListWidthsFor(tasks []TaskExport) taskListWidths {
	widths := taskListWidths{
		Urgency:     len("Urgency"),
		Priority:    len("Priority"),
		UUID:        len("UUID"),
		Status:      len("Status"),
		Tags:        len("Tags"),
		Description: len("Description"),
	}
	for _, t := range tasks {
		widths.Urgency = maxInt(widths.Urgency, len(fmt.Sprintf("%.1f", t.Urgency)))
		widths.Priority = maxInt(widths.Priority, len(t.Priority))
		widths.UUID = maxInt(widths.UUID, len(t.UUID))
		widths.Status = maxInt(widths.Status, len(t.Status))
		widths.Tags = maxInt(widths.Tags, len(formatTaskTags(t.Tags)))
		widths.Description = maxInt(widths.Description, len(formatTaskDescription(t.Description)))
	}
	return widths
}

func writeTaskListHeader(b *strings.Builder, widths taskListWidths) {
	fmt.Fprintf(b, "%-*s | %-*s | %-*s | %-*s | %-*s | %-*s\n",
		widths.Urgency, "Urgency",
		widths.Priority, "Priority",
		widths.UUID, "UUID",
		widths.Status, "Status",
		widths.Tags, "Tags",
		widths.Description, "Description",
	)
}

func writeTaskListSeparator(b *strings.Builder, widths taskListWidths) {
	total := widths.Urgency + widths.Priority + widths.UUID + widths.Status + widths.Tags + widths.Description + 15
	io.WriteString(b, strings.Repeat("-", total)+"\n")
}

func writeTaskListRow(b *strings.Builder, widths taskListWidths, t TaskExport) {
	fmt.Fprintf(b, "%-*s | %-*s | %-*s | %-*s | %-*s | %-*s\n",
		widths.Urgency, fmt.Sprintf("%.1f", t.Urgency),
		widths.Priority, t.Priority,
		widths.UUID, t.UUID,
		widths.Status, t.Status,
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func FormatTaskInfo(t TaskExport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "UUID:        %s\n", t.UUID)
	fmt.Fprintf(&b, "Description: %s\n", t.Description)
	fmt.Fprintf(&b, "Status:      %s\n", t.Status)
	fmt.Fprintf(&b, "Priority:    %s\n", t.Priority)
	fmt.Fprintf(&b, "Urgency:     %.1f\n", t.Urgency)
	if len(t.Tags) > 0 {
		fmt.Fprintf(&b, "Tags:        %s\n", strings.Join(t.Tags, ", "))
	}
	if t.Start != "" {
		fmt.Fprintf(&b, "Started:     %s\n", t.Start)
	}
	if len(t.Depends) > 0 {
		fmt.Fprintf(&b, "Depends:     %s\n", strings.Join(t.Depends, ", "))
	}
	if len(t.Annotations) > 0 {
		io.WriteString(&b, "Annotations:\n")
		for _, a := range t.Annotations {
			fmt.Fprintf(&b, "  - %s (added %s)\n", a.Description, a.Entry)
		}
	}
	return b.String()
}

func FormatSuccess(uuid string) string {
	return fmt.Sprintf("ok %s\n", uuid)
}

func FormatError(err error, uuid string) string {
	if uuid != "" {
		return fmt.Sprintf("error %s: %v\n", uuid, err)
	}
	return fmt.Sprintf("error: %v\n", err)
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
