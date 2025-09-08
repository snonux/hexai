package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"codeberg.org/snonux/hexai/internal/textutil"
)

// Enabled reports whether tmux status updates are enabled via env (default: on).
func Enabled() bool {
	v := strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS"))
	if v == "" {
		return true
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// SetUserOption sets a global tmux user option like @hexai_status to value.
func SetUserOption(key, value string) error {
	if !Enabled() || !HasBinary() || !InSession() {
		return nil
	}
	k := strings.TrimPrefix(strings.TrimSpace(key), "@")
	if k == "" {
		return nil
	}
	// Use set-option -g so it appears for all windows
	return exec.Command("tmux", "set-option", "-g", "@"+k, value).Run()
}

// SetStatus is a convenience for setting @hexai_status.
func SetStatus(value string) error { return SetUserOption("hexai_status", applyTheme(value)) }

// FormatLLMStatsStatus builds a compact tmux status string for LLM heartbeats.
// Example: "LLM:gpt-4.1 5r 0.8rpm in12k out34k"
func FormatLLMStatsStatus(model string, reqs int64, rpm float64, inBytes, outBytes int64) string {
	return fmt.Sprintf("LLM:%s %dr %.1frpm in%s out%s", model, reqs, rpm, textutil.HumanBytes(inBytes), textutil.HumanBytes(outBytes))
}

// FormatLLMStatsStatusColored is like FormatLLMStatsStatus but includes provider and
// tmux color segments for readability. Uses up/down arrows for bytes.
// Example (with colors): "LLM:openai:gpt-4.1 ↑12k ↓34k 0.8rpm 5r"
func FormatLLMStatsStatusColored(provider, model string, reqs int64, rpm float64, inBytes, outBytes int64) string {
	in := textutil.HumanBytes(inBytes)
	out := textutil.HumanBytes(outBytes)
	// Keep it compact; colorize prefix and arrows; use fg resets so a themed bg can persist.
	// colour8 = dim grey, colour3 = yellow (up), colour2 = green (down)
	return fmt.Sprintf(
		"#[fg=colour8]LLM:#[fg=default]%s:%s #[fg=colour3]↑%s#[fg=default] #[fg=colour2]↓%s#[fg=default] %.1frpm %dr",
		provider, model, in, out, rpm, reqs,
	)
}

// FormatLLMStartStatus renders a short colored heartbeat at start/initialize time.
// Example: "LLM:openai:gpt-4.1 ⏳"
func FormatLLMStartStatus(provider, model string) string {
	return fmt.Sprintf("#[fg=colour8]LLM:#[fg=default]%s:%s #[fg=colour11]⏳#[fg=default]", provider, model)
}

// applyTheme wraps the status string with a user-selected tmux style if requested.
// Set HEXAI_TMUX_STATUS_THEME=white-on-purple to get white-on-purple background.
func applyTheme(s string) string {
	theme := strings.ToLower(strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS_THEME")))
	// Allow explicit fg/bg override
	fg := strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS_FG"))
	bg := strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS_BG"))
	if fg != "" || bg != "" {
		if fg == "" {
			fg = "default"
		}
		if bg == "" {
			bg = "default"
		}
		return "#[fg=" + fg + ",bg=" + bg + "]" + s + "#[fg=default,bg=default]"
	}
	if theme == "white-on-purple" || theme == "purple" || theme == "magenta" || theme == "white-on-magenta" {
		return "#[fg=white,bg=magenta]" + s + "#[fg=default,bg=default]"
	}
	if theme == "black-on-yellow" || theme == "yellow" || theme == "black-on-gold" {
		return "#[fg=black,bg=yellow]" + s + "#[fg=default,bg=default]"
	}
	if theme == "white-on-blue" || theme == "blue" || theme == "white-on-navy" {
		return "#[fg=white,bg=blue]" + s + "#[fg=default,bg=default]"
	}
	return s
}
