package apply_wal

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpointPath string
	flagMergedWALPath  string
	flagOutputPath     string
)

var Cmd = &cobra.Command{
	Use:   "apply-wal",
	Short: "Benchmark applying merged WAL to a trie",
	Long: `Loads a checkpoint trie and applies a merged WAL file to it.

The benchmark:
  1. Loads checkpoint into memory (full trie)
  2. Reads merged WAL file
  3. Applies updates to trie
  4. Measures time and resources

This measures:
  - Apply time
  - Registers updated
  - Apply rate (registers/sec)
  - Memory usage

Output: JSON file with benchmark results`,
	RunE: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointPath, "checkpoint", "",
		"Path to checkpoint file")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().StringVar(&flagMergedWALPath, "merged-wal", "",
		"Path to merged WAL file (from merge-wal benchmark)")
	_ = Cmd.MarkFlagRequired("merged-wal")

	Cmd.Flags().StringVar(&flagOutputPath, "output", "",
		"Path to output JSON results file")
	_ = Cmd.MarkFlagRequired("output")
}

// BenchmarkResult holds all metrics from the benchmark
type BenchmarkResult struct {
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	Inputs    struct {
		CheckpointPath string `json:"checkpoint_path"`
		MergedWALPath  string `json:"merged_wal_path"`
	} `json:"inputs"`
	Results struct {
		LoadCheckpointTimeSec float64 `json:"load_checkpoint_time_sec"`
		ReadWALTimeSec        float64 `json:"read_wal_time_sec"`
		ApplyTimeSec          float64 `json:"apply_time_sec"`
		TotalTimeSec          float64 `json:"total_time_sec"`
		RegistersUpdated      int64   `json:"registers_updated"`
		ApplyRatePerSec       float64 `json:"apply_rate_per_sec"`
		TrieCountBefore       int     `json:"trie_count_before"`
		NewRootHash           string  `json:"new_root_hash"`
	} `json:"results"`
	Resources struct {
		PeakMemoryMB          float64 `json:"peak_memory_mb"`
		MemoryAfterLoadMB     float64 `json:"memory_after_load_mb"`
		MemoryAfterApplyMB    float64 `json:"memory_after_apply_mb"`
		Samples               []ResourceSample `json:"samples"`
	} `json:"resources"`
}

type ResourceSample struct {
	ElapsedSec float64 `json:"elapsed_sec"`
	MemoryMB   float64 `json:"memory_mb"`
	Phase      string  `json:"phase"`
}

// MergedEntry represents an entry from the merged WAL file
type MergedEntry struct {
	Path    ledger.Path
	Payload []byte
}

