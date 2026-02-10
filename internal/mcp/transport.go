// Summary: MCP transport utilities for reading and writing JSON-RPC messages with Content-Length framing.
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
)

// readMessage reads a Content-Length framed JSON-RPC message from the input stream.
// Returns the raw JSON bytes. Follows LSP/JSON-RPC framing convention.
func (s *Server) readMessage() ([]byte, error) {
	tp := textproto.NewReader(s.in)
	var contentLength int
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == "" { // end of headers
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "content-length":
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("missing or invalid Content-Length")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(s.in, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// writeMessage writes a JSON-RPC response with Content-Length framing.
// Thread-safe via mutex lock.
func (s *Server) writeMessage(v any) error {
	s.outMu.Lock()
	defer s.outMu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(s.out, header); err != nil {
		return fmt.Errorf("write header error: %w", err)
	}
	if _, err := s.out.Write(data); err != nil {
		return fmt.Errorf("write body error: %w", err)
	}
	return nil
}
