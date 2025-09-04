package lsp

import (
    "bufio"
    "bytes"
    "testing"
)

func TestReadMessage_ParsesContentLength(t *testing.T) {
    body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
    frame := []byte("Content-Length: ")
    frame = append(frame, []byte(stringInt(len(body)))...)
    frame = append(frame, []byte("\r\n\r\n")...)
    frame = append(frame, body...)
    s := &Server{in: bufio.NewReader(bytes.NewReader(frame))}
    got, err := s.readMessage()
    if err != nil || string(got) != string(body) { t.Fatalf("readMessage failed: %v %q", err, string(got)) }
}

func stringInt(n int) string {
    if n == 0 { return "0" }
    var b [20]byte
    i := len(b)
    for n > 0 { i--; b[i] = byte('0' + n%10); n /= 10 }
    return string(b[i:])
}

