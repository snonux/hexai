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

type requestArgs struct {
	model   string
	options []llm.RequestOption
}

func providerOf(c any) string {
	if n, ok := c.(providerNamer); ok {
		return n.Name()
	}
	return "llm"
}

func canonicalProvider(name string) string {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return "openai"
	}
	return p
}

func defaultModelForProvider(cfg appconfig.App, provider string) string {
	switch provider {
	case "ollama":
		return cfg.OllamaModel
	case "copilot":
		return cfg.CopilotModel
	default:
		return cfg.OpenAIModel
	}
}

func selectActionTemperature(cfg appconfig.App, provider string, entry appconfig.SurfaceConfig, model string) (float64, bool) {
	if entry.Temperature != nil {
		return *entry.Temperature, true
	}
	if cfg.CodingTemperature != nil {
		temp := *cfg.CodingTemperature
		if provider == "openai" && strings.HasPrefix(strings.ToLower(model), "gpt-5") && temp == 0.2 {
			temp = 1.0
		}
		return temp, true
	}
	if provider == "openai" && strings.HasPrefix(strings.ToLower(model), "gpt-5") {
		return 1.0, true
	}
	return 0, false
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

func runOnceWithOpts(ctx context.Context, client chatDoer, sys, user string, req requestArgs) (string, error) {
	msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	txt, err := client.Chat(ctx, msgs, req.options...)
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
	model := strings.TrimSpace(req.model)
	if model == "" {
		model = client.DefaultModel()
	}
	_ = stats.Update(ctx, providerOf(client), model, sent, recv)
	if snap, err := stats.TakeSnapshot(); err == nil {
		minsWin := snap.Window.Minutes()
		if minsWin <= 0 {
			minsWin = 0.001
		}
		scopeReqs := int64(0)
		if pe, ok := snap.Providers[providerOf(client)]; ok {
			if mc, ok2 := pe.Models[model]; ok2 {
				scopeReqs = mc.Reqs
			}
		}
		scopeRPM := float64(scopeReqs) / minsWin
		_ = tmux.SetStatus(tmux.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, providerOf(client), model, scopeRPM, scopeReqs, snap.Window))
	}
	return out, nil
}

// reqOptsFrom builds LLM request options similar to LSP behavior.
func reqOptsFrom(cfg appconfig.App) requestArgs {
	opts := make([]llm.RequestOption, 0, 3)
	if cfg.MaxTokens > 0 {
		opts = append(opts, llm.WithMaxTokens(cfg.MaxTokens))
	}
	provider := canonicalProvider(cfg.Provider)
	entries := cfg.CodeActionConfigs
	if len(entries) == 0 {
		entries = []appconfig.SurfaceConfig{{Provider: cfg.Provider, Model: strings.TrimSpace(defaultModelForProvider(cfg, provider))}}
	}
	primary := entries[0]
	if strings.TrimSpace(primary.Provider) != "" {
		provider = canonicalProvider(primary.Provider)
	}
	model := strings.TrimSpace(primary.Model)
	if model == "" {
		model = strings.TrimSpace(defaultModelForProvider(cfg, provider))
	}
	if strings.TrimSpace(primary.Model) != "" {
		opts = append(opts, llm.WithModel(strings.TrimSpace(primary.Model)))
	}
	if temp, ok := selectActionTemperature(cfg, provider, primary, model); ok {
		opts = append(opts, llm.WithTemperature(temp))
	}
	return requestArgs{model: model, options: opts}
}

// Timeout helpers to mirror LSP behavior.
func timeout10s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 20*time.Second)
}

func timeout8s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 18*time.Second)
}
