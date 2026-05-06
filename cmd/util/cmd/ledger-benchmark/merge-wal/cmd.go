package merge_wal

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	prometheusWAL "github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	flagWALDir       string
	flagOutputDir    string
	flagResultsPath  string
	flagSegmentCount int
)

var Cmd = &cobra.Command{
	Use:   "merge-wal",
	Short: "Benchmark WAL file merging",
	Long: `Merges multiple WAL segment files into a single merged file.

The merge algorithm:
  1. Read N WAL segments sequentially
  2. For each trie update, extract (path, payload) pairs
  3. Store in map[path] = payload (later values overwrite earlier)
  4. Write merged result to output file

This measures:
  - Merge time
  - Input/output sizes
  - Deduplication ratio
  - Memory usage

Output: JSON file with benchmark results and merged WAL file`,
	RunE: run,
}

func init() {
	Cmd.Flags().StringVar(&flagWALDir, "wal-dir", "",
		"Path to directory containing WAL segment files")
	_ = Cmd.MarkFlagRequired("wal-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Path to output directory for merged WAL file")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagResultsPath, "results", "",
		"Path to output JSON results file")
	_ = Cmd.MarkFlagRequired("results")

	Cmd.Flags().IntVar(&flagSegmentCount, "segments", 100,
		"Number of WAL segments to merge")
}

// BenchmarkResult holds all metrics from the benchmark
type BenchmarkResult struct {
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	Inputs    struct {
		WALDir       string `json:"wal_dir"`
		SegmentCount int    `json:"segment_count"`
	} `json:"inputs"`
	Results struct {
		MergeTimeSec             float64 `json:"merge_time_sec"`
		InputSizeBytes           int64   `json:"input_size_bytes"`
		OutputSizeBytes          int64   `json:"output_size_bytes"`
		TotalUpdatesPreMerge     int64   `json:"total_updates_pre_merge"`
		UniqueRegistersPostMerge int64   `json:"unique_registers_post_merge"`
		DeduplicationRatio       float64 `json:"deduplication_ratio_pct"`
		TrieUpdatesProcessed     int64   `json:"trie_updates_processed"`
	} `json:"results"`
	Resources struct {
		PeakMemoryMB float64          `json:"peak_memory_mb"`
		Samples      []ResourceSample `json:"samples"`
	} `json:"resources"`
	Output struct {
		MergedWALPath string `json:"merged_wal_path"`
	} `json:"output"`
}

type ResourceSample struct {
	ElapsedSec      float64 `json:"elapsed_sec"`
	MemoryMB        float64 `json:"memory_mb"`
	SegmentsRead    int     `json:"segments_read"`
	UniqueRegisters int     `json:"unique_registers"`
}

