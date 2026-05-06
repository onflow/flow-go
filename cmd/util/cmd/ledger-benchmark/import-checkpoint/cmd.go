package import_checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
)

var (
	flagCheckpointPath string
	flagStorehousePath string
	flagOutputPath     string
	flagWorkerCount    int
	flagRootHeight     uint64
)

var Cmd = &cobra.Command{
	Use:   "import-checkpoint",
	Short: "Import a checkpoint file into storehouse (Pebble)",
	Long: `Reads a checkpoint file and imports all register payloads into a Pebble database.

This benchmark measures:
  - Total import time
  - Register count and total bytes
  - Import rate (registers/sec, MB/sec)
  - Peak memory usage
  - CPU utilization

Output: JSON file with benchmark results`,
	RunE: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointPath, "checkpoint", "",
		"Path to checkpoint file (e.g., /data/root.checkpoint)")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().StringVar(&flagStorehousePath, "storehouse", "",
		"Path to output Pebble database directory")
	_ = Cmd.MarkFlagRequired("storehouse")

	Cmd.Flags().StringVar(&flagOutputPath, "output", "",
		"Path to output JSON results file")
	_ = Cmd.MarkFlagRequired("output")

	Cmd.Flags().IntVar(&flagWorkerCount, "workers", 4,
		"Number of workers for parallel checkpoint reading")

	Cmd.Flags().Uint64Var(&flagRootHeight, "root-height", 0,
		"Root height for the checkpoint (required for storehouse)")
	_ = Cmd.MarkFlagRequired("root-height")
}

// BenchmarkResult holds all metrics from the benchmark
type BenchmarkResult struct {
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	Inputs    struct {
		CheckpointPath string `json:"checkpoint_path"`
		StorehousePath string `json:"storehouse_path"`
		WorkerCount    int    `json:"worker_count"`
		RootHeight     uint64 `json:"root_height"`
	} `json:"inputs"`
	Results struct {
		TotalTimeSec       float64 `json:"total_time_sec"`
		RegisterCount      int64   `json:"register_count"`
		TotalBytes         int64   `json:"total_bytes"`
		RateRegistersPerSec float64 `json:"rate_registers_per_sec"`
		RateMBPerSec       float64 `json:"rate_mb_per_sec"`
	} `json:"results"`
	Resources struct {
		PeakMemoryMB float64          `json:"peak_memory_mb"`
		AvgCPUPct    float64          `json:"avg_cpu_pct"`
		Samples      []ResourceSample `json:"samples"`
	} `json:"resources"`
}

type ResourceSample struct {
	ElapsedSec float64 `json:"elapsed_sec"`
	MemoryMB   float64 `json:"memory_mb"`
	Registers  int64   `json:"registers"`
	Bytes      int64   `json:"bytes"`
}

