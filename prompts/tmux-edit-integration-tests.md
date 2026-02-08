# hexai-tmux-edit Integration Test Runbook

Real-life integration tests against actual tmux panes running AI agent CLIs.
These tests verify prompt capture, agent detection, multi-line extraction,
text clearing, and the full edit-and-replace flow.

**Test Status**:
- ✅ **cursor-agent**: All tests passed (Feb 8, 2026)
- ✅ **amp**: All tests passed (Feb 8, 2026) - Note: amp uses TUI mode with Emacs/readline keybindings
- ⏳ **claude**: Needs testing
- ⏳ **aider**: Needs testing

**Important**: Agent detection and prompt extraction rely on regex patterns
matched against each agent's TUI output (box-drawing characters, prompt
symbols, status text). When agents update their TUI, these patterns may
break. If tests fail after an agent update, check the built-in patterns in
`internal/tmuxedit/agent.go` `builtinAgents()` and adjust the
`DetectPattern`, `PromptPattern`, and `StripPatterns` fields accordingly.
Users can also override patterns via `[[tmux_edit.agents]]` in config
without code changes.

## Prerequisites

- Must be running inside a tmux session
- `hexai-tmux-edit` binary must be installed (`go build -o ~/go/bin/hexai-tmux-edit ./cmd/hexai-tmux-edit/`)
- At least one tmux pane running `cursor-agent` with an empty prompt
- All unit tests must pass first: `go test ./internal/tmuxedit/`

## Finding a test pane

List all panes and pick one running cursor-agent with an idle prompt:

```sh
tmux list-panes -a -F '#{pane_id} #{window_name} #{pane_current_command}'
```

Look for a pane running `cursor-agent`. Use its `%NN` pane ID throughout.
Verify it has an empty input prompt:

```sh
tmux capture-pane -p -t '%NN' | tail -10
```

You should see the box-drawing prompt:
```
 ┌─────────────────────┐
 │ → Add a follow-up   │
 └─────────────────────┘
```

If the prompt has text in it, clear it first:

```sh
tmux send-keys -t '%NN' End && sleep 0.1 && tmux send-keys -t '%NN' -N 200 BSpace
```

## Test 1: Single-line prompt capture

**Goal**: Verify that text typed into cursor-agent's prompt is correctly
detected and extracted.

```sh
# 1. Send test text to the prompt
tmux send-keys -t '%NN' 'hello world test'

# 2. Wait for the TUI to render
sleep 1

# 3. Verify text appeared in prompt
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → hello world test │

# 4. Write a Go test script to verify extraction logic
cat > /tmp/test_capture.go << 'GOEOF'
//go:build ignore

package main

import (
    "fmt"
    "os"
    "os/exec"
    "regexp"
    "strings"
)

func main() {
    pane := os.Args[1]
    out, err := exec.Command("tmux", "capture-pane", "-p", "-t", pane).Output()
    if err != nil {
        fmt.Printf("FAIL: capture error: %v\n", err)
        os.Exit(1)
    }
    content := string(out)

    // Verify cursor agent detected
    detectRe := regexp.MustCompile(`(│\s*→|/ commands · @ files)`)
    if !detectRe.MatchString(content) {
        fmt.Println("FAIL: cursor agent not detected")
        os.Exit(1)
    }
    fmt.Println("PASS: cursor agent detected")

    // Verify prompt extracted
    promptRe := regexp.MustCompile(`(?m)│\s*→?\s*(.+?)\s*│\s*$`)
    m := promptRe.FindStringSubmatch(content)
    if len(m) < 2 {
        fmt.Println("FAIL: no prompt match")
        os.Exit(1)
    }
    text := m[1]
    for _, s := range []string{"INSERT", "Add a follow-up", "ctrl+c to stop"} {
        text = strings.ReplaceAll(text, s, "")
    }
    text = strings.TrimSpace(text)
    fmt.Printf("PASS: extracted %q\n", text)
}
GOEOF

go run /tmp/test_capture.go '%NN'
# Expected: PASS: cursor agent detected
#           PASS: extracted "hello world test"

# 5. Clean up
tmux send-keys -t '%NN' End && sleep 0.1 && tmux send-keys -t '%NN' -N 200 BSpace
rm /tmp/test_capture.go
```

## Test 2: Multi-box disambiguation

**Goal**: Verify that when cursor-agent shows multiple box-drawing sections
(e.g. follow-ups box + input prompt), only the last box (the input prompt)
is captured.

This test requires a pane that has a follow-ups section above the input
prompt. You can create this state by sending text with Shift+Enter (which
cursor interprets as submit-and-queue-follow-up).

