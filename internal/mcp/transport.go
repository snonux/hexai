// MCP transport utilities for reading and writing JSON-RPC messages
// using newline-delimited JSON (JSONL) as required by the MCP stdio protocol.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// readMessage reads a newline-delimited JSON-RPC message from the input stream.
// Returns the raw JSON bytes. Follows the MCP stdio transport specification
// which uses newline-delimited JSON (JSONL), not LSP-style Content-Length framing.
// Uses the server's bufio.Reader (s.in) to avoid losing buffered data between calls.
func (s *Server) readMessage() ([]byte, error) {
	for {
		line, err := s.in.ReadString('\n')
		if err != nil && len(line) == 0 {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("read error: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			// If we hit EOF on an empty line, propagate it
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			continue // skip empty lines
		}
		return []byte(line), nil
	}
}

// writeMessage writes a JSON-RPC response as newline-delimited JSON.
// Thread-safe via mutex lock.
func (s *Server) writeMessage(v any) error {
	s.outMu.Lock()
	defer s.outMu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	// Write JSON followed by newline (JSONL format)
	if _, err := s.out.Write(data); err != nil {
		return fmt.Errorf("write body error: %w", err)
	}
	if _, err := io.WriteString(s.out, "\n"); err != nil {
		return fmt.Errorf("write newline error: %w", err)
	}
	return nil
}
