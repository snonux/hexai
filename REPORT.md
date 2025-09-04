# Unit Test Improvement Report

This report outlines areas for improvement in the project's unit tests. While the existing tests provide good coverage, the following suggestions aim to enhance their realism, robustness, and maintainability.

## 1. `internal/hexaicli/run_test.go`

- **`TestRunChat_StreamAndNonStream`**: The fake client and streamer return very simplistic, hardcoded responses (`"Hi!"`, `"Yo"`).
  - **Recommendation**: Enhance the fake client to return more realistic, multi-line, or structured code/text responses. This would better test the output handling and parsing logic. Consider adding cases for empty or malformed LLM responses.

## 2. `internal/lsp/codeaction_test.go`

- **`TestBuildRewriteCodeAction_LazyAndResolves`**: The `fakeLLM` returns a simple, hardcoded string (`"REWRITTEN"`).
  - **Recommendation**: Test with more complex and realistic code transformations. For example, the fake LLM could return a multi-line code block, a function with a different signature, or even code with syntax errors to test how the client-side handles such responses.

- **`TestBuildDiagnosticsCodeAction_LazyAndResolves`**: Similar to the rewrite action, the `fakeLLM` returns a simple string (`"FIXED"`).
  - **Recommendation**: The fake LLM should return a code snippet that actually addresses the provided diagnostic. This would make the test a more faithful representation of the feature's intended behavior.

## 3. `internal/lsp/handlers_end_to_end_test.go`

- **`TestDetectAndHandleChat_InsertsReply`**: The `fakeLLM` returns a single word (`"Hello"`).
  - **Recommendation**: A more realistic test would involve a multi-word or multi-line response, which would better test the formatting and insertion logic (e.g., how newlines are handled).

- **`TestHandleCodeActionResolve_Document`**: The `fakeLLM` returns a simple, hardcoded response.
  - **Recommendation**: The fake LLM's response should be a more realistic documentation block for the given function. This would help verify that the documentation generation and insertion logic works as expected with real-world-like data.

## 4. `internal/lsp/completion_prefix_strip_test.go`

- **`TestTryLLMCompletion_ManualInvokeAfterWhitespace_Allows`**: The `fakeLLM` returns a very short, non-representative code snippet (`"() *CustData"`).
  - **Recommendation**: Use a more complete and realistic code suggestion to test the completion logic, including how it handles longer suggestions and potential formatting.

## 5. `internal/llm/*_http_test.go` (New Findings)

- **`TestOpenAI_Chat_Success`** in `openai_http_test.go` and **`TestCopilot_EnsureSession_AndChat_Success`** in `copilot_http_test.go` use `httptest` to mock the backend services, which is great. However, the mocked responses are minimal (e.g., `{"choices":[{"message":{"content":"OK"}}]}`).
  - **Recommendation**: Expand these tests to handle more complex and realistic payloads from the LLM providers. This includes multi-choice responses, responses with `finish_reason` other than `stop`, and error objects in the response body. This will make the client code more robust.

## 6. General Recommendations

- **Table-Driven Tests**: Some test files contain multiple, repetitive test functions that could be consolidated into table-driven tests. This would improve readability and make it easier to add new test cases. Examples include `internal/lsp/handlers_test.go` and `internal/lsp/completion_prefix_strip_test.go`.

- **More Realistic Mock Data**: Across the board, the mock data used in tests is often very simplistic. While this is acceptable for basic unit tests, creating a set of more realistic mock responses from the LLM would allow for more robust testing of the parsing, formatting, and error-handling logic. This could include:
  - Multi-line code snippets.
  - Code with complex syntax.
  - Responses containing Markdown formatting.
  - Malformed or incomplete JSON/code.
  - Empty responses.

By addressing these points, the test suite will be more robust and provide a higher degree of confidence in the application's behavior when interacting with a real LLM.

---

## Plan and Status (living checklist)

Legend: [ ] pending · [~] in progress · [x] done/partially done

1) internal/hexaicli/run_test.go
- [ ] Enhance fake client/streamer responses to multi-line/structured outputs in TestRunChat_StreamAndNonStream.
- [ ] Add cases for empty/malformed LLM responses and ensure graceful handling.

