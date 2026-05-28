package payload_analysis

import (
	"fmt"
	"os"
	"sort"
	"time"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	walDir     string
	walFromSeg int
	walToSeg   int
)

var walCmd = &cobra.Command{
	Use:   "wal",
	Short: "Analyze payload sizes in WAL files",
	Long: `Scan WAL files and analyze payload size distribution.
This helps estimate full checkpoint generation time.`,
	RunE: runWALAnalysis,
}

func init() {
	walCmd.Flags().StringVar(&walDir, "wal-dir", "", "Directory containing WAL files")
	walCmd.Flags().IntVar(&walFromSeg, "from-segment", -1, "Starting WAL segment number (-1 for first)")
	walCmd.Flags().IntVar(&walToSeg, "to-segment", -1, "Ending WAL segment number (-1 for last)")
	_ = walCmd.MarkFlagRequired("wal-dir")
}

// PayloadStats holds statistics about payload sizes
type PayloadStats struct {
	TotalPayloads   int64
	TotalBytes      int64
	EmptyPayloads   int64
	SizeBuckets     map[string]*BucketStats
	PerSegmentStats []SegmentStats
}

type BucketStats struct {
	Count      int64
	TotalBytes int64
	MinSize    int
	MaxSize    int
}

type SegmentStats struct {
	Segment       int
	Payloads      int64
	Bytes         int64
	SmallPayloads int64 // < 32 bytes
	Updates       int   // number of trie updates in segment
}

func newPayloadStats() *PayloadStats {
	return &PayloadStats{
		SizeBuckets: map[string]*BucketStats{
			"0 (empty)":   {MinSize: 0, MaxSize: 0},
			"1-7":         {MinSize: 1, MaxSize: 7},
			"8-15":        {MinSize: 8, MaxSize: 15},
			"16-23":       {MinSize: 16, MaxSize: 23},
			"24-31":       {MinSize: 24, MaxSize: 31},
			"32 (hash)":   {MinSize: 32, MaxSize: 32},
			"33-64":       {MinSize: 33, MaxSize: 64},
			"65-104":      {MinSize: 65, MaxSize: 104},
			"105-240":     {MinSize: 105, MaxSize: 240},
			"241-512":     {MinSize: 241, MaxSize: 512},
			"513-1024":    {MinSize: 513, MaxSize: 1024},
			"1025-4096":   {MinSize: 1025, MaxSize: 4096},
			"4097+":       {MinSize: 4097, MaxSize: int(^uint(0) >> 1)},
		},
		PerSegmentStats: make([]SegmentStats, 0),
	}
}

func (s *PayloadStats) addPayload(size int) {
	s.TotalPayloads++
	s.TotalBytes += int64(size)

	if size == 0 {
		s.EmptyPayloads++
	}

	for name, bucket := range s.SizeBuckets {
		if size >= bucket.MinSize && size <= bucket.MaxSize {
			bucket.Count++
			bucket.TotalBytes += int64(size)
			// Handle the "4097+" bucket which has no real max
			if name == "4097+" || size <= bucket.MaxSize {
				break
			}
		}
	}
}

