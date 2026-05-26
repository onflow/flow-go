package payload_analysis

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/storage"
)

var (
	benchDBPath    string
	benchLimit     int
	benchSamples   int
	benchWriteTest bool
)

var hashVsStorageCmd = &cobra.Command{
	Use:   "hash-vs-storage",
	Short: "Benchmark hash computation vs storage read/write times",
	Long: `Compare the time to compute a payload hash vs reading/writing from storage.
This helps determine whether to cache payload hashes or compute them on the fly.

The benchmark:
1. Collects payload samples of various sizes from chunk data packs
2. Measures hash computation time for each size bucket
3. Measures pebble read time (simulating hash lookup)
4. Optionally measures pebble write time
5. Compares results to recommend caching strategy`,
	RunE: runHashVsStorageBenchmark,
}

func init() {
	hashVsStorageCmd.Flags().StringVar(&benchDBPath, "db-path", "", "Path to chunk data pack pebble database")
	hashVsStorageCmd.Flags().IntVar(&benchLimit, "limit", 5000, "Limit number of chunk data packs to sample from")
	hashVsStorageCmd.Flags().IntVar(&benchSamples, "samples", 1000, "Number of samples per size bucket for benchmarking")
	hashVsStorageCmd.Flags().BoolVar(&benchWriteTest, "write-test", false, "Also benchmark write times (creates temp DB)")
	_ = hashVsStorageCmd.MarkFlagRequired("db-path")
}

type PayloadSample struct {
	Value []byte
	Size  int
}

type BenchResult struct {
	SizeBucket     string
	MinSize        int
	MaxSize        int
	SampleCount    int
	AvgSize        float64
	HashTimeNs     float64 // nanoseconds per hash
	ReadTimeNs     float64 // nanoseconds per read
	WriteTimeNs    float64 // nanoseconds per write (if tested)
	HashPerRead    float64 // ratio: how many hashes can we compute in the time of one read
}

func runHashVsStorageBenchmark(cmd *cobra.Command, args []string) error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	if _, err := os.Stat(benchDBPath); os.IsNotExist(err) {
		return fmt.Errorf("database path does not exist: %s", benchDBPath)
	}

	fmt.Printf("=== Hash vs Storage Benchmark ===\n")
	fmt.Printf("Database Path: %s\n", benchDBPath)
	fmt.Printf("Chunk Data Pack Limit: %d\n", benchLimit)
	fmt.Printf("Samples per bucket: %d\n", benchSamples)
	fmt.Println()

	// Open the pebble database
	db, err := pebble.Open(benchDBPath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open pebble database: %w", err)
	}
	defer db.Close()

	// Collect payload samples by size bucket
	fmt.Println("Phase 1: Collecting payload samples...")
	samples := collectPayloadSamples(db, logger, benchLimit, benchSamples)

	// Benchmark hash computation
	fmt.Println("\nPhase 2: Benchmarking hash computation...")
	results := benchmarkHashing(samples)

	// Benchmark storage reads
	fmt.Println("\nPhase 3: Benchmarking storage reads...")
	benchmarkStorageReads(db, samples, results)

	// Optionally benchmark writes
	if benchWriteTest {
		fmt.Println("\nPhase 4: Benchmarking storage writes...")
		benchmarkStorageWrites(samples, results)
	}

	// Print results
	printBenchResults(results)

	return nil
}

func collectPayloadSamples(db *pebble.DB, logger zerolog.Logger, limit, samplesPerBucket int) map[string][]*PayloadSample {
	buckets := map[string]*struct {
		minSize int
		maxSize int
		samples []*PayloadSample
	}{
		"0-31":      {0, 31, make([]*PayloadSample, 0, samplesPerBucket)},
		"32-64":     {32, 64, make([]*PayloadSample, 0, samplesPerBucket)},
		"65-128":    {65, 128, make([]*PayloadSample, 0, samplesPerBucket)},
		"129-256":   {129, 256, make([]*PayloadSample, 0, samplesPerBucket)},
		"257-512":   {257, 512, make([]*PayloadSample, 0, samplesPerBucket)},
		"513-1024":  {513, 1024, make([]*PayloadSample, 0, samplesPerBucket)},
		"1025-4096": {1025, 4096, make([]*PayloadSample, 0, samplesPerBucket)},
		"4097-16KB": {4097, 16384, make([]*PayloadSample, 0, samplesPerBucket)},
		"16KB-64KB": {16385, 65536, make([]*PayloadSample, 0, samplesPerBucket)},
		"64KB+":     {65537, int(^uint(0) >> 1), make([]*PayloadSample, 0, samplesPerBucket)},
	}

	prefix := []byte{100}
	upperBound := []byte{101}

	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: upperBound,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to create iterator")
		return nil
	}
	defer iter.Close()

	cdpCount := 0
	allFull := false

	for iter.First(); iter.Valid() && !allFull; iter.Next() {
		if limit > 0 && cdpCount >= limit {
			break
		}

		value, err := iter.ValueAndErr()
		if err != nil {
			continue
		}

		var scdp storage.StoredChunkDataPack
		if err := msgpack.Unmarshal(value, &scdp); err != nil {
			continue
		}

		cdpCount++

		if len(scdp.Proof) == 0 {
			continue
		}

		batchProof, err := ledger.DecodeTrieBatchProof(scdp.Proof)
		if err != nil {
			continue
		}

		for _, proof := range batchProof.Proofs {
			if proof.Payload == nil || proof.Payload.IsEmpty() {
				continue
			}

			valueBytes := proof.Payload.Value()
			size := len(valueBytes)

			for _, bucket := range buckets {
				if size >= bucket.minSize && size <= bucket.maxSize {
					if len(bucket.samples) < samplesPerBucket {
						// Make a copy of the value
						valueCopy := make([]byte, len(valueBytes))
						copy(valueCopy, valueBytes)
						bucket.samples = append(bucket.samples, &PayloadSample{
							Value: valueCopy,
							Size:  size,
						})
					}
					break
				}
			}
		}

		// Check if all buckets are full
		allFull = true
		for _, bucket := range buckets {
			if len(bucket.samples) < samplesPerBucket {
				allFull = false
				break
			}
		}

		if cdpCount%1000 == 0 {
			collected := 0
			for _, b := range buckets {
				collected += len(b.samples)
			}
			fmt.Printf("  Processed %d CDPs, collected %d samples...\n", cdpCount, collected)
		}
	}

	// Convert to result map
	result := make(map[string][]*PayloadSample)
	for name, bucket := range buckets {
		if len(bucket.samples) > 0 {
			result[name] = bucket.samples
		}
	}

	return result
}

