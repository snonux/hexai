package askcli

import (
	"fmt"
	"io"
	"strings"
)

func FormatTaskList(tasks []TaskExport) string {
	var b strings.Builder
	io.WriteString(&b, "UUID | Priority | Status | Tags | Description | Urgency\n")
	io.WriteString(&b, strings.Repeat("-", 120)+"\n")
	for _, t := range tasks {
		tags := strings.Join(t.Tags, ",")
		if tags == "" {
			tags = "-"
		}
		desc := t.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(&b, "%s | %s | %s | %s | %s | %.1f\n", t.UUID, t.Priority, t.Status, tags, desc, t.Urgency)
	}
	return b.String()
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