func runWALAnalysis(cmd *cobra.Command, args []string) error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Check WAL directory exists
	if _, err := os.Stat(walDir); os.IsNotExist(err) {
		return fmt.Errorf("WAL directory does not exist: %s", walDir)
	}

	// Find WAL segment range
	segments, err := findWALSegments(walDir)
	if err != nil {
		return fmt.Errorf("failed to find WAL segments: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("no WAL segments found in %s", walDir)
	}

	// Determine segment range
	firstSeg := segments[0]
	lastSeg := segments[len(segments)-1]

	if walFromSeg >= 0 {
		firstSeg = walFromSeg
	}
	if walToSeg >= 0 {
		lastSeg = walToSeg
	}

	fmt.Printf("=== WAL Payload Analysis ===\n")
	fmt.Printf("WAL Directory: %s\n", walDir)
	fmt.Printf("Available Segments: %d to %d (%d total)\n", segments[0], segments[len(segments)-1], len(segments))
	fmt.Printf("Analyzing Segments: %d to %d\n", firstSeg, lastSeg)
	fmt.Println()

	stats := newPayloadStats()
	startTime := time.Now()

	// Create segment reader
	sr, err := prometheusWAL.NewSegmentsRangeReader(logger, prometheusWAL.SegmentRange{
		Dir:   walDir,
		First: firstSeg,
		Last:  lastSeg,
	})
	if err != nil {
		return fmt.Errorf("cannot create segment reader: %w", err)
	}
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)

	recordCount := 0
	currentSegment := firstSeg
	segStats := SegmentStats{Segment: currentSegment}

	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to decode WAL record")
			continue
		}

		recordCount++

		// Only process update operations
		if operation != wal.WALUpdate || update == nil {
			continue
		}

		segStats.Updates++

		// Analyze each payload in the update
		for _, payload := range update.Payloads {
			size := 0
			if payload != nil && !payload.IsEmpty() {
				size = payload.Value().Size()
			}

			stats.addPayload(size)
			segStats.Payloads++
			segStats.Bytes += int64(size)
			if size < 32 {
				segStats.SmallPayloads++
			}
		}

		// Progress output every 100k records
		if recordCount%100000 == 0 {
			fmt.Printf("Processed %d records, %d payloads so far...\n",
				recordCount, stats.TotalPayloads)
		}
	}

	if err := reader.Err(); err != nil {
		logger.Warn().Err(err).Msg("error reading WAL")
	}

	// Save final segment stats
	if segStats.Payloads > 0 {
		stats.PerSegmentStats = append(stats.PerSegmentStats, segStats)
	}

	elapsed := time.Since(startTime)

	// Print results
	printWALResults(stats, elapsed, recordCount)

	return nil
}

func findWALSegments(dir string) ([]int, error) {
	var segments []int

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		// WAL segments are 8-digit numbers (00000000, 00000001, etc.)
		if len(name) == 8 {
			var num int
			if _, err := fmt.Sscanf(name, "%08d", &num); err == nil {
				segments = append(segments, num)
			}
		}
	}

	sort.Ints(segments)
	return segments, nil
}

func printWALResults(stats *PayloadStats, elapsed time.Duration, recordCount int) {
	fmt.Println()
	fmt.Println("=== WAL Analysis Results ===")
	fmt.Println()

	// Summary
	fmt.Printf("WAL Records Processed: %d\n", recordCount)
	fmt.Printf("Total Payloads: %d\n", stats.TotalPayloads)
	fmt.Printf("Total Bytes: %s\n", formatBytes(stats.TotalBytes))
	fmt.Printf("Empty Payloads: %d (%.2f%%)\n", stats.EmptyPayloads,
		percentage(stats.EmptyPayloads, stats.TotalPayloads))
	fmt.Printf("Analysis Time: %v\n", elapsed)
	if stats.TotalPayloads > 0 {
		fmt.Printf("Avg Payload Size: %.1f bytes\n", float64(stats.TotalBytes)/float64(stats.TotalPayloads))
	}
	fmt.Println()

	// Size distribution
	fmt.Println("=== Payload Size Distribution ===")
	fmt.Println()
	fmt.Printf("%-15s %12s %10s %15s\n", "Size Range", "Count", "Percent", "Total Bytes")
	fmt.Println("-------------------------------------------------------")

	// Sort bucket names for consistent output
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

		// Categorize for hash time estimation
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

		// Print subtotal after 24-31 bucket (small payload optimization boundary)
		if name == "24-31" {
			fmt.Println("  ------------- -------- --------- ---------------")
			fmt.Printf("  %-13s %12d %9.2f%% %15s\n",
				"SUBTOTAL 0-31", smallPayloads, percentage(smallPayloads, stats.TotalPayloads), formatBytes(smallBytes))
			fmt.Println("-------------------------------------------------------")
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

	// Hash time estimation
	fmt.Println("=== Hash Time Estimation (Full Checkpoint Generation) ===")
	fmt.Println()
	fmt.Println("Based on benchmark: ~370ns for 0-104 bytes, ~720ns for 105-240, ~1100ns for 241+")
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
	fmt.Printf("\nEstimated Total Hash Time: %.2f seconds\n", totalHashTime/1e9)

	// With small payload optimization
	optimizedPayloads := stats.TotalPayloads - smallPayloads
	optimizedTime := totalHashTime - float64(smallPayloads)*370
	fmt.Printf("\nWith small payload optimization (skip hashing < 32 bytes):\n")
	fmt.Printf("  Payloads to hash: %d (saved %d)\n", optimizedPayloads, smallPayloads)
	fmt.Printf("  Estimated Hash Time: %.2f seconds (saved %.2f seconds)\n",
		optimizedTime/1e9, float64(smallPayloads)*370/1e9)
}

func percentage(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
