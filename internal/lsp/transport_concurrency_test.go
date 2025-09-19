package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type raceBuffer struct {
	buf     bytes.Buffer
	bufMu   sync.Mutex
	inWrite atomic.Int32
	raced   atomic.Bool
}

func (b *raceBuffer) Write(p []byte) (int, error) {
	if b.inWrite.Swap(1) != 0 {
		b.raced.Store(true)
	}
	runtime.Gosched()
	b.bufMu.Lock()
	n, err := b.buf.Write(p)
	b.bufMu.Unlock()
	b.inWrite.Store(0)
	return n, err
}

func (b *raceBuffer) Bytes() []byte {
	b.bufMu.Lock()
	defer b.bufMu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *raceBuffer) Raced() bool {
	return b.raced.Load()
}

func TestServerReplySerializesWrites(t *testing.T) {
	t.Parallel()

	writer := &raceBuffer{}
	srv := NewServer(bytes.NewReader([]byte{}), writer, log.New(io.Discard, "", 0), ServerOptions{})

	const goroutines = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			id := json.RawMessage(strconv.Itoa(i + 1))
			srv.reply(id, map[string]int{"index": i}, nil)
		}(i)
	}
	close(start)
	wg.Wait()

	if writer.Raced() {
		t.Fatalf("detected overlapping writes to server output")
	}

	data := writer.Bytes()
	for len(data) > 0 {
		headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
		if headerEnd < 0 {
			t.Fatalf("missing header delimiter in %q", string(data))
		}
		header := string(data[:headerEnd])
		if !strings.HasPrefix(header, "Content-Length: ") {
			t.Fatalf("unexpected header %q", header)
		}
		lengthStr := strings.TrimSpace(header[len("Content-Length: "):])
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			t.Fatalf("invalid content length %q: %v", lengthStr, err)
		}
		payloadStart := headerEnd + 4
		payloadEnd := payloadStart + length
		if payloadEnd > len(data) {
			t.Fatalf("payload truncated: need %d bytes, have %d", length, len(data)-payloadStart)
		}
		data = data[payloadEnd:]
	}
}
