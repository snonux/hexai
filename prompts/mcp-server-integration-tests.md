# hexai-mcp-server Integration Test Runbook

Real-life integration tests against Claude Code CLI using the hexai MCP server.
These tests verify the full MCP protocol integration: server discovery, prompt
listing/retrieval via slash commands, and file persistence across sessions.

**Protocol**: The MCP stdio transport uses newline-delimited JSON (JSONL), not
LSP-style Content-Length framing. Each message is a single JSON object followed
by a newline character.

**Test Status**:
- ✓ **Claude Code CLI**: Tested and working (2026-02-11)
- ⏳ **Cursor**: Needs testing (future)

## Prerequisites

- Must be running inside a tmux session
- Claude Code CLI must be installed and authenticated
- `hexai-mcp-server` binary must be built: `go build -o ~/go/bin/hexai-mcp-server ./cmd/hexai-mcp-server/`
- `jq` command-line JSON processor installed
- All unit tests must pass first: `go test ./internal/mcp/` and `go test ./internal/hexaimcp/`

## Setup: Create Test Environment

Before running tests, set up an isolated test environment:

```sh
# 1. Create temporary prompts directory for testing
TEST_PROMPTS_DIR="/tmp/hexai-mcp-test-$(date +%s)"
mkdir -p "$TEST_PROMPTS_DIR"
echo "Test prompts directory: $TEST_PROMPTS_DIR"

# 2. Create minimal test prompts in JSONL format
cat > "$TEST_PROMPTS_DIR/default.jsonl" << 'EOF'
{"name":"test_review","title":"Test Code Review","description":"A test prompt for code review","arguments":[{"name":"code","description":"Code to review","required":true}],"messages":[{"role":"user","content":{"type":"text","text":"Review this code:\n\n{{code}}\n\nProvide:\n1. Issues found\n2. Suggestions for improvement"}}],"tags":["testing","code-quality"],"created":"2026-02-10T00:00:00Z","updated":"2026-02-10T00:00:00Z"}
{"name":"test_explain","title":"Test Explain Code","description":"A test prompt to explain code","arguments":[{"name":"code","description":"Code to explain","required":true}],"messages":[{"role":"user","content":{"type":"text","text":"Explain what this code does:\n\n{{code}}"}}],"tags":["testing","documentation"],"created":"2026-02-10T00:00:00Z","updated":"2026-02-10T00:00:00Z"}
EOF

# 3. Create empty user.jsonl (for testing creation)
touch "$TEST_PROMPTS_DIR/user.jsonl"

# 4. Register test MCP server with Claude Code CLI (user scope)
claude mcp add --transport stdio --scope user hexai-test -- \
  "$HOME/go/bin/hexai-mcp-server" \
  --prompts-dir "$TEST_PROMPTS_DIR" \
  --log /tmp/hexai-mcp-test.log
echo "✓ Registered test MCP server"

# 5. Verify server is connected
claude mcp list | grep hexai-test
# Expected: hexai-test: ✓ Connected

# 6. Start Claude Code CLI in a new tmux pane
CLAUDE_PANE=$(tmux split-window -v -d -P -F '#{pane_id}' 'claude')
sleep 3
echo "Claude running in pane: $CLAUDE_PANE"

# 7. Verify MCP server started (check logs)
if grep -q "hexai-mcp-server starting" /tmp/hexai-mcp-test.log 2>/dev/null; then
    echo "✓ MCP server started successfully"
else
    echo "✗ MCP server failed to start (check /tmp/hexai-mcp-test.log)"
fi
```

Expected state after setup:
- Test prompts directory created with 2 default prompts
- MCP server registered via `claude mcp add`
- Claude Code CLI running in tmux pane
- MCP server initialized and connected

## Test 1: Server Connection and Initialization

**Goal**: Verify that Claude Code successfully connects to the MCP server
and completes the initialization handshake.

```sh
# 1. Check MCP server log for initialization sequence
grep "initialize" /tmp/hexai-mcp-test.log
# Expected: Log entries showing initialization from client

# 2. Check for any connection errors
grep -i "error\|fail" /tmp/hexai-mcp-test.log || echo "✓ No errors in log"

# 3. Verify Claude is responsive in the pane
tmux send-keys -t "$CLAUDE_PANE" "test connection"
sleep 1
tmux capture-pane -p -t "$CLAUDE_PANE" | tail -5
```

## Test 2: Prompt Discovery via Slash Commands

**Goal**: Verify that prompts from the MCP server appear as slash commands
in Claude Code CLI. MCP prompts appear as `/hexai-test:<prompt_name>`.

```sh
# 1. Type the slash command prefix to trigger autocomplete
tmux send-keys -t "$CLAUDE_PANE" "/hexai-test:"
sleep 2

# 2. Capture the autocomplete suggestions
tmux capture-pane -p -t "$CLAUDE_PANE" -S -20
# Expected: Should show test_review and test_explain as available commands

# 3. Press Escape to clear
tmux send-keys -t "$CLAUDE_PANE" Escape
sleep 0.5

# 4. Check MCP server log for prompts/list request
grep "prompts/list" /tmp/hexai-mcp-test.log
```

