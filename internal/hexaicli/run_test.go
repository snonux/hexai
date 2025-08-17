// Summary: Unit tests for Hexai CLI helpers and run flow (input parsing, messages, streaming).
package hexaicli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestReadInput_ArgsOnly(t *testing.T) {
	restore, f := setStdin(t, "")
	defer restore()
	// Pass the same file reader used for os.Stdin (empty)
	got, err := readInput(f, []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "hello world"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestReadInput_StdinOnly(t *testing.T) {
	restore, f := setStdin(t, "payload")
	defer restore()
	got, err := readInput(f, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "payload" {
		t.Fatalf("want %q, got %q", "payload", got)
	}
}

func TestReadInput_Combined(t *testing.T) {
	restore, f := setStdin(t, "payload")
	defer restore()
	got, err := readInput(f, []string{"subject"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "subject:\n\npayload"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestReadInput_EmptyError(t *testing.T) {
	restore, f := setStdin(t, "")
	defer restore()
	_, err := readInput(f, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no input") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildMessages_DefaultAndExplain(t *testing.T) {
	// Default concise
	msgs := buildMessages("list files in folder")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("unexpected roles: %+v", msgs)
	}
	if !strings.Contains(msgs[0].Content, "very short, concise answers") {
		t.Fatalf("unexpected system message: %q", msgs[0].Content)
	}
	if msgs[1].Content != "list files in folder" {
		t.Fatalf("unexpected user content: %q", msgs[1].Content)
	}

	// Verbose explain
	msgs2 := buildMessages("please explain how this works")
	if len(msgs2) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs2))
	}
	if !strings.Contains(strings.ToLower(msgs2[0].Content), "requested an explanation") {
		t.Fatalf("unexpected system message: %q", msgs2[0].Content)
	}
	if msgs2[1].Content != "please explain how this works" {
		t.Fatalf("unexpected user content: %q", msgs2[1].Content)
	}
}

func TestRunChat_NonStreaming(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	fc := fakeClient{name: "fake", model: "m", resp: "OUTPUT"}
	if err := runChat(context.Background(), &fc, nil, "input", &out, &errb); err != nil {
		t.Fatalf("runChat error: %v", err)
	}
	if out.String() != "OUTPUT" {
		t.Fatalf("stdout want %q, got %q", "OUTPUT", out.String())
	}
	es := errb.String()
	if !strings.Contains(es, "done provider=fake model=m") {
		t.Fatalf("stderr missing provider/model: %q", es)
	}
	if !strings.Contains(es, "in_bytes=5") || !strings.Contains(es, "out_bytes=6") {
		t.Fatalf("stderr missing byte counts: %q", es)
	}
}

func TestRunChat_Streaming(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	fs := fakeStreamer{fakeClient: fakeClient{name: "fake", model: "m"}, chunks: []string{"OUT", "PUT"}}
	if err := runChat(context.Background(), &fs, nil, "input", &out, &errb); err != nil {
		t.Fatalf("runChat error: %v", err)
	}
	if out.String() != "OUTPUT" {
		t.Fatalf("stdout want %q, got %q", "OUTPUT", out.String())
	}
	es := errb.String()
	if !strings.Contains(es, "done provider=fake model=m") {
		t.Fatalf("stderr missing provider/model: %q", es)
	}
	if !strings.Contains(es, "in_bytes=5") || !strings.Contains(es, "out_bytes=6") {
		t.Fatalf("stderr missing byte counts: %q", es)
	}
}

func TestPrintProviderInfo(t *testing.T) {
	var b bytes.Buffer
	fc := fakeClient{name: "fake", model: "m"}
	printProviderInfo(&b, &fc)
	s := b.String()
	if !strings.Contains(s, "provider=fake model=m") {
		t.Fatalf("unexpected banner: %q", s)
	}
}

func TestRunWithClient_NonStreaming(t *testing.T) {
	restore, f := setStdin(t, "")
	defer restore()
	var out bytes.Buffer
	var errb bytes.Buffer
	fc := fakeClient{name: "fake", model: "m", resp: "OK"}
	if err := RunWithClient(context.Background(), []string{"ask"}, f, &out, &errb, &fc); err != nil {
		t.Fatalf("RunWithClient error: %v", err)
	}
	if out.String() != "OK" {
		t.Fatalf("stdout want %q, got %q", "OK", out.String())
	}
	if !strings.Contains(errb.String(), "provider=fake model=m") {
		t.Fatalf("missing banner: %q", errb.String())
	}
}

func TestRunWithClient_Streaming(t *testing.T) {
	restore, f := setStdin(t, "")
	defer restore()
	var out bytes.Buffer
	var errb bytes.Buffer
	fs := fakeStreamer{fakeClient: fakeClient{name: "fake", model: "m"}, chunks: []string{"A", "B"}}
	if err := RunWithClient(context.Background(), []string{"ask"}, f, &out, &errb, &fs); err != nil {
		t.Fatalf("RunWithClient error: %v", err)
	}
	if out.String() != "AB" {
		t.Fatalf("stdout want %q, got %q", "AB", out.String())
	}
	if !strings.Contains(errb.String(), "provider=fake model=m") {
		t.Fatalf("missing banner: %q", errb.String())
	}
}

func TestRunWithClient_CombinedInput_UsesCombinedMessage(t *testing.T) {
	restore, f := setStdin(t, "payload")
	defer restore()
	var out bytes.Buffer
	var errb bytes.Buffer
	fc := fakeClient{name: "fake", model: "m", resp: "OK"}
	if err := RunWithClient(context.Background(), []string{"subject"}, f, &out, &errb, &fc); err != nil {
		t.Fatalf("RunWithClient error: %v", err)
	}
	if out.String() != "OK" {
		t.Fatalf("stdout want %q, got %q", "OK", out.String())
	}
	if len(fc.gotMsgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(fc.gotMsgs))
	}
	if fc.gotMsgs[1].Content != "subject:\n\npayload" {
		t.Fatalf("unexpected user message: %q", fc.gotMsgs[1].Content)
	}
}
