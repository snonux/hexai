// Tests for ignore config, tmux-edit config, and low-level parsing helpers
// (temperature, model entries, surface entries, resolved model).
package appconfig

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIgnoreConfig_Defaults(t *testing.T) {
	clearHexaiEnv(t)
	cfg := Load(context.Background(), nil)
	if cfg.IgnoreGitignore == nil || !*cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore default true")
	}
	if cfg.IgnoreLSPNotify == nil || !*cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify default true")
	}
	if len(cfg.IgnoreExtraPatterns) != 0 {
		t.Errorf("expected empty IgnoreExtraPatterns, got %v", cfg.IgnoreExtraPatterns)
	}
}

func TestIgnoreConfig_FromFile(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = false
extra_patterns = ["*.min.js", "dist/**"]
lsp_notify_ignored = false
`)
	cfg := LoadWithOptions(context.Background(), newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: dir})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore false from file")
	}
	if cfg.IgnoreLSPNotify == nil || *cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify false from file")
	}
	want := []string{"*.min.js", "dist/**"}
	if !reflect.DeepEqual(cfg.IgnoreExtraPatterns, want) {
		t.Errorf("IgnoreExtraPatterns = %v, want %v", cfg.IgnoreExtraPatterns, want)
	}
}

func TestIgnoreConfig_EnvOverrides(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = true
lsp_notify_ignored = true
`)
	withEnv(t, "HEXAI_IGNORE_GITIGNORE", "false")
	withEnv(t, "HEXAI_IGNORE_LSP_NOTIFY", "0")
	withEnv(t, "HEXAI_IGNORE_EXTRA_PATTERNS", "*.bak,*.tmp")
	cfg := LoadWithOptions(context.Background(), newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: dir})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore false from env override")
	}
	if cfg.IgnoreLSPNotify == nil || *cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify false from env override")
	}
	want := []string{"*.bak", "*.tmp"}
	if !reflect.DeepEqual(cfg.IgnoreExtraPatterns, want) {
		t.Errorf("IgnoreExtraPatterns = %v, want %v", cfg.IgnoreExtraPatterns, want)
	}
}

func TestIgnoreConfig_ProjectOverride(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = true
`)
	// Set up a fake git repo with project override
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	projectCfg := filepath.Join(projectDir, ProjectConfigFilename)
	writeFile(t, projectCfg, `
[ignore]
gitignore = false
extra_patterns = ["build/**"]
`)
	cfg := LoadWithOptions(context.Background(), newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: projectDir})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected project override to set IgnoreGitignore false")
	}
	want := []string{"build/**"}
	if !reflect.DeepEqual(cfg.IgnoreExtraPatterns, want) {
		t.Errorf("IgnoreExtraPatterns = %v, want %v", cfg.IgnoreExtraPatterns, want)
	}
}

func TestIgnoreConfig_DisableGitignore(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[ignore]
gitignore = false
`)
	cfg := LoadWithOptions(context.Background(), newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: dir})
	if cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore {
		t.Error("expected IgnoreGitignore false")
	}
	// LSP notify should still be true (default, not overridden)
	if cfg.IgnoreLSPNotify == nil || !*cfg.IgnoreLSPNotify {
		t.Error("expected IgnoreLSPNotify to remain true (default)")
	}
}

