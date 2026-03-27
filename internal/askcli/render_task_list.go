package askcli

import (
	"encoding/json"
	"fmt"
	"io"
)

var taskListAliasLoader = ensureTaskAliases

func renderTaskList(tasks []TaskExport, stdout, stderr io.Writer, jsonOutput bool) (int, error) {
	if jsonOutput {
		data, err := json.Marshal(tasks)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to marshal JSON: %v\n", err)
			return 1, nil
		}
		_, _ = stdout.Write(data)
		_, _ = io.WriteString(stdout, "\n")
		return 0, nil
	}

	aliases, err := taskListAliasLoader(tasks)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to load task aliases: %v\n", err)
		return 1, nil
	}
	_, _ = io.WriteString(stdout, FormatTaskListForWidth(tasks, aliases, detectTaskListTerminalWidth(stdout)))
	return 0, nil
}
