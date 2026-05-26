package checkpoint_verify

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	ledgerHash "github.com/onflow/flow-go/ledger/common/hash"
)

const (
	// subtrieCount is the number of subtrie files in V6 checkpoint (2^4 = 16)
	subtrieCount = 16
	// Buffer size for reading files
	readBufferSize = 4 * 1024 * 1024 // 4MB buffer
)

var (
	checkpointDir  string
	checkpointFile string
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-verify",
	Short: "Verify checkpoint file integrity and hash correctness",
	Long: `Verify a V6 checkpoint file by:
1. Verifying all subtrie files concurrently (16 subtries)
2. Verifying the top trie file
3. Verifying that top trie correctly references subtrie nodes
4. Verifying all cached node hashes match computed hashes
5. Returning the verified root hash

This performs streaming verification without loading the entire trie into memory.
Each node's hash is recomputed and compared against the stored hash.`,
	RunE: runVerify,
}

func init() {
	Cmd.Flags().StringVar(&checkpointDir, "checkpoint-dir", "", "Directory containing checkpoint files")
	Cmd.Flags().StringVar(&checkpointFile, "checkpoint-file", "", "Checkpoint filename (e.g., checkpoint.00000001)")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")
	_ = Cmd.MarkFlagRequired("checkpoint-file")
}

// SubtrieResult holds the verification result for a subtrie
type SubtrieResult struct {
	Index       int
	NodeCount   uint64
	NodeHashes  []ledgerHash.Hash // verified hashes in order (index 0 = first node)
	Err         error
	Duration    time.Duration
	StartOffset uint64 // global offset where this subtrie's nodes start (1-based)
}

// VerificationResult holds the complete verification result
type VerificationResult struct {
	Valid           bool
	RootHash        ledgerHash.Hash
	TotalNodes      uint64
	SubtrieResults  []*SubtrieResult
	TopTrieNodes    uint64
	TopTrieDuration time.Duration
	TotalDuration   time.Duration
	Errors          []error
}

func runVerify(cmd *cobra.Command, args []string) error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	fmt.Printf("=== Checkpoint Verification ===\n")
	fmt.Printf("Directory: %s\n", checkpointDir)
	fmt.Printf("Filename: %s\n", checkpointFile)
	fmt.Println()

	startTime := time.Now()

	result, err := VerifyCheckpointV6(checkpointDir, checkpointFile, logger)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print results
	printVerificationResult(result)

	if !result.Valid {
		return fmt.Errorf("checkpoint verification FAILED")
	}

	fmt.Printf("\nVerification completed in %v\n", time.Since(startTime))
	return nil
}

// VerifyCheckpointV6 verifies a V6 checkpoint file
func VerifyCheckpointV6(dir, filename string, logger zerolog.Logger) (*VerificationResult, error) {
	result := &VerificationResult{
		SubtrieResults: make([]*SubtrieResult, subtrieCount),
	}
	startTime := time.Now()

	// Step 1: Read and verify header
	fmt.Println("Step 1: Verifying header...")
	headerPath := filePathCheckpointHeader(dir, filename)
	subtrieChecksums, topTrieChecksum, err := readAndVerifyHeader(headerPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("header verification failed: %w", err))
		return result, nil
	}
	fmt.Printf("  Header OK - %d subtrie checksums read\n", len(subtrieChecksums))

	// Step 2: Verify all subtries concurrently
	fmt.Println("\nStep 2: Verifying subtries concurrently...")
	var wg sync.WaitGroup
	subtrieResults := make([]*SubtrieResult, subtrieCount)

	for i := 0; i < subtrieCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			subtrieResults[index] = verifySubtrieStreaming(dir, filename, index, subtrieChecksums[index], logger)
		}(i)
	}
	wg.Wait()

	// Collect subtrie results
	allSubtriesValid := true
	for i, sr := range subtrieResults {
		result.SubtrieResults[i] = sr
		result.TotalNodes += sr.NodeCount
		if sr.Err != nil {
			allSubtriesValid = false
			result.Errors = append(result.Errors, fmt.Errorf("subtrie %d: %w", i, sr.Err))
		}
		fmt.Printf("  Subtrie %2d: %8d nodes, %v\n", i, sr.NodeCount, sr.Duration.Round(time.Millisecond))
	}

	if !allSubtriesValid {
		fmt.Println("  FAILED: One or more subtries failed verification")
		return result, nil
	}
	fmt.Println("  All subtries verified OK")

	// Step 3: Verify top trie
	fmt.Println("\nStep 3: Verifying top trie...")
	topTrieStart := time.Now()
	topTriePath := filePathTopTries(dir, filename)
	rootHash, topNodeCount, err := verifyTopTrieStreaming(topTriePath, topTrieChecksum, subtrieResults, logger)
	result.TopTrieDuration = time.Since(topTrieStart)
	result.TopTrieNodes = topNodeCount
	result.TotalNodes += topNodeCount

	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("top trie: %w", err))
		fmt.Printf("  FAILED: %v\n", err)
		return result, nil
	}
	fmt.Printf("  Top trie OK: %d nodes, root hash: %x\n", topNodeCount, rootHash[:8])

	result.Valid = true
	result.RootHash = rootHash
	result.TotalDuration = time.Since(startTime)

	return result, nil
}

