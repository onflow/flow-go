// wal-payload-stats analyzes WAL files and counts payload values by type in TrieUpdate records.
//
// Usage:
//
//	go run cmd/util/cmd/wal-payload-stats/main.go --wal-dir /path/to/wal --from 7298 --to 7298
//
// Note: Segment numbers should be specified as integers (e.g., 7298), not as filenames (e.g., 00007298).
// The utility automatically converts segment numbers to the zero-padded 8-digit filenames.
//
// The utility prints three counts:
//  1. Number of payloads with nil values
//  2. Number of payloads with empty slice []byte{} values
//  3. Number of payloads with non-empty values
package main

import (
	"flag"
	"fmt"
	"os"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

func main() {
	var walDir string
	var fromSegment, toSegment int
	flag.StringVar(&walDir, "wal-dir", "", "Directory containing WAL files")
	flag.IntVar(&fromSegment, "from", 0, "Starting segment number (inclusive)")
	flag.IntVar(&toSegment, "to", 0, "Ending segment number (inclusive)")
	flag.Parse()

	if walDir == "" {
		fmt.Fprintf(os.Stderr, "Error: --wal-dir flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger with timestamps to stderr
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Log input
	logger.Info().
		Str("wal_dir", walDir).
		Int("from", fromSegment).
		Int("to", toSegment).
		Msg("Starting WAL payload analysis")

	// Validate segment range
	if fromSegment < 0 {
		logger.Fatal().Msg("from segment must be >= 0")
	}
	if toSegment < fromSegment {
		logger.Fatal().Msg("to segment must be >= from segment")
	}

	// Check available segments
	first, last, err := prometheusWAL.Segments(walDir)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get available segments")
	}

	if first < 0 {
		logger.Fatal().Msg("No WAL segments found in directory")
	}

	logger.Info().
		Int("available_first", first).
		Int("available_last", last).
		Msg("Available WAL segments")

	// Validate requested range against available segments
	if fromSegment < first {
		logger.Warn().
			Int("requested_from", fromSegment).
			Int("available_first", first).
			Msg("Requested 'from' segment is before first available segment, using first available")
		fromSegment = first
	}
	if toSegment > last {
		logger.Warn().
			Int("requested_to", toSegment).
			Int("available_last", last).
			Msg("Requested 'to' segment is after last available segment, using last available")
		toSegment = last
	}

	// Log which segment files will be read
	logger.Info().
		Str("from_file", prometheusWAL.SegmentName(walDir, fromSegment)).
		Str("to_file", prometheusWAL.SegmentName(walDir, toSegment)).
		Int("from_segment", fromSegment).
		Int("to_segment", toSegment).
		Msg("Reading WAL segment files")

	// Create segment range reader
	sr, err := prometheusWAL.NewSegmentsRangeReader(logger, prometheusWAL.SegmentRange{
		Dir:   walDir,
		First: fromSegment,
		Last:  toSegment,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create segments range reader")
	}
	defer sr.Close()

	// Create WAL reader
	reader := prometheusWAL.NewReader(sr)

	// Count payloads by value type
	var nilCount, emptySliceCount, nonEmptyCount int
	var updateCount int

	// Process all records
	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to decode WAL record")
		}

		// Only process WALUpdate operations
		if operation == wal.WALUpdate {
			updateCount++
			if update == nil {
				continue
			}

			// Check all payloads in the TrieUpdate
			for _, payload := range update.Payloads {
				if payload == nil {
					continue
				}

				value := payload.Value()
				if value == nil {
					nilCount++
				} else if len(value) == 0 {
					emptySliceCount++
				} else {
					nonEmptyCount++
				}
			}
		}

		if err := reader.Err(); err != nil {
			logger.Fatal().Err(err).Msg("Error reading WAL")
		}
	}

	logger.Info().
		Int("update_count", updateCount).
		Msg("Finished processing WAL records")

	// Print results
	fmt.Printf("Payload statistics:\n")
	fmt.Printf("  nil values:        %d\n", nilCount)
	fmt.Printf("  empty slice []byte{}: %d\n", emptySliceCount)
	fmt.Printf("  non-empty values:  %d\n", nonEmptyCount)
	fmt.Printf("  total payloads:    %d\n", nilCount+emptySliceCount+nonEmptyCount)
}
