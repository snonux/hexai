// Summary: Hexai CLI runner; reads input, creates an LLM client, builds messages,
// streams or collects the model output, and prints a short summary to stderr.
package hexaicli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"hexai/internal/appconfig"
	"hexai/internal/llm"
	"hexai/internal/logging"
)

// Run executes the Hexai CLI behavior given arguments and I/O streams.
// It assumes flags have already been parsed by the caller.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg := appconfig.Load(nil)
	client, err := newClientFromConfig(cfg)
	if err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: LLM disabled: %v"+logging.AnsiReset+"\n", err)
		return err
	}

	return RunWithClient(ctx, args, stdin, stdout, stderr, client)
}

// RunWithClient executes the CLI flow using an already-constructed client.
// Useful for testing and embedding.
func RunWithClient(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, client llm.Client) error {
	input, err := readInput(stdin, args)
	if err != nil {
		fmt.Fprintln(stderr, logging.AnsiBase+err.Error()+logging.AnsiReset)
		return err
	}
	printProviderInfo(stderr, client)
	msgs := buildMessages(input)
	if err := runChat(ctx, client, msgs, input, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, logging.AnsiBase+"hexai: error: %v"+logging.AnsiReset+"\n", err)
		return err
	}
	return nil
}

// readInput reads from stdin and args, then combines them per CLI rules.
func readInput(stdin io.Reader, args []string) (string, error) {
	var stdinData string
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		b, _ := io.ReadAll(bufio.NewReader(stdin))
		stdinData = strings.TrimSpace(string(b))
	}
	argData := strings.TrimSpace(strings.Join(args, " "))
	switch {
	case stdinData != "" && argData != "":
		return fmt.Sprintf("%s:\n\n%s", argData, stdinData), nil
	case stdinData != "":
		return stdinData, nil
	case argData != "":
		return argData, nil
	default:
		return "", fmt.Errorf("hexai: no input provided; pass text as an argument or via stdin")
	}
}

// newClientFromConfig builds an LLM client from the app config and env keys.
func newClientFromConfig(cfg appconfig.App) (llm.Client, error) {
    llmCfg := llm.Config{
        Provider:       cfg.Provider,
        OpenAIBaseURL:  cfg.OpenAIBaseURL,
        OpenAIModel:    cfg.OpenAIModel,
        OpenAITemperature: cfg.OpenAITemperature,
        OllamaBaseURL:  cfg.OllamaBaseURL,
        OllamaModel:    cfg.OllamaModel,
        OllamaTemperature: cfg.OllamaTemperature,
        CopilotBaseURL: cfg.CopilotBaseURL,
        CopilotModel:   cfg.CopilotModel,
        CopilotTemperature: cfg.CopilotTemperature,
    }
	oaKey := os.Getenv("OPENAI_API_KEY")
	cpKey := os.Getenv("COPILOT_API_KEY")
	return llm.NewFromConfig(llmCfg, oaKey, cpKey)
}

// buildMessages creates system and user messages based on input content.
func buildMessages(input string) []llm.Message {
	lower := strings.ToLower(input)
	system := "You are Hexai CLI. Default to very short, concise answers. If the user asks for commands, output only the commands (one per line) with no commentary or explanation. Only when the word 'explain' appears in the prompt, produce a verbose explanation."
	if strings.Contains(lower, "explain") {
		system = "You are Hexai CLI. The user requested an explanation. Provide a clear, verbose explanation with reasoning and details. If commands are needed, include them with brief context."
	}
	return []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: input},
	}
}

// runChat executes the chat request, handling streaming and summary output.
func runChat(ctx context.Context, client llm.Client, msgs []llm.Message, input string, out io.Writer, errw io.Writer) error {
	start := time.Now()
	var output string
	if s, ok := client.(llm.Streamer); ok {
		var b strings.Builder
		if err := s.ChatStream(ctx, msgs, func(chunk string) {
			b.WriteString(chunk)
			fmt.Fprint(out, chunk)
		}); err != nil {
			return err
		}
		output = b.String()
	} else {
		txt, err := client.Chat(ctx, msgs)
		if err != nil {
			return err
		}
		output = txt
		fmt.Fprint(out, output)
	}
	dur := time.Since(start)
	fmt.Fprintf(errw, "\n"+logging.AnsiBase+"done provider=%s model=%s time=%s in_bytes=%d out_bytes=%d"+logging.AnsiReset+"\n",
		client.Name(), client.DefaultModel(), dur.Round(time.Millisecond), len(input), len(output))
	return nil
}

// printProviderInfo writes the provider/model line to stderr.
func printProviderInfo(errw io.Writer, client llm.Client) {
	fmt.Fprintf(errw, logging.AnsiBase+"provider=%s model=%s"+logging.AnsiReset+"\n", client.Name(), client.DefaultModel())
}
