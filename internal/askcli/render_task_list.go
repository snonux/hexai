package askcli

import (
	"encoding/json"
	"fmt"
	"io"
)

type taskListAliasLoader func([]TaskExport) (map[string]string, error)

func renderTaskList(tasks []TaskExport, stdout, stderr io.Writer, jsonOutput bool) (int, error) {
	return renderTaskListWithAliasLoader(tasks, stdout, stderr, jsonOutput, ensureTaskAliases)
}

func renderTaskListWithAliasLoader(tasks []TaskExport, stdout, stderr io.Writer, jsonOutput bool, loadAliases taskListAliasLoader) (int, error) {
	aliases, err := loadAliases(tasks)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to load task aliases: %v\n", err)
		return 1, nil
	}

	if jsonOutput {
		data, err := json.Marshal(withTaskIDs(tasks, aliases))
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to marshal JSON: %v\n", err)
			return 1, nil
		}
		_, _ = stdout.Write(data)
		_, _ = io.WriteString(stdout, "\n")
		return 0, nil
	}

	_, _ = io.WriteString(stdout, FormatTaskListForWidth(tasks, aliases, detectTaskListTerminalWidth(stdout)))
	return 0, nil
}