func run(cmd *cobra.Command, args []string) error {
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	logger.Info().
		Str("wal_dir", flagWALDir).
		Int("segments", flagSegmentCount).
		Str("output_dir", flagOutputDir).
		Msg("Starting WAL merge benchmark")

	// Validate inputs
	if _, err := os.Stat(flagWALDir); os.IsNotExist(err) {
		return fmt.Errorf("WAL directory not found: %s", flagWALDir)
	}

	// Convert to absolute path for symlinks to work correctly
	absWALDir, err := filepath.Abs(flagWALDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for WAL dir: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Initialize result
	result := BenchmarkResult{
		Phase:     "merge-wal",
		Timestamp: time.Now(),
	}
	result.Inputs.WALDir = flagWALDir

	// Get WAL segment files and create temp directory with only N segments
	segments, err := getWALSegmentFiles(absWALDir, flagSegmentCount)
	if err != nil {
		return fmt.Errorf("failed to get WAL segments: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("no WAL segments found in %s", absWALDir)
	}
	logger.Info().Int("segments_found", len(segments)).Int("segments_requested", flagSegmentCount).Msg("Found WAL segments")

	// Record actual segment count used
	result.Inputs.SegmentCount = len(segments)

	// Calculate input size from selected segments
	var inputSize int64
	for _, seg := range segments {
		info, err := os.Stat(seg)
		if err == nil {
			inputSize += info.Size()
		}
	}
	result.Results.InputSizeBytes = inputSize

	// Create temp directory with symlinks to only the selected segments
	tempDir, err := os.MkdirTemp("", "wal-merge-benchmark-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for _, seg := range segments {
		segName := filepath.Base(seg)
		linkPath := filepath.Join(tempDir, segName)
		if err := os.Symlink(seg, linkPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", segName, err)
		}
	}

	logger.Info().
		Str("temp_dir", tempDir).
		Int("segments", len(segments)).
		Int64("input_size_bytes", inputSize).
		Msg("Created temp directory with segment symlinks")

	// Create WAL for reading from temp directory
	noopMetrics := &metrics.NoopCollector{}
	noopRegistry := prometheusWAL.NewRegistry()

	diskWAL, err := wal.NewDiskWAL(
		logger,
		noopRegistry,
		noopMetrics,
		tempDir,
		100, // capacity (doesn't matter for replay)
		32,  // pathByteSize
		wal.SegmentSize,
	)
	if err != nil {
		return fmt.Errorf("failed to create WAL reader: %w", err)
	}

	// Map to hold merged payloads: path -> payload
	mergedPayloads := make(map[ledger.Path]*ledger.Payload)
	var totalUpdates int64
	var trieUpdatesProcessed int64
	var peakMemoryMB float64

	// Start timing
	startTime := time.Now()
	lastLogTime := startTime

	// Replay WAL and merge
	logger.Info().Msg("Starting WAL replay and merge...")

	err = diskWAL.Replay(
		// Checkpoint callback - skip
		func(tries []*trie.MTrie) error {
			return nil
		},
		// Update callback - merge payloads
		func(update *ledger.TrieUpdate) error {
			trieUpdatesProcessed++

			paths := update.Paths
			payloads := update.Payloads

			for i, path := range paths {
				if i < len(payloads) {
					totalUpdates++
					// Later values overwrite earlier values
					mergedPayloads[path] = payloads[i]
				}
			}

			// Log progress periodically
			if time.Since(lastLogTime) > 10*time.Second {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				memMB := float64(m.Alloc) / 1024 / 1024
				if memMB > peakMemoryMB {
					peakMemoryMB = memMB
				}

				elapsed := time.Since(startTime).Seconds()
				result.Resources.Samples = append(result.Resources.Samples, ResourceSample{
					ElapsedSec:      elapsed,
					MemoryMB:        memMB,
					SegmentsRead:    len(segments),
					UniqueRegisters: len(mergedPayloads),
				})

				logger.Info().
					Int64("trie_updates", trieUpdatesProcessed).
					Int64("total_register_updates", totalUpdates).
					Int("unique_registers", len(mergedPayloads)).
					Float64("memory_mb", memMB).
					Msg("Merge progress")

				lastLogTime = time.Now()
			}

			return nil
		},
		// Delete callback - skip
		func(rootHash ledger.RootHash) error {
			return nil
		},
	)

	if err != nil {
		return fmt.Errorf("failed to replay WAL: %w", err)
	}

	mergeTime := time.Since(startTime)

	// Final memory reading
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	finalMemMB := float64(m.Alloc) / 1024 / 1024
	if finalMemMB > peakMemoryMB {
		peakMemoryMB = finalMemMB
	}

	logger.Info().
		Int64("trie_updates", trieUpdatesProcessed).
		Int64("total_register_updates", totalUpdates).
		Int("unique_registers", len(mergedPayloads)).
		Float64("merge_time_sec", mergeTime.Seconds()).
		Msg("WAL replay complete")

	// Write merged WAL file
	mergedWALPath := filepath.Join(flagOutputDir, fmt.Sprintf("merged-%d.wal", flagSegmentCount))
	outputSize, err := writeMergedWAL(mergedWALPath, mergedPayloads, logger)
	if err != nil {
		return fmt.Errorf("failed to write merged WAL: %w", err)
	}

	// Populate results
	result.Results.MergeTimeSec = mergeTime.Seconds()
	result.Results.OutputSizeBytes = outputSize
	result.Results.TotalUpdatesPreMerge = totalUpdates
	result.Results.UniqueRegistersPostMerge = int64(len(mergedPayloads))
	result.Results.TrieUpdatesProcessed = trieUpdatesProcessed
	if totalUpdates > 0 {
		result.Results.DeduplicationRatio = float64(totalUpdates-int64(len(mergedPayloads))) / float64(totalUpdates) * 100
	}
	result.Resources.PeakMemoryMB = peakMemoryMB
	result.Output.MergedWALPath = mergedWALPath

	// Log final stats
	logger.Info().
		Float64("merge_time_sec", result.Results.MergeTimeSec).
		Int64("input_size_bytes", result.Results.InputSizeBytes).
		Int64("output_size_bytes", result.Results.OutputSizeBytes).
		Int64("total_updates", result.Results.TotalUpdatesPreMerge).
		Int64("unique_registers", result.Results.UniqueRegistersPostMerge).
		Float64("dedup_ratio_pct", result.Results.DeduplicationRatio).
		Float64("peak_memory_mb", result.Resources.PeakMemoryMB).
		Str("merged_wal", mergedWALPath).
		Msg("Merge benchmark complete")

	// Write results to file
	if err := writeResults(flagResultsPath, result); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	logger.Info().Str("output", flagResultsPath).Msg("Results written")
	return nil
}

// getWALSegmentFiles returns the first N WAL segment files from the directory
func getWALSegmentFiles(dir string, maxSegments int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var segments []string

	// Sort entries to get segments in order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if it looks like a WAL segment (numeric name, 8 characters)
		name := entry.Name()
		if len(name) == 8 {
			segments = append(segments, filepath.Join(dir, name))
			if len(segments) >= maxSegments {
				break
			}
		}
	}

	return segments, nil
}

func writeMergedWAL(path string, payloads map[ledger.Path]*ledger.Payload, logger zerolog.Logger) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("failed to create merged WAL file: %w", err)
	}
	defer f.Close()

	// Write a simple binary format:
	// [4 bytes] number of entries
	// For each entry:
	//   [32 bytes] path
	//   [4 bytes] payload length
	//   [N bytes] payload data

	// Write count
	count := uint32(len(payloads))
	if err := binary.Write(f, binary.BigEndian, count); err != nil {
		return 0, err
	}

	// Write entries
	for path, payload := range payloads {
		// Write path (32 bytes)
		if _, err := f.Write(path[:]); err != nil {
			return 0, err
		}

		// Get payload value
		var valueBytes []byte
		if payload != nil {
			valueBytes = payload.Value()
		}

		// Write value length
		valueLen := uint32(len(valueBytes))
		if err := binary.Write(f, binary.BigEndian, valueLen); err != nil {
			return 0, err
		}

		// Write value data
		if len(valueBytes) > 0 {
			if _, err := f.Write(valueBytes); err != nil {
				return 0, err
			}
		}
	}

	// Get file size
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	logger.Info().
		Str("path", path).
		Int64("size_bytes", info.Size()).
		Int("entries", len(payloads)).
		Msg("Merged WAL written")

	return info.Size(), nil
}

func writeResults(path string, result BenchmarkResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
