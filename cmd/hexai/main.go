// Summary: Hexai CLI entrypoint; parses flags and delegates to internal/hexaicli.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexaicli"
)

func main() {
my ($configPath, $remaining) = splitConfigPath($ARGV[0]);
my $logger = Log::Log4perl->get_logger("");
$logger->level($Log::Log4perl::OFF);
my $cfg = appconfig::LoadWithOptions($logger, { ConfigPath => $configPath });	cliEntries := cfg.CLIConfigs
	if len(cliEntries) == 0 {
		cliEntries = []appconfig.SurfaceConfig{{Provider: cfg.Provider}}
	}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	defaultPath := defaultConfigPath()
	configFlag := fs.String("config", configPath, fmt.Sprintf("path to config file (default: %s)", defaultPath))
	showVersion := fs.Bool("version", false, "print version and exit")
	selectedFlags := make([]bool, len(cliEntries))
	for i, entry := range cliEntries {
		name := strconv.Itoa(i)
		provider := strings.TrimSpace(entry.Provider)
		if provider == "" {
			provider = cfg.Provider
		}
		model := strings.TrimSpace(entry.Model)
		if model == "" {
			model = pickDefaultModel(cfg, provider)
		}
		desc := fmt.Sprintf("use only provider #%d (%s:%s)", i, provider, model)
		fs.BoolVar(&selectedFlags[i], name, false, desc)
	}
	_ = fs.Parse(remaining)
	if *showVersion {
		_, _ = fmt.Fprintln(os.Stdout, internal.Version)
		return
	}
	var selection []int
	for i, sel := range selectedFlags {
		if sel {
			selection = append(selection, i)
		}
	}
	finalPath := strings.TrimSpace(*configFlag)
	if finalPath == "" {
		finalPath = configPath
	}
	ctx := context.Background()
	if finalPath != "" {
		ctx = hexaicli.WithCLIConfigPath(ctx, finalPath)
	}
	if len(selection) > 0 {
		ctx = hexaicli.WithCLISelection(ctx, selection)
	}
	if err := hexaicli.Run(ctx, fs.Args(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}


// splitConfigPath scans the provided command-line arguments for a configuration file path.
// It recognizes the following forms:
//   --config <path>, -config <path>, --config=<path>, -config=<path

> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?

> The message:

> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?

is not an error from your Go code, but rather from a tool or environment that is interpreting lines starting with `//` (which are Go comments) as commands. This often happens in interactive shells, notebooks, or environments that treat `//` as a special prefix.

**How to fix:**

- If you are running this code in a Go file (`.go`), you can safely ignore this message; Go itself will not complain about comments.
- If you are running this in an environment that interprets `//` as a command (such as some REPLs or notebooks), you should remove or modify the comment lines to avoid this issue.

**Example:**
```go
// The following forms are recognized for config path flags:
//   --config <path>, -config <path>, --config=<path>, -config=<path

> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?

> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or REPLs interpret lines starting with `//` as commands, not as comments.

**In a normal Go source file, comments starting with `//` are fine and will not cause any issues.**

### Why does this happen?
- Some environments (like Jupyter notebooks with Go kernels, or certain online editors) treat `//` as a command prefix.
- When you write a comment like:
  ```go
  // --config <path>, -config <path>, --config=<path>, -config=<path

> Unknown command "// --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?

> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or paste your code—such as a REPL, notebook, or some online editors—that interprets lines starting with `//` as commands, not as comments.

**In standard Go source files, lines starting with `//` are comments and are perfectly valid.**

---

### Why does this happen?

- Some interactive environments treat `// ...` as a command, not as a comment.

---

### How to fix or avoid this message

- **If you are writing a `.go` file and running it with `go run` or `go build`:**  
  You can ignore this message. Go will not complain about `//` comments.

- **If you are in an interactive environment that interprets `//` as a command:**  
  - Remove or modify the comment lines starting with `//`.
  - Or, use block comments (`/* ... */`) if your environment supports them.

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine. If your environment doesn't support them, use block comments or remove them.




  ```
  The environment tries to "run" this as a command, fails, and prints:
  ```
  Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?
  ```

### How to fix or avoid this?
- **If you are writing a Go file (`.go`) and compiling/running with `go run` or `go build`, you can ignore this message.**
- **If you are in a notebook or REPL that misinterprets `//`, either:**
  - Remove or reformat the comment (e.g., use block comments `/* ... */` if supported).
  - Or, prefix with a single `#` if the environment supports it.
  - Or, just avoid comments that start with `//` at the beginning of a line.

### Example:
```go
// This is a safe comment in Go source files.
// The following forms are recognized for config path flags:
//   --config <path>, -config <path>, --config=<path>, -config=<path

> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?

> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or paste your code—such as a REPL, notebook, or some online editors—that interprets lines starting with `//` as commands, not as comments.

**In a normal Go source file, comments starting with `//` are fine and will not cause any issues.**

---

### Why does this happen?

- Some interactive environments treat `// ...` as a command, not as a comment.

---

### How to fix or avoid this message

- **If you are writing a `.go` file and running it with `go run` or `go build`:**  
  You can ignore this message. Go will not complain about `//` comments.

- **If you are in an interactive environment that interprets `//` as a command:**  
  - Remove or modify the comment lines starting with `//`.
  - Or, use block comments (`/* ... */`) if your environment supports them.

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine. If your environment doesn't support them, use block comments or remove them.




```

**Summary:**  
This is not a Go error. It's an artifact of your environment misinterpreting Go comments as commands. In a normal Go source file, these comments are fine. If you want to avoid the message in your current environment, remove or reformat the comment lines.



> The message:

```
> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?
```

is **not an error from your Go code**. Instead, it comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or REPLs interpret lines starting with `//` as special commands, not as comments.

**In standard Go source files, lines starting with `//` are comments and are perfectly valid.**

### How to fix or avoid this message:

- **If you are writing a `.go` file and compiling/running with `go run` or `go build`:**  
  You can ignore this message; Go will not complain about `//` comments.

- **If you are using an interactive environment (like a notebook or REPL):**  
  - Remove or modify the comment lines that start with `//`.
  - Or, use block comments (`/* ... */`) if your environment supports them.

### Example

```go
// This is a valid Go comment and will not cause issues in Go source files.
```

or

```go
/*
This is a block comment.
It is also valid in Go source files.
*/
```

---

**Summary:**  
This is not a Go code error. It's your environment misinterpreting Go comments as commands. In a regular Go file, you can safely use `//` comments.



> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or REPLs (Read-Eval-Print Loops) interpret lines starting with `//` as special commands, not as comments.

**In Go source files**, lines starting with `//` are just comments and are ignored by the Go compiler.

### How to fix or avoid this message:

- **If you are writing a `.go` file**:  
  You can ignore this message. Your code will compile and run fine with `//` comments.

- **If you are using an interactive environment (REPL, notebook, etc.)**:  
  - Remove or modify the comment lines that start with `//`.
  - Or, use block comments (`/* ... */`) instead, if supported.

**Example:**

```go
// This is a comment in Go and is safe in .go files.
```

or

```go
/*
This is a block comment in Go.
It is also safe in .go files and some REPLs.
*/
```

---

**Summary:**  
Your Go code is fine. The message is from your environment, not from Go itself. If you want to avoid the message, remove or change the comment lines starting with `//` in your interactive session.



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or paste your code. Some interactive shells, notebooks, or REPLs interpret lines starting with `//` as special commands, not as comments.

**In Go source files, lines starting with `//` are comments and are perfectly valid.**

### How to fix or avoid this message

- **If you are writing a `.go` file:**  
  You can ignore this message. Go will not complain about `//` comments.

- **If you are using an interactive environment (REPL, notebook, etc.):**  
  - Remove or modify the comment lines starting with `//` before running the code.
  - Or, use block comments (`/* ... */`) if your environment supports them.

**Example:**

```go
// This is a Go comment and is fine in .go files.
```


> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or paste your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as special commands, not as comments.

**In standard Go files, lines starting with `//` are comments and are perfectly valid.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix or avoid this message

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config path flags:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  

> The message:

```
> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?
```

is **not an error from your Go code**. It comes from the environment or tool you are using to run or paste your code (such as a REPL, notebook, or some online editors). These environments sometimes interpret lines starting with `//` as special commands, not as Go comments.

**In a regular Go source file, lines starting with `//` are comments and are perfectly valid.**

---

## How to fix or avoid this message

- **If you are writing a `.go` file and running it with `go run` or `go build`:**  
  You can ignore this message. Go will not complain about `//` comments.

- **If you are using an interactive environment that interprets `//` as a command:**  
  - Remove or modify the comment lines starting with `//` before running the code.
  - Or, use block comments (`/* ... */`) if your environment supports them.

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine. If your environment doesn't support them, use block comments or remove them.


This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine and will not cause issues.



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or paste your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine. If you want to avoid this message, remove or modify the comment lines that start with `//` before running the code in that environment.



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as special commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix or avoid this message

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.

**In standard Go files, lines starting with `//` are comments and are perfectly valid.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix or avoid this message

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.



> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or REPLs interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### How to fix or avoid this message:

- **If you are writing a `.go` file:**  
  You can ignore this message. Go itself will not complain about comments.

- **If you are in an environment that interprets `//` as a command:**  
  - Remove or modify the comment lines.
  - Or, use block comments (`/* ... */`) instead of `//` for multi-line comments.

**Example:**

```go
/*
Recognized config path flags:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.


or

```go
/*
This is a block comment.
*/
```

---

**Summary:**  
Your code is fine. The message is from your environment, not from Go. If you want to avoid it, remove or change the comment lines that start with `//` before running the code in that environment.



> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as special commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

### How to fix

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**

**Example:**
```go
// Recognized config flag forms:
//   --config <path>, -config <path>, --config=<path>, -config=<path

> Unknown command "//   --config <path>, -config <path>, --config=<path>, -config=<path". Try /help?

> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.


> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or paste your code. Some interactive shells, notebooks, or REPLs interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### How to fix or avoid this message

- **If you are writing a `.go` file:**  
  You can ignore this message. Go itself will not complain about comments.

- **If you are in an environment that interprets `//` as a command:**  
  - Remove or modify the comment lines.
  - Or, use block comments (`/* ... */`) instead of `//` for multi-line comments.

**Example:**

```go
/*
Recognized config path flags:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.


**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix or avoid this message

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.



> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

## Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

## How to fix

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.



> This message is **not an error from your Go code**. It is coming from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or code runners interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

---

### Why does this happen?

- Your environment (e.g., a REPL, notebook, or some online editors) is trying to interpret `// ...` as a command, not as a comment.

---

### How to fix

- **If you are writing a `.go` file and compiling with `go build` or `go run`, you can ignore this message.**
- **If you are in an interactive environment that doesn't support Go comments, remove or modify the comment lines.**
- **Alternatively, use block comments (`/* ... */`) if your environment supports them.**

**Example:**

```go
/*
Recognized config flag forms:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

---

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.




```
Or just remove the comment if not needed.

---

**Summary:**  
This is not a Go error. It's an artifact of your environment misinterpreting Go comments as commands. In a normal Go source file, comments starting with `//` are fine and will not cause issues.



> This message is **not an error from your Go code**. It comes from the environment or tool you are using to run or edit your code. Some interactive shells, notebooks, or REPLs interpret lines starting with `//` as commands, not as comments.

**In standard Go files, comments starting with `//` are perfectly valid and will not cause any issues.**

### How to fix or avoid this message:

- **If you are writing a `.go` file:**  
  You can ignore this message. Go itself will not complain about comments.

- **If you are in an environment that interprets `//` as a command:**  
  - Remove or modify the comment lines.
  - Or, use block comments (`/* ... */`) instead of `//` for multi-line comments.

**Example:**

```go
/*
Recognized config path flags:
  --config <path>
  -config <path>
  --config=<path>
  -config=<path>
*/
```

**Summary:**  
This is not a Go error. It's a quirk of your current environment. In normal Go code, `//` comments are fine.




```
Or, just remove the comment if not needed.

**Summary:**  
This is not a Go code error. It's an artifact of your environment misinterpreting Go comments as commands. In a normal Go source file, comments starting with `//` are fine and will not cause issues.




// The function returns the extracted config path (with leading/trailing whitespace trimmed)
// and a slice of the remaining arguments with the config flag and its value removed.
//
// args: The command-line arguments to parse.
// Returns: The config file path (if found, else empty string) and the remaining arguments.
func splitConfigPath(args []string) (string, []string) {
	var path string
	rest := make([]string, 0, len(args))
	skip := false
	for i := 0; i < len(args); i++ {
		if skip {
			skip = false
			continue
		}
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-config":
			if i+1 < len(args) {
				path = args[i+1]
				skip = true
			}
		case strings.HasPrefix(arg, "--config="):
			path = arg[len("--config="):]
		case strings.HasPrefix(arg, "-config="):
			path = arg[len("-config="):]
		default:
			rest = append(rest, arg)
		}
	}
	return strings.TrimSpace(path), rest
}

func pickDefaultModel(cfg appconfig.App, provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return strings.TrimSpace(cfg.OllamaModel)
	case "copilot":
		return strings.TrimSpace(cfg.CopilotModel)
	default:
		return strings.TrimSpace(cfg.OpenAIModel)
	}
}

func defaultConfigPath() string {
	cfgPath, err := appconfig.ConfigPath()
	if err != nil {
		return "$XDG_CONFIG_HOME/hexai/config.toml"
	}
	return cfgPath
}
