package hexaiaction

import (
	"context"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/textutil"
	"codeberg.org/snonux/hexai/internal/tmux"
)

// Render performs simple {{var}} replacement like LSP.
func Render(t string, vars map[string]string) string { return textutil.RenderTemplate(t, vars) }

// StripFences removes surrounding markdown code fences.
func StripFences(s string) string { return textutil.StripCodeFences(s) }

type chatDoer interface {
	Chat(ctx context.Context, msgs []llm.Message, opts ...llm.RequestOption) (string, error)
	DefaultModel() string
}

type providerNamer interface{ Name() string }

func providerOf(c any) string {
	if n, ok := c.(providerNamer); ok {
		return n.Name()
	}
	return "llm"
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

func runSimplify(ctx context.Context, cfg appconfig.App, client chatDoer, selection string) (string, error) {
	sys := cfg.PromptCodeActionSimplifySystem
	user := Render(cfg.PromptCodeActionSimplifyUser, map[string]string{"selection": selection})
	return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runGoTest(ctx context.Context, cfg appconfig.App, client chatDoer, funcCode string) (string, error) {
	sys := cfg.PromptCodeActionGoTestSystem
	user := Render(cfg.PromptCodeActionGoTestUser, map[string]string{"function": funcCode})
	return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runCustom(ctx context.Context, cfg appconfig.App, client chatDoer, ca appconfig.CustomAction, parts InputParts) (string, error) {
	// If user template is provided, prefer it and optional system
	if strings.TrimSpace(ca.User) != "" {
		sys := cfg.PromptCodeActionRewriteSystem
		if strings.TrimSpace(ca.System) != "" {
			sys = ca.System
		}
		// Currently only selection is available in tmux path; diagnostics list not wired
		user := Render(ca.User, map[string]string{"selection": parts.Selection, "diagnostics": strings.Join(parts.Diagnostics, "\n")})
		return runOnceWithOpts(ctx, client, sys, user, reqOptsFrom(cfg))
	}
	// Else, use fixed instruction through rewrite template
	return runRewrite(ctx, cfg, client, ca.Instruction, parts.Selection)
}

func runOnce(ctx context.Context, client chatDoer, sys, user string) (string, error) {
	msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	txt, err := client.Chat(ctx, msgs)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(StripFences(txt))
	// Contribute to global stats and update tmux status
	sent := 0
	for _, m := range msgs {
		sent += len(m.Content)
	}
	recv := len(out)
	_ = stats.Update(ctx, providerOf(client), client.DefaultModel(), sent, recv)
	if snap, err := stats.TakeSnapshot(); err == nil {
		minsWin := snap.Window.Minutes()
		if minsWin <= 0 {
			minsWin = 0.001
		}
		scopeReqs := int64(0)
		if pe, ok := snap.Providers[providerOf(client)]; ok {
			if mc, ok2 := pe.Models[client.DefaultModel()]; ok2 {
				scopeReqs = mc.Reqs
			}
		}
		scopeRPM := float64(scopeReqs) / minsWin
		_ = tmux.SetStatus(tmux.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, providerOf(client), client.DefaultModel(), scopeRPM, scopeReqs, snap.Window))
	}
	return out, nil
}

func runOnceWithOpts(ctx context.Context, client chatDoer, sys, user string, opts []llm.RequestOption) (string, error) {
	msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	txt, err := client.Chat(ctx, msgs, opts...)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(StripFences(txt))
	// Contribute to global stats and update tmux status
	sent := 0
	for _, m := range msgs {
		sent += len(m.Content)
	}
	recv := len(out)
	_ = stats.Update(ctx, providerOf(client), client.DefaultModel(), sent, recv)
	if snap, err := stats.TakeSnapshot(); err == nil {
		minsWin := snap.Window.Minutes()
		if minsWin <= 0 {
			minsWin = 0.001
		}
		scopeReqs := int64(0)
		if pe, ok := snap.Providers[providerOf(client)]; ok {
			if mc, ok2 := pe.Models[client.DefaultModel()]; ok2 {
				scopeReqs = mc.Reqs
			}
		}
		scopeRPM := float64(scopeReqs) / minsWin
		_ = tmux.SetStatus(tmux.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, providerOf(client), client.DefaultModel(), scopeRPM, scopeReqs, snap.Window))
	}
	return out, nil
}

// reqOptsFrom builds LLM request options similar to LSP behavior.
func reqOptsFrom(cfg appconfig.App) []llm.RequestOption {
	opts := []llm.RequestOption{llm.WithMaxTokens(cfg.MaxTokens)}
	// Apply temperature, with special-case for gpt-5 (default temp must be 1.0)
	if cfg.CodingTemperature != nil {
		temp := *cfg.CodingTemperature
		prov := strings.ToLower(strings.TrimSpace(cfg.Provider))
		model := strings.ToLower(strings.TrimSpace(cfg.OpenAIModel))
		if prov == "openai" && strings.HasPrefix(model, "gpt-5") {
			temp = 1.0
		}
		opts = append(opts, llm.WithTemperature(temp))
	}
	return opts
}

// Timeout helpers to mirror LSP behavior.
func timeout10s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 20*time.Second)
}

func timeout8s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 18*time.Second)
}