func TestTmuxEditConfig_FromFile(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[tmux_edit]
popup_width = "90%"
popup_height = "85%"
default_agent = "claude"

[[tmux_edit.agents]]
name = "claude"
display_name = "Claude Code"
detect_pattern = "(?i)(claude|anthropic)"
prompt_pattern = '(?s)>\s*(.+?)$'
clear_first = true
clear_keys = "C-u"
newline_keys = "S-Enter"
submit_keys = "Enter"

[[tmux_edit.agents]]
name = "cursor"
display_name = "Cursor"
detect_pattern = "(?i)cursor"
prompt_pattern = '(?s)│\s*(.+?)$'
strip_patterns = ["INSERT", "Add a follow-up"]
clear_first = true
clear_keys = "C-u"
newline_keys = "S-Enter"
submit_keys = "Enter"
`)
	cfg := LoadWithOptions(context.Background(), newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: dir})
	if cfg.TmuxEditPopupWidth != "90%" {
		t.Errorf("PopupWidth = %q, want 90%%", cfg.TmuxEditPopupWidth)
	}
	if cfg.TmuxEditPopupHeight != "85%" {
		t.Errorf("PopupHeight = %q, want 85%%", cfg.TmuxEditPopupHeight)
	}
	if cfg.TmuxEditDefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want claude", cfg.TmuxEditDefaultAgent)
	}
	if len(cfg.TmuxEditAgents) != 2 {
		t.Fatalf("got %d agents, want 2", len(cfg.TmuxEditAgents))
	}
	a := cfg.TmuxEditAgents[0]
	if a.Name != "claude" || a.DisplayName != "Claude Code" {
		t.Errorf("agent[0] = %q/%q, want claude/Claude Code", a.Name, a.DisplayName)
	}
	if a.ClearFirst == nil || !*a.ClearFirst {
		t.Error("expected ClearFirst = true for claude agent")
	}
	b := cfg.TmuxEditAgents[1]
	if b.Name != "cursor" {
		t.Errorf("agent[1].Name = %q, want cursor", b.Name)
	}
	if len(b.StripPatterns) != 2 {
		t.Errorf("agent[1].StripPatterns = %v, want 2 entries", b.StripPatterns)
	}
}

func TestTmuxEditConfig_Merge(t *testing.T) {
	clearHexaiEnv(t)
	a := newDefaultConfig()
	b := App{
		FeatureConfig: FeatureConfig{
			TmuxEditPopupWidth:   "70%",
			TmuxEditDefaultAgent: "amp",
			TmuxEditAgents: []TmuxEditAgentCfg{
				{Name: "amp", DisplayName: "Amp"},
			},
		},
	}
	a.mergeWith(&b)
	if a.TmuxEditPopupWidth != "70%" {
		t.Errorf("PopupWidth = %q, want 70%%", a.TmuxEditPopupWidth)
	}
	if a.TmuxEditDefaultAgent != "amp" {
		t.Errorf("DefaultAgent = %q, want amp", a.TmuxEditDefaultAgent)
	}
	if len(a.TmuxEditAgents) != 1 || a.TmuxEditAgents[0].Name != "amp" {
		t.Errorf("Agents = %v, want single amp", a.TmuxEditAgents)
	}
}

func TestTmuxEditConfig_SkipsEmptyName(t *testing.T) {
	clearHexaiEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeFile(t, cfgPath, `
[tmux_edit]
[[tmux_edit.agents]]
name = ""
display_name = "Empty"
`)
	cfg := LoadWithOptions(context.Background(), newLogger(), LoadOptions{ConfigPath: cfgPath, ProjectRoot: dir})
	if len(cfg.TmuxEditAgents) != 0 {
		t.Errorf("got %d agents, want 0 (empty name should be skipped)", len(cfg.TmuxEditAgents))
	}
}

// --- Config Parsing Tests ---

func TestParseTemperatureValue(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantValue *float64
		wantOK    bool
	}{
		{"float64 zero", float64(0.0), floatPtr(0.0), true},
		{"float64 half", float64(0.5), floatPtr(0.5), true},
		{"float64 one", float64(1.0), floatPtr(1.0), true},
		{"float64 two", float64(2.0), floatPtr(2.0), true},
		{"int64 zero", int64(0), floatPtr(0.0), true},
		{"int64 one", int64(1), floatPtr(1.0), true},
		{"int64 two", int64(2), floatPtr(2.0), true},
		{"string zero", "0", floatPtr(0.0), true},
		{"string one", "1", floatPtr(1.0), true},
		{"string two", "2", floatPtr(2.0), true},
		{"string float", "0.75", floatPtr(0.75), true},
		{"string empty", "", nil, true},
		{"string whitespace", "  ", nil, true},
		{"string invalid", "invalid", nil, false},
		{"string negative", "-0.5", floatPtr(-0.5), true},
		{"string very small", "0.0001", floatPtr(0.0001), true},
		{"string high precision", "1.123456789", floatPtr(1.123456789), true},
		{"nil value", nil, nil, false},
		{"bool value", true, nil, false},
		{"map value", map[string]any{}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTemperatureValue(tt.input, "test", newLogger())
			if ok != tt.wantOK {
				t.Errorf("parseTemperatureValue() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if (got == nil) != (tt.wantValue == nil) {
				t.Errorf("parseTemperatureValue() = %v, want %v", got, tt.wantValue)
				return
			}
			if got != nil && tt.wantValue != nil && *got != *tt.wantValue {
				t.Errorf("parseTemperatureValue() = %v, want %v", *got, *tt.wantValue)
			}
		})
	}
}

func TestDecodeModelEntry(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantCfg *SurfaceConfig
		wantOK  bool
	}{
		{
			name:    "simple string model",
			input:   "gpt-4",
			wantCfg: &SurfaceConfig{Model: "gpt-4"},
			wantOK:  true,
		},
		{
			name:    "empty string",
			input:   "",
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name:    "whitespace string",
			input:   "  ",
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name: "object with all fields",
			input: map[string]any{
				"model":       "claude-3",
				"provider":    "anthropic",
				"temperature": float64(0.7),
			},
			wantCfg: &SurfaceConfig{
				Model:       "claude-3",
				Provider:    "anthropic",
				Temperature: floatPtr(0.7),
			},
			wantOK: true,
		},
		{
			name: "object with model only",
			input: map[string]any{
				"model": "gpt-4o",
			},
			wantCfg: &SurfaceConfig{Model: "gpt-4o"},
			wantOK:  true,
		},
		{
			name: "object with provider only",
			input: map[string]any{
				"provider": "openai",
			},
			wantCfg: &SurfaceConfig{Provider: "openai"},
			wantOK:  true,
		},
		{
			name: "object with temperature only",
			input: map[string]any{
				"temperature": float64(0.5),
			},
			wantCfg: &SurfaceConfig{Temperature: floatPtr(0.5)},
			wantOK:  true,
		},
		{
			name: "object with empty fields",
			input: map[string]any{
				"model":    "",
				"provider": "",
			},
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name: "object with invalid model type",
			input: map[string]any{
				"model": 123,
			},
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name: "object with invalid provider type",
			input: map[string]any{
				"provider": 456,
			},
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name: "object with invalid temperature",
			input: map[string]any{
				"model":       "gpt-4",
				"temperature": "not a number",
			},
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name:    "nil input",
			input:   nil,
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name:    "invalid type (int)",
			input:   123,
			wantCfg: nil,
			wantOK:  false,
		},
		{
			name:    "invalid type (slice)",
			input:   []string{"gpt-4"},
			wantCfg: nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeModelEntry(tt.input, "test", newLogger())
			if ok != tt.wantOK {
				t.Errorf("decodeModelEntry() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if (got == nil) != (tt.wantCfg == nil) {
				t.Errorf("decodeModelEntry() = %v, want %v", got, tt.wantCfg)
				return
			}
			if got == nil {
				return
			}
			if got.Model != tt.wantCfg.Model {
				t.Errorf("Model = %q, want %q", got.Model, tt.wantCfg.Model)
			}
			if got.Provider != tt.wantCfg.Provider {
				t.Errorf("Provider = %q, want %q", got.Provider, tt.wantCfg.Provider)
			}
			if (got.Temperature == nil) != (tt.wantCfg.Temperature == nil) {
				t.Errorf("Temperature nil mismatch: got %v, want %v", got.Temperature, tt.wantCfg.Temperature)
			} else if got.Temperature != nil && *got.Temperature != *tt.wantCfg.Temperature {
				t.Errorf("Temperature = %v, want %v", *got.Temperature, *tt.wantCfg.Temperature)
			}
		})
	}
}

func TestResolvedModel(t *testing.T) {
	tests := []struct {
		name    string
		section sectionOpenAI
		want    string
	}{
		{
			name:    "explicit model no presets",
			section: sectionOpenAI{Model: "gpt-4"},
			want:    "gpt-4",
		},
		{
			name:    "empty model",
			section: sectionOpenAI{Model: ""},
			want:    "",
		},
		{
			name:    "whitespace model",
			section: sectionOpenAI{Model: "  "},
			want:    "",
		},
		{
			name: "preset match exact case",
			section: sectionOpenAI{
				Model:   "fast",
				Presets: map[string]string{"fast": "gpt-3.5-turbo"},
			},
			want: "gpt-3.5-turbo",
		},
		{
			name: "preset match case insensitive",
			section: sectionOpenAI{
				Model:   "FAST",
				Presets: map[string]string{"fast": "gpt-3.5-turbo"},
			},
			want: "gpt-3.5-turbo",
		},
		{
			name: "no preset match returns original",
			section: sectionOpenAI{
				Model:   "custom-model",
				Presets: map[string]string{"fast": "gpt-3.5-turbo"},
			},
			want: "custom-model",
		},
		{
			name: "preset empty value returns original",
			section: sectionOpenAI{
				Model:   "fast",
				Presets: map[string]string{"fast": ""},
			},
			want: "fast",
		},
		{
			name: "preset whitespace value returns original",
			section: sectionOpenAI{
				Model:   "fast",
				Presets: map[string]string{"fast": "  "},
			},
			want: "fast",
		},
		{
			name: "multiple presets uses correct one",
			section: sectionOpenAI{
				Model: "smart",
				Presets: map[string]string{
					"fast":  "gpt-3.5-turbo",
					"smart": "gpt-4",
					"mini":  "gpt-3.5-mini",
				},
			},
			want: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.section.resolvedModel()
			if got != tt.want {
				t.Errorf("resolvedModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSurfaceEntries(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantLen int
		wantOK  bool
	}{
		{
			name:    "nil input",
			input:   nil,
			wantLen: 0,
			wantOK:  false,
		},
		{
			name:    "single string",
			input:   "gpt-4",
			wantLen: 1,
			wantOK:  true,
		},
		{
			name: "single map",
			input: map[string]any{
				"model":    "claude-3",
				"provider": "anthropic",
			},
			wantLen: 1,
			wantOK:  true,
		},
		{
			name: "array of strings",
			input: []any{
				"gpt-4",
				"claude-3",
			},
			wantLen: 2,
			wantOK:  true,
		},
		{
			name: "array of maps",
			input: []any{
				map[string]any{"model": "gpt-4", "provider": "openai"},
				map[string]any{"model": "claude-3", "provider": "anthropic"},
			},
			wantLen: 2,
			wantOK:  true,
		},
		{
			name: "array with invalid entries",
			input: []any{
				"gpt-4",
				123,
				"claude-3",
			},
			wantLen: 2,
			wantOK:  true,
		},
		{
			name: "array with all invalid entries",
			input: []any{
				123,
				true,
				nil,
			},
			wantLen: 0,
			wantOK:  false,
		},
		{
			name:    "empty array",
			input:   []any{},
			wantLen: 0,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSurfaceEntries(tt.input, "test", newLogger())
			if ok != tt.wantOK {
				t.Errorf("parseSurfaceEntries() ok = %v, want %v", ok, tt.wantOK)
			}
			if len(got) != tt.wantLen {
				t.Errorf("parseSurfaceEntries() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}
