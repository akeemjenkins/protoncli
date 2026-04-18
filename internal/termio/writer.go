package termio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Writer routes structured and human output to configured sinks with optional color on Err.
type Writer struct {
	Out   io.Writer
	Err   io.Writer
	Color bool
}

// NewDefault returns a Writer wired to os.Stdout/os.Stderr with color auto-detected.
func NewDefault() *Writer {
	return &Writer{
		Out:   os.Stdout,
		Err:   os.Stderr,
		Color: StderrSupportsColor(),
	}
}

// PrintJSON writes v as indented JSON followed by a newline to Out.
func (w *Writer) PrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Out.Write(b); err != nil {
		return err
	}
	_, err = w.Out.Write([]byte{'\n'})
	return err
}

// PrintNDJSON writes v as a compact JSON line followed by a newline to Out.
func (w *Writer) PrintNDJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Out.Write(b); err != nil {
		return err
	}
	_, err = w.Out.Write([]byte{'\n'})
	return err
}

// Status writes a sanitized status message to Err.
func (w *Writer) Status(msg string) {
	fmt.Fprintln(w.Err, SanitizeForTerminal(msg))
}

// Warn writes a sanitized warning message (yellow label when color) to Err.
func (w *Writer) Warn(msg string) {
	label := "warning:"
	if w.Color {
		label = "\x1b[1;33m" + label + "\x1b[0m"
	}
	fmt.Fprintf(w.Err, "%s %s\n", label, SanitizeForTerminal(msg))
}

// Errorf writes a sanitized, formatted error message (red label when color) to Err.
func (w *Writer) Errorf(format string, args ...any) {
	label := "error:"
	if w.Color {
		label = "\x1b[1;31m" + label + "\x1b[0m"
	}
	msg := SanitizeForTerminal(fmt.Sprintf(format, args...))
	fmt.Fprintf(w.Err, "%s %s\n", label, msg)
}

// Infof writes a sanitized, formatted info message to Err.
func (w *Writer) Infof(format string, args ...any) {
	fmt.Fprintln(w.Err, SanitizeForTerminal(fmt.Sprintf(format, args...)))
}
