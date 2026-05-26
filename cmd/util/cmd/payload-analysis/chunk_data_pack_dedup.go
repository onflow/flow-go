package payload_analysis

import (
	"crypto/sha256"
	"encoding/hex"
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
	dedupDBPath string
	dedupLimit  int
)

var cdpDedupCmd = &cobra.Command{
	Use:   "chunk-data-pack-dedup",
	Short: "Analyze payload deduplication potential in chunk data packs",
	Long: `Scan chunk data packs and analyze how many payloads are duplicated.
This helps determine if storing payloads as separate key-value pairs (keyed by hash)
would provide meaningful storage savings through deduplication.

The analysis tracks:
- Total payloads vs unique payloads
- Storage savings from deduplication
- Distribution of duplicate frequencies
- Size distribution of duplicated payloads`,
	RunE: runCDPDedupAnalysis,
}

func init() {
	cdpDedupCmd.Flags().StringVar(&dedupDBPath, "db-path", "", "Path to chunk data pack pebble database")
	cdpDedupCmd.Flags().IntVar(&dedupLimit, "limit", 0, "Limit number of chunk data packs to analyze (0 = no limit)")
	_ = cdpDedupCmd.MarkFlagRequired("db-path")
}

// DedupStats tracks deduplication statistics
type DedupStats struct {
	TotalPayloads    int64
	TotalBytes       int64
	UniquePayloads   int64
	UniqueBytes      int64
	EmptyPayloads    int64
	PayloadHashCount map[[32]byte]*PayloadInfo // hash -> info about this payload
	LargestPayloads  []*LargePayloadInfo       // top N largest payloads
}

// PayloadInfo tracks information about a unique payload
type PayloadInfo struct {
	Size        int    // size of the payload value
	Occurrences int64  // number of times this payload appears
	Owner       string // register owner (hex encoded)
	Key         string // register key
}

// LargePayloadInfo tracks info about large payloads for reporting
type LargePayloadInfo struct {
	Size        int
	Occurrences int64
	Owner       string
	Key         string
	Hash        [32]byte
}

const maxLargestPayloads = 50 // Track top 50 largest payloads

func newDedupStats() *DedupStats {
	return &DedupStats{
		PayloadHashCount: make(map[[32]byte]*PayloadInfo),
		LargestPayloads:  make([]*LargePayloadInfo, 0, maxLargestPayloads),
	}
}

func (s *DedupStats) addPayload(value []byte, owner, key string) {
	s.TotalPayloads++
	size := len(value)
	s.TotalBytes += int64(size)

	if size == 0 {
		s.EmptyPayloads++
		// Empty payloads all hash to the same thing
	}

	// Compute hash of the payload value
	hash := sha256.Sum256(value)

	if info, exists := s.PayloadHashCount[hash]; exists {
		info.Occurrences++
	} else {
		s.PayloadHashCount[hash] = &PayloadInfo{
			Size:        size,
			Occurrences: 1,
			Owner:       owner,
			Key:         key,
		}
		s.UniquePayloads++
		s.UniqueBytes += int64(size)

		// Track largest payloads (only for new unique payloads)
		s.trackLargestPayload(size, owner, key, hash)
	}
}

