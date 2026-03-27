package askcli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"codeberg.org/snonux/hexai/internal/termprint"
)

func FormatTaskList(tasks []TaskExport, aliases map[string]string) string {
	return FormatTaskListForWidth(tasks, aliases, 0)
}

func FormatTaskListForWidth(tasks []TaskExport, aliases map[string]string, terminalWidth int) string {
	widths := taskListWidthsFor(tasks, aliases, terminalWidth)
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

func taskListWidthsFor(tasks []TaskExport, aliases map[string]string, terminalWidth int) taskListWidths {
	widths := taskListWidths{
		Urgency:     len("Urgency"),
		Priority:    len("Prio"),
		ID:          len("ID"),
		Status:      len("Status"),
		Started:     len("Started"),
		Tags:        len("Tags"),
		Description: len("Description"),
	}
	longestDescription := widths.Description
	for _, t := range tasks {
		widths.Urgency = max(widths.Urgency, len(fmt.Sprintf("%.1f", t.Urgency)))
		widths.Priority = max(widths.Priority, len(t.Priority))
		widths.ID = max(widths.ID, len(displayTaskAlias(t.UUID, aliases)))
		widths.Status = max(widths.Status, len(t.Status))
		widths.Started = max(widths.Started, len(formatTaskStarted(t)))
		widths.Tags = max(widths.Tags, len(formatTaskTags(t.Tags)))
		longestDescription = max(longestDescription, len(t.Description))
	}
	widths.Description = taskListDescriptionWidth(widths, terminalWidth, longestDescription)
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
		widths.Description, formatTaskDescription(t.Description, widths.Description),
	)
}

func formatTaskTags(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	return strings.Join(tags, ",")
}

func formatTaskDescription(desc string, width int) string {
	if width <= 0 || len(desc) <= width {
		return desc
	}
	if width <= 3 {
		return desc[:width]
	}
	return desc[:width-3] + "..."
}

func formatTaskStarted(t TaskExport) string {
	if t.Start == "" {
		return "no"
	}
	return "yes"
}

func taskListDescriptionWidth(widths taskListWidths, terminalWidth, longestDescription int) int {
	if terminalWidth <= 0 {
		return longestDescription
	}
	available := terminalWidth - taskListFixedWidth(widths)
	if available < len("Description") {
		return len("Description")
	}
	if available < longestDescription {
		return available
	}
	return longestDescription
}

func taskListFixedWidth(widths taskListWidths) int {
	return widths.Urgency + widths.Priority + widths.ID + widths.Status + widths.Started + widths.Tags + 18
}

func detectTaskListTerminalWidth(w io.Writer) int {
	return termprint.DetectTerminalWidth(w)
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
