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

// BenchmarkSuppressedEvent compares a debug event suppressed at info level between
// a plain zerolog logger and a registry-backed logger.
func BenchmarkSuppressedEvent(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		logger := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
		suppressEvent(b, logger)
	})
	b.Run("registry", func(b *testing.B) {
		r, base := benchRegistry(zerolog.InfoLevel)
		logger := r.Logger(base, "hotstuff")
		suppressEvent(b, logger)
	})
}

// BenchmarkActiveEvent compares an info event that passes through to the discard sink
// between a plain zerolog logger and a registry-backed logger.
func BenchmarkActiveEvent(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		logger := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
		activeEvent(b, logger)
	})
	b.Run("registry", func(b *testing.B) {
		r, base := benchRegistry(zerolog.InfoLevel)
		logger := r.Logger(base, "hotstuff")
		activeEvent(b, logger)
	})
}

// BenchmarkActiveEvent_Parallel compares concurrent logging throughput across two
// independent loggers between plain zerolog and the registry.
func BenchmarkActiveEvent_Parallel(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		loggerA := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
		loggerB := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
		parallelTest(b, loggerA, loggerB)
	})
	b.Run("registry", func(b *testing.B) {
		r, base := benchRegistry(zerolog.InfoLevel)
		loggerA := r.Logger(base, "component-a")
		loggerB := r.Logger(base, "component-b")
		parallelTest(b, loggerA, loggerB)
	})
}

// BenchmarkChildLogger_SuppressedEvent compares suppression cost for a child logger
// derived via With() between plain zerolog and the registry.
func BenchmarkChildLogger_SuppressedEvent(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		parent := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
		child := parent.With().Str("sub", "voter").Logger()
		suppressEvent(b, child)
	})
	b.Run("registry", func(b *testing.B) {
		r, base := benchRegistry(zerolog.InfoLevel)
		parent := r.Logger(base, "hotstuff")
		child := parent.With().Str("sub", "voter").Logger()
		suppressEvent(b, child)
	})
}

// BenchmarkChildLogger_ActiveEvent compares active event cost for a child logger
// derived via With() between plain zerolog and the registry.
func BenchmarkChildLogger_ActiveEvent(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		parent := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
		child := parent.With().Str("sub", "voter").Logger()
		activeEvent(b, child)
	})
	b.Run("registry", func(b *testing.B) {
		r, base := benchRegistry(zerolog.InfoLevel)
		parent := r.Logger(base, "hotstuff")
		child := parent.With().Str("sub", "voter").Logger()
		activeEvent(b, child)
	})
}

func suppressEvent(b *testing.B, logger zerolog.Logger) {
	for b.Loop() {
		logger.Debug().Msg("suppressed")
	}
}

func activeEvent(b *testing.B, logger zerolog.Logger) {
	for b.Loop() {
		logger.Info().Msg("active")
	}
}

func parallelTest(b *testing.B, loggerA, loggerB zerolog.Logger) {
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
