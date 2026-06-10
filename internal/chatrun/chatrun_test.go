package chatrun

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/llm"
)

// simpleClient implements only Chatter (no streaming).
type simpleClient struct {
	resp string
	err  error
}

func (c simpleClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return c.resp, c.err
}

// streamClient implements Chatter and llm.Streamer.
type streamClient struct {
	chunks []string
	err    error
}

func (c streamClient) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return strings.Join(c.chunks, ""), nil
}

func (c streamClient) ChatStream(_ context.Context, _ []llm.Message, onDelta func(string), _ ...llm.RequestOption) error {
	if c.err != nil {
		return c.err
	}
	for _, chunk := range c.chunks {
		onDelta(chunk)
	}
	return nil
}

// errWriter always fails, to exercise the write-error paths.
type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestInvoke_SimpleCollectsAndWrites(t *testing.T) {
	var out bytes.Buffer
	got, err := Invoke(context.Background(), simpleClient{resp: "hello"}, nil, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" || out.String() != "hello" {
		t.Fatalf("got=%q out=%q, want hello/hello", got, out.String())
	}
}

func TestInvoke_SimpleNilWriter(t *testing.T) {
	got, err := Invoke(context.Background(), simpleClient{resp: "x"}, nil, nil, nil)
	if err != nil || got != "x" {
		t.Fatalf("got=%q err=%v, want x/nil", got, err)
	}
}

func TestInvoke_SimpleChatError(t *testing.T) {
	_, err := Invoke(context.Background(), simpleClient{err: errors.New("boom")}, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected chat error, got %v", err)
	}
}

func TestInvoke_SimpleWriteError(t *testing.T) {
	_, err := Invoke(context.Background(), simpleClient{resp: "x"}, nil, nil, errWriter{err: errors.New("wfail")})
	if err == nil || !strings.Contains(err.Error(), "wfail") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestInvoke_StreamingCollectsAndWrites(t *testing.T) {
	var out bytes.Buffer
	got, err := Invoke(context.Background(), streamClient{chunks: []string{"a", "b", "c"}}, nil, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc" || out.String() != "abc" {
		t.Fatalf("got=%q out=%q, want abc/abc", got, out.String())
	}
}

func TestInvoke_StreamingNilWriter(t *testing.T) {
	got, err := Invoke(context.Background(), streamClient{chunks: []string{"a", "b"}}, nil, nil, nil)
	if err != nil || got != "ab" {
		t.Fatalf("got=%q err=%v, want ab/nil", got, err)
	}
}

func TestInvoke_StreamingError(t *testing.T) {
	_, err := Invoke(context.Background(), streamClient{err: errors.New("sfail")}, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "sfail") {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestInvoke_StreamingWriteError(t *testing.T) {
	// Full text must still be collected even though the writer fails.
	_, err := Invoke(context.Background(), streamClient{chunks: []string{"a", "b"}}, nil, nil, errWriter{err: errors.New("wfail")})
	if err == nil || !strings.Contains(err.Error(), "wfail") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestSentBytes(t *testing.T) {
	msgs := []llm.Message{
		{Role: "system", Content: "abc"},
		{Role: "user", Content: "de"},
	}
	if got := SentBytes(msgs); got != 5 {
		t.Fatalf("SentBytes = %d, want 5", got)
	}
	if got := SentBytes(nil); got != 0 {
		t.Fatalf("SentBytes(nil) = %d, want 0", got)
	}
}

func TestAccount(t *testing.T) {
	msgs := []llm.Message{{Role: "user", Content: "1234"}}
	sent, recv := Account(context.Background(), "prov", "model", msgs, "response")
	if sent != 4 {
		t.Fatalf("sent = %d, want 4", sent)
	}
	if recv != len("response") {
		t.Fatalf("recv = %d, want %d", recv, len("response"))
	}
}
