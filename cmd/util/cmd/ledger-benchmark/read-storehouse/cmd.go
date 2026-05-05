package read_storehouse

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
)

var (
	flagCheckpointPath string
	flagStorehousePath string
	flagOutputPath     string
	flagWorkerCount    int
	flagHeight         uint64
)

var Cmd = &cobra.Command{
	Use:   "read-storehouse",
	Short: "Benchmark concurrent reads from storehouse",
	Long: `Reads register values from storehouse using concurrent workers.

The benchmark:
  1. Loads checkpoint to get leaf node paths (register IDs)
  2. Creates a Pebble snapshot for consistent reads
  3. Uses N worker goroutines to read register values
  4. Measures latency distribution (avg, P50, P95, P99)

Output: JSON file with benchmark results including latency histogram`,
	RunE: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointPath, "checkpoint", "",
		"Path to checkpoint file (for leaf node iteration)")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().StringVar(&flagStorehousePath, "storehouse", "",
		"Path to populated Pebble database directory")
	_ = Cmd.MarkFlagRequired("storehouse")

	Cmd.Flags().StringVar(&flagOutputPath, "output", "",
		"Path to output JSON results file")
	_ = Cmd.MarkFlagRequired("output")

	Cmd.Flags().IntVar(&flagWorkerCount, "workers", 16,
		"Number of concurrent reader workers")

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"Height to read registers at")
	_ = Cmd.MarkFlagRequired("height")
}

