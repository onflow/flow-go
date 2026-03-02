package logging_test

import (
	"io"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"
)

// benchRegistry returns a LogRegistry and a base logger that discard all output,
// suitable for benchmarks where I/O cost should not affect results.
func benchRegistry(defaultLevel zerolog.Level) (*logging.LogRegistry, zerolog.Logger) {
	r := logging.NewLogRegistry(io.Discard, defaultLevel, nil)
	base := zerolog.New(io.Discard).Level(zerolog.TraceLevel)
	return r, base
}

// BenchmarkBaseline_SuppressedEvent is the plain-zerolog baseline for BenchmarkLogger_SuppressedEvent.
func BenchmarkBaseline_SuppressedEvent(b *testing.B) {
	logger := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	for b.Loop() {
		logger.Debug().Msg("suppressed")
	}
}

// BenchmarkBaseline_ActiveEvent is the plain-zerolog baseline for BenchmarkLogger_ActiveEvent.
func BenchmarkBaseline_ActiveEvent(b *testing.B) {
	logger := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	for b.Loop() {
		logger.Info().Msg("active")
	}
}

// BenchmarkBaseline_ActiveEvent_Parallel is the plain-zerolog baseline for
// BenchmarkLogger_ActiveEvent_Parallel.
func BenchmarkBaseline_ActiveEvent_Parallel(b *testing.B) {
	loggerA := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	loggerB := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				loggerA.Info().Msg("a")
			} else {
				loggerB.Info().Msg("b")
			}
			i++
		}
	})
}

// BenchmarkBaseline_ChildLogger_SuppressedEvent is the plain-zerolog baseline for
// BenchmarkLogger_ChildLogger_SuppressedEvent.
func BenchmarkBaseline_ChildLogger_SuppressedEvent(b *testing.B) {
	parent := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	child := parent.With().Str("sub", "voter").Logger()
	for b.Loop() {
		child.Debug().Msg("suppressed")
	}
}

// BenchmarkBaseline_ChildLogger_ActiveEvent is the plain-zerolog baseline for
// BenchmarkLogger_ChildLogger_ActiveEvent.
func BenchmarkBaseline_ChildLogger_ActiveEvent(b *testing.B) {
	parent := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	child := parent.With().Str("sub", "voter").Logger()
	for b.Loop() {
		child.Info().Msg("active")
	}
}

// BenchmarkLogger_SuppressedEvent measures the cost of a debug event that is suppressed
// because the component level is info. This exercises zerolog's global-level short-circuit
// and the componentLevelWriter filter together.
func BenchmarkLogger_SuppressedEvent(b *testing.B) {
	r, base := benchRegistry(zerolog.InfoLevel)
	logger := r.Logger(base, "hotstuff")
	for b.Loop() {
		logger.Debug().Msg("suppressed")
	}
}

// BenchmarkLogger_ActiveEvent measures the cost of an info event that passes through
// the componentLevelWriter and is written to the discard sink.
func BenchmarkLogger_ActiveEvent(b *testing.B) {
	r, base := benchRegistry(zerolog.InfoLevel)
	logger := r.Logger(base, "hotstuff")
	for b.Loop() {
		logger.Info().Msg("active")
	}
}

// BenchmarkLogger_ActiveEvent_Parallel measures throughput when multiple goroutines
// log concurrently through independent component loggers.
func BenchmarkLogger_ActiveEvent_Parallel(b *testing.B) {
	r, base := benchRegistry(zerolog.InfoLevel)
	loggerA := r.Logger(base, "component-a")
	loggerB := r.Logger(base, "component-b")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				loggerA.Info().Msg("a")
			} else {
				loggerB.Info().Msg("b")
			}
			i++
		}
	})
}

// BenchmarkLogger_ChildLogger_SuppressedEvent measures suppression cost for a child
// logger derived via With(), which shares the parent's componentLevelWriter.
func BenchmarkLogger_ChildLogger_SuppressedEvent(b *testing.B) {
	r, base := benchRegistry(zerolog.InfoLevel)
	parent := r.Logger(base, "hotstuff")
	child := parent.With().Str("sub", "voter").Logger()
	for b.Loop() {
		child.Debug().Msg("suppressed")
	}
}

// BenchmarkLogger_ChildLogger_ActiveEvent measures the cost of an active event from a
// child logger derived via With().
func BenchmarkLogger_ChildLogger_ActiveEvent(b *testing.B) {
	r, base := benchRegistry(zerolog.InfoLevel)
	parent := r.Logger(base, "hotstuff")
	child := parent.With().Str("sub", "voter").Logger()
	for b.Loop() {
		child.Info().Msg("active")
	}
}
