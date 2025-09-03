Implement the changes outlined in this TODO document. Mark phases as done once completed. Before marking a phase done, ensure that all unit tests pass.

When implementing new code, ensure that there is also unit test coverage for it. Once unit tests pass, commit changes always to git after every phase completion.

Problem statement: When implementing a LSP server for Helix Editor for Auto Code completion, what are the strategies avoiding excessive completion calls? Especially when we use an LLM in the backend it can strain the performance. Is there anything with a debounce strategy we could implement?

Solutions (sub-divided into phases):

To avoid excessive completion calls in a Helix Editor LSP server—especially when an LLM is on the backend—debouncing is indeed the primary strategy. Here are the key optimization techniques:

Phase 1: Remove the LSP auto-completion rate limiter from the Hexai source code.

Status: Done — removed the single in-flight LLM request concurrency gate so
multiple completion requests can proceed concurrently. Also ensured manual
invocation (TriggerKind=1) always triggers completion even after whitespace.
Updated tests accordingly.

Phase 2: Debounce completion requests: Introduce a configurable delay (e.g., 100–500 ms) before sending a completion request to the LLM. This prevents a flood of calls while typing. 
        
Status: Done — added `completion_debounce_ms` (default 200). Server waits until
no recent input activity for at least this duration before LLM calls (both chat
and provider-native paths). Added unit test `TestCompletionDebounce_WaitsUntilQuiet`.
        
Phase 3: Throttle on the server side: Beyond debouncing, implement request throttling to cap the maximum rate of LLM calls (e.g., one per 500 ms). This is especially useful when debounce alone isn’t enough under rapid editing.

Status: Done — added `completion_throttle_ms` (default 0/disabled). Server
serializes LLM calls to maintain a minimum spacing across both chat and
provider-native completion paths. Added unit test
`TestCompletionThrottle_SerializesCalls`.

Phase 4: I think this is already implemented, verify: Filter incomplete triggers: Avoid sending requests for short or non-meaningful prefixes (e.g., less than 2–3 characters). This reduces noise and unnecessary LLM calls.

Status: Verified — `prefixHeuristicAllows` enforces a minimal prefix length
unless there is an inline prompt or structural trigger (., :, /, _, )). Manual
invoke may be constrained by `manual_invoke_min_prefix` (default 0). Existing
tests cover prefix handling.

Phase 5: I think this is already implemented, verify: Server-side caching: Cache recent completions keyed by prefix and file context. This avoids recomputation for repeated or similar queries.

Status: Verified — small LRU cache (~10) implemented (keyed by URI, position,
left/right text, and context). Tests exist in `completion_cache_test.go`.
