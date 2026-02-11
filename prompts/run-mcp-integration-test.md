# Prompt: Run MCP Server Integration Tests

**Context**: You are running integration tests for the hexai-mcp-server against a real Claude Code CLI instance in tmux. These tests verify the full Model Context Protocol integration including prompt discovery via slash commands and persistence across sessions.

**Your Role**: Execute each test systematically, report results clearly, and identify any failures.

## Instructions

Read the test runbook at `prompts/mcp-server-integration-tests.md` and execute each test in order. For each test:

1. **Announce** what you're testing
2. **Execute** the test commands
3. **Verify** the expected outcomes
4. **Report** results clearly (PASS or FAIL)
5. **Investigate** any failures before moving to next test

## Test Execution Flow

### Phase 1: Setup
- Create test environment (temp directory, test prompts)
- Register MCP server via `claude mcp add --transport stdio --scope user`
- Start Claude Code CLI in tmux pane
- Verify MCP server connection via `claude mcp list`

### Phase 2: Core Tests
Execute tests 1-5 in sequence:
1. Server Connection and Initialization
2. Prompt Discovery via Slash Commands (`/hexai-test:<name>`)
3. Prompt Retrieval via Slash Command
4. Persistence Across Sessions (kill and restart Claude)
5. Error Handling

### Phase 3: Cleanup
- Kill Claude pane
- Remove test MCP server: `claude mcp remove hexai-test -s user`
- Report final summary

## Key Details

- MCP prompts appear as slash commands: `/hexai-test:<prompt_name>`
- The MCP stdio transport uses JSONL (newline-delimited JSON), not Content-Length
- Register servers with `claude mcp add`, not by editing JSON config files
- Always clean up: kill panes and remove test server registration

## After Completion

Report:
1. **Overall result**: X/5 tests passed
2. **Failures**: List any failed tests with details
3. **Artifacts location**: Where logs and test data are stored
4. **Recommendations**: Any bugs found or improvements needed

Start with Phase 1 (Setup) and proceed systematically through all phases.
