package hexaiaction

import (
	"context"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/chatrun"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
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

// selectActionTemperature resolves the effective temperature for a code action,
// delegating GPT-5 override logic to llmutils.ResolveTemperature.
func selectActionTemperature(cfg actionConfig, provider string, entry appconfig.SurfaceConfig, model string) (float64, bool) {
	return llmutils.ResolveTemperature(provider, model, entry.Temperature, cfg.CoreSection().CodingTemperature)
}

func runRewrite(ctx context.Context, cfg actionConfig, client chatDoer, instruction, selection string) (string, error) {
	prompts := cfg.PromptSection()
	sys := prompts.PromptCodeActionRewriteSystem
	user := Render(prompts.PromptCodeActionRewriteUser, map[string]string{"instruction": instruction, "selection": selection})
	return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runDiagnostics(ctx context.Context, cfg actionConfig, client chatDoer, diags []string, selection string) (string, error) {
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
	prompts := cfg.PromptSection()
	sys := prompts.PromptCodeActionDiagnosticsSystem
	user := Render(prompts.PromptCodeActionDiagnosticsUser, map[string]string{"diagnostics": b.String(), "selection": selection})
	return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runDocument(ctx context.Context, cfg actionConfig, client chatDoer, selection string) (string, error) {
	prompts := cfg.PromptSection()
	sys := prompts.PromptCodeActionDocumentSystem
	user := Render(prompts.PromptCodeActionDocumentUser, map[string]string{"selection": selection})
	return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runSimplify(ctx context.Context, cfg actionConfig, client chatDoer, selection string) (string, error) {
	prompts := cfg.PromptSection()
	sys := prompts.PromptCodeActionSimplifySystem
	user := Render(prompts.PromptCodeActionSimplifyUser, map[string]string{"selection": selection})
	return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runFixTypos(ctx context.Context, cfg actionConfig, client chatDoer, selection string) (string, error) {
	prompts := cfg.PromptSection()
	sys := prompts.PromptCodeActionFixTyposSystem
	user := Render(prompts.PromptCodeActionFixTyposUser, map[string]string{"selection": selection})
	return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runGoTest(ctx context.Context, cfg actionConfig, client chatDoer, funcCode string) (string, error) {
	prompts := cfg.PromptSection()
	sys := prompts.PromptCodeActionGoTestSystem
	user := Render(prompts.PromptCodeActionGoTestUser, map[string]string{"function": funcCode})
	return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
}

func runCustom(ctx context.Context, cfg actionConfig, client chatDoer, ca appconfig.CustomAction, parts InputParts) (string, error) {
	prompts := cfg.PromptSection()
	// If user template is provided, prefer it and optional system
	if strings.TrimSpace(ca.User) != "" {
		sys := prompts.PromptCodeActionRewriteSystem
		if strings.TrimSpace(ca.System) != "" {
			sys = ca.System
		}
		// Currently only selection is available in tmux path; diagnostics list not wired
		user := Render(ca.User, map[string]string{"selection": parts.Selection, "diagnostics": strings.Join(parts.Diagnostics, "\n")})
		return runOnce(ctx, client, sys, user, reqOptsFrom(cfg))
	}
	// Else, use fixed instruction through rewrite template
	return runRewrite(ctx, cfg, client, ca.Instruction, parts.Selection)
}

// runOnce sends a single system+user prompt pair to the LLM, strips code
// fences from the response, records stats, and updates the tmux status line.
// Pass a zero-value requestArgs{} when no extra options are needed.
//
// The LLM invocation and byte/stats accounting are delegated to the shared
// chatrun package so this surface stays in lock-step with the CLI and LSP. The
// tmux status update is kept here because it is specific to the action tool's
// reporting style.
func runOnce(ctx context.Context, client chatDoer, sys, user string, req requestArgs) (string, error) {
	msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	// runOnce never streams to a writer, so pass nil out.
	txt, err := chatrun.Invoke(ctx, client, msgs, req.options, nil)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(StripFences(txt))
	model := strings.TrimSpace(req.model)
	if model == "" {
		model = client.DefaultModel()
	}
	provider := providerOf(client)
	// Account for the exchange against the post-fence-strip response so the
	// recorded recv bytes match what the user actually receives.
	chatrun.Account(ctx, provider, model, msgs, out)
	if snap, err := stats.TakeSnapshot(); err == nil {
		scopeReqs := snap.ScopeReqs(provider, model)
		scopeRPM := snap.ScopeRPM(provider, model)
		_ = tmux.SetStatus(tmux.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, provider, model, scopeRPM, scopeReqs, snap.Window))
	}
	return out, nil
}

// reqOptsFrom builds LLM request options similar to LSP behavior.
func reqOptsFrom(cfg actionConfig) requestArgs {
	core := cfg.CoreSection()
	providers := cfg.ProviderSection()
	fullCfg := actionConfigAsApp(cfg)
	opts := make([]llm.RequestOption, 0, 3)
	if core.MaxTokens > 0 {
		opts = append(opts, llm.WithMaxTokens(core.MaxTokens))
	}
	provider := llmutils.CanonicalProvider(core.Provider)
	entries := providers.CodeActionConfigs
	if len(entries) == 0 {
		entries = []appconfig.SurfaceConfig{{Provider: core.Provider, Model: strings.TrimSpace(llmutils.DefaultModelForProvider(fullCfg, provider))}}
	}
	primary := entries[0]
	if strings.TrimSpace(primary.Provider) != "" {
		provider = llmutils.CanonicalProvider(primary.Provider)
	}
	model := strings.TrimSpace(primary.Model)
	if model == "" {
		model = strings.TrimSpace(llmutils.DefaultModelForProvider(fullCfg, provider))
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
func timeout20s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 20*time.Second)
}

func timeout18s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 18*time.Second)
}
