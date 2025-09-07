package hexaiaction

import (
    "bytes"
    "context"
    "os"
    "testing"
)

// Covers the early error path in Run when no API key is available for the default provider.
func TestRun_MissingAPIKey(t *testing.T) {
    // Ensure no provider API keys in env
    for _, k := range []string{"HEXAI_OPENAI_API_KEY", "OPENAI_API_KEY", "HEXAI_COPILOT_API_KEY", "COPILOT_API_KEY"} {
        t.Setenv(k, "")
    }
    // Provide minimal stdin to get past empty input check (if reached)
    in := bytes.NewBufferString("some selection text")
    var out bytes.Buffer
    var errBuf bytes.Buffer
    // Expect an error due to missing OPENAI_API_KEY (default provider is openai)
    if err := Run(context.Background(), in, &out, &errBuf); err == nil {
        t.Fatal("expected error when API key is missing")
    }
    _ = os.Stderr
}

