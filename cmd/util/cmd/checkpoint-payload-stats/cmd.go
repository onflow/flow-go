package checkpoint_payload_stats

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	flagCheckpoint string
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-payload-stats",
	Short: "Analyze payload size distribution in a checkpoint file (streaming)",
	Long: `Analyze a checkpoint file and report:
- Number of interim nodes
- Number of leaf nodes (payloads)
- Payload value size distribution by buckets

Memory efficient: streams through checkpoint files without loading the trie into memory.`,
	Run: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "", "checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint")
}

// SizeBucket represents a payload size range
type SizeBucket struct {
	Name    string
	MinSize int
	MaxSize int
	Count   int64
	Bytes   int64
}

// Stats holds all statistics
type Stats struct {
	InterimNodes int64
	LeafNodes    int64
	TotalBytes   int64
	Buckets      []*SizeBucket
}

func newStats() *Stats {
	return &Stats{
		Buckets: []*SizeBucket{
			{Name: "0 (empty)", MinSize: 0, MaxSize: 0},
			{Name: "1-7", MinSize: 1, MaxSize: 7},
			{Name: "8-15", MinSize: 8, MaxSize: 15},
			{Name: "16-23", MinSize: 16, MaxSize: 23},
			{Name: "24-31", MinSize: 24, MaxSize: 31},
			{Name: "32 (hash)", MinSize: 32, MaxSize: 32},
			{Name: "33-64", MinSize: 33, MaxSize: 64},
			{Name: "65-104", MinSize: 65, MaxSize: 104},
			{Name: "105-240", MinSize: 105, MaxSize: 240},
			{Name: "241-512", MinSize: 241, MaxSize: 512},
			{Name: "513-1024", MinSize: 513, MaxSize: 1024},
			{Name: "1025-4096", MinSize: 1025, MaxSize: 4096},
			{Name: "4097+", MinSize: 4097, MaxSize: int(^uint(0) >> 1)},
		},
	}
}

func (s *Stats) addLeaf(valueSize int) {
	s.LeafNodes++
	s.TotalBytes += int64(valueSize)

	for _, bucket := range s.Buckets {
		if valueSize >= bucket.MinSize && valueSize <= bucket.MaxSize {
			bucket.Count++
			bucket.Bytes += int64(valueSize)
			return
		}
	}
}

func (s *Stats) addInterim() {
	s.InterimNodes++
}

// Checkpoint file constants (from wal package)
const (
	encMagicSize        = 2
	encVersionSize      = 2
	encSubtrieCountSize = 2
	crc32SumSize        = 4
	encNodeCountSize    = 8
	encTrieCountSize    = 2

	// Node encoding constants
	encNodeTypeSize  = 1
	encHeightSize    = 2
	encHashSize      = 32
	encPathSize      = 32
	encNodeIndexSize = 8

	// Payload encoding
	encPayloadLengthSize = 4
	encKeySizeLength     = 4
	encValueSizeLength   = 4

	// Node types
	leafNodeType   = 0
	interimNodeType = 1

	// Magic bytes
	magicCheckpointHeader  = uint16(0x2189)
	magicCheckpointSubtrie = uint16(0x2190)
	magicCheckpointToptrie = uint16(0x2191)

	subtrieCount = 16
)

func run(*cobra.Command, []string) {
	log.Info().Msgf("analyzing checkpoint %s (streaming mode)", flagCheckpoint)

	dir := filepath.Dir(flagCheckpoint)
	fileName := filepath.Base(flagCheckpoint)

	stats := newStats()

	// Process all subtrie files (16 files)
	for i := 0; i < subtrieCount; i++ {
		subtrieFile := filepath.Join(dir, fmt.Sprintf("%s.%03d", fileName, i))
		log.Info().Msgf("processing subtrie file %d/16: %s", i+1, subtrieFile)

		err := processSubtrieFile(subtrieFile, stats)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to process subtrie file %s", subtrieFile)
		}
	}

	// Process top trie file
	topTrieFile := filepath.Join(dir, fmt.Sprintf("%s.%03d", fileName, subtrieCount))
	log.Info().Msgf("processing top trie file: %s", topTrieFile)

	err := processTopTrieFile(topTrieFile, stats)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to process top trie file %s", topTrieFile)
	}

	printStats(stats)
}

