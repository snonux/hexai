package lsp

import (
    "encoding/json"
    "fmt"
    "hexai/internal/logging"
    "io"
    "net/textproto"
    "strconv"
    "strings"
)

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

func (s *Server) writeMessage(v any) {
	data, err := json.Marshal(v)
    if err != nil {
        logging.Logf("lsp ", "marshal error: %v", err)
        return
    }
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
    if _, err := io.WriteString(s.out, header); err != nil {
        logging.Logf("lsp ", "write header error: %v", err)
        return
    }
    if _, err := s.out.Write(data); err != nil {
        logging.Logf("lsp ", "write body error: %v", err)
        return
    }
}
