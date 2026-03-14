package hexaicli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

func TestRun_DefaultRequestTimeoutIsTenMinutes(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HEXAI_REQUEST_TIMEOUT", "")

	oldNew := newClientFromApp
	defer func() { newClientFromApp = oldNew }()

	seenTimeout := 0
	newClientFromApp = func(cfg appconfig.App) (llm.Client, error) {
		seenTimeout = cfg.RequestTimeout
		return okClient{}, nil
	}

	var out, errb bytes.Buffer
	if err := Run(context.Background(), []string{"hello"}, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if seenTimeout != 600 {
		t.Fatalf("RequestTimeout = %d, want 600", seenTimeout)
	}
	if !strings.Contains(out.String(), "OK") {
		t.Fatalf("expected output from fake client, got %q", out.String())
	}
}
