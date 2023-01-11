package unittest

import (
	"runtime"

	"github.com/rs/zerolog"
)

// PrintHeapInfo prints heap object allocation through given logger.
func PrintHeapInfo(logger zerolog.Logger) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	logger.Info().
		Uint64(".Alloc", m.Alloc).
		Uint64(".AllocObjects", m.HeapObjects).
		Uint64(".TotalAlloc", m.TotalAlloc).
		Uint32(".NumGC", m.NumGC).
		Msg("heap allocation digest")
}