2) internal/lsp/codeaction_test.go and related e2e tests
- [ ] Make fake LLM rewrite responses multi-line and structural (e.g., signature change) and validate insertion.
- [ ] Make diagnostics-fix responses actually address a provided diagnostic; assert the fix is reflected in text edits.
- [ ] Document-code action: return realistic docblocks (multi-line) and assert formatting/placement.

3) internal/lsp/handlers_end_to_end_test.go
- [ ] Use multi-line replies in TestDetectAndHandleChat_InsertsReply; verify newline formatting and cursor placement in edits.
- [ ] Use more realistic documentation blocks in TestHandleCodeActionResolve_Document; verify correct insertion range.

4) internal/lsp/completion_prefix_strip_test.go
- [ ] Replace short snippet ("() *CustData") with fuller realistic suggestions; add additional cases to exercise prefix/indent logic with longer outputs.

5) internal/llm/*_http_test.go
- [x] OpenAI success: basic chat completion via httptest.
- [x] OpenAI stream: SSE delta accumulation in ChatStream.
- [x] Copilot token + chat: ensureSession + /chat/completions success.
- [x] Copilot CodeCompletion: SSE-style stream with multiple choices.
- [x] Expand OpenAI mocked responses: multi-choice, different finish_reason, error objects; assert parsing.
- [x] Expand Copilot mocked responses: multi-choice, error object in body; assert parsing and error propagation.

6) General
- [x] Convert repetitive tests to table-driven style where appropriate (e.g., completion prefix/strip; instruction markers; label/filter; code fences/inline spans).
- [ ] Introduce a shared set of realistic mock responses (multi-line code, markdown, malformed json) and reuse across tests.

## Progress (latest)

- [x] Coverage gates and CI ergonomics
  - Added `mage covercheck` with per-package totals and exceptions.

- [x] Coverage raised to ≥80%:
  - internal/lsp: ~81.2% (new e2e and helper tests)
  - internal/llm: ~80.3% (OpenAI/Copilot HTTP + SSE + token + CodeCompletion)
  - internal/hexaicli, internal/hexailsp, internal/appconfig, internal/logging all ≥90%

- [x] Provider realism improvements (partial):
  - OpenAI: added ChatStream SSE and success path via httptest.
  - Copilot: added ensureSession (token) + chat success and Codex CodeCompletion SSE.
  - Next: multi-choice and finish_reason variants; error objects coverage.

## Status updates (since last run)

- [~] 1) hexaicli: adjusted tests for environment variability; realism enhancements pending.
- [x] 2) lsp code actions: document-code and diagnostics tests now use multi-line responses in fake LLM to better simulate real outputs.
- [x]    Added rewrite/diagnostics realism tests that validate multi-line replacements and exact range preservation.
- [x] 3) lsp e2e chat/document: chat test now uses multi-line reply and validates insertion contains both lines; document resolve uses multi-line docblock.
- [x] 4) lsp completion: manual-invoke test now uses a multi-line realistic function signature with body; still passes and exercises formatting.
- [x] 5) llm providers: added OpenAI success + SSE stream and Copilot token+chat + Codex SSE tests; coverage ≥80%. Expanded with multi-choice and error-body cases.
- [x] 6) General: introduced shared fixtures (internal/testutil) and added table-driven tests for code fences, inline spans, label selection, prefix stripping, and instruction markers. Documented patterns in docs/testing.md.
- [x]    Added table-driven tests for instruction marker extraction and prefix stripping.

## Next actions (prioritized)

1. LSP realism
- Implement multi-line rewrite/diagnostics/doc responses from fake LLM, assert proper NewText and ranges.
- Expand chat reply test to multi-line; verify inserted formatting.

2. Provider payload breadth
- OpenAI: multi-choice responses, finish_reason != stop, error bodies; negative SSE chunks.
- Copilot: multi-choice in chat, error body propagation in non-2xx; expand CodeCompletion SSE variants.

3. Table-driven refactors
- Convert repetitive cases (prefix stripping, instruction extraction, label selection) to table-driven style to ease adding new scenarios.

4. Negative/malformed inputs
- Add malformed/missing fields, empty model responses, and malformed SSE to assert robust error handling in clients and LSP handlers.