// BenchmarkResult holds all metrics from the benchmark
type BenchmarkResult struct {
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	Inputs    struct {
		CheckpointPath string `json:"checkpoint_path"`
		StorehousePath string `json:"storehouse_path"`
		WorkerCount    int    `json:"worker_count"`
		Height         uint64 `json:"height"`
	} `json:"inputs"`
	Results struct {
		TotalTimeSec        float64 `json:"total_time_sec"`
		RegisterCount       int64   `json:"register_count"`
		TotalBytes          int64   `json:"total_bytes"`
		RateRegistersPerSec float64 `json:"rate_registers_per_sec"`
		RateMBPerSec        float64 `json:"rate_mb_per_sec"`
		AvgLatencyUs        float64 `json:"avg_latency_us"`
		P50LatencyUs        float64 `json:"p50_latency_us"`
		P95LatencyUs        float64 `json:"p95_latency_us"`
		P99LatencyUs        float64 `json:"p99_latency_us"`
		MaxLatencyUs        float64 `json:"max_latency_us"`
		ErrorCount          int64   `json:"error_count"`
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

// ReadJob represents a register to read
type ReadJob struct {
	Path       ledger.Path
	RegisterID flow.RegisterID
}

func run(cmd *cobra.Command, args []string) error {
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	logger.Info().
		Str("checkpoint", flagCheckpointPath).
		Str("storehouse", flagStorehousePath).
		Int("workers", flagWorkerCount).
		Uint64("height", flagHeight).
		Msg("Starting storehouse read benchmark")

	// Validate inputs
	if _, err := os.Stat(flagCheckpointPath); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint file not found: %s", flagCheckpointPath)
	}
	if _, err := os.Stat(flagStorehousePath); os.IsNotExist(err) {
		return fmt.Errorf("storehouse directory not found: %s", flagStorehousePath)
	}

	// Initialize result
	result := BenchmarkResult{
		Phase:     "read-storehouse",
		Timestamp: time.Now(),
	}
	result.Inputs.CheckpointPath = flagCheckpointPath
	result.Inputs.StorehousePath = flagStorehousePath
	result.Inputs.WorkerCount = flagWorkerCount
	result.Inputs.Height = flagHeight

	// Open Pebble database
	logger.Info().Msg("Opening storehouse database...")
	registers, db, err := pebbleStorage.NewBootstrappedRegistersWithPath(logger, flagStorehousePath)
	if err != nil {
		return fmt.Errorf("failed to open storehouse: %w", err)
	}
	defer db.Close()

	logger.Info().
		Uint64("first_height", registers.FirstHeight()).
		Uint64("latest_height", registers.LatestHeight()).
		Msg("Storehouse opened")

	// Validate height is within range
	if flagHeight < registers.FirstHeight() || flagHeight > registers.LatestHeight() {
		return fmt.Errorf("height %d is outside indexed range [%d, %d]",
			flagHeight, registers.FirstHeight(), registers.LatestHeight())
	}

	// Load checkpoint leaf nodes
	logger.Info().Msg("Loading checkpoint leaf nodes...")
	dir := filepath.Dir(flagCheckpointPath)
	fileName := filepath.Base(flagCheckpointPath)

	rootHashes, err := wal.ReadTriesRootHash(logger, dir, fileName)
	if err != nil {
		return fmt.Errorf("failed to read root hashes: %w", err)
	}
	if len(rootHashes) == 0 {
		return fmt.Errorf("no tries found in checkpoint")
	}

	// Read leaf nodes via channel
	leafNodesCh := make(chan *wal.LeafNode, 100000)
	var readErr error
	go func() {
		readErr = wal.OpenAndReadLeafNodesFromCheckpointV6(
			leafNodesCh,
			dir,
			fileName,
			rootHashes[len(rootHashes)-1],
			logger,
		)
	}()

	// Collect all leaf nodes (register IDs)
	var jobs []ReadJob
	for leaf := range leafNodesCh {
		if leaf.Payload != nil && !leaf.Payload.IsEmpty() {
			key, err := leaf.Payload.Key()
			if err != nil {
				continue
			}
			jobs = append(jobs, ReadJob{
				Path: leaf.Path,
				RegisterID: flow.RegisterID{
					Owner: string(key.KeyParts[0].Value),
					Key:   string(key.KeyParts[1].Value),
				},
			})
		}
	}

	if readErr != nil {
		return fmt.Errorf("failed to read leaf nodes: %w", readErr)
	}

	logger.Info().Int("leaf_count", len(jobs)).Msg("Leaf nodes loaded")

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

				rate := float64(0)
				if elapsed > 0 {
					rate = float64(regs) / elapsed
				}

				logger.Info().
					Int64("registers", regs).
					Int("total", len(jobs)).
					Float64("pct", float64(regs)/float64(len(jobs))*100).
					Float64("rate_per_sec", rate).
					Float64("memory_mb", memMB).
					Msg("Progress")
			}
		}
	}()

	// Create job channel
	jobCh := make(chan ReadJob, 10000)

	// Latency collection
	var latenciesMu sync.Mutex
	var latencies []time.Duration
	var errorCount atomic.Int64

	// Start workers
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < flagWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for job := range jobCh {
				readStart := time.Now()
				value, err := registers.Get(job.RegisterID, flagHeight)
				readDuration := time.Since(readStart)

				if err != nil {
					errorCount.Add(1)
					continue
				}

				registerCount.Add(1)
				totalBytes.Add(int64(len(value)))

				latenciesMu.Lock()
				latencies = append(latencies, readDuration)
				latenciesMu.Unlock()
			}
		}(i)
	}

	// Feed jobs
	go func() {
		for _, job := range jobs {
			jobCh <- job
		}
		close(jobCh)
	}()

	// Wait for completion
	wg.Wait()
	elapsed := time.Since(startTime)

	// Stop monitoring
	close(stopMonitor)
	<-monitorDone

	// Calculate latency statistics
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}

	n := len(latencies)
	if n > 0 {
		result.Results.AvgLatencyUs = float64(totalLatency.Microseconds()) / float64(n)
		result.Results.P50LatencyUs = float64(latencies[n*50/100].Microseconds())
		result.Results.P95LatencyUs = float64(latencies[n*95/100].Microseconds())
		result.Results.P99LatencyUs = float64(latencies[n*99/100].Microseconds())
		result.Results.MaxLatencyUs = float64(latencies[n-1].Microseconds())
	}

	// Final memory reading
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	finalMemMB := float64(m.Alloc) / 1024 / 1024
	if finalMemMB > peakMemoryMB {
		peakMemoryMB = finalMemMB
	}

	// Populate results
	result.Results.TotalTimeSec = elapsed.Seconds()
	result.Results.RegisterCount = registerCount.Load()
	result.Results.TotalBytes = totalBytes.Load()
	result.Results.ErrorCount = errorCount.Load()
	if elapsed.Seconds() > 0 {
		result.Results.RateRegistersPerSec = float64(result.Results.RegisterCount) / elapsed.Seconds()
		result.Results.RateMBPerSec = float64(result.Results.TotalBytes) / 1024 / 1024 / elapsed.Seconds()
	}
	result.Resources.PeakMemoryMB = peakMemoryMB

	// Log final stats
	logger.Info().
		Float64("total_time_sec", result.Results.TotalTimeSec).
		Int64("register_count", result.Results.RegisterCount).
		Int64("total_bytes", result.Results.TotalBytes).
		Float64("rate_reg_per_sec", result.Results.RateRegistersPerSec).
		Float64("avg_latency_us", result.Results.AvgLatencyUs).
		Float64("p50_latency_us", result.Results.P50LatencyUs).
		Float64("p95_latency_us", result.Results.P95LatencyUs).
		Float64("p99_latency_us", result.Results.P99LatencyUs).
		Float64("peak_memory_mb", result.Resources.PeakMemoryMB).
		Int64("errors", result.Results.ErrorCount).
		Msg("Read benchmark complete")

	// Write results to file
	if err := writeResults(flagOutputPath, result); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	logger.Info().Str("output", flagOutputPath).Msg("Results written")
	return nil
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
