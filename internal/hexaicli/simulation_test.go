package hexaicli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

func TestParseTPSSimulation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantMin float64
		wantMax float64
		wantErr bool
	}{
		{name: "single value", value: "12", wantMin: 12, wantMax: 12},
		{name: "dash range", value: "8-16", wantMin: 8, wantMax: 16},
		{name: "colon range", value: "4:9", wantMin: 4, wantMax: 9},
		{name: "empty", value: "", wantErr: true},
		{name: "zero", value: "0", wantErr: true},
		{name: "reversed", value: "20-10", wantErr: true},
		{name: "invalid", value: "fast", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, err := parseTPSSimulation(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.min != tc.wantMin || spec.max != tc.wantMax {
				t.Fatalf("unexpected spec: %+v", spec)
			}
		})
	}
}

func TestReadSimulationInput_DefaultsToSampleText(t *testing.T) {
	restore, f := setStdin(t, "")
	defer restore()

	input, err := readSimulationInput(f, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(input, "Hexai TPS simulation mode") {
		t.Fatalf("expected default simulation text, got %q", input)
	}
}

func TestRun_TPSSimulationBypassesClientSetup(t *testing.T) {
	clientFactory := func(appconfig.App) (llm.Client, error) {
		t.Fatalf("client setup should not be called in TPS simulation mode")
		return nil, nil
	}

	var out, errb bytes.Buffer
	ctx := withCLIClientFactory(WithCLITPSSimulation(context.Background(), "1000000"), clientFactory)
	if err := Run(ctx, []string{"simulated", "output"}, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if out.String() != "simulated output" {
		t.Fatalf("unexpected simulation output: %q", out.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errb.String())
	}
}

func TestRun_TPSSimulationUsesStdin(t *testing.T) {
	clientFactory := func(appconfig.App) (llm.Client, error) {
		t.Fatalf("client setup should not be called in TPS simulation mode")
		return nil, nil
	}

	restore, f := setStdin(t, "from-stdin")
	defer restore()

	var out, errb bytes.Buffer
	ctx := withCLIClientFactory(WithCLITPSSimulation(context.Background(), "1000000"), clientFactory)
	if err := Run(ctx, nil, f, &out, &errb); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if out.String() != "from-stdin" {
		t.Fatalf("unexpected simulation output: %q", out.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errb.String())
	}
}