```sh
# 1. Verify the pane has multiple boxes visible
tmux capture-pane -p -t '%NN' | grep '│' | head -20
# Look for multiple │ lines from different boxes

# 2. Type text into the (bottom) input prompt
tmux send-keys -t '%NN' 'only this should be captured'
sleep 1

# 3. Run extraction and verify only the last box is picked
cat > /tmp/test_multibox.go << 'GOEOF'
//go:build ignore

package main

import (
    "fmt"
    "os"
    "os/exec"
    "regexp"
    "strings"
)

type promptMatch struct {
    lineNum int
    text    string
}

func main() {
    pane := os.Args[1]
    out, err := exec.Command("tmux", "capture-pane", "-p", "-t", pane).Output()
    if err != nil {
        fmt.Printf("FAIL: capture error: %v\n", err)
        os.Exit(1)
    }
    content := string(out)
    re := regexp.MustCompile(`(?m)│\s*→?\s*(.+?)\s*│\s*$`)
    strips := []string{"INSERT", "Add a follow-up", "ctrl+c to stop"}

    // Find all matches with line numbers
    paneLines := strings.Split(content, "\n")
    var matches []promptMatch
    for i, line := range paneLines {
        m := re.FindStringSubmatch(line)
        if len(m) >= 2 {
            matches = append(matches, promptMatch{i, m[1]})
        }
    }
    fmt.Printf("Total matches across all boxes: %d\n", len(matches))

    // Take last contiguous block
    if len(matches) == 0 {
        fmt.Println("FAIL: no matches found")
        os.Exit(1)
    }
    last := len(matches) - 1
    start := last
    for start > 0 && matches[start].lineNum-matches[start-1].lineNum == 1 {
        start--
    }

    fmt.Printf("Last contiguous block: lines %d-%d (%d matches)\n",
        matches[start].lineNum, matches[last].lineNum, last-start+1)

    var lines []string
    for i := start; i <= last; i++ {
        text := matches[i].text
        for _, s := range strips {
            text = strings.ReplaceAll(text, s, "")
        }
        text = strings.TrimSpace(text)
        if text != "" {
            lines = append(lines, text)
        }
    }
    result := strings.Join(lines, "\n")
    fmt.Printf("EXTRACTED: %q\n", result)

    if strings.Contains(result, "only this should be captured") {
        fmt.Println("PASS: last box correctly isolated")
    } else {
        fmt.Println("FAIL: wrong box content captured")
        os.Exit(1)
    }
}
GOEOF

go run /tmp/test_multibox.go '%NN'
# Expected: Total matches > 1 (from multiple boxes)
#           PASS: last box correctly isolated

# 3. Clean up
tmux send-keys -t '%NN' End && sleep 0.1 && tmux send-keys -t '%NN' -N 200 BSpace
rm /tmp/test_multibox.go
```

## Test 3: Clear and retype flow

**Goal**: Verify that `End + BSpace*200` clears the existing prompt text
and new text can be inserted afterwards.

```sh
# 1. Type original text
tmux send-keys -t '%NN' 'original text here'
sleep 0.5
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → original text here │

# 2. Clear using the same method hexai-tmux-edit uses
tmux send-keys -t '%NN' End
sleep 0.1
tmux send-keys -t '%NN' -N 200 BSpace
sleep 0.5

# 3. Verify prompt is empty
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → Add a follow-up │  (placeholder = empty)

# 4. Type replacement text
tmux send-keys -t '%NN' 'replacement text here'
sleep 0.5
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → replacement text here │

# 5. Clean up
tmux send-keys -t '%NN' End && sleep 0.1 && tmux send-keys -t '%NN' -N 200 BSpace
```

## Test 4: Full end-to-end with mock editor

**Goal**: Run the actual `hexai-tmux-edit` binary against a real pane,
using a mock editor to automate the popup interaction. Verifies the
complete flow: capture -> detect agent -> extract prompt -> edit -> clear
-> send back.

**Note**: Mock editors should include a small delay before exiting to ensure
tmux popups close cleanly. Without the delay, the popup might not register
the editor exit properly and could hang.

