package payload_analysis

import (
	"fmt"
	"os"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/storage"
)

var (
	cdpDBPath string
	cdpLimit  int
)

var cdpCmd = &cobra.Command{
	Use:   "chunk-data-pack",
	Short: "Analyze payload sizes in chunk data packs",
	Long: `Scan chunk data packs and analyze payload size distribution.
This helps estimate trie-storehouse consistency check overhead (proof generation).

Chunk data packs contain proofs for ALL touched registers (reads + writes),
unlike WAL which only contains writes.`,
	RunE: runCDPAnalysis,
}

func init() {
	cdpCmd.Flags().StringVar(&cdpDBPath, "db-path", "", "Path to chunk data pack pebble database")
	cdpCmd.Flags().IntVar(&cdpLimit, "limit", 0, "Limit number of chunk data packs to analyze (0 = no limit)")
	_ = cdpCmd.MarkFlagRequired("db-path")
}

func runCDPAnalysis(cmd *cobra.Command, args []string) error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Check DB path exists
	if _, err := os.Stat(cdpDBPath); os.IsNotExist(err) {
		return fmt.Errorf("database path does not exist: %s", cdpDBPath)
	}

	fmt.Printf("=== Chunk Data Pack Payload Analysis ===\n")
	fmt.Printf("Database Path: %s\n", cdpDBPath)
	if cdpLimit > 0 {
		fmt.Printf("Limit: %d chunk data packs\n", cdpLimit)
	}
	fmt.Println()

	// Open the pebble database in read-only mode
	db, err := pebble.Open(cdpDBPath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open pebble database: %w", err)
	}
	defer db.Close()

	stats := newPayloadStats()
	startTime := time.Now()

	cdpCount := 0
	proofCount := 0

	// Iterate over all chunk data packs using prefix scan
	// codeChunkDataPack = 100
	prefix := []byte{100}
	upperBound := []byte{101} // exclusive upper bound

	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: upperBound,
	})
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if cdpLimit > 0 && cdpCount >= cdpLimit {
			break
		}

		value, err := iter.ValueAndErr()
		if err != nil {
			logger.Warn().Err(err).Msg("failed to get value")
			continue
		}

		// Decode the StoredChunkDataPack
		var scdp storage.StoredChunkDataPack
		err = msgpack.Unmarshal(value, &scdp)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to decode chunk data pack")
			continue
		}

		cdpCount++

		// Decode the proof to get payloads
		if len(scdp.Proof) == 0 {
			continue
		}

		batchProof, err := ledger.DecodeTrieBatchProof(scdp.Proof)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to decode batch proof")
			continue
		}

		// Analyze each proof's payload
		for _, proof := range batchProof.Proofs {
			proofCount++
			size := 0
			if proof.Payload != nil && !proof.Payload.IsEmpty() {
				size = proof.Payload.Value().Size()
			}
			stats.addPayload(size)
		}

		// Progress output
		if cdpCount%1000 == 0 {
			fmt.Printf("Processed %d chunk data packs, %d proofs, %d payloads so far...\n",
				cdpCount, proofCount, stats.TotalPayloads)
		}
	}

	if err := iter.Error(); err != nil {
		logger.Warn().Err(err).Msg("iterator error")
	}

	elapsed := time.Since(startTime)

	// Print results
	printCDPResults(stats, elapsed, cdpCount, proofCount)

	return nil
}

