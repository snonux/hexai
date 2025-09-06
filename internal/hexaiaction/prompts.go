package hexaiaction

import (
    "context"
    "strings"
    "time"

    "codeberg.org/snonux/hexai/internal/appconfig"
    "codeberg.org/snonux/hexai/internal/llm"
    "codeberg.org/snonux/hexai/internal/textutil"
)

// Render performs simple {{var}} replacement like LSP.
func Render(t string, vars map[string]string) string { return textutil.RenderTemplate(t, vars) }

// StripFences removes surrounding markdown code fences.
func StripFences(s string) string { return textutil.StripCodeFences(s) }

type chatDoer interface {
	Chat(ctx context.Context, msgs []llm.Message, opts ...llm.RequestOption) (string, error)
}

func runRewrite(ctx context.Context, cfg appconfig.App, client chatDoer, instruction, selection string) (string, error) {
    sys := cfg.PromptCodeActionRewriteSystem
    user := Render(cfg.PromptCodeActionRewriteUser, map[string]string{"instruction": instruction, "selection": selection})
    return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runDiagnostics(ctx context.Context, cfg appconfig.App, client chatDoer, diags []string, selection string) (string, error) {
	var b strings.Builder
	for i, d := range diags {
		if strings.TrimSpace(d) == "" {
			continue
		}
		b.WriteString(strings.TrimSpace(d))
		if i < len(diags)-1 {
			b.WriteString("\n")
		}
	}
	sys := cfg.PromptCodeActionDiagnosticsSystem
	user := Render(cfg.PromptCodeActionDiagnosticsUser, map[string]string{"diagnostics": b.String(), "selection": selection})
    return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runDocument(ctx context.Context, cfg appconfig.App, client chatDoer, selection string) (string, error) {
	sys := cfg.PromptCodeActionDocumentSystem
	user := Render(cfg.PromptCodeActionDocumentUser, map[string]string{"selection": selection})
    return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runGoTest(ctx context.Context, cfg appconfig.App, client chatDoer, funcCode string) (string, error) {
	sys := cfg.PromptCodeActionGoTestSystem
	user := Render(cfg.PromptCodeActionGoTestUser, map[string]string{"function": funcCode})
    return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runOnce(ctx context.Context, client chatDoer, sys, user string) (string, error) {
    msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
    txt, err := client.Chat(ctx, msgs)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(StripFences(txt)), nil
}

func runOnceWithOpts(ctx context.Context, client chatDoer, sys, user string, opts []llm.RequestOption) (string, error) {
    msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
    txt, err := client.Chat(ctx, msgs, opts...)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(StripFences(txt)), nil
}

// reqOptsFrom builds LLM request options similar to LSP behavior.
func reqOptsFrom(cfg appconfig.App) []llm.RequestOption {
    opts := []llm.RequestOption{llm.WithMaxTokens(cfg.MaxTokens)}
    if cfg.CodingTemperature != nil {
        opts = append(opts, llm.WithTemperature(*cfg.CodingTemperature))
    }
    return opts
}

// Timeout helpers to mirror LSP behavior.
func timeout10s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 10*time.Second)
}

func timeout8s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 8*time.Second)
}