## Test 3: Prompt Retrieval via Slash Command

**Goal**: Verify that a specific prompt can be retrieved and used via
the slash command mechanism with argument substitution.

```sh
# 1. Use the test_review prompt via slash command
tmux send-keys -t "$CLAUDE_PANE" "/hexai-test:test_review"
sleep 1
# Select the prompt from autocomplete if needed
tmux send-keys -t "$CLAUDE_PANE" Enter
sleep 1

# 2. Fill in the code argument when prompted
tmux send-keys -t "$CLAUDE_PANE" "func Add(a, b int) int { return a + b }"
tmux send-keys -t "$CLAUDE_PANE" Enter
sleep 4

# 3. Check MCP server log for prompts/get request
grep "prompts/get" /tmp/hexai-mcp-test.log | tail -1

# 4. Capture Claude's response
tmux capture-pane -p -t "$CLAUDE_PANE" -S -30 | tail -15

# 5. Clear for next test
tmux send-keys -t "$CLAUDE_PANE" Escape
sleep 0.5
```

## Test 4: Persistence Across Sessions

**Goal**: Verify that prompts persist when starting a new Claude Code CLI session.

```sh
# 1. Kill the current Claude pane
tmux kill-pane -t "$CLAUDE_PANE"
sleep 1

# 2. Start a fresh Claude session
CLAUDE_PANE=$(tmux split-window -v -d -P -F '#{pane_id}' 'claude')
sleep 3

# 3. Verify the MCP server reconnects (check new log entries)
grep "initialize" /tmp/hexai-mcp-test.log | wc -l
# Expected: Should show 2 or more initialization entries

# 4. Try using the prompt in the new session
tmux send-keys -t "$CLAUDE_PANE" "/hexai-test:test_review"
sleep 2
tmux capture-pane -p -t "$CLAUDE_PANE" -S -10
# Expected: Prompt should appear in autocomplete

# 5. Clear
tmux send-keys -t "$CLAUDE_PANE" Escape
sleep 0.5
```

## Test 5: Error Handling

**Goal**: Verify proper error messages for invalid operations.

```sh
# Test 5a: Get non-existent prompt
tmux send-keys -t "$CLAUDE_PANE" "Get the prompt called 'nonexistent_prompt' from the hexai-test MCP server"
tmux send-keys -t "$CLAUDE_PANE" Enter
sleep 3

# Check for error in logs
grep -i "not found" /tmp/hexai-mcp-test.log | tail -1

tmux send-keys -t "$CLAUDE_PANE" Escape
sleep 0.5
```

## Cleanup

After all tests, remove the test server and clean up:

```sh
# 1. Kill Claude pane
if [ -n "$CLAUDE_PANE" ]; then
    tmux kill-pane -t "$CLAUDE_PANE"
    echo "✓ Closed Claude pane: $CLAUDE_PANE"
fi

# 2. Remove test MCP server registration
claude mcp remove hexai-test -s user
echo "✓ Removed test MCP server"

# 3. Keep test directory and logs for inspection
echo "Test artifacts preserved:"
echo "  Prompts: $TEST_PROMPTS_DIR"
echo "  Log: /tmp/hexai-mcp-test.log"
echo ""
echo "To clean up manually:"
echo "  rm -rf $TEST_PROMPTS_DIR"
echo "  rm /tmp/hexai-mcp-test.log"

# 4. Check for hung MCP server processes
HUNG_PROCS=$(pgrep -f hexai-mcp-server | wc -l)
if [ "$HUNG_PROCS" -gt 0 ]; then
    echo "⚠ Warning: $HUNG_PROCS hexai-mcp-server processes still running"
    echo "Run: pkill -f hexai-mcp-server"
else
    echo "✓ No hung MCP server processes"
fi
```

## Troubleshooting

**Claude doesn't see MCP server**:
- Verify registration: `claude mcp list` should show `hexai-test: ✓ Connected`
- If not connected, re-register: `claude mcp add --transport stdio --scope user hexai-test -- ...`
- Restart Claude Code CLI (kill pane and start new one)
- Check `/tmp/hexai-mcp-test.log` for startup errors

**MCP server not starting**:
- Verify binary is executable: `ls -la ~/go/bin/hexai-mcp-server`
- Check prompts directory exists: `ls -la $TEST_PROMPTS_DIR`
- Try running manually: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | hexai-mcp-server --prompts-dir $TEST_PROMPTS_DIR --log /tmp/manual-test.log`

**Prompts not appearing as slash commands**:
- MCP prompts appear as `/hexai-test:<name>` in Claude Code CLI
- Type `/hexai-test:` and wait for autocomplete
- Check prompts directory has valid JSONL files
- Verify JSONL: `jq -s '.' $TEST_PROMPTS_DIR/default.jsonl`

**Protocol errors**:
- The MCP stdio transport uses JSONL (newline-delimited JSON), not Content-Length framing
- Check JSON-RPC format in logs
- Test with: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | hexai-mcp-server`