func benchmarkHashing(samples map[string][]*PayloadSample) map[string]*BenchResult {
	results := make(map[string]*BenchResult)

	bucketOrder := []string{"0-31", "32-64", "65-128", "129-256", "257-512", "513-1024", "1025-4096", "4097-16KB", "16KB-64KB", "64KB+"}
	bucketRanges := map[string][2]int{
		"0-31":      {0, 31},
		"32-64":     {32, 64},
		"65-128":    {65, 128},
		"129-256":   {129, 256},
		"257-512":   {257, 512},
		"513-1024":  {513, 1024},
		"1025-4096": {1025, 4096},
		"4097-16KB": {4097, 16384},
		"16KB-64KB": {16385, 65536},
		"64KB+":     {65537, int(^uint(0) >> 1)},
	}

	for _, name := range bucketOrder {
		bucketSamples, exists := samples[name]
		if !exists || len(bucketSamples) == 0 {
			continue
		}

		ranges := bucketRanges[name]

		// Calculate average size
		totalSize := 0
		for _, s := range bucketSamples {
			totalSize += s.Size
		}
		avgSize := float64(totalSize) / float64(len(bucketSamples))

		// Warm up
		for i := 0; i < 100 && i < len(bucketSamples); i++ {
			sha256.Sum256(bucketSamples[i].Value)
		}

		// Benchmark hashing
		iterations := len(bucketSamples) * 10 // Multiple passes for accuracy
		start := time.Now()
		for i := 0; i < iterations; i++ {
			sample := bucketSamples[i%len(bucketSamples)]
			sha256.Sum256(sample.Value)
		}
		elapsed := time.Since(start)

		hashTimeNs := float64(elapsed.Nanoseconds()) / float64(iterations)

		results[name] = &BenchResult{
			SizeBucket:  name,
			MinSize:     ranges[0],
			MaxSize:     ranges[1],
			SampleCount: len(bucketSamples),
			AvgSize:     avgSize,
			HashTimeNs:  hashTimeNs,
		}

		fmt.Printf("  %s: %.0f ns/hash (avg size: %.0f bytes, %d samples)\n",
			name, hashTimeNs, avgSize, len(bucketSamples))
	}

	return results
}

func benchmarkStorageReads(db *pebble.DB, samples map[string][]*PayloadSample, results map[string]*BenchResult) {
	// We'll read existing keys from the database to measure read latency
	// First, collect some real keys

	prefix := []byte{100}
	upperBound := []byte{101}

	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: upperBound,
	})
	if err != nil {
		fmt.Printf("  Error creating iterator: %v\n", err)
		return
	}
	defer iter.Close()

	// Collect keys for reading
	var keys [][]byte
	for iter.First(); iter.Valid() && len(keys) < 10000; iter.Next() {
		keyCopy := make([]byte, len(iter.Key()))
		copy(keyCopy, iter.Key())
		keys = append(keys, keyCopy)
	}

	if len(keys) == 0 {
		fmt.Println("  No keys found for read benchmark")
		return
	}

	// Warm up reads
	for i := 0; i < 100 && i < len(keys); i++ {
		db.Get(keys[i])
	}

	// Benchmark reads
	iterations := 10000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		key := keys[i%len(keys)]
		val, closer, err := db.Get(key)
		if err == nil && closer != nil {
			_ = val // Use the value
			closer.Close()
		}
	}
	elapsed := time.Since(start)

	readTimeNs := float64(elapsed.Nanoseconds()) / float64(iterations)
	fmt.Printf("  Average read time: %.0f ns/read (%d iterations)\n", readTimeNs, iterations)

	// Update all results with read time
	for _, result := range results {
		result.ReadTimeNs = readTimeNs
		result.HashPerRead = readTimeNs / result.HashTimeNs
	}
}

