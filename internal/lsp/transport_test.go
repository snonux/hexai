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

func TestWriteMessage_FramesJSON(t *testing.T) {
    var out bytes.Buffer
    s := &Server{out: &out}
    payload := struct{ JSONRPC string `json:"jsonrpc"`; Ping string `json:"ping"` }{JSONRPC: "2.0", Ping: "pong"}
    s.writeMessage(payload)
    got := out.String()
    if !bytes.HasPrefix([]byte(got), []byte("Content-Length: ")) { t.Fatalf("missing Content-Length header: %q", got) }
    // Header/body delimiter must be present
    idx := bytes.Index([]byte(got), []byte("\r\n\r\n"))
    if idx < 0 { t.Fatalf("missing CRLFCRLF delimiter: %q", got) }
    body := got[idx+4:]
    if body == "" || body[0] != '{' || body[len(body)-1] != '}' { t.Fatalf("body not JSON: %q", body) }
}

func stringInt(n int) string {
    if n == 0 { return "0" }
    var b [20]byte
    i := len(b)
    for n > 0 { i--; b[i] = byte('0' + n%10); n /= 10 }
    return string(b[i:])
}
