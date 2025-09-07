package hexaiaction

import (
    "context"
    "fmt"
    "io"
    "log"
    "strings"

    "codeberg.org/snonux/hexai/internal/appconfig"
    "codeberg.org/snonux/hexai/internal/logging"
    "codeberg.org/snonux/hexai/internal/llmutils"
)

// Run executes the hexai-tmux-action command flow.
// seams for testability
var chooseActionFn = RunTUI
var newClientFromApp = llmutils.NewClientFromApp

func Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
    logger := log.New(stderr, "hexai-tmux-action ", log.LstdFlags|log.Lmsgprefix)
    cfg := appconfig.Load(logger)
    client, err := newClientFromApp(cfg)
    if err != nil {
        fmt.Fprintf(stderr, logging.AnsiBase+"hexai-tmux-action: LLM disabled: %v"+logging.AnsiReset+"\n", err)
        return err
    }
    parts, err := ParseInput(stdin)
	if err != nil {
        fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: failed to read input"+logging.AnsiReset)
		return err
	}
	if strings.TrimSpace(parts.Selection) == "" {
        return fmt.Errorf("hexai-tmux-action: no input provided on stdin")
	}
    kind, err := chooseActionFn()
    if err != nil {
        return err
    }
	out, err := executeAction(ctx, kind, parts, cfg, client, stderr)
	if err != nil {
		return err
	}
	io.WriteString(stdout, out)
	return nil
}

func executeAction(ctx context.Context, kind ActionKind, parts InputParts, cfg appconfig.App, client chatDoer, stderr io.Writer) (string, error) {
	switch kind {
	case ActionSkip:
		return parts.Selection, nil
	case ActionRewrite:
		instr, cleaned := ExtractInstruction(parts.Selection)
		if strings.TrimSpace(instr) == "" {
            fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: no inline instruction found; echoing input"+logging.AnsiReset)
			return parts.Selection, nil
		}
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runRewrite(cctx, cfg, client, instr, cleaned)
	case ActionDiagnostics:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runDiagnostics(cctx, cfg, client, parts.Diagnostics, parts.Selection)
	case ActionDocument:
		cctx, cancel := timeout10s(ctx)
		defer cancel()
		return runDocument(cctx, cfg, client, parts.Selection)
	case ActionGoTest:
		cctx, cancel := timeout8s(ctx)
		defer cancel()
		return runGoTest(cctx, cfg, client, parts.Selection)
	default:
		return parts.Selection, nil
	}
}

// client construction is shared via internal/llmutils
