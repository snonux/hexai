# Codebase Review Findings

This document outlines the results of a codebase review for the `hexai` project, focusing on readability, maintainability, Go best practices, and adherence to the guidelines in `AGENTS.md`.

## 1. Executive Summary

The `hexai` codebase is well-structured, with a clear separation of concerns between the LSP server, LLM providers, and CLI components. Test coverage appears to be good for the core LLM provider logic.

However, several key areas require attention to improve maintainability and adhere to the project's coding standards. The most critical issues are:

-   **Large, complex functions:** Several functions, particularly within the LSP message handling logic, significantly exceed the 50-line limit. This makes them difficult to read, understand, and maintain.
-   **Large source files:** The primary LSP handler file (`internal/lsp/handlers.go`) has grown too large, violating the 1000-line limit.
-   **Centralized request handling:** The main request loop in `internal/lsp/server.go` is a large monolithic function that dispatches all LSP messages.

Addressing these issues by refactoring large functions and splitting up large files will significantly improve the long-term health of the codebase.

## 2. File and Function Size Violations

The following files and functions violate the size constraints defined in `AGENTS.md`.

### 2.1. Files Exceeding 1000 Lines

-   **`internal/lsp/handlers.go`**: This file is significantly over the 1000-line limit. It contains the logic for many different LSP requests.
    -   **Recommendation**: Split this file into multiple smaller files, each responsible for a specific set of related LSP features (e.g., `handlers_completion.go`, `handlers_codeaction.go`, `handlers_commands.go`).

### 2.2. Functions Exceeding 50 Lines

-   **`internal/lsp/server.go`**:
    -   `Serve()`: This function is the main request loop and is very long.
    -   **Recommendation**: Refactor this method. Instead of a single large `switch` statement, use a map of method names to handler functions (e.g., `map[string]func(*jsonrpc2.Request) error`). This is a common pattern in LSP servers and will make the code much cleaner and more extensible.

-   **`internal/lsp/handlers.go`**:
    -   `handleTextDocumentCompletion()`: This function is extremely large and complex. It handles completion requests, interacts with the LLM, manages caching, and formats the response.
    -   `handleCodeAction()`: This function is also very large and contains complex logic for determining available code actions.
    -   `handleExecuteCommand()`: This function has a large `switch` statement for dispatching different commands.
    -   **Recommendation**: Break down these functions into smaller, more focused helper functions. For example, `handleTextDocumentCompletion` could be split into functions for:
        1.  Checking if a completion should be triggered.
        2.  Fetching results from the cache.
        3.  Preparing the request for the LLM.
        4.  Calling the LLM and handling its response.
        5.  Formatting the completion items.

-   **`Magefile.go`**:
    -   Several build functions are slightly over the 50-line limit.
    -   **Recommendation**: While less critical than the application code, consider breaking down the larger Mage functions into smaller, reusable helper functions.

## 3. Readability and Maintainability

-   **Complex Conditionals**: Functions like `handleTextDocumentCompletion` have deeply nested `if` and `switch` statements. This makes the logic flow very difficult to follow. Refactoring into smaller functions will help flatten these conditionals.
-   **Lack of Comments for Complex Logic**: While the code is generally clean, some of the more complex parts of the LSP logic (e.g., position calculations, completion context) could benefit from comments explaining the *why* behind the code.

## 4. Testing

-   **Good Coverage for LLM Providers**: The `internal/llm` package has a good set of tests for each provider. This is excellent.
-   **`handlers_test.go` is Large**: Similar to `handlers.go`, the corresponding test file is also very large. Splitting the handlers into smaller files should be mirrored in the tests.
-   **Main Packages Not Tested**: The `main` functions in `cmd/hexai-lsp/main.go` and `cmd/hexai/main.go` contain some logic that is not unit tested.
    -   **Recommendation**: Extract the core application logic from the `main` functions into separate functions (e.g., `run() error`) in a different file/package so that it can be tested. The `internal/hexaicli/run.go` and `internal/hexailsp/run.go` files seem to be a good step in this direction.

## 5. Go Best Practices & Conventions

-   **Error Handling**: The project follows Go's error handling conventions well.
-   **Variable Naming**: The code generally uses descriptive variable names, avoiding single-letter identifiers except in idiomatic cases (e.g., loop counters).

## 6. Summary of Recommendations

1.  [x] Refactor JSON-RPC dispatch: replace the large `switch` with a handler map. Implemented via `Server.handlers` and `handle` now dispatches through the map.
2.  [x] Split `internal/lsp/handlers.go`: Extracted feature-specific files `internal/lsp/handlers_codeaction.go` and `internal/lsp/handlers_completion.go`.
3.  [x] Refactor large handler functions: `handleTextDocumentCompletion` split into focused helpers (prefix heuristics, cache, provider-native path, chat path, post-processing). `handleCodeAction` already small; no `handleExecuteCommand` present.
4.  [x] Mirror test structure: Feature-specific tests already exist (`codeaction_test.go`, `completion_*_test.go`); no changes needed.
5.  [x] Extract logic from `main`: Entrypoints already delegate to `internal/hexailsp.Run` and `internal/hexaicli.Run`, both tested.