func (s *DedupStats) trackLargestPayload(size int, owner, key string, hash [32]byte) {
	// Only track payloads >= 4KB
	if size < 4096 {
		return
	}

	info := &LargePayloadInfo{
		Size:  size,
		Owner: owner,
		Key:   key,
		Hash:  hash,
	}

	// Insert in sorted order (largest first)
	inserted := false
	for i, existing := range s.LargestPayloads {
		if size > existing.Size {
			// Insert at position i
			s.LargestPayloads = append(s.LargestPayloads[:i], append([]*LargePayloadInfo{info}, s.LargestPayloads[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted && len(s.LargestPayloads) < maxLargestPayloads {
		s.LargestPayloads = append(s.LargestPayloads, info)
	}

	// Trim to max size
	if len(s.LargestPayloads) > maxLargestPayloads {
		s.LargestPayloads = s.LargestPayloads[:maxLargestPayloads]
	}
}

// updateLargestPayloadOccurrences updates occurrence counts after scanning is complete
func (s *DedupStats) updateLargestPayloadOccurrences() {
	for _, large := range s.LargestPayloads {
		if info, exists := s.PayloadHashCount[large.Hash]; exists {
			large.Occurrences = info.Occurrences
		}
	}
}

func runCDPDedupAnalysis(cmd *cobra.Command, args []string) error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Check DB path exists
	if _, err := os.Stat(dedupDBPath); os.IsNotExist(err) {
		return fmt.Errorf("database path does not exist: %s", dedupDBPath)
	}

	fmt.Printf("=== Chunk Data Pack Deduplication Analysis ===\n")
	fmt.Printf("Database Path: %s\n", dedupDBPath)
	if dedupLimit > 0 {
		fmt.Printf("Limit: %d chunk data packs\n", dedupLimit)
	}
	fmt.Println()

	// Open the pebble database in read-only mode
	db, err := pebble.Open(dedupDBPath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open pebble database: %w", err)
	}
	defer db.Close()

	stats := newDedupStats()
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
		if dedupLimit > 0 && cdpCount >= dedupLimit {
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
			var valueBytes []byte
			var owner, key string
			if proof.Payload != nil && !proof.Payload.IsEmpty() {
				valueBytes = proof.Payload.Value()
				k, err := proof.Payload.Key()
				if err == nil {
					owner = hex.EncodeToString(k.KeyParts[0].Value)
					if len(k.KeyParts) > 1 {
						key = string(k.KeyParts[1].Value)
					}
				}
			}
			stats.addPayload(valueBytes, owner, key)
		}

		// Progress output
		if cdpCount%1000 == 0 {
			fmt.Printf("Processed %d chunk data packs, %d proofs, %d unique payloads so far...\n",
				cdpCount, proofCount, stats.UniquePayloads)
		}
	}

	if err := iter.Error(); err != nil {
		logger.Warn().Err(err).Msg("iterator error")
	}

	elapsed := time.Since(startTime)

	// Update occurrence counts for largest payloads
	stats.updateLargestPayloadOccurrences()

	// Print results
	printDedupResults(stats, elapsed, cdpCount, proofCount)

	return nil
}

func printDedupResults(stats *DedupStats, elapsed time.Duration, cdpCount, proofCount int) {
	fmt.Println()
	fmt.Println("=== Deduplication Analysis Results ===")
	fmt.Println()

	// Summary
	fmt.Printf("Chunk Data Packs Processed: %d\n", cdpCount)
	fmt.Printf("Total Proofs: %d\n", proofCount)
	fmt.Printf("Total Payloads: %d\n", stats.TotalPayloads)
	fmt.Printf("Total Bytes: %s\n", formatBytes(stats.TotalBytes))
	fmt.Printf("Empty Payloads: %d (%.2f%%)\n", stats.EmptyPayloads,
		percentage(stats.EmptyPayloads, stats.TotalPayloads))
	fmt.Printf("Analysis Time: %v\n", elapsed)
	fmt.Println()

	// Deduplication stats
	fmt.Println("=== Deduplication Statistics ===")
	fmt.Println()
	fmt.Printf("Unique Payloads: %d\n", stats.UniquePayloads)
	fmt.Printf("Unique Bytes: %s\n", formatBytes(stats.UniqueBytes))
	fmt.Printf("Duplicate Payloads: %d (%.2f%%)\n",
		stats.TotalPayloads-stats.UniquePayloads,
		percentage(stats.TotalPayloads-stats.UniquePayloads, stats.TotalPayloads))
	fmt.Printf("Deduplication Ratio: %.2fx\n",
		float64(stats.TotalPayloads)/float64(stats.UniquePayloads))
	fmt.Println()

	// Storage savings
	fmt.Println("=== Storage Savings Analysis ===")
	fmt.Println()
	bytesSaved := stats.TotalBytes - stats.UniqueBytes
	fmt.Printf("Bytes Saved by Deduplication: %s (%.2f%%)\n",
		formatBytes(bytesSaved),
		percentage(bytesSaved, stats.TotalBytes))

	// Overhead analysis: if we store payloads separately with 32-byte hash keys
	// Overhead per unique payload: 32 bytes (hash key)
	// Overhead per reference: 32 bytes (hash reference instead of inline value)
	hashOverhead := stats.UniquePayloads * 32                                  // keys for unique payloads
	referenceOverhead := (stats.TotalPayloads - stats.UniquePayloads) * 32     // references for duplicates
	newTotalBytes := stats.UniqueBytes + hashOverhead + referenceOverhead + 32 // +32 for each unique payload's hash key

	fmt.Printf("\nWith hash-based deduplication:\n")
	fmt.Printf("  Unique payload storage: %s\n", formatBytes(stats.UniqueBytes))
	fmt.Printf("  Hash key overhead (unique): %s (%d × 32 bytes)\n",
		formatBytes(hashOverhead), stats.UniquePayloads)
	fmt.Printf("  Hash reference overhead (duplicates): %s (%d × 32 bytes)\n",
		formatBytes(referenceOverhead), stats.TotalPayloads-stats.UniquePayloads)
	fmt.Printf("  New total: %s\n", formatBytes(newTotalBytes))
	fmt.Printf("  Net savings: %s (%.2f%%)\n",
		formatBytes(stats.TotalBytes-newTotalBytes),
		percentage(stats.TotalBytes-newTotalBytes, stats.TotalBytes))
	fmt.Println()

	// Frequency distribution
	fmt.Println("=== Duplicate Frequency Distribution ===")
	fmt.Println()
	fmt.Println("How many times each unique payload appears:")
	fmt.Println()

	// Count payloads by occurrence frequency
	freqCount := make(map[int64]int64) // occurrence count -> number of unique payloads with that count
	freqBytes := make(map[int64]int64) // occurrence count -> total bytes of those payloads

	for _, info := range stats.PayloadHashCount {
		freqCount[info.Occurrences]++
		freqBytes[info.Occurrences] += int64(info.Size)
	}

	// Sort frequencies
	var frequencies []int64
	for freq := range freqCount {
		frequencies = append(frequencies, freq)
	}
	sort.Slice(frequencies, func(i, j int) bool { return frequencies[i] < frequencies[j] })

	fmt.Printf("%-15s %15s %15s %15s\n", "Occurrences", "Unique Payloads", "Total Bytes", "Space Saved")
	fmt.Println("---------------------------------------------------------------")

	for _, freq := range frequencies {
		count := freqCount[freq]
		bytes := freqBytes[freq]
		// Space saved = (occurrences - 1) * bytes for each unique payload
		spaceSaved := (freq - 1) * bytes

		label := fmt.Sprintf("%d×", freq)
		if freq == 1 {
			label = "1× (unique)"
		}

		fmt.Printf("%-15s %15d %15s %15s\n",
			label, count, formatBytes(bytes), formatBytes(spaceSaved))

		// Only show first 20 frequencies in detail
		if len(frequencies) > 20 && freq == frequencies[19] {
			fmt.Printf("... and %d more frequency levels ...\n", len(frequencies)-20)
			break
		}
	}
	fmt.Println()

	// Size distribution of duplicated payloads
	fmt.Println("=== Size Distribution of Duplicated Payloads ===")
	fmt.Println()
	fmt.Println("Which size payloads are being duplicated most?")
	fmt.Println()

	type sizeBucketInfo struct {
		name        string
		minSize     int
		maxSize     int
		uniqueCount int64
		dupCount    int64
		uniqueBytes int64
		dupBytes    int64
	}

	sizeBuckets := []*sizeBucketInfo{
		{name: "0 (empty)", minSize: 0, maxSize: 0},
		{name: "1-31", minSize: 1, maxSize: 31},
		{name: "32 (hash)", minSize: 32, maxSize: 32},
		{name: "33-64", minSize: 33, maxSize: 64},
		{name: "65-128", minSize: 65, maxSize: 128},
		{name: "129-256", minSize: 129, maxSize: 256},
		{name: "257-512", minSize: 257, maxSize: 512},
		{name: "513-1024", minSize: 513, maxSize: 1024},
		{name: "1025-4096", minSize: 1025, maxSize: 4096},
		{name: "4097+", minSize: 4097, maxSize: int(^uint(0) >> 1)},
	}

	for _, info := range stats.PayloadHashCount {
		for _, bucket := range sizeBuckets {
			if info.Size >= bucket.minSize && info.Size <= bucket.maxSize {
				bucket.uniqueCount++
				bucket.uniqueBytes += int64(info.Size)
				if info.Occurrences > 1 {
					bucket.dupCount += info.Occurrences - 1
					bucket.dupBytes += int64(info.Size) * (info.Occurrences - 1)
				}
				break
			}
		}
	}

	fmt.Printf("%-15s %12s %12s %15s %15s\n",
		"Size Range", "Unique", "Duplicates", "Dup Bytes", "Dup Ratio")
	fmt.Println("-----------------------------------------------------------------------")

	for _, bucket := range sizeBuckets {
		totalInBucket := bucket.uniqueCount + bucket.dupCount
		if totalInBucket == 0 {
			continue
		}
		dupRatio := float64(bucket.dupCount) / float64(bucket.uniqueCount)
		fmt.Printf("%-15s %12d %12d %15s %14.2fx\n",
			bucket.name, bucket.uniqueCount, bucket.dupCount,
			formatBytes(bucket.dupBytes), dupRatio)
	}
	fmt.Println()

	// Largest payloads
	if len(stats.LargestPayloads) > 0 {
		fmt.Println("=== Largest Payloads (Top 50) ===")
		fmt.Println()
		fmt.Printf("%-10s %12s %-20s %s\n", "Size", "Occurrences", "Owner (hex)", "Key")
		fmt.Println("--------------------------------------------------------------------------------")

		for i, p := range stats.LargestPayloads {
			ownerDisplay := p.Owner
			if len(ownerDisplay) > 18 {
				ownerDisplay = ownerDisplay[:8] + "..." + ownerDisplay[len(ownerDisplay)-8:]
			}
			keyDisplay := p.Key
			if len(keyDisplay) > 50 {
				keyDisplay = keyDisplay[:47] + "..."
			}
			fmt.Printf("%-10s %12d %-20s %s\n",
				formatBytes(int64(p.Size)), p.Occurrences, ownerDisplay, keyDisplay)
			if i >= 49 {
				break
			}
		}
		fmt.Println()
	}

	// Recommendation
	fmt.Println("=== Recommendation ===")
	fmt.Println()
	netSavings := stats.TotalBytes - newTotalBytes
	savingsPercent := percentage(netSavings, stats.TotalBytes)

	if savingsPercent > 20 {
		fmt.Printf("Deduplication would save %.2f%% storage - RECOMMENDED\n", savingsPercent)
	} else if savingsPercent > 5 {
		fmt.Printf("Deduplication would save %.2f%% storage - MARGINAL BENEFIT\n", savingsPercent)
	} else if savingsPercent > 0 {
		fmt.Printf("Deduplication would save %.2f%% storage - MINIMAL BENEFIT\n", savingsPercent)
	} else {
		fmt.Printf("Deduplication would INCREASE storage by %.2f%% - NOT RECOMMENDED\n", -savingsPercent)
		fmt.Println("The overhead of hash keys exceeds the savings from deduplication.")
	}
}
