package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"codeberg.org/snonux/hexai/internal/textutil"
)

// baseFGToken is a placeholder inserted by status formatters wherever the
// base foreground color should be restored. The theming layer (applyTheme)
// replaces this token with a tmux color sequence matching the active theme's
// foreground, which fixes readability when a theme sets a non-default fg.
const (
	baseFGToken    = "\x1EHEXAI_BASE_FG\x1E"
	arrowUpToken   = "\x1EHEXAI_ARROW_UP\x1E"
	arrowDownToken = "\x1EHEXAI_ARROW_DOWN\x1E"
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
	// Arrows use theme-aware styles; bytes immediately switch to base fg for contrast.
	return fmt.Sprintf(
		"%sLLM:%s:%s %s↑%s%s %s↓%s%s %.1frpm %dr",
		baseFGToken, provider, model, arrowUpToken, baseFGToken, in, arrowDownToken, baseFGToken, out, rpm, reqs,
	)
}

// FormatLLMStartStatus renders a short colored heartbeat at start/initialize time.
// Example: "LLM:openai:gpt-4.1 ⏳"
func FormatLLMStartStatus(provider, model string) string {
	return fmt.Sprintf("%sLLM:%s:%s #[fg=colour11]⏳%s", baseFGToken, provider, model, baseFGToken)
}

// applyTheme wraps the status string with a user-selected tmux style if requested.
// Set HEXAI_TMUX_STATUS_THEME=white-on-purple to get white-on-purple background.
func applyTheme(s string) string {
	theme := strings.ToLower(strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS_THEME")))
	// Allow explicit fg/bg override
	fg := strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS_FG"))
	bg := strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS_BG"))
	// Determine base foreground and background from env or theme presets
	baseFG := ""
	wrap := false
	if fg != "" || bg != "" { // explicit override path
		wrap = true
		if fg == "" {
			baseFG = "default"
		} else {
			baseFG = fg
		}
		// bg used as provided (may be empty)
	} else {
		switch theme {
		case "white-on-purple", "purple", "magenta", "white-on-magenta":
			baseFG, bg, wrap = "white", "magenta", true
		case "black-on-yellow", "yellow", "black-on-gold":
			baseFG, bg, wrap = "black", "yellow", true
		case "white-on-blue", "blue", "white-on-navy":
			baseFG, bg, wrap = "white", "blue", true
		}
		if baseFG == "" { // no theme selected
			baseFG = "default"
		}
	}

	// Theme-aware arrow styles
	upStyle, downStyle := "#[fg=colour3]", "#[fg=colour2]" // defaults: yellow up, green down
	if fg != "" || bg != "" {                              // explicit override path: match arrows to base fg, bold for visibility
		upStyle = "#[bold,fg=" + baseFG + "]"
		downStyle = upStyle
	} else {
		switch theme {
		case "white-on-purple", "purple", "magenta", "white-on-magenta":
			upStyle, downStyle = "#[bold,fg=black]", "#[bold,fg=black]"
		case "black-on-yellow", "yellow", "black-on-gold":
			upStyle, downStyle = "#[bold,fg=black]", "#[bold,fg=black]"
		case "white-on-blue", "blue", "white-on-navy":
			upStyle, downStyle = "#[bold,fg=white]", "#[bold,fg=white]"
		}
	}

	// Replace base-foreground and arrow placeholders with selected styles
	if strings.Contains(s, baseFGToken) {
		s = strings.ReplaceAll(s, baseFGToken, "#[fg="+baseFG+"]")
	}
	if strings.Contains(s, arrowUpToken) {
		s = strings.ReplaceAll(s, arrowUpToken, upStyle)
	}
	if strings.Contains(s, arrowDownToken) {
		s = strings.ReplaceAll(s, arrowDownToken, downStyle)
	}

	if !wrap {
		return s
	}
	// Wrap with base fg and optional bg, then reset at the end
	prefix := "#[fg=" + baseFG
	if bg != "" {
		prefix += ",bg=" + bg
	}
	prefix += "]"
	return prefix + s + "#[fg=default,bg=default]"
}