func printCDPResults(stats *PayloadStats, elapsed time.Duration, cdpCount, proofCount int) {
	fmt.Println()
	fmt.Println("=== Chunk Data Pack Analysis Results ===")
	fmt.Println()

	// Summary
	fmt.Printf("Chunk Data Packs Processed: %d\n", cdpCount)
	fmt.Printf("Total Proofs: %d\n", proofCount)
	fmt.Printf("Total Payloads: %d\n", stats.TotalPayloads)
	fmt.Printf("Total Bytes: %s\n", formatBytes(stats.TotalBytes))
	fmt.Printf("Empty Payloads: %d (%.2f%%)\n", stats.EmptyPayloads,
		percentage(stats.EmptyPayloads, stats.TotalPayloads))
	fmt.Printf("Analysis Time: %v\n", elapsed)
	if stats.TotalPayloads > 0 {
		fmt.Printf("Avg Payload Size: %.1f bytes\n", float64(stats.TotalBytes)/float64(stats.TotalPayloads))
	}
	if cdpCount > 0 {
		fmt.Printf("Avg Payloads per Chunk: %.1f\n", float64(stats.TotalPayloads)/float64(cdpCount))
	}
	fmt.Println()

	// Size distribution
	fmt.Println("=== Payload Size Distribution ===")
	fmt.Println()
	fmt.Printf("%-15s %12s %10s %15s\n", "Size Range", "Count", "Percent", "Total Bytes")
	fmt.Println("-------------------------------------------------------")

	bucketOrder := []string{
		"0 (empty)", "1-7", "8-15", "16-23", "24-31", "32 (hash)",
		"33-64", "65-104", "105-240", "241-512", "513-1024", "1025-4096", "4097+",
	}

	var smallPayloads int64
	var smallBytes int64
	var singlePermPayloads int64
	var doublePermPayloads int64
	var triplePermPayloads int64

	for _, name := range bucketOrder {
		bucket := stats.SizeBuckets[name]
		pct := percentage(bucket.Count, stats.TotalPayloads)
		fmt.Printf("%-15s %12d %9.2f%% %15s\n",
			name, bucket.Count, pct, formatBytes(bucket.TotalBytes))

		if bucket.MaxSize < 32 {
			smallPayloads += bucket.Count
			smallBytes += bucket.TotalBytes
		}
		if bucket.MaxSize <= 104 {
			singlePermPayloads += bucket.Count
		} else if bucket.MaxSize <= 240 {
			doublePermPayloads += bucket.Count
		} else {
			triplePermPayloads += bucket.Count
		}
	}

	fmt.Println()

	// Small payload optimization impact
	fmt.Println("=== Small Payload Optimization Impact ===")
	fmt.Println()
	fmt.Printf("Payloads < 32 bytes: %d (%.2f%%)\n",
		smallPayloads, percentage(smallPayloads, stats.TotalPayloads))
	fmt.Printf("Actual bytes in small payloads: %s\n", formatBytes(smallBytes))
	fmt.Printf("Would store as hashes: %s\n", formatBytes(smallPayloads*32))
	fmt.Printf("Storage overhead avoided: %s\n", formatBytes(smallPayloads*32-smallBytes))
	fmt.Println()

	// Hash time estimation for trie-storehouse consistency check
	fmt.Println("=== Hash Time Estimation (Trie-Storehouse Consistency Check) ===")
	fmt.Println()
	fmt.Println("Based on benchmark: ~370ns for 0-104 bytes, ~720ns for 105-240, ~1100ns for 241+")
	fmt.Println("Note: This is for ALL touched registers (reads + writes) during proof generation")
	fmt.Println()

	singlePermTime := float64(singlePermPayloads) * 370
	doublePermTime := float64(doublePermPayloads) * 720
	triplePermTime := float64(triplePermPayloads) * 1100
	totalHashTime := singlePermTime + doublePermTime + triplePermTime

	fmt.Printf("Single permutation (0-104 bytes): %d payloads × 370ns = %.2f seconds\n",
		singlePermPayloads, singlePermTime/1e9)
	fmt.Printf("Double permutation (105-240 bytes): %d payloads × 720ns = %.2f seconds\n",
		doublePermPayloads, doublePermTime/1e9)
	fmt.Printf("Triple+ permutation (241+ bytes): %d payloads × 1100ns = %.2f seconds\n",
		triplePermPayloads, triplePermTime/1e9)

	if cdpCount > 0 {
		fmt.Printf("\nEstimated Total Hash Time per chunk: %.4f ms (avg)\n",
			(totalHashTime/1e6)/float64(cdpCount))

		// With small payload optimization
		optimizedPayloads := stats.TotalPayloads - smallPayloads
		optimizedTime := totalHashTime - float64(smallPayloads)*370
		fmt.Printf("\nWith small payload optimization (skip hashing < 32 bytes):\n")
		fmt.Printf("  Payloads to hash: %d (saved %d)\n", optimizedPayloads, smallPayloads)
		fmt.Printf("  Estimated Hash Time per chunk: %.4f ms (saved %.4f ms)\n",
			(optimizedTime/1e6)/float64(cdpCount), (float64(smallPayloads)*370/1e6)/float64(cdpCount))
	}
}
