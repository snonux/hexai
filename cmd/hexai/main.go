package main

import (
    "bufio"
    "context"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "hexai/internal"
    "hexai/internal/appconfig"
    "hexai/internal/llm"
)

func main() {
    showVersion := flag.Bool("version", false, "print version and exit")
    flag.Parse()
    if *showVersion {
        fmt.Fprintln(os.Stdout, internal.Version)
        return
    }

    // Read stdin if present
    var stdinData string
    if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
        b, _ := io.ReadAll(bufio.NewReader(os.Stdin))
        stdinData = string(b)
    }

    // Read argument input (join all remaining args with space)
    argData := strings.TrimSpace(strings.Join(flag.Args(), " "))

    // Combine inputs
    var input string
    switch {
    case stdinData != "" && argData != "":
        input = strings.TrimSpace(stdinData) + "\n\n" + argData
    case stdinData != "":
        input = strings.TrimSpace(stdinData)
    case argData != "":
        input = argData
    default:
        fmt.Fprintln(os.Stderr, "hexai: no input provided; pass text as an argument or via stdin")
        os.Exit(2)
    }

    // Load config (no external logging for CLI)
    cfg := appconfig.Load(nil)

    // Build LLM client
    llmCfg := llm.Config{
        Provider:       cfg.Provider,
        OpenAIBaseURL:  cfg.OpenAIBaseURL,
        OpenAIModel:    cfg.OpenAIModel,
        OllamaBaseURL:  cfg.OllamaBaseURL,
        OllamaModel:    cfg.OllamaModel,
        CopilotBaseURL: cfg.CopilotBaseURL,
        CopilotModel:   cfg.CopilotModel,
    }
    oaKey := os.Getenv("OPENAI_API_KEY")
    cpKey := os.Getenv("COPILOT_API_KEY")
    client, err := llm.NewFromConfig(llmCfg, oaKey, cpKey)
    if err != nil {
        fmt.Fprintf(os.Stderr, "hexai: LLM disabled: %v\n", err)
        os.Exit(1)
    }

    // Print provider/model immediately to stderr
    fmt.Fprintf(os.Stderr, "provider=%s model=%s\n", client.Name(), client.DefaultModel())

    // Prepare and send request
    start := time.Now()
    msgs := []llm.Message{{Role: "user", Content: input}}
    out, err := client.Chat(context.Background(), msgs)
    dur := time.Since(start)
    if err != nil {
        fmt.Fprintf(os.Stderr, "hexai: error: %v\n", err)
        os.Exit(1)
    }

    // Write assistant output to stdout
    fmt.Fprint(os.Stdout, out)

    // Summary to stderr (preceded by a blank line)
    inSize := len(input)
    outSize := len(out)
    fmt.Fprintf(os.Stderr, "\ndone provider=%s model=%s time=%s in_bytes=%d out_bytes=%d\n", client.Name(), client.DefaultModel(), dur.Round(time.Millisecond), inSize, outSize)
}
