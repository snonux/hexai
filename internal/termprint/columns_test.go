package termprint

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewColumnPrinter(t *testing.T) {
	t.Run("returns nil when no columns are configured", func(t *testing.T) {
		if got := NewColumnPrinter(&bytes.Buffer{}, nil, nil); got != nil {
			t.Fatalf("NewColumnPrinter() = %#v, want nil", got)
		}
	})

	t.Run("uses longest input slice and fallback width", func(t *testing.T) {
		cp := NewColumnPrinter(&bytes.Buffer{}, []string{"openai"}, []string{"gpt-5", "haiku"})
		if cp == nil {
			t.Fatal("NewColumnPrinter() returned nil")
		}
		if cp.columns != 2 {
			t.Fatalf("columns = %d, want 2", cp.columns)
		}
		if cp.colWidth != 48 {
			t.Fatalf("colWidth = %d, want 48", cp.colWidth)
		}
		if got, want := cp.providers, []string{"openai", ""}; !equalStrings(got, want) {
			t.Fatalf("providers = %#v, want %#v", got, want)
		}
		if got, want := cp.models, []string{"gpt-5", "haiku"}; !equalStrings(got, want) {
			t.Fatalf("models = %#v, want %#v", got, want)
		}
	})
}

func TestDetectTerminalWidth(t *testing.T) {
	if got := detectTerminalWidth(&bytes.Buffer{}); got != 0 {
		t.Fatalf("detectTerminalWidth() = %d, want 0", got)
	}
}

func TestHeaderCells(t *testing.T) {
	cp := &ColumnPrinter{
		columns:   4,
		providers: []string{" openai ", "", "anthropic", ""},
		models:    []string{" gpt-5 ", "haiku", "", ""},
	}

	got := cp.headerCells()
	want := []string{"openai:gpt-5", "haiku", "anthropic", ""}
	if !equalStrings(got, want) {
		t.Fatalf("headerCells() = %#v, want %#v", got, want)
	}
}

func TestDividerCells(t *testing.T) {
	cp := &ColumnPrinter{columns: 2, colWidth: 4}

	got := cp.dividerCells()
	want := []string{"────", "────"}
	if !equalStrings(got, want) {
		t.Fatalf("dividerCells() = %#v, want %#v", got, want)
	}
}

func TestPrintHeader(t *testing.T) {
	buf := &bytes.Buffer{}
	cp := &ColumnPrinter{
		stdout:    buf,
		columns:   2,
		colWidth:  6,
		providers: []string{"openai", "anth"},
		models:    []string{"gpt-5", ""},
	}

	cp.PrintHeader()

	got := buf.String()
	if !strings.Contains(got, "opena…") {
		t.Fatalf("PrintHeader() output = %q, want truncated provider/model header", got)
	}
	if !strings.Contains(got, "anth") {
		t.Fatalf("PrintHeader() output = %q, want second header cell", got)
	}
	if !strings.Contains(got, "──────") {
		t.Fatalf("PrintHeader() output = %q, want divider row", got)
	}
}

func TestWrap(t *testing.T) {
	cp := &ColumnPrinter{colWidth: 4}

	t.Run("returns single segment when text fits", func(t *testing.T) {
		got := cp.wrap("go")
		want := []string{"go"}
		if !equalStrings(got, want) {
			t.Fatalf("wrap() = %#v, want %#v", got, want)
		}
	})

	t.Run("wraps long text and expands tabs", func(t *testing.T) {
		got := cp.wrap("ab\tcd")
		want := []string{"ab  ", "  cd"}
		if !equalStrings(got, want) {
			t.Fatalf("wrap() = %#v, want %#v", got, want)
		}
	})
}

func TestWriteAndFlush(t *testing.T) {
	buf := &bytes.Buffer{}
	cp := &ColumnPrinter{
		stdout:   buf,
		columns:  2,
		colWidth: 10,
		partial:  make([]string, 2),
	}

	writer := cp.Writer(1)
	if _, err := writer.Write([]byte("alpha")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := cp.partial[1]; got != "alpha" {
		t.Fatalf("partial[1] = %q, want %q", got, "alpha")
	}
	if got := buf.Len(); got != 0 {
		t.Fatalf("buffer length = %d, want 0 before newline", got)
	}

	if n, err := cp.write(1, "beta\nnext"); err != nil {
		t.Fatalf("write() error = %v", err)
	} else if n != len("beta\nnext") {
		t.Fatalf("write() bytes = %d, want %d", n, len("beta\nnext"))
	}
	if got := cp.partial[1]; got != "next" {
		t.Fatalf("partial[1] = %q, want %q", got, "next")
	}

	cp.Flush(1)

	got := buf.String()
	if !strings.Contains(got, "alphabeta") {
		t.Fatalf("buffer = %q, want first emitted line", got)
	}
	if !strings.Contains(got, "next") {
		t.Fatalf("buffer = %q, want flushed line", got)
	}
	if cp.partial[1] != "" {
		t.Fatalf("partial[1] = %q, want empty after Flush", cp.partial[1])
	}

	if n, err := cp.write(-1, "ignored"); err != nil {
		t.Fatalf("write() invalid index error = %v", err)
	} else if n != len("ignored") {
		t.Fatalf("write() invalid index bytes = %d, want %d", n, len("ignored"))
	}
}

func TestWriteLineToPadsAndTruncates(t *testing.T) {
	buf := &bytes.Buffer{}
	cp := &ColumnPrinter{columns: 1, colWidth: 4}

	cp.writeLineTo(buf, []string{"abcdef"})

	if got, want := buf.String(), "abc…\n"; got != want {
		t.Fatalf("writeLineTo() = %q, want %q", got, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
