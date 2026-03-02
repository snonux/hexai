package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"testing"
	"time"
)

func TestHandleShutdown_Replies(t *testing.T) {
	var out bytes.Buffer
	s := &Server{logger: log.New(io.Discard, "", 0), docs: make(map[string]*document), out: &out}
	initServerDefaults(s)
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("12"), Method: "shutdown"}
	out.Reset()
	s.handleShutdown(req)
	resp := captureResponse(t, &out)
	if string(resp.ID) != "12" || resp.Error != nil {
		t.Fatalf("unexpected shutdown response: %+v", resp)
	}
}

func TestHandleShutdown_CancelsServerContext(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{})
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("12"), Method: "shutdown"}
	s.handleShutdown(req)

	ctx, cancel := s.requestTimeoutContext(2 * time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("expected canceled context, got %v", ctx.Err())
		}
	default:
		t.Fatalf("expected canceled context after shutdown")
	}
}

func TestHandleExit_CancelsServerContext(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(bytes.NewReader(nil), &out, log.New(io.Discard, "", 0), ServerOptions{})
	s.handleExit()
	if !s.exited.Load() {
		t.Fatalf("expected exited flag to be set")
	}

	ctx, cancel := s.requestTimeoutContext(2 * time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("expected canceled context, got %v", ctx.Err())
		}
	default:
		t.Fatalf("expected canceled context after exit")
	}
}
