package termio

import (
	"fmt"
	"sync"
)

var (
	defaultOnce   sync.Once
	defaultWriter *Writer
)

// Default returns the lazily-initialized package-level Writer.
func Default() *Writer {
	defaultOnce.Do(func() {
		defaultWriter = NewDefault()
	})
	return defaultWriter
}

// SetDefault replaces the package-level Writer (intended for tests).
func SetDefault(w *Writer) {
	defaultOnce.Do(func() {})
	defaultWriter = w
}

// Info logs a sanitized info message via the default Writer.
func Info(format string, args ...any) {
	Default().Infof(format, args...)
}

// Warn logs a sanitized warning via the default Writer.
func Warn(format string, args ...any) {
	Default().Warn(fmt.Sprintf(format, args...))
}

// Error logs a sanitized error via the default Writer.
func Error(format string, args ...any) {
	Default().Errorf(format, args...)
}