func processSubtrieFile(filepath string, stats *Stats) error {
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	// Read footer to get node count
	nodeCount, err := readSubtrieFooter(f)
	if err != nil {
		return fmt.Errorf("cannot read footer: %w", err)
	}

	// Seek back to start and skip header
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("cannot seek to start: %w", err)
	}

	reader := bufio.NewReaderSize(f, 1024*1024) // 1MB buffer

	// Skip file header (magic + version)
	_, err = reader.Discard(encMagicSize + encVersionSize)
	if err != nil {
		return fmt.Errorf("cannot skip header: %w", err)
	}

	// Process nodes
	return processNodes(reader, nodeCount, stats)
}

func processTopTrieFile(filepath string, stats *Stats) error {
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	// Read footer to get node count
	nodeCount, err := readTopTrieFooter(f)
	if err != nil {
		return fmt.Errorf("cannot read footer: %w", err)
	}

	// Seek back to start
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("cannot seek to start: %w", err)
	}

	reader := bufio.NewReaderSize(f, 1024*1024) // 1MB buffer

	// Skip file header (magic + version)
	_, err = reader.Discard(encMagicSize + encVersionSize)
	if err != nil {
		return fmt.Errorf("cannot skip header: %w", err)
	}

	// Skip subtrie node count (8 bytes)
	_, err = reader.Discard(encNodeCountSize)
	if err != nil {
		return fmt.Errorf("cannot skip subtrie node count: %w", err)
	}

	// Process nodes
	return processNodes(reader, nodeCount, stats)
}

func processNodes(reader *bufio.Reader, nodeCount uint64, stats *Stats) error {
	scratch := make([]byte, 4096)

	for i := uint64(0); i < nodeCount; i++ {
		err := processOneNode(reader, scratch, stats)
		if err != nil {
			return fmt.Errorf("cannot process node %d: %w", i, err)
		}

		// Progress logging
		if (i+1)%10_000_000 == 0 {
			log.Info().Msgf("  processed %d/%d nodes", i+1, nodeCount)
		}
	}

	return nil
}

func processOneNode(reader *bufio.Reader, scratch []byte, stats *Stats) error {
	// Read fixed part: type (1) + height (2) + hash (32) = 35 bytes
	const fixedSize = encNodeTypeSize + encHeightSize + encHashSize
	_, err := io.ReadFull(reader, scratch[:fixedSize])
	if err != nil {
		return fmt.Errorf("cannot read node header: %w", err)
	}

	nodeType := scratch[0]

	if nodeType == interimNodeType {
		// Interim node: skip left/right child indices (8 + 8 = 16 bytes)
		_, err = reader.Discard(encNodeIndexSize * 2)
		if err != nil {
			return fmt.Errorf("cannot skip interim node children: %w", err)
		}
		stats.addInterim()
		return nil
	}

	if nodeType != leafNodeType {
		return fmt.Errorf("unknown node type: %d", nodeType)
	}

	// Leaf node: read path (32 bytes) - skip it
	_, err = reader.Discard(encPathSize)
	if err != nil {
		return fmt.Errorf("cannot skip path: %w", err)
	}

	// Read payload encoded size (4 bytes)
	_, err = io.ReadFull(reader, scratch[:encPayloadLengthSize])
	if err != nil {
		return fmt.Errorf("cannot read payload size: %w", err)
	}
	payloadEncodedSize := binary.BigEndian.Uint32(scratch[:encPayloadLengthSize])

	if payloadEncodedSize == 0 {
		// Empty payload
		stats.addLeaf(0)
		return nil
	}

	// Read key size (4 bytes)
	_, err = io.ReadFull(reader, scratch[:encKeySizeLength])
	if err != nil {
		return fmt.Errorf("cannot read key size: %w", err)
	}
	keySize := binary.BigEndian.Uint32(scratch[:encKeySizeLength])

	// Skip key
	_, err = reader.Discard(int(keySize))
	if err != nil {
		return fmt.Errorf("cannot skip key: %w", err)
	}

	// Read value size (4 bytes)
	_, err = io.ReadFull(reader, scratch[:encValueSizeLength])
	if err != nil {
		return fmt.Errorf("cannot read value size: %w", err)
	}
	valueSize := binary.BigEndian.Uint32(scratch[:encValueSizeLength])

	// Skip value
	_, err = reader.Discard(int(valueSize))
	if err != nil {
		return fmt.Errorf("cannot skip value: %w", err)
	}

	stats.addLeaf(int(valueSize))
	return nil
}