func run(cmd *cobra.Command, args []string) error {
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	logger.Info().
		Str("checkpoint", flagCheckpointPath).
		Str("merged_wal", flagMergedWALPath).
		Msg("Starting WAL apply benchmark")

	// Validate inputs
	if _, err := os.Stat(flagCheckpointPath); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint file not found: %s", flagCheckpointPath)
	}
	if _, err := os.Stat(flagMergedWALPath); os.IsNotExist(err) {
		return fmt.Errorf("merged WAL file not found: %s", flagMergedWALPath)
	}

	// Initialize result
	result := BenchmarkResult{
		Phase:     "apply-wal",
		Timestamp: time.Now(),
	}
	result.Inputs.CheckpointPath = flagCheckpointPath
	result.Inputs.MergedWALPath = flagMergedWALPath

	var peakMemoryMB float64
	totalStartTime := time.Now()

	// Helper to track memory
	trackMemory := func(phase string) float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memMB := float64(m.Alloc) / 1024 / 1024
		if memMB > peakMemoryMB {
			peakMemoryMB = memMB
		}
		result.Resources.Samples = append(result.Resources.Samples, ResourceSample{
			ElapsedSec: time.Since(totalStartTime).Seconds(),
			MemoryMB:   memMB,
			Phase:      phase,
		})
		return memMB
	}

	// Phase 1: Load checkpoint
	logger.Info().Msg("Loading checkpoint...")
	loadStart := time.Now()

	tries, err := wal.LoadCheckpoint(flagCheckpointPath, logger)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}

	loadTime := time.Since(loadStart)
	result.Results.LoadCheckpointTimeSec = loadTime.Seconds()
	result.Results.TrieCountBefore = len(tries)
	result.Resources.MemoryAfterLoadMB = trackMemory("after_load")

	logger.Info().
		Int("trie_count", len(tries)).
		Float64("load_time_sec", loadTime.Seconds()).
		Float64("memory_mb", result.Resources.MemoryAfterLoadMB).
		Msg("Checkpoint loaded")

	if len(tries) == 0 {
		return fmt.Errorf("no tries found in checkpoint")
	}

	// Get the latest trie
	baseTrie := tries[len(tries)-1]
	logger.Info().
		Str("root_hash", baseTrie.RootHash().String()).
		Uint64("reg_count", baseTrie.AllocatedRegCount()).
		Msg("Using latest trie")

	// Phase 2: Read merged WAL file
	logger.Info().Msg("Reading merged WAL file...")
	readStart := time.Now()

	entries, err := readMergedWAL(flagMergedWALPath)
	if err != nil {
		return fmt.Errorf("failed to read merged WAL: %w", err)
	}

	readTime := time.Since(readStart)
	result.Results.ReadWALTimeSec = readTime.Seconds()
	trackMemory("after_read_wal")

	logger.Info().
		Int("entries", len(entries)).
		Float64("read_time_sec", readTime.Seconds()).
		Msg("Merged WAL read")

	// Phase 3: Apply updates to trie
	logger.Info().Msg("Applying updates to trie...")
	applyStart := time.Now()

	// Convert entries to paths and payloads
	paths := make([]ledger.Path, len(entries))
	payloads := make([]ledger.Payload, len(entries))

	for i, entry := range entries {
		paths[i] = entry.Path
		if len(entry.Payload) > 0 {
			payload, err := ledger.DecodePayloadWithoutPrefix(entry.Payload, false, 1)
			if err != nil {
				// Try as raw value
				payloads[i] = *ledger.NewPayload(ledger.Key{}, entry.Payload)
			} else {
				payloads[i] = *payload
			}
		} else {
			payloads[i] = *ledger.EmptyPayload()
		}
	}

	// Apply update to trie
	// Note: This creates a new trie with the updates applied
	newTrie, _, err := trie.NewTrieWithUpdatedRegisters(baseTrie, paths, payloads, true)
	if err != nil {
		return fmt.Errorf("failed to apply updates to trie: %w", err)
	}

	applyTime := time.Since(applyStart)
	result.Results.ApplyTimeSec = applyTime.Seconds()
	result.Results.RegistersUpdated = int64(len(entries))
	result.Results.NewRootHash = newTrie.RootHash().String()
	result.Resources.MemoryAfterApplyMB = trackMemory("after_apply")

	if applyTime.Seconds() > 0 {
		result.Results.ApplyRatePerSec = float64(len(entries)) / applyTime.Seconds()
	}

	logger.Info().
		Int("registers_updated", len(entries)).
		Float64("apply_time_sec", applyTime.Seconds()).
		Float64("apply_rate_per_sec", result.Results.ApplyRatePerSec).
		Str("new_root_hash", result.Results.NewRootHash).
		Msg("Updates applied")

	// Final stats
	totalTime := time.Since(totalStartTime)
	result.Results.TotalTimeSec = totalTime.Seconds()
	result.Resources.PeakMemoryMB = peakMemoryMB

	// Log final stats
	logger.Info().
		Float64("load_checkpoint_sec", result.Results.LoadCheckpointTimeSec).
		Float64("read_wal_sec", result.Results.ReadWALTimeSec).
		Float64("apply_sec", result.Results.ApplyTimeSec).
		Float64("total_sec", result.Results.TotalTimeSec).
		Int64("registers_updated", result.Results.RegistersUpdated).
		Float64("apply_rate_per_sec", result.Results.ApplyRatePerSec).
		Float64("peak_memory_mb", result.Resources.PeakMemoryMB).
		Msg("Apply benchmark complete")

	// Write results to file
	if err := writeResults(flagOutputPath, result); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	logger.Info().Str("output", flagOutputPath).Msg("Results written")
	return nil
}

func readMergedWAL(path string) ([]MergedEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read count
	var count uint32
	if err := binary.Read(f, binary.BigEndian, &count); err != nil {
		return nil, fmt.Errorf("failed to read entry count: %w", err)
	}

	entries := make([]MergedEntry, 0, count)

	for i := uint32(0); i < count; i++ {
		var entry MergedEntry

		// Read path (32 bytes)
		if _, err := io.ReadFull(f, entry.Path[:]); err != nil {
			return nil, fmt.Errorf("failed to read path %d: %w", i, err)
		}

		// Read payload length
		var payloadLen uint32
		if err := binary.Read(f, binary.BigEndian, &payloadLen); err != nil {
			return nil, fmt.Errorf("failed to read payload length %d: %w", i, err)
		}

		// Read payload data
		if payloadLen > 0 {
			entry.Payload = make([]byte, payloadLen)
			if _, err := io.ReadFull(f, entry.Payload); err != nil {
				return nil, fmt.Errorf("failed to read payload %d: %w", i, err)
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
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
