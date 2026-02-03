package pebble

import (
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
)

const (
	// DefaultMinCompactionSizeBytes is the default minimum input size for logging compactions.
	// Small compactions below this threshold are ignored.
	DefaultMinCompactionSizeBytes int64 = 200 << 20 // 200 MB

	// DefaultMinCompactionDuration is the default minimum duration for logging compactions.
	// Fast compactions below this threshold are ignored.
	DefaultMinCompactionDuration = 5 * time.Second

	// DefaultMinCompactionFiles is the default minimum number of input files for logging compactions.
	// Compactions with fewer files are ignored.
	DefaultMinCompactionFiles = 5
)

// CompactionEventListener implements pebble.EventListener to log compaction events.
// Only significant compactions (large size, long duration, or many files) are logged.
type CompactionEventListener struct {
	logger       zerolog.Logger
	minSizeBytes int64
	minDuration  time.Duration
	minFiles     int
}

// NewCompactionEventListener creates a new CompactionEventListener with default thresholds.
func NewCompactionEventListener(logger zerolog.Logger) *CompactionEventListener {
	return NewCompactionEventListenerWithThresholds(
		logger,
		DefaultMinCompactionSizeBytes,
		DefaultMinCompactionDuration,
		DefaultMinCompactionFiles,
	)
}

// NewCompactionEventListenerWithThresholds creates a new CompactionEventListener with custom thresholds.
func NewCompactionEventListenerWithThresholds(
	logger zerolog.Logger,
	minSizeBytes int64,
	minDuration time.Duration,
	minFiles int,
) *CompactionEventListener {
	return &CompactionEventListener{
		logger:       logger.With().Str("module", "pebble_compaction").Logger(),
		minSizeBytes: minSizeBytes,
		minDuration:  minDuration,
		minFiles:     minFiles,
	}
}

// calculateTotalSize calculates the total size of tables in a level.
func calculateTotalSize(level pebble.LevelInfo) int64 {
	var totalSize int64
	for _, table := range level.Tables {
		totalSize += int64(table.Size)
	}
	return totalSize
}

// isSignificantCompaction checks if a compaction is significant enough to log
// based on size, file count, or duration (for end events).
func (e *CompactionEventListener) isSignificantCompaction(
	totalInputSize int64,
	totalInputFiles int,
	duration time.Duration,
) bool {
	// Log if size exceeds threshold
	if totalInputSize >= e.minSizeBytes {
		return true
	}
	// Log if file count exceeds threshold
	if totalInputFiles >= e.minFiles {
		return true
	}
	// Log if duration exceeds threshold (only available for end events)
	if duration > 0 && duration >= e.minDuration {
		return true
	}
	return false
}

// CompactionBegin logs when a significant compaction starts.
func (e *CompactionEventListener) CompactionBegin(info pebble.CompactionInfo) {
	var totalInputFiles int
	var totalInputSize int64
	var inputLevels []int

	for _, level := range info.Input {
		totalInputFiles += len(level.Tables)
		totalInputSize += calculateTotalSize(level)
		inputLevels = append(inputLevels, level.Level)
	}

	// Only log if compaction is significant (size or file count threshold)
	if !e.isSignificantCompaction(totalInputSize, totalInputFiles, 0) {
		return
	}

	e.logger.Debug().
		Int("job", info.JobID).
		Str("reason", info.Reason).
		Ints("input_levels", inputLevels).
		Int("output_level", info.Output.Level).
		Int("num_input_files", totalInputFiles).
		Int64("input_size_bytes", totalInputSize).
		Msg("compaction started")
}

// CompactionEnd logs when a significant compaction ends.
func (e *CompactionEventListener) CompactionEnd(info pebble.CompactionInfo) {
	var totalInputFiles int
	var totalInputSize int64
	var inputLevels []int

	for _, level := range info.Input {
		totalInputFiles += len(level.Tables)
		totalInputSize += calculateTotalSize(level)
		inputLevels = append(inputLevels, level.Level)
	}

	// Only log if compaction is significant (size, file count, or duration threshold)
	if !e.isSignificantCompaction(totalInputSize, totalInputFiles, info.Duration) {
		return
	}

	outputSize := calculateTotalSize(info.Output)

	e.logger.Info().
		Int("job", info.JobID).
		Ints("input_levels", inputLevels).
		Int("output_level", info.Output.Level).
		Int("num_input_files", totalInputFiles).
		Int("num_output_files", len(info.Output.Tables)).
		Int64("input_size_bytes", totalInputSize).
		Int64("output_size_bytes", outputSize).
		Float64("duration_seconds", info.Duration.Seconds()).
		Msg("compaction finished")
}
