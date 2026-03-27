package termprint

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// ColumnPrinter streams provider output in side-by-side columns.
type ColumnPrinter struct {
	mu        sync.Mutex
	stdout    io.Writer
	columns   int
	colWidth  int
	partial   []string
	providers []string
	models    []string
}

type columnWriter struct {
	printer *ColumnPrinter
	index   int
}

// NewColumnPrinter builds a multi-column printer for the provider/model pairs.
func NewColumnPrinter(stdout io.Writer, providers []string, models []string) *ColumnPrinter {
	cols := len(providers)
	if len(models) > cols {
		cols = len(models)
	}
	if cols == 0 {
		return nil
	}

	width := DetectTerminalWidth(stdout)
	if width <= 0 {
		width = 100
	}
	sepWidth := (cols - 1) * 3
	colWidth := (width - sepWidth) / cols
	if colWidth < 20 {
		colWidth = 20
	}

	providerCols := make([]string, cols)
	copy(providerCols, providers)
	modelCols := make([]string, cols)
	copy(modelCols, models)

	return &ColumnPrinter{
		stdout:    stdout,
		columns:   cols,
		colWidth:  colWidth,
		partial:   make([]string, cols),
		providers: providerCols,
		models:    modelCols,
	}
}

func DetectTerminalWidth(w io.Writer) int {
	type fder interface{ Fd() uintptr }
	if f, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil {
			return width
		}
	}
	if f, ok := w.(fder); ok {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil {
			return width
		}
	}
	return 0
}

func detectTerminalWidth(w io.Writer) int {
	return DetectTerminalWidth(w)
}

// Writer returns an io.Writer that routes chunks to a single column index.
func (cp *ColumnPrinter) Writer(idx int) io.Writer {
	return columnWriter{printer: cp, index: idx}
}

// PrintHeader writes provider/model headers and a divider row.
func (cp *ColumnPrinter) PrintHeader() {
	cp.PrintHeaderTo(cp.stdout)
}

// PrintHeaderTo writes provider/model headers and a divider row to w.
func (cp *ColumnPrinter) PrintHeaderTo(w io.Writer) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.writeLineTo(w, cp.headerCells())
	cp.writeLineTo(w, cp.dividerCells())
}

// Flush emits any buffered partial line for a column.
func (cp *ColumnPrinter) Flush(idx int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if idx < 0 || idx >= len(cp.partial) {
		return
	}
	if cp.partial[idx] == "" {
		return
	}
	cp.emitJobLine(idx, cp.partial[idx])
	cp.partial[idx] = ""
}

func (w columnWriter) Write(p []byte) (int, error) {
	return w.printer.write(w.index, string(p))
}

func (cp *ColumnPrinter) write(idx int, data string) (int, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if idx < 0 || idx >= len(cp.partial) {
		return len(data), nil
	}
	data = strings.ReplaceAll(data, "\r", "")
	cp.partial[idx] += data
	for strings.Contains(cp.partial[idx], "\n") {
		line, rest, _ := strings.Cut(cp.partial[idx], "\n")
		cp.partial[idx] = rest
		cp.emitJobLine(idx, line)
	}
	return len(data), nil
}

func (cp *ColumnPrinter) emitJobLine(idx int, line string) {
	segments := cp.wrap(line)
	for _, seg := range segments {
		cells := make([]string, cp.columns)
		if idx >= 0 && idx < len(cells) {
			cells[idx] = seg
		}
		cp.writeLine(cells)
	}
}

func (cp *ColumnPrinter) wrap(text string) []string {
	text = strings.ReplaceAll(text, "\t", "    ")
	if runewidth.StringWidth(text) <= cp.colWidth {
		return []string{text}
	}
	var lines []string
	var current strings.Builder
	width := 0
	for _, r := range text {
		rw := runewidth.RuneWidth(r)
		if width+rw > cp.colWidth && current.Len() > 0 {
			lines = append(lines, current.String())
			current.Reset()
			width = 0
		}
		current.WriteRune(r)
		width += rw
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

func (cp *ColumnPrinter) writeLine(cells []string) {
	cp.writeLineTo(cp.stdout, cells)
}

func (cp *ColumnPrinter) headerCells() []string {
	cells := make([]string, cp.columns)
	for i := 0; i < cp.columns; i++ {
		provider := strings.TrimSpace(cp.providers[i])
		model := strings.TrimSpace(cp.models[i])
		switch {
		case provider != "" && model != "":
			cells[i] = provider + ":" + model
		case provider != "":
			cells[i] = provider
		case model != "":
			cells[i] = model
		default:
			cells[i] = ""
		}
	}
	return cells
}

func (cp *ColumnPrinter) dividerCells() []string {
	cells := make([]string, cp.columns)
	line := strings.Repeat("─", cp.colWidth)
	for i := range cells {
		cells[i] = line
	}
	return cells
}

func (cp *ColumnPrinter) writeLineTo(w io.Writer, cells []string) {
	if len(cells) < cp.columns {
		extra := make([]string, cp.columns-len(cells))
		cells = append(cells, extra...)
	}
	var builder strings.Builder
	for i := 0; i < cp.columns; i++ {
		cell := cells[i]
		width := runewidth.StringWidth(cell)
		if width > cp.colWidth {
			cell = runewidth.Truncate(cell, cp.colWidth, "…")
			width = runewidth.StringWidth(cell)
		}
		builder.WriteString(cell)
		if pad := cp.colWidth - width; pad > 0 {
			builder.WriteString(strings.Repeat(" ", pad))
		}
		if i != cp.columns-1 {
			builder.WriteString(" │ ")
		}
	}
	builder.WriteByte('\n')
	_, _ = w.Write([]byte(builder.String()))
}