func readSubtrieFooter(f *os.File) (uint64, error) {
	// Footer: node count (8 bytes) + checksum (4 bytes)
	const footerOffset = encNodeCountSize + crc32SumSize
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("cannot seek to footer: %w", err)
	}

	buf := make([]byte, encNodeCountSize)
	_, err = io.ReadFull(f, buf)
	if err != nil {
		return 0, fmt.Errorf("cannot read node count: %w", err)
	}

	return binary.BigEndian.Uint64(buf), nil
}

func readTopTrieFooter(f *os.File) (uint64, error) {
	// Footer: node count (8 bytes) + trie count (2 bytes) + checksum (4 bytes)
	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("cannot seek to footer: %w", err)
	}

	buf := make([]byte, encNodeCountSize)
	_, err = io.ReadFull(f, buf)
	if err != nil {
		return 0, fmt.Errorf("cannot read node count: %w", err)
	}

	return binary.BigEndian.Uint64(buf), nil
}

func printStats(stats *Stats) {
	fmt.Println()
	fmt.Println("=== Checkpoint Payload Statistics ===")
	fmt.Println()

	totalNodes := stats.InterimNodes + stats.LeafNodes
	fmt.Printf("Total Nodes:    %d\n", totalNodes)
	fmt.Printf("Interim Nodes:  %d (%.2f%%)\n", stats.InterimNodes, pct(stats.InterimNodes, totalNodes))
	fmt.Printf("Leaf Nodes:     %d (%.2f%%)\n", stats.LeafNodes, pct(stats.LeafNodes, totalNodes))
	fmt.Printf("Total Payload:  %s\n", formatBytes(stats.TotalBytes))
	if stats.LeafNodes > 0 {
		fmt.Printf("Avg Payload:    %.1f bytes\n", float64(stats.TotalBytes)/float64(stats.LeafNodes))
	}
	fmt.Println()

	fmt.Println("=== Payload Size Distribution ===")
	fmt.Println()
	fmt.Printf("%-15s %15s %10s %15s\n", "Size Range", "Count", "Percent", "Total Bytes")
	fmt.Println("─────────────────────────────────────────────────────────────")

	var smallCount, smallBytes int64

	for _, bucket := range stats.Buckets {
		if bucket.Count > 0 {
			fmt.Printf("%-15s %15d %9.2f%% %15s\n",
				bucket.Name,
				bucket.Count,
				pct(bucket.Count, stats.LeafNodes),
				formatBytes(bucket.Bytes))
		}

		// Track small payloads (< 32 bytes)
		if bucket.MaxSize < 32 {
			smallCount += bucket.Count
			smallBytes += bucket.Bytes
		}
	}

	fmt.Println()
	fmt.Println("=== Small Payload Summary (<32 bytes) ===")
	fmt.Println()
	fmt.Printf("Count:          %d (%.2f%% of all payloads)\n", smallCount, pct(smallCount, stats.LeafNodes))
	fmt.Printf("Actual bytes:   %s\n", formatBytes(smallBytes))
	fmt.Printf("If stored as hash: %s\n", formatBytes(smallCount*32))
	fmt.Printf("Overhead if hashed: %s\n", formatBytes(smallCount*32-smallBytes))
}

func pct(part, total int64) float64 {
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
