package hexaiaction

import (
    "context"
    "strings"
    "testing"

    "codeberg.org/snonux/hexai/internal/llm"
)

type simpleDoer struct{ s string }

func (d simpleDoer) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) { return d.s, nil }

func TestRunOnce_StripsFences(t *testing.T) {
    got, err := runOnce(context.Background(), simpleDoer{"```\nok\n```"}, "SYS", "USER")
    if err != nil { t.Fatalf("runOnce: %v", err) }
    if strings.TrimSpace(got) != "ok" { t.Fatalf("got %q", got) }
}