func run(cmd *cobra.Command, args []string) error {
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	logger.Info().
		Str("checkpoint", flagCheckpointPath).
		Str("storehouse", flagStorehousePath).
		Int("workers", flagWorkerCount).
		Uint64("root_height", flagRootHeight).
		Msg("Starting checkpoint import benchmark")

	// Validate checkpoint exists
	if _, err := os.Stat(flagCheckpointPath); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint file not found: %s", flagCheckpointPath)
	}

	// Create storehouse directory
	if err := os.MkdirAll(flagStorehousePath, 0755); err != nil {
		return fmt.Errorf("failed to create storehouse directory: %w", err)
	}

	// Initialize result
	result := BenchmarkResult{
		Phase:     "import-checkpoint",
		Timestamp: time.Now(),
	}
	result.Inputs.CheckpointPath = flagCheckpointPath
	result.Inputs.StorehousePath = flagStorehousePath
	result.Inputs.WorkerCount = flagWorkerCount
	result.Inputs.RootHeight = flagRootHeight

	// Start resource monitoring
	var registerCount atomic.Int64
	var totalBytes atomic.Int64
	var peakMemoryMB float64
	stopMonitor := make(chan struct{})
	monitorDone := make(chan struct{})

	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		startTime := time.Now()

		for {
			select {
			case <-stopMonitor:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				memMB := float64(m.Alloc) / 1024 / 1024
				if memMB > peakMemoryMB {
					peakMemoryMB = memMB
				}

				regs := registerCount.Load()
				bytes := totalBytes.Load()
				elapsed := time.Since(startTime).Seconds()

				result.Resources.Samples = append(result.Resources.Samples, ResourceSample{
					ElapsedSec: elapsed,
					MemoryMB:   memMB,
					Registers:  regs,
					Bytes:      bytes,
				})

				logger.Info().
					Int64("registers", regs).
					Int64("bytes", bytes).
					Float64("memory_mb", memMB).
					Float64("elapsed_sec", elapsed).
					Msg("Progress")
			}
		}
	}()

	// Get root hash from checkpoint
	dir := filepath.Dir(flagCheckpointPath)
	fileName := filepath.Base(flagCheckpointPath)
	rootHashes, err := wal.ReadTriesRootHash(logger, dir, fileName)
	if err != nil {
		close(stopMonitor)
		<-monitorDone
		return fmt.Errorf("failed to read root hashes from checkpoint: %w", err)
	}

	if len(rootHashes) == 0 {
		close(stopMonitor)
		<-monitorDone
		return fmt.Errorf("no tries found in checkpoint")
	}

	// Use the last root hash (most recent state)
	rootHash := rootHashes[len(rootHashes)-1]
	logger.Info().
		Str("root_hash", rootHash.String()).
		Int("trie_count", len(rootHashes)).
		Msg("Found root hash in checkpoint")

	// Open Pebble database
	db, err := pebbleStorage.OpenRegisterPebbleDB(logger, flagStorehousePath)
	if err != nil {
		close(stopMonitor)
		<-monitorDone
		return fmt.Errorf("failed to open Pebble database: %w", err)
	}
	defer db.Close()

	// Create bootstrap instance
	bootstrap, err := pebbleStorage.NewRegisterBootstrap(
		db,
		flagCheckpointPath,
		flagRootHeight,
		ledger.RootHash(rootHash),
		logger,
	)
	if err != nil {
		close(stopMonitor)
		<-monitorDone
		return fmt.Errorf("failed to create bootstrap: %w", err)
	}

	// Run the import with timing
	startTime := time.Now()

	ctx := context.Background()
	err = bootstrap.IndexCheckpointFile(ctx, flagWorkerCount)
	if err != nil {
		close(stopMonitor)
		<-monitorDone
		return fmt.Errorf("failed to index checkpoint: %w", err)
	}

	elapsed := time.Since(startTime)

	// Stop monitoring
	close(stopMonitor)
	<-monitorDone

	// Get final counts from the database
	registers, err := pebbleStorage.NewRegisters(db, pebbleStorage.PruningDisabled)
	if err != nil {
		return fmt.Errorf("failed to create registers: %w", err)
	}

	// Get final register count from bootstrap
	finalRegCount := int64(bootstrap.RegisterCount())
	finalBytes := totalBytes.Load()

	// Calculate final metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	finalMemMB := float64(m.Alloc) / 1024 / 1024
	if finalMemMB > peakMemoryMB {
		peakMemoryMB = finalMemMB
	}

	result.Results.TotalTimeSec = elapsed.Seconds()
	result.Results.RegisterCount = finalRegCount
	result.Results.TotalBytes = finalBytes
	if elapsed.Seconds() > 0 {
		result.Results.RateRegistersPerSec = float64(finalRegCount) / elapsed.Seconds()
		result.Results.RateMBPerSec = float64(finalBytes) / 1024 / 1024 / elapsed.Seconds()
	}
	result.Resources.PeakMemoryMB = peakMemoryMB

	// Log final stats
	logger.Info().
		Float64("total_time_sec", result.Results.TotalTimeSec).
		Int64("register_count", result.Results.RegisterCount).
		Int64("total_bytes", result.Results.TotalBytes).
		Float64("rate_reg_per_sec", result.Results.RateRegistersPerSec).
		Float64("rate_mb_per_sec", result.Results.RateMBPerSec).
		Float64("peak_memory_mb", result.Resources.PeakMemoryMB).
		Uint64("latest_height", registers.LatestHeight()).
		Msg("Import complete")

	// Write results to file
	if err := writeResults(flagOutputPath, result); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	logger.Info().Str("output", flagOutputPath).Msg("Results written")
	return nil
}

func writeResults(path string, result BenchmarkResult) error {
	// Create output directory if needed
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
