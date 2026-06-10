// Package chatrun holds the shared chat-running primitives that used to be
// duplicated across the three Hexai surfaces: the CLI (internal/hexaicli), the
// LSP server (internal/lsp) and the tmux code-action tool (internal/hexaiaction).
//
// Every surface had to do the same two things when talking to an LLM:
//
//  1. Invoke the client, preferring streaming (llm.Streamer) when the provider
//     implements it and falling back to a single Chat call otherwise, while
//     collecting the full response text.
//  2. Account for the exchange: count the bytes sent (sum of message contents)
//     and received (response length) and feed them into the stats package.
//
// Both pieces lived in slightly diverging copies (runChatRequest/runStreaming
// Chat/runSimpleChat in the CLI, chatWithStats in the LSP, runOnce in the
// action tool). This package centralises them so all three share one
// implementation and behaviour stays identical.
package chatrun

import (
	"context"
	"fmt"
	"io"
	"strings"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/stats"
)

// Chatter is the minimal client capability Invoke needs: a single Chat call.
// It is deliberately narrower than llm.Client (no Name/DefaultModel) so callers
// that only have a chat-doer abstraction — such as the code-action surface —
// can use Invoke without widening their own interface. Clients that also
// implement llm.Streamer get streaming automatically via a type assertion.
type Chatter interface {
	Chat(ctx context.Context, messages []llm.Message, opts ...llm.RequestOption) (string, error)
}

// Invoke sends msgs to the client and returns the full assistant response.
//
// When the client implements llm.Streamer the response is streamed and, if out
// is non-nil, each chunk is forwarded to out as it arrives (this is how the CLI
// renders incremental output). Otherwise a single Chat call is made and, when
// out is non-nil, the whole response is written to it once.
//
// Passing a nil out collects the response without writing it anywhere, which is
// what the LSP and code-action surfaces need (they post-process the text before
// applying it to a document).
func Invoke(ctx context.Context, client Chatter, msgs []llm.Message, opts []llm.RequestOption, out io.Writer) (string, error) {
	if streamer, ok := client.(llm.Streamer); ok {
		return invokeStreaming(ctx, streamer, msgs, opts, out)
	}
	return invokeSimple(ctx, client, msgs, opts, out)
}

// invokeStreaming drives ChatStream, accumulating the full text while
// optionally mirroring each chunk to out. A write error to out aborts further
// writes and is returned once streaming finishes.
func invokeStreaming(ctx context.Context, client llm.Streamer, msgs []llm.Message, opts []llm.RequestOption, out io.Writer) (string, error) {
	var output strings.Builder
	var writeErr error
	err := client.ChatStream(ctx, msgs, func(chunk string) {
		output.WriteString(chunk)
		if out == nil || writeErr != nil {
			return
		}
		if _, werr := io.WriteString(out, chunk); werr != nil {
			writeErr = werr
		}
	}, opts...)
	if err != nil {
		return "", err
	}
	if writeErr != nil {
		return "", writeErr
	}
	return output.String(), nil
}

// invokeSimple performs a single Chat call and, when out is non-nil, writes the
// whole response to it.
func invokeSimple(ctx context.Context, client Chatter, msgs []llm.Message, opts []llm.RequestOption, out io.Writer) (string, error) {
	output, err := client.Chat(ctx, msgs, opts...)
	if err != nil {
		return "", err
	}
	if out != nil {
		if _, werr := fmt.Fprint(out, output); werr != nil {
			return "", werr
		}
	}
	return output, nil
}

// SentBytes returns the total number of content bytes across msgs. This is the
// "sent" figure every surface reports and feeds into the stats package.
func SentBytes(msgs []llm.Message) int {
	sent := 0
	for _, m := range msgs {
		sent += len(m.Content)
	}
	return sent
}

// Account records a completed exchange in the stats package and returns the
// sent/received byte counts so callers can build their own summaries. The
// stats.Update error is intentionally swallowed because none of the surfaces
// treat a stats failure as fatal; callers that want to log it can call
// stats.Update directly instead.
func Account(ctx context.Context, provider, model string, msgs []llm.Message, output string) (sent, recv int) {
	sent = SentBytes(msgs)
	recv = len(output)
	_ = stats.Update(ctx, provider, model, sent, recv)
	return sent, recv
}
