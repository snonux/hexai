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
    "hexai/internal/logging"
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
        fmt.Fprintln(os.Stderr, logging.AnsiBase+"hexai: no input provided; pass text as an argument or via stdin"+logging.AnsiReset)
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
        fmt.Fprintf(os.Stderr, logging.AnsiBase+"hexai: LLM disabled: %v"+logging.AnsiReset+"\n", err)
        os.Exit(1)
    }

    // Print provider/model immediately to stderr
    fmt.Fprintf(os.Stderr, logging.AnsiBase+"provider=%s model=%s"+logging.AnsiReset+"\n", client.Name(), client.DefaultModel())

    // Prepare and send request
    start := time.Now()
    lower := strings.ToLower(input)
    system := "You are Hexai CLI. Default to very short, concise answers. If the user asks for commands, output only the commands (one per line) with no commentary or explanation. Only when the word 'explain' appears in the prompt, produce a verbose explanation."
    if strings.Contains(lower, "explain") {
        system = "You are Hexai CLI. The user requested an explanation. Provide a clear, verbose explanation with reasoning and details. If commands are needed, include them with brief context."
    }
    msgs := []llm.Message{
        {Role: "system", Content: system},
        {Role: "user", Content: input},
    }
    out, err := client.Chat(context.Background(), msgs)
    dur := time.Since(start)
    if err != nil {
        fmt.Fprintf(os.Stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
        os.Exit(1)
    }

    // Write assistant output to stdout
    fmt.Fprint(os.Stdout, out)

    // Summary to stderr (preceded by a blank line)
    inSize := len(input)
    outSize := len(out)
    fmt.Fprintf(os.Stderr, "\n"+logging.AnsiBase+"done provider=%s model=%s time=%s in_bytes=%d out_bytes=%d"+logging.AnsiReset+"\n", client.Name(), client.DefaultModel(), dur.Round(time.Millisecond), inSize, outSize)
}
