# Add configurable request timeout for LLM calls

## Motivation

When using locally-hosted LLMs (via LM Studio, Ollama, or other OpenAI-compatible servers), the default 30-second HTTP timeout is often insufficient. Local models, especially larger ones like Gemma 3 27B, can take significantly longer to generate responses, resulting in "context deadline exceeded" errors.

This change allows users to configure a longer timeout to accommodate slower local inference.

## Changes

### New configuration option

Added `request_timeout` (in seconds) to the `[general]` section:

```toml
[general]
request_timeout = 120  # seconds, default 30
```

Or via environment variable:
```sh
export HEXAI_REQUEST_TIMEOUT=120
```

### Implementation

- Added `RequestTimeout` field to config structs with default of 30 seconds
- Each LLM client constructor now has a `*WithTimeout` variant that accepts the timeout
- Original constructors remain unchanged (delegate to `*WithTimeout` with default)
- `NewFromConfig` passes the configured timeout to clients

### Files modified

- `internal/appconfig/config.go` - Config field, parsing, merge logic, env var support
- `internal/llm/provider.go` - Added `RequestTimeout` to `Config`, calls `*WithTimeout` constructors
- `internal/llm/openai.go` - Added `newOpenAIWithTimeout`
- `internal/llm/ollama.go` - Added `newOllamaWithTimeout`
- `internal/llm/openrouter.go` - Added `newOpenRouterWithTimeout`
- `internal/llm/anthropic.go` - Added `newAnthropicWithTimeout`
- `internal/hexailsp/run.go` - Pass `RequestTimeout` to `llm.Config`
- `internal/llmutils/client.go` - Pass `RequestTimeout` to `llm.Config`
- `internal/lsp/server.go` - Pass `RequestTimeout` to `llm.Config`
- `config.toml.example` - Documented option
- `docs/configuration.md` - Added `HEXAI_REQUEST_TIMEOUT` to env vars list

### Test impact

No test files were modified. The original constructor signatures are preserved, so all existing tests continue to work unchanged.

## Testing

```sh
# Set timeout in config
[general]
request_timeout = 120

# Test with local LLM
cat somefile.go | hexai "review this code"
```

All existing tests pass:
```sh
HEXAI_TEST_SKIP_NET=1 go test ./...
```
