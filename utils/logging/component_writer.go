package logging

import (
	"io"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// componentLevelWriter is a zerolog.LevelWriter that filters log events below a dynamically
// controlled level. The level is read atomically on every write, enabling lock-free updates
// from admin commands without any coordination with writers.
//
// All exported methods are safe for concurrent access.
type componentLevelWriter struct {
	level    *atomic.Int32
	delegate zerolog.LevelWriter
}

// NewComponentLevelWriter returns a zerolog.LevelWriter that forwards events to delegate only
// when the event level is at or above the value stored in level.
func NewComponentLevelWriter(level *atomic.Int32, delegate zerolog.LevelWriter) zerolog.LevelWriter {
	return &componentLevelWriter{level: level, delegate: delegate}
}

// Write forwards p to the delegate unconditionally. This path is only taken by callers that
// bypass zerolog's level-aware dispatch; in normal zerolog usage WriteLevel is called instead.
//
// No error returns are expected during normal operation.
func (w *componentLevelWriter) Write(p []byte) (int, error) {
	return w.delegate.Write(p)
}

// WriteLevel forwards p to the delegate if l is at or above the configured level. If the
// event is filtered, WriteLevel returns (len(p), nil) without writing — matching zerolog's
// discard convention.
//
// No error returns are expected during normal operation.
func (w *componentLevelWriter) WriteLevel(l zerolog.Level, p []byte) (int, error) {
	if l < zerolog.Level(w.level.Load()) {
		return len(p), nil
	}
	return w.delegate.WriteLevel(l, p)
}

// NoopLevelWriter wraps an io.Writer as a zerolog.LevelWriter, ignoring the level argument.
// Intended for tests.
func NoopLevelWriter(w io.Writer) zerolog.LevelWriter {
	return &noopLevelWriter{w}
}

type noopLevelWriter struct{ io.Writer }

func (n *noopLevelWriter) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	return n.Write(p)
}
