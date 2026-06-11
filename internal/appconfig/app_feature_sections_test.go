package appconfig

import (
	"reflect"
	"testing"
)

// buildFeatureApp returns an App whose FeatureConfig subsystems are all
// populated, used to verify the fine-grained section accessors.
func buildFeatureApp() App {
	cfg := App{}
	cfg.ApplyFeatureSection(testFeatureConfig())
	return cfg
}

func TestIgnoreSectionCopiesAndReads(t *testing.T) {
	cfg := buildFeatureApp()
	got := cfg.IgnoreSection()
	if !reflect.DeepEqual(got.IgnoreExtraPatterns, []string{"vendor/**", "tmp/**"}) {
		t.Fatalf("IgnoreExtraPatterns = %v", got.IgnoreExtraPatterns)
	}
	// Mutating the returned copy must not affect the source App.
	got.IgnoreExtraPatterns[0] = "mutated"
	if cfg.IgnoreExtraPatterns[0] == "mutated" {
		t.Fatal("IgnoreSection did not return a defensive copy")
	}
}

func TestStatsSectionReads(t *testing.T) {
	cfg := buildFeatureApp()
	if got := cfg.StatsSection(); got.StatsWindowMinutes != 15 {
		t.Fatalf("StatsWindowMinutes = %d, want 15", got.StatsWindowMinutes)
	}
}

func TestTmuxEditSectionCopies(t *testing.T) {
	cfg := buildFeatureApp()
	got := cfg.TmuxEditSection()
	if got.TmuxEditDefaultAgent != "codex" || len(got.TmuxEditAgents) != 1 {
		t.Fatalf("unexpected tmux edit section: %+v", got)
	}
	got.TmuxEditAgents[0].Name = "mutated"
	if cfg.TmuxEditAgents[0].Name == "mutated" {
		t.Fatal("TmuxEditSection did not return a defensive copy")
	}
}

func TestTmuxActionSectionCopies(t *testing.T) {
	cfg := App{}
	cfg.TmuxActionMenu = []TmuxActionMenuEntry{{Kind: "rewrite"}}
	got := cfg.TmuxActionSection()
	if len(got.TmuxActionMenu) != 1 || got.TmuxActionMenu[0].Kind != "rewrite" {
		t.Fatalf("unexpected tmux action section: %+v", got)
	}
	got.TmuxActionMenu[0].Kind = "mutated"
	if cfg.TmuxActionMenu[0].Kind == "mutated" {
		t.Fatal("TmuxActionSection did not return a defensive copy")
	}
}

func TestMCPSectionReads(t *testing.T) {
	cfg := buildFeatureApp()
	got := cfg.MCPSection()
	if got.MCPPromptsDir != ".hexai/prompts" || !got.MCPSlashCommandSync ||
		got.MCPSlashCommandDir != ".hexai/slash" {
		t.Fatalf("unexpected MCP section: %+v", got)
	}
}