```sh
# 1. Type text into the prompt
tmux send-keys -t '%NN' 'hello world'
sleep 0.5
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → hello world │

# 2. Create a mock editor that replaces content (with delay for clean popup close)
cat > /tmp/mock-editor.sh << 'SH'
#!/bin/sh
# Write new content
echo "hello universe" > "$1"
# Small delay ensures tmux popup closes cleanly
sleep 0.1
SH
chmod +x /tmp/mock-editor.sh

# 3. Run hexai-tmux-edit with the mock editor
HEXAI_EDITOR=/tmp/mock-editor.sh hexai-tmux-edit --pane '%NN'
# Wait for popup to close
sleep 0.5

# 4. Verify the prompt was updated
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → hello universe │

# 5. Check debug log for the full trace
cat /tmp/hexai-tmux-edit.log
# Expected log should show:
#   - agent detected as "cursor"
#   - extractPrompt result: "hello world"
#   - editor returned: "hello universe"
#   - deduplicateText result: "hello universe"
#   - sending to pane: "hello universe"
#   - === done ===

# 6. Clean up
rm /tmp/mock-editor.sh
tmux send-keys -t '%NN' End && sleep 0.1 && tmux send-keys -t '%NN' -N 200 BSpace
```

## Test 5: Agent detection does not false-positive on model names

**Goal**: Verify that a cursor-agent pane showing "Claude 4.5 Sonnet" as
the model name is detected as cursor (not claude).

```sh
# 1. Capture pane content and check for model name
tmux capture-pane -p -t '%NN' | grep -i 'claude\|sonnet'
# This may show "Claude 4.5 Sonnet (Thinking)" in the status line

# 2. Verify detection is still "cursor"
cat > /tmp/test_detect.go << 'GOEOF'
//go:build ignore

package main

import (
    "fmt"
    "os"
    "os/exec"
    "regexp"
)

func main() {
    pane := os.Args[1]
    out, _ := exec.Command("tmux", "capture-pane", "-p", "-t", pane).Output()
    content := string(out)

    agents := []struct {
        name   string
        detect string
    }{
        {"cursor", `(│\s*→|/ commands · @ files)`},
        {"claude", `(❯|claude code|anthropic)`},
    }
    for _, a := range agents {
        re := regexp.MustCompile(a.detect)
        if re.MatchString(content) {
            fmt.Printf("Detected: %s\n", a.name)
            if a.name == "cursor" {
                fmt.Println("PASS: correctly detected as cursor")
            } else {
                fmt.Println("FAIL: should be cursor, not " + a.name)
                os.Exit(1)
            }
            return
        }
    }
    fmt.Println("FAIL: no agent detected")
    os.Exit(1)
}
GOEOF

go run /tmp/test_detect.go '%NN'
# Expected: PASS: correctly detected as cursor

rm /tmp/test_detect.go
```

## Test 6: Unchanged text results in no-op

**Goal**: If the user opens the editor and saves without changes,
nothing should be sent to the pane.

```sh
# 1. Type text into the prompt
tmux send-keys -t '%NN' 'do not change me'
sleep 0.5

# 2. Create a mock editor that keeps the content unchanged (with delay)
cat > /tmp/mock-noop-editor.sh << 'SH'
#!/bin/sh
# Do nothing -- file already has the pre-filled content
# Small delay for clean popup close
sleep 0.1
SH
chmod +x /tmp/mock-noop-editor.sh

# 3. Run hexai-tmux-edit
HEXAI_EDITOR=/tmp/mock-noop-editor.sh hexai-tmux-edit --pane '%NN'
sleep 0.5

# 4. Check debug log -- should show "nothing to send"
grep 'nothing to send' /tmp/hexai-tmux-edit.log
# Expected: "nothing to send, exiting"

# 5. Verify prompt text is unchanged (still has original text)
tmux capture-pane -p -t '%NN' | grep '│.*→'
# Expected: │ → do not change me │

# 6. Clean up
rm /tmp/mock-noop-editor.sh
tmux send-keys -t '%NN' End && sleep 0.1 && tmux send-keys -t '%NN' -N 200 BSpace
```

## Cleanup

After all tests, remove any leftover temp files and check for hung popups:

```sh
# Remove test files
rm -f /tmp/test_capture.go /tmp/test_multibox.go /tmp/test_detect.go
rm -f /tmp/mock-editor.sh /tmp/mock-noop-editor.sh

# Check for any hung tmux popups (should show nothing)
tmux list-panes -a | grep popup || echo "✓ No hung popups"

# If popups are hung, kill them
# pkill -f 'tmux.*popup' || true
```

## Troubleshooting

**Popup doesn't close**: Mock editors that exit instantly (< 50ms) might not
give tmux enough time to register the exit. Add `sleep 0.1` before the editor
script exits.

**Popup hangs**: If a popup is stuck open:
```sh
# Find popup panes
tmux list-panes -a | grep popup

# Kill popup processes
pkill -f 'tmux.*popup'

# Or press Escape in the tmux session to close active popup
```