func benchmarkStorageWrites(samples map[string][]*PayloadSample, results map[string]*BenchResult) {
	// Create a temporary database for write testing
	tmpDir, err := os.MkdirTemp("", "pebble-write-bench-*")
	if err != nil {
		fmt.Printf("  Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	db, err := pebble.Open(tmpDir, &pebble.Options{})
	if err != nil {
		fmt.Printf("  Error creating temp database: %v\n", err)
		return
	}
	defer db.Close()

	// Benchmark writes for each size bucket
	for name, bucketSamples := range samples {
		if len(bucketSamples) == 0 {
			continue
		}

		result, exists := results[name]
		if !exists {
			continue
		}

		// Warm up
		for i := 0; i < 100 && i < len(bucketSamples); i++ {
			key := sha256.Sum256(bucketSamples[i].Value)
			db.Set(key[:], bucketSamples[i].Value, pebble.Sync)
		}

		// Benchmark writes
		iterations := len(bucketSamples)
		start := time.Now()
		for i := 0; i < iterations; i++ {
			sample := bucketSamples[i]
			key := sha256.Sum256(sample.Value)
			db.Set(key[:], sample.Value, pebble.NoSync) // NoSync for speed
		}
		elapsed := time.Since(start)

		result.WriteTimeNs = float64(elapsed.Nanoseconds()) / float64(iterations)
		fmt.Printf("  %s: %.0f ns/write\n", name, result.WriteTimeNs)
	}
}

func printBenchResults(results map[string]*BenchResult) {
	fmt.Println()
	fmt.Println("=== Benchmark Results ===")
	fmt.Println()

	bucketOrder := []string{"0-31", "32-64", "65-128", "129-256", "257-512", "513-1024", "1025-4096", "4097-16KB", "16KB-64KB", "64KB+"}

	// Sort results by bucket order
	var sortedResults []*BenchResult
	for _, name := range bucketOrder {
		if r, exists := results[name]; exists {
			sortedResults = append(sortedResults, r)
		}
	}

	// Also add any results not in the predefined order
	for name, r := range results {
		found := false
		for _, orderedName := range bucketOrder {
			if name == orderedName {
				found = true
				break
			}
		}
		if !found {
			sortedResults = append(sortedResults, r)
		}
	}

	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].MinSize < sortedResults[j].MinSize
	})

	fmt.Printf("%-12s %10s %12s %12s %12s %s\n",
		"Size Bucket", "Avg Size", "Hash (ns)", "Read (ns)", "Hash/Read", "Recommendation")
	fmt.Println("-------------------------------------------------------------------------------------------")

	for _, r := range sortedResults {
		recommendation := "COMPUTE"
		if r.HashPerRead < 1 {
			recommendation = "CACHE (hash slower than read)"
		} else if r.HashPerRead < 2 {
			recommendation = "BORDERLINE"
		} else {
			recommendation = fmt.Sprintf("COMPUTE (%.1fx faster)", r.HashPerRead)
		}

		fmt.Printf("%-12s %10.0f %12.0f %12.0f %12.1fx %s\n",
			r.SizeBucket, r.AvgSize, r.HashTimeNs, r.ReadTimeNs, r.HashPerRead, recommendation)
	}

	fmt.Println()
	fmt.Println("=== Analysis ===")
	fmt.Println()

	// Find crossover point
	var crossoverSize float64 = -1
	for _, r := range sortedResults {
		if r.HashPerRead < 1 {
			crossoverSize = r.AvgSize
			break
		}
	}

	if crossoverSize > 0 {
		fmt.Printf("Crossover point: ~%.0f bytes\n", crossoverSize)
		fmt.Printf("  - Below this size: Computing hash is faster than reading from storage\n")
		fmt.Printf("  - Above this size: Reading cached hash from storage is faster\n")
	} else {
		fmt.Println("No crossover found - computing hash is always faster than storage read")
		fmt.Println("Recommendation: Always compute hashes, don't cache them")
	}

	fmt.Println()
	fmt.Println("=== Interpretation ===")
	fmt.Println()
	fmt.Println("Hash/Read ratio meaning:")
	fmt.Println("  > 1.0: Computing hash is FASTER than reading from storage")
	fmt.Println("  < 1.0: Reading from storage is FASTER than computing hash")
	fmt.Println()
	fmt.Println("For payload hash caching decision:")
	fmt.Println("  - If Hash/Read > 2: Definitely compute on the fly")
	fmt.Println("  - If Hash/Read 1-2: Borderline, consider other factors")
	fmt.Println("  - If Hash/Read < 1: Consider caching hashes in storage")
}
