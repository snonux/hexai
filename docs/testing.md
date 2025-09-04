# Testing Guide

This repository includes a growing test suite designed to be realistic and robust.

Key patterns:

- Table‑driven tests: consolidate repetitive scenarios into concise tables (see `internal/lsp/*_table_test.go`).
- Shared fixtures: use `internal/testutil/fixtures.go` for multi‑line docblocks, chat replies, function suggestions, and markdown fences.
- Provider mocks: use `httptest.Server` and/or custom `http.RoundTripper` to simulate OpenAI/Copilot/Ollama responses, including success, stream (SSE), and error cases.
- E2E LSP tests: capture JSON‑RPC frames from the in‑memory server (`captureResponse`, `captureRequest`) and validate code actions, resolves, and chat edits.

Suggested additions:

- Expand table‑driven coverage for completion edit computations and label/filter selection.
- Add more negative tests (malformed SSE/JSON payloads) to assert robust error handling.

