// Summary: Test helpers for Hexai CLI tests (stdin swapping and fake LLM clients/streamers).
// Not yet reviewed by a human
package hexaicli

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "hexai/internal/llm"
)

// setStdin sets os.Stdin from a string and returns a restore func and reader.
func setStdin(t *testing.T, content string) (func(), *os.File) {
    t.Helper()
    tmpDir := t.TempDir()
    fpath := filepath.Join(tmpDir, "stdin.txt")
    if err := os.WriteFile(fpath, []byte(content), 0o600); err != nil {
        t.Fatalf("write temp stdin: %v", err)
    }
    f, err := os.Open(fpath)
    if err != nil {
        t.Fatalf("open temp stdin: %v", err)
    }
    old := os.Stdin
    os.Stdin = f
    restore := func() {
        f.Close()
        os.Stdin = old
    }
    return restore, f
}

// fakeClient implements llm.Client for tests.
type fakeClient struct {
    name    string
    model   string
    resp    string
    gotMsgs []llm.Message
}

func (f *fakeClient) Chat(ctx context.Context, messages []llm.Message, opts ...llm.RequestOption) (string, error) {
    f.gotMsgs = append([]llm.Message{}, messages...)
    return f.resp, nil
}
func (f fakeClient) Name() string        { return f.name }
func (f fakeClient) DefaultModel() string { return f.model }

// fakeStreamer implements llm.Streamer over fakeClient.
type fakeStreamer struct {
    fakeClient
    chunks []string
    sMsgs  []llm.Message
}

func (s *fakeStreamer) ChatStream(ctx context.Context, messages []llm.Message, onDelta func(string), opts ...llm.RequestOption) error {
    s.sMsgs = append([]llm.Message{}, messages...)
    for _, c := range s.chunks {
        onDelta(c)
    }
    return nil
}