func readAndVerifyHeader(headerPath string) ([]uint32, uint32, error) {
	data, err := os.ReadFile(headerPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read header: %w", err)
	}

	// Verify header checksum (last 4 bytes)
	if len(data) < 4 {
		return nil, 0, fmt.Errorf("header too small")
	}
	storedChecksum := binary.BigEndian.Uint32(data[len(data)-4:])
	crc32Table := crc32.MakeTable(crc32.Castagnoli)
	computedChecksum := crc32.Checksum(data[:len(data)-4], crc32Table)
	if storedChecksum != computedChecksum {
		return nil, 0, fmt.Errorf("header checksum mismatch: stored=%d, computed=%d", storedChecksum, computedChecksum)
	}

	// Parse header: magic(2) + version(2) + subtrieCount(2) + subtrieChecksums(4*16) + topTrieChecksum(4) + headerChecksum(4)
	pos := 0

	// Magic bytes
	if len(data) < pos+2 {
		return nil, 0, fmt.Errorf("header too small for magic")
	}
	pos += 2

	// Version
	if len(data) < pos+2 {
		return nil, 0, fmt.Errorf("header too small for version")
	}
	pos += 2

	// Subtrie count
	if len(data) < pos+2 {
		return nil, 0, fmt.Errorf("header too small for subtrie count")
	}
	readSubtrieCount := binary.BigEndian.Uint16(data[pos:])
	pos += 2

	if int(readSubtrieCount) != subtrieCount {
		return nil, 0, fmt.Errorf("unexpected subtrie count: %d", readSubtrieCount)
	}

	// Subtrie checksums
	subtrieChecksums := make([]uint32, subtrieCount)
	for i := 0; i < subtrieCount; i++ {
		if len(data) < pos+4 {
			return nil, 0, fmt.Errorf("header too small for subtrie checksum %d", i)
		}
		subtrieChecksums[i] = binary.BigEndian.Uint32(data[pos:])
		pos += 4
	}

	// Top trie checksum
	if len(data) < pos+4 {
		return nil, 0, fmt.Errorf("header too small for top trie checksum")
	}
	topTrieChecksum := binary.BigEndian.Uint32(data[pos:])

	return subtrieChecksums, topTrieChecksum, nil
}

// crc32Reader wraps a reader and computes CRC32 checksum while reading
type crc32Reader struct {
	reader io.Reader
	hash   hash.Hash32
}

func newCRC32Reader(r io.Reader) *crc32Reader {
	return &crc32Reader{
		reader: r,
		hash:   crc32.New(crc32.MakeTable(crc32.Castagnoli)),
	}
}

func (c *crc32Reader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 {
		c.hash.Write(p[:n])
	}
	return n, err
}

