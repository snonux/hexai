# Go unit tests via code action

Hexai can generate Go unit tests for the function at your cursor.

- Scope: Available only for Go source files ending with `.go` (not `_test.go`).
- Trigger: Use your editor's code actions on the current selection/position and pick "Implement unit test".

What happens

- Function detection: Hexai finds the nearest `func` definition above the cursor and captures the function body by balancing braces.
- Test generation:
  - If an LLM provider is configured, Hexai asks it to generate one or more `Test*` functions using the `testing` package. The provider must return only the test function code (no package/import lines).
  - If no provider is configured or the request fails, Hexai inserts a small stub test `Test<Name>` with a TODO.
- File handling and navigation:
  - If `<file>_test.go` exists, the test function is appended to the end of that file.
  - If it does not exist, Hexai creates it, writing `package <pkg>` (inferred from the source file) and `import "testing"`, followed by the generated test function(s).
  - After applying the edit, Hexai asks the editor to focus the test file and place the cursor at the start of the newly added test function.

Notes and limitations

- Imports on append: when appending to an existing test file, Hexai assumes `testing` is available. If not, add `import "testing"` to the test file and re-run `go test`.
- Method names: for methods with receivers, test names default to `TestMethod` (stub fallback). Future improvement may generate `TestType_Method` automatically.
- Formatting: run `go fmt ./...` or your editor's formatter to normalize whitespace if needed.

Examples

In Helix, position the cursor inside a function and invoke code actions; choose "Implement unit test". Hexai will create or update `<file>_test.go` accordingly.
