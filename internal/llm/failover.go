package llm

import (
	"context"
	"fmt"
	"strings"
)

// Target identifies one configured provider profile and its client.
type Target struct {
	Name   string
	Model  string
	Client Client
}

// Result records which target produced a successful response.
type Result struct {
	Target Target
	Text   string
}

// Chat tries targets in order, moving to the next target only for an outage
// error. The request options are applied unchanged to every target.
func Chat(ctx context.Context, targets []Target, messages []Message, opts ...RequestOption) (Result, error) {
	var failures []string
	for _, target := range targets {
		if target.Client == nil {
			continue
		}
		text, err := target.Client.Chat(ctx, messages, opts...)
		if err == nil {
			return Result{Target: target, Text: text}, nil
		}
		if !ShouldFailover(err) || isContextCancel(err) {
			return Result{}, err
		}
		failures = append(failures, targetFailure(target, err))
	}
	return Result{}, combineFailures(failures)
}

// Stream tries targets in order. A target that emits any non-empty chunk owns
// the request; a later error is returned without appending another response.
func Stream(ctx context.Context, targets []Target, messages []Message, onDelta func(string), opts ...RequestOption) (Target, error) {
	var failures []string
	for _, target := range targets {
		if target.Client == nil {
			continue
		}
		emitted := false
		wrapped := func(delta string) {
			if strings.TrimSpace(delta) != "" {
				emitted = true
			}
			onDelta(delta)
		}
		var err error
		if streamer, ok := target.Client.(Streamer); ok {
			err = streamer.ChatStream(ctx, messages, wrapped, opts...)
		} else {
			var text string
			text, err = target.Client.Chat(ctx, messages, opts...)
			if err == nil {
				wrapped(text)
			}
		}
		if err == nil {
			return target, nil
		}
		if emitted || !ShouldFailover(err) || isContextCancel(err) {
			return target, err
		}
		failures = append(failures, targetFailure(target, err))
	}
	return Target{}, combineFailures(failures)
}

// CodeCompletion tries provider-native completion implementations in order.
// Targets without native support are skipped so callers can choose a generic
// chat completion route explicitly.
func CodeCompletion(ctx context.Context, targets []Target, prompt, suffix string, n int, language string, temperature float64) ([]string, Target, error) {
	var failures []string
	for _, target := range targets {
		completer, ok := target.Client.(CodeCompleter)
		if !ok {
			continue
		}
		suggestions, err := completer.CodeCompletion(ctx, prompt, suffix, n, language, temperature)
		if err == nil {
			return suggestions, target, nil
		}
		if !ShouldFailover(err) || isContextCancel(err) {
			return nil, target, err
		}
		failures = append(failures, targetFailure(target, err))
	}
	return nil, Target{}, combineFailures(failures)
}

func targetFailure(target Target, err error) string {
	name := strings.TrimSpace(target.Name)
	if name == "" {
		name = target.Client.Name()
	}
	return fmt.Sprintf("%s:%s: %v", name, strings.TrimSpace(target.Model), err)
}

func combineFailures(failures []string) error {
	if len(failures) == 0 {
		return fmt.Errorf("llm: no configured provider target available")
	}
	return fmt.Errorf("llm: all provider targets failed: %s", strings.Join(failures, "; "))
}