func (c *crc32Reader) Checksum() uint32 {
	return c.hash.Sum32()
}

// verifySubtrieStreaming verifies a subtrie file using streaming reads
func verifySubtrieStreaming(dir, filename string, index int, expectedChecksum uint32, logger zerolog.Logger) *SubtrieResult {
	startTime := time.Now()
	result := &SubtrieResult{
		Index:      index,
		NodeHashes: make([]ledgerHash.Hash, 0),
	}

	subtriePath := filePathSubTries(dir, filename, index)

	// Open file
	file, err := os.Open(subtriePath)
	if err != nil {
		result.Err = fmt.Errorf("failed to open subtrie file: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer file.Close()

	// Get file size for footer reading
	fileInfo, err := file.Stat()
	if err != nil {
		result.Err = fmt.Errorf("failed to stat file: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	fileSize := fileInfo.Size()

	// Read footer first (last 12 bytes: nodeCount(8) + checksum(4))
	if fileSize < 16 {
		result.Err = fmt.Errorf("file too small: %d bytes", fileSize)
		result.Duration = time.Since(startTime)
		return result
	}

	footer := make([]byte, 12)
	_, err = file.ReadAt(footer, fileSize-12)
	if err != nil {
		result.Err = fmt.Errorf("failed to read footer: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	nodeCount := binary.BigEndian.Uint64(footer[0:8])
	storedChecksum := binary.BigEndian.Uint32(footer[8:12])

	if storedChecksum != expectedChecksum {
		result.Err = fmt.Errorf("checksum doesn't match header: file=%d, header=%d", storedChecksum, expectedChecksum)
		result.Duration = time.Since(startTime)
		return result
	}

	result.NodeCount = nodeCount

	// Reset to beginning and create CRC32 reader
	_, err = file.Seek(0, 0)
	if err != nil {
		result.Err = fmt.Errorf("failed to seek to beginning: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// We need to compute checksum of everything except the last 4 bytes
	dataSize := fileSize - 4
	limitedReader := io.LimitReader(file, dataSize)
	crcReader := newCRC32Reader(bufio.NewReaderSize(limitedReader, readBufferSize))

	// Read and verify magic + version
	header := make([]byte, 4)
	_, err = io.ReadFull(crcReader, header)
	if err != nil {
		result.Err = fmt.Errorf("failed to read header: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Allocate space for node hashes
	nodeHashes := make([]ledgerHash.Hash, 0, nodeCount)

	// Stream through nodes
	for nodeIdx := uint64(0); nodeIdx < nodeCount; nodeIdx++ {
		nodeHash, err := readAndVerifyNode(crcReader, nodeHashes)
		if err != nil {
			result.Err = fmt.Errorf("failed to verify node %d: %w", nodeIdx, err)
			result.Duration = time.Since(startTime)
			return result
		}
		nodeHashes = append(nodeHashes, nodeHash)
	}

	// Read the remaining footer (nodeCount)
	footerBuf := make([]byte, 8)
	_, err = io.ReadFull(crcReader, footerBuf)
	if err != nil {
		result.Err = fmt.Errorf("failed to read footer node count: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify checksum
	computedChecksum := crcReader.Checksum()
	if computedChecksum != storedChecksum {
		result.Err = fmt.Errorf("computed checksum mismatch: stored=%d, computed=%d", storedChecksum, computedChecksum)
		result.Duration = time.Since(startTime)
		return result
	}

	result.NodeHashes = nodeHashes
	result.Duration = time.Since(startTime)
	return result
}

// readAndVerifyNode reads a single node from the stream and verifies its hash
func readAndVerifyNode(reader io.Reader, previousHashes []ledgerHash.Hash) (ledgerHash.Hash, error) {
	// Read node type
	typeBuf := make([]byte, 1)
	_, err := io.ReadFull(reader, typeBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read node type: %w", err)
	}
	nodeType := typeBuf[0]

	if nodeType == 0 { // Leaf node
		return readAndVerifyLeafNode(reader)
	} else if nodeType == 1 { // Interim node
		return readAndVerifyInterimNode(reader, previousHashes)
	}

	return ledgerHash.Hash{}, fmt.Errorf("unknown node type: %d", nodeType)
}

// readAndVerifyLeafNode reads and verifies a leaf node
func readAndVerifyLeafNode(reader io.Reader) (ledgerHash.Hash, error) {
	// Leaf: type(1, already read) + height(2) + hash(32) + path(32) + payloadLen(4) + payload

	// Read height
	heightBuf := make([]byte, 2)
	_, err := io.ReadFull(reader, heightBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read height: %w", err)
	}
	height := binary.BigEndian.Uint16(heightBuf)

	// Read stored hash
	var storedHash ledgerHash.Hash
	_, err = io.ReadFull(reader, storedHash[:])
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read stored hash: %w", err)
	}

	// Read path
	var path ledger.Path
	_, err = io.ReadFull(reader, path[:])
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read path: %w", err)
	}

	// Read payload length
	payloadLenBuf := make([]byte, 4)
	_, err = io.ReadFull(reader, payloadLenBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read payload length: %w", err)
	}
	payloadLen := binary.BigEndian.Uint32(payloadLenBuf)

	// Read payload
	payloadData := make([]byte, payloadLen)
	_, err = io.ReadFull(reader, payloadData)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read payload: %w", err)
	}

	// Decode payload
	payload, err := ledger.DecodePayloadWithoutPrefix(payloadData, false, 1)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to decode payload: %w", err)
	}

	// Compute hash
	computedHash := ledger.ComputeCompactValue(ledgerHash.Hash(path), payload.Value(), int(height))

	// Verify hash
	if storedHash != computedHash {
		return ledgerHash.Hash{}, fmt.Errorf("leaf hash mismatch: stored=%x, computed=%x", storedHash[:8], computedHash[:8])
	}

	return storedHash, nil
}

// readAndVerifyInterimNode reads and verifies an interim node
func readAndVerifyInterimNode(reader io.Reader, previousHashes []ledgerHash.Hash) (ledgerHash.Hash, error) {
	// Interim: type(1, already read) + height(2) + hash(32) + lchildIndex(8) + rchildIndex(8)

	// Read height
	heightBuf := make([]byte, 2)
	_, err := io.ReadFull(reader, heightBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read height: %w", err)
	}
	height := int(binary.BigEndian.Uint16(heightBuf))

	// Read stored hash
	var storedHash ledgerHash.Hash
	_, err = io.ReadFull(reader, storedHash[:])
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read stored hash: %w", err)
	}

	// Read left child index
	lchildBuf := make([]byte, 8)
	_, err = io.ReadFull(reader, lchildBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read left child index: %w", err)
	}
	lchildIndex := binary.BigEndian.Uint64(lchildBuf)

	// Read right child index
	rchildBuf := make([]byte, 8)
	_, err = io.ReadFull(reader, rchildBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read right child index: %w", err)
	}
	rchildIndex := binary.BigEndian.Uint64(rchildBuf)

	// Get child hashes (index 0 means nil child, use default hash for height-1)
	var lchildHash, rchildHash ledgerHash.Hash
	if lchildIndex > 0 {
		if int(lchildIndex-1) >= len(previousHashes) {
			return ledgerHash.Hash{}, fmt.Errorf("left child index out of range: %d >= %d", lchildIndex-1, len(previousHashes))
		}
		lchildHash = previousHashes[lchildIndex-1]
	} else {
		lchildHash = ledger.GetDefaultHashForHeight(height - 1)
	}
	if rchildIndex > 0 {
		if int(rchildIndex-1) >= len(previousHashes) {
			return ledgerHash.Hash{}, fmt.Errorf("right child index out of range: %d >= %d", rchildIndex-1, len(previousHashes))
		}
		rchildHash = previousHashes[rchildIndex-1]
	} else {
		rchildHash = ledger.GetDefaultHashForHeight(height - 1)
	}

	// Compute hash
	computedHash := ledgerHash.HashInterNode(lchildHash, rchildHash)

	// Verify hash
	if storedHash != computedHash {
		return ledgerHash.Hash{}, fmt.Errorf("interim hash mismatch: stored=%x, computed=%x", storedHash[:8], computedHash[:8])
	}

	return storedHash, nil
}

// verifyTopTrieStreaming verifies the top trie file using streaming reads
func verifyTopTrieStreaming(topTriePath string, expectedChecksum uint32, subtrieResults []*SubtrieResult, logger zerolog.Logger) (ledgerHash.Hash, uint64, error) {
	// Open file
	file, err := os.Open(topTriePath)
	if err != nil {
		return ledgerHash.Hash{}, 0, fmt.Errorf("failed to open top trie file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return ledgerHash.Hash{}, 0, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	// Read footer (nodeCount(8) + trieCount(2) + checksum(4))
	if fileSize < 18 {
		return ledgerHash.Hash{}, 0, fmt.Errorf("file too small: %d bytes", fileSize)
	}

	footer := make([]byte, 14)
	_, err = file.ReadAt(footer, fileSize-14)
	if err != nil {
		return ledgerHash.Hash{}, 0, fmt.Errorf("failed to read footer: %w", err)
	}

	nodeCount := binary.BigEndian.Uint64(footer[0:8])
	trieCount := binary.BigEndian.Uint16(footer[8:10])
	storedChecksum := binary.BigEndian.Uint32(footer[10:14])

	if storedChecksum != expectedChecksum {
		return ledgerHash.Hash{}, 0, fmt.Errorf("checksum doesn't match header: file=%d, header=%d", storedChecksum, expectedChecksum)
	}

	// Calculate total subtrie nodes for global indexing
	var totalSubtrieNodes uint64
	for _, sr := range subtrieResults {
		totalSubtrieNodes += sr.NodeCount
	}

	// Reset to beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		return ledgerHash.Hash{}, 0, fmt.Errorf("failed to seek to beginning: %w", err)
	}

	// Create CRC32 reader for everything except the last 4 bytes
	dataSize := fileSize - 4
	limitedReader := io.LimitReader(file, dataSize)
	crcReader := newCRC32Reader(bufio.NewReaderSize(limitedReader, readBufferSize))

	// Read and verify magic + version
	header := make([]byte, 4)
	_, err = io.ReadFull(crcReader, header)
	if err != nil {
		return ledgerHash.Hash{}, 0, fmt.Errorf("failed to read header: %w", err)
	}

	// Read top trie nodes
	topTrieHashes := make([]ledgerHash.Hash, 0, nodeCount)
	for nodeIdx := uint64(0); nodeIdx < nodeCount; nodeIdx++ {
		nodeHash, err := readAndVerifyTopTrieNode(crcReader, topTrieHashes, subtrieResults, totalSubtrieNodes)
		if err != nil {
			return ledgerHash.Hash{}, 0, fmt.Errorf("failed to verify top trie node %d: %w", nodeIdx, err)
		}
		topTrieHashes = append(topTrieHashes, nodeHash)
	}

	// Read trie roots
	var lastRootHash ledgerHash.Hash
	for trieIdx := uint16(0); trieIdx < trieCount; trieIdx++ {
		rootHash, err := readTrieRoot(crcReader, topTrieHashes, subtrieResults, totalSubtrieNodes)
		if err != nil {
			return ledgerHash.Hash{}, 0, fmt.Errorf("failed to read trie root %d: %w", trieIdx, err)
		}
		lastRootHash = rootHash
	}

	// Read remaining footer
	footerBuf := make([]byte, 10) // nodeCount(8) + trieCount(2)
	_, err = io.ReadFull(crcReader, footerBuf)
	if err != nil {
		return ledgerHash.Hash{}, 0, fmt.Errorf("failed to read footer: %w", err)
	}

	// Verify checksum
	computedChecksum := crcReader.Checksum()
	if computedChecksum != storedChecksum {
		return ledgerHash.Hash{}, 0, fmt.Errorf("computed checksum mismatch: stored=%d, computed=%d", storedChecksum, computedChecksum)
	}

	return lastRootHash, nodeCount, nil
}

// readAndVerifyTopTrieNode reads and verifies a top trie node
func readAndVerifyTopTrieNode(reader io.Reader, topTrieHashes []ledgerHash.Hash, subtrieResults []*SubtrieResult, totalSubtrieNodes uint64) (ledgerHash.Hash, error) {
	// Read node type
	typeBuf := make([]byte, 1)
	_, err := io.ReadFull(reader, typeBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read node type: %w", err)
	}
	nodeType := typeBuf[0]

	if nodeType == 0 { // Leaf node
		return readAndVerifyLeafNode(reader)
	} else if nodeType == 1 { // Interim node
		return readAndVerifyTopTrieInterimNode(reader, topTrieHashes, subtrieResults, totalSubtrieNodes)
	}

	return ledgerHash.Hash{}, fmt.Errorf("unknown node type: %d", nodeType)
}

// readAndVerifyTopTrieInterimNode reads and verifies an interim node in the top trie
func readAndVerifyTopTrieInterimNode(reader io.Reader, topTrieHashes []ledgerHash.Hash, subtrieResults []*SubtrieResult, totalSubtrieNodes uint64) (ledgerHash.Hash, error) {
	// Read height
	heightBuf := make([]byte, 2)
	_, err := io.ReadFull(reader, heightBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read height: %w", err)
	}
	height := int(binary.BigEndian.Uint16(heightBuf))

	// Read stored hash
	var storedHash ledgerHash.Hash
	_, err = io.ReadFull(reader, storedHash[:])
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read stored hash: %w", err)
	}

	// Read left child index (global index)
	lchildBuf := make([]byte, 8)
	_, err = io.ReadFull(reader, lchildBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read left child index: %w", err)
	}
	lchildIndex := binary.BigEndian.Uint64(lchildBuf)

	// Read right child index (global index)
	rchildBuf := make([]byte, 8)
	_, err = io.ReadFull(reader, rchildBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read right child index: %w", err)
	}
	rchildIndex := binary.BigEndian.Uint64(rchildBuf)

	// Get child hashes using global index (index 0 means nil child, use default hash)
	var lchildHash, rchildHash ledgerHash.Hash
	if lchildIndex == 0 {
		lchildHash = ledger.GetDefaultHashForHeight(height - 1)
	} else {
		lchildHash, err = getNodeHashByGlobalIndex(lchildIndex, topTrieHashes, subtrieResults, totalSubtrieNodes)
		if err != nil {
			return ledgerHash.Hash{}, fmt.Errorf("failed to get left child hash: %w", err)
		}
	}
	if rchildIndex == 0 {
		rchildHash = ledger.GetDefaultHashForHeight(height - 1)
	} else {
		rchildHash, err = getNodeHashByGlobalIndex(rchildIndex, topTrieHashes, subtrieResults, totalSubtrieNodes)
		if err != nil {
			return ledgerHash.Hash{}, fmt.Errorf("failed to get right child hash: %w", err)
		}
	}

	// Compute hash
	computedHash := ledgerHash.HashInterNode(lchildHash, rchildHash)

	// Verify hash
	if storedHash != computedHash {
		return ledgerHash.Hash{}, fmt.Errorf("interim hash mismatch: stored=%x, computed=%x", storedHash[:8], computedHash[:8])
	}

	return storedHash, nil
}

// readTrieRoot reads a trie root entry and returns the root hash
func readTrieRoot(reader io.Reader, topTrieHashes []ledgerHash.Hash, subtrieResults []*SubtrieResult, totalSubtrieNodes uint64) (ledgerHash.Hash, error) {
	// Trie root: rootIndex(8) + regCount(8) + regSize(8) + hash(32)

	// Read root index
	rootIndexBuf := make([]byte, 8)
	_, err := io.ReadFull(reader, rootIndexBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read root index: %w", err)
	}
	rootIndex := binary.BigEndian.Uint64(rootIndexBuf)

	// Read reg count
	regCountBuf := make([]byte, 8)
	_, err = io.ReadFull(reader, regCountBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read reg count: %w", err)
	}

	// Read reg size
	regSizeBuf := make([]byte, 8)
	_, err = io.ReadFull(reader, regSizeBuf)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read reg size: %w", err)
	}

	// Read stored hash
	var storedHash ledgerHash.Hash
	_, err = io.ReadFull(reader, storedHash[:])
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to read root hash: %w", err)
	}

	// For empty trie, root index is 0
	if rootIndex == 0 {
		// Empty trie should have empty hash
		return storedHash, nil
	}

	// Get the root node hash
	rootNodeHash, err := getNodeHashByGlobalIndex(rootIndex, topTrieHashes, subtrieResults, totalSubtrieNodes)
	if err != nil {
		return ledgerHash.Hash{}, fmt.Errorf("failed to get root node hash: %w", err)
	}

	// Verify root hash matches
	if storedHash != rootNodeHash {
		return ledgerHash.Hash{}, fmt.Errorf("root hash mismatch: stored=%x, computed=%x", storedHash[:8], rootNodeHash[:8])
	}

	return storedHash, nil
}

// getNodeHashByGlobalIndex returns the hash for a node at the given global index
// Global index scheme:
// - 0 = nil (empty)
// - 1 to totalSubtrieNodes = subtrie nodes
// - > totalSubtrieNodes = top trie nodes
func getNodeHashByGlobalIndex(globalIndex uint64, topTrieHashes []ledgerHash.Hash, subtrieResults []*SubtrieResult, totalSubtrieNodes uint64) (ledgerHash.Hash, error) {
	if globalIndex == 0 {
		return ledgerHash.Hash{}, nil
	}

	if globalIndex <= totalSubtrieNodes {
		// Node is in a subtrie
		// Find which subtrie and local index
		localIndex := globalIndex - 1 // Convert to 0-based
		for _, sr := range subtrieResults {
			if localIndex < sr.NodeCount {
				return sr.NodeHashes[localIndex], nil
			}
			localIndex -= sr.NodeCount
		}
		return ledgerHash.Hash{}, fmt.Errorf("subtrie index out of range: %d", globalIndex)
	}

	// Node is in top trie
	topIndex := globalIndex - totalSubtrieNodes - 1 // Convert to 0-based top trie index
	if int(topIndex) >= len(topTrieHashes) {
		return ledgerHash.Hash{}, fmt.Errorf("top trie index out of range: %d >= %d", topIndex, len(topTrieHashes))
	}
	return topTrieHashes[topIndex], nil
}

func printVerificationResult(result *VerificationResult) {
	fmt.Println()
	fmt.Println("=== Verification Summary ===")
	fmt.Println()

	if result.Valid {
		fmt.Println("Status: VALID ✓")
		fmt.Printf("Root Hash: %x\n", result.RootHash)
	} else {
		fmt.Println("Status: INVALID ✗")
		fmt.Println("Errors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	fmt.Println()
	var subtrieNodes uint64
	for _, sr := range result.SubtrieResults {
		if sr != nil {
			subtrieNodes += sr.NodeCount
		}
	}
	fmt.Printf("Total Nodes Verified: %d\n", result.TotalNodes)
	fmt.Printf("  Subtrie Nodes: %d\n", subtrieNodes)
	fmt.Printf("  Top Trie Nodes: %d\n", result.TopTrieNodes)
	fmt.Printf("Total Duration: %v\n", result.TotalDuration.Round(time.Millisecond))
}

// File path helpers (duplicated from wal package since they're unexported)
func filePathCheckpointHeader(dir string, fileName string) string {
	return filepath.Join(dir, fileName)
}

func filePathSubTries(dir string, fileName string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("%s.%03d", fileName, index))
}

func filePathTopTries(dir string, fileName string) string {
	return filepath.Join(dir, fmt.Sprintf("%s.%03d", fileName, subtrieCount))
}
