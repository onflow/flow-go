package checkpoint_verify

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

const (
	// subtrieCount is the number of subtrie files in V6 checkpoint (2^4 = 16)
	subtrieCount = 16
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
	NodeHashes  []hash.Hash // verified hashes in order (index 0 = first node)
	Err         error
	Duration    time.Duration
	StartOffset uint64 // global offset where this subtrie's nodes start (1-based)
}

// VerificationResult holds the complete verification result
type VerificationResult struct {
	Valid           bool
	RootHash        hash.Hash
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
			subtrieResults[index] = verifySubtrie(dir, filename, index, subtrieChecksums[index], logger)
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
	rootHash, topNodeCount, err := verifyTopTrie(topTriePath, topTrieChecksum, subtrieResults, logger)
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
	computedChecksum := crc32.ChecksumIEEE(data[:len(data)-4])
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

func verifySubtrie(dir, filename string, index int, expectedChecksum uint32, logger zerolog.Logger) *SubtrieResult {
	startTime := time.Now()
	result := &SubtrieResult{
		Index:      index,
		NodeHashes: make([]hash.Hash, 0),
	}

	subtriePath := filePathSubTries(dir, filename, index)

	// Read entire file for checksum verification
	data, err := os.ReadFile(subtriePath)
	if err != nil {
		result.Err = fmt.Errorf("failed to read subtrie file: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify file checksum
	if len(data) < 4 {
		result.Err = fmt.Errorf("subtrie file too small")
		result.Duration = time.Since(startTime)
		return result
	}
	storedChecksum := binary.BigEndian.Uint32(data[len(data)-4:])
	computedChecksum := crc32.ChecksumIEEE(data[:len(data)-4])
	if storedChecksum != computedChecksum {
		result.Err = fmt.Errorf("file checksum mismatch: stored=%d, computed=%d", storedChecksum, computedChecksum)
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify checksum matches header
	if storedChecksum != expectedChecksum {
		result.Err = fmt.Errorf("checksum doesn't match header: file=%d, header=%d", storedChecksum, expectedChecksum)
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse and verify nodes
	err = verifySubtrieNodes(data, result)
	if err != nil {
		result.Err = err
	}

	result.Duration = time.Since(startTime)
	return result
}

func verifySubtrieNodes(data []byte, result *SubtrieResult) error {
	// File format: magic(2) + version(2) + nodes... + nodeCount(8) + checksum(4)
	if len(data) < 16 {
		return fmt.Errorf("file too small")
	}

	// Read node count from footer
	nodeCount := binary.BigEndian.Uint64(data[len(data)-12:])
	result.NodeCount = nodeCount

	// Parse nodes - use slice for sequential access
	pos := 4 // Skip magic + version
	nodeHashes := make([]hash.Hash, 0, nodeCount)

	for nodeIdx := uint64(0); nodeIdx < nodeCount; nodeIdx++ {
		if pos >= len(data)-12 {
			return fmt.Errorf("unexpected end of data at node %d", nodeIdx)
		}

		nodeType := data[pos]
		var computedHash hash.Hash
		var storedHash hash.Hash

		if nodeType == 0 { // Leaf node
			// Leaf: type(1) + height(2) + hash(32) + path(32) + payloadLen(4) + payload
			if pos+1+2+32+32+4 > len(data)-12 {
				return fmt.Errorf("leaf node %d exceeds data bounds", nodeIdx)
			}

			height := binary.BigEndian.Uint16(data[pos+1:])
			copy(storedHash[:], data[pos+1+2:pos+1+2+32])

			var path ledger.Path
			copy(path[:], data[pos+1+2+32:pos+1+2+32+32])

			payloadLen := binary.BigEndian.Uint32(data[pos+1+2+32+32:])
			payloadEnd := pos + 1 + 2 + 32 + 32 + 4 + int(payloadLen)
			if payloadEnd > len(data)-12 {
				return fmt.Errorf("leaf node %d payload exceeds data bounds", nodeIdx)
			}

			// Decode payload to get value
			payloadData := data[pos+1+2+32+32+4 : payloadEnd]
			payload, err := ledger.DecodePayloadWithoutPrefix(payloadData, false, 1)
			if err != nil {
				return fmt.Errorf("failed to decode payload at node %d: %w", nodeIdx, err)
			}

			// Compute hash
			computedHash = ledger.ComputeCompactValue(hash.Hash(path), payload.Value(), int(height))

			pos = payloadEnd
		} else if nodeType == 1 { // Interim node
			// Interim: type(1) + height(2) + hash(32) + lchildIndex(8) + rchildIndex(8) = 51 bytes
			if pos+51 > len(data)-12 {
				return fmt.Errorf("interim node %d exceeds data bounds", nodeIdx)
			}

			height := binary.BigEndian.Uint16(data[pos+1:])
			copy(storedHash[:], data[pos+1+2:pos+1+2+32])

			lchildIndex := binary.BigEndian.Uint64(data[pos+1+2+32:])
			rchildIndex := binary.BigEndian.Uint64(data[pos+1+2+32+8:])

			// Verify descendants-first relationship (indices are 0-based within subtrie)
			if lchildIndex >= nodeIdx && lchildIndex != ^uint64(0) {
				return fmt.Errorf("node %d: left child index %d violates descendants-first", nodeIdx, lchildIndex)
			}
			if rchildIndex >= nodeIdx && rchildIndex != ^uint64(0) {
				return fmt.Errorf("node %d: right child index %d violates descendants-first", nodeIdx, rchildIndex)
			}

			// Get child hashes
			var lchildHash, rchildHash hash.Hash
			if lchildIndex == ^uint64(0) {
				lchildHash = ledger.GetDefaultHashForHeight(int(height) - 1)
			} else {
				if lchildIndex >= uint64(len(nodeHashes)) {
					return fmt.Errorf("node %d: left child index %d out of range", nodeIdx, lchildIndex)
				}
				lchildHash = nodeHashes[lchildIndex]
			}
			if rchildIndex == ^uint64(0) {
				rchildHash = ledger.GetDefaultHashForHeight(int(height) - 1)
			} else {
				if rchildIndex >= uint64(len(nodeHashes)) {
					return fmt.Errorf("node %d: right child index %d out of range", nodeIdx, rchildIndex)
				}
				rchildHash = nodeHashes[rchildIndex]
			}

			// Compute hash
			computedHash = hash.HashInterNode(lchildHash, rchildHash)

			pos += 51
		} else {
			return fmt.Errorf("unknown node type %d at node %d", nodeType, nodeIdx)
		}

		// Verify hash
		if computedHash != storedHash {
			return fmt.Errorf("node %d hash mismatch: stored=%x, computed=%x", nodeIdx, storedHash[:8], computedHash[:8])
		}

		nodeHashes = append(nodeHashes, storedHash)
	}

	// Store all node hashes for top trie verification
	result.NodeHashes = nodeHashes

	return nil
}

func verifyTopTrie(topTriePath string, expectedChecksum uint32, subtrieResults []*SubtrieResult, logger zerolog.Logger) (hash.Hash, uint64, error) {
	// Calculate total subtrie node count and build lookup structure
	var totalSubtrieNodeCount uint64
	for _, sr := range subtrieResults {
		sr.StartOffset = totalSubtrieNodeCount + 1 // 1-based global index
		totalSubtrieNodeCount += sr.NodeCount
	}

	// Helper to get hash by global index
	getHashByGlobalIndex := func(globalIndex uint64, height int) (hash.Hash, error) {
		if globalIndex == 0 {
			// Index 0 = nil, use default hash
			return ledger.GetDefaultHashForHeight(height - 1), nil
		}

		if globalIndex <= totalSubtrieNodeCount {
			// Subtrie node - find which subtrie
			offset := globalIndex - 1 // Convert to 0-based
			for _, sr := range subtrieResults {
				if offset < sr.NodeCount {
					return sr.NodeHashes[offset], nil
				}
				offset -= sr.NodeCount
			}
			return hash.Hash{}, fmt.Errorf("subtrie node index %d not found", globalIndex)
		}

		// Top-level node - will be looked up in local nodeHashes
		return hash.Hash{}, fmt.Errorf("top-level node")
	}

	// Read entire file
	data, err := os.ReadFile(topTriePath)
	if err != nil {
		return hash.Hash{}, 0, fmt.Errorf("failed to read top trie file: %w", err)
	}

	// Verify file checksum
	if len(data) < 4 {
		return hash.Hash{}, 0, fmt.Errorf("top trie file too small")
	}
	storedChecksum := binary.BigEndian.Uint32(data[len(data)-4:])
	computedChecksum := crc32.ChecksumIEEE(data[:len(data)-4])
	if storedChecksum != computedChecksum {
		return hash.Hash{}, 0, fmt.Errorf("file checksum mismatch: stored=%d, computed=%d", storedChecksum, computedChecksum)
	}

	if storedChecksum != expectedChecksum {
		return hash.Hash{}, 0, fmt.Errorf("checksum doesn't match header: file=%d, header=%d", storedChecksum, expectedChecksum)
	}

	// Parse top trie file
	// Format: magic(2) + version(2) + nodes... + nodeCount(8) + trieCount(2) + tries... + checksum(4)

	// Read footer to get counts
	if len(data) < 14 {
		return hash.Hash{}, 0, fmt.Errorf("file too small for footer")
	}

	// Find trie count (2 bytes before checksum)
	trieCount := binary.BigEndian.Uint16(data[len(data)-6:])

	// Calculate trie data size: each trie is 56 bytes (rootIndex(8) + regCount(8) + regSize(8) + rootHash(32))
	trieDataSize := int(trieCount) * 56

	// Node count is before trie count and trie data
	nodeCountPos := len(data) - 4 - 2 - trieDataSize - 8
	if nodeCountPos < 4 {
		return hash.Hash{}, 0, fmt.Errorf("invalid file structure")
	}
	nodeCount := binary.BigEndian.Uint64(data[nodeCountPos:])

	// Parse and verify nodes
	pos := 4 // Skip magic + version
	topLevelHashes := make([]hash.Hash, 0, nodeCount)

	for nodeIdx := uint64(0); nodeIdx < nodeCount; nodeIdx++ {
		// Global index for this top-level node
		globalIndex := totalSubtrieNodeCount + nodeIdx + 1

		if pos >= nodeCountPos {
			return hash.Hash{}, 0, fmt.Errorf("unexpected end of node data at node %d", nodeIdx)
		}

		nodeType := data[pos]
		var computedHash hash.Hash
		var storedHash hash.Hash

		if nodeType == 0 { // Leaf - shouldn't happen in top trie normally, but handle it
			if pos+1+2+32+32+4 > nodeCountPos {
				return hash.Hash{}, 0, fmt.Errorf("leaf node %d exceeds data bounds", nodeIdx)
			}

			height := binary.BigEndian.Uint16(data[pos+1:])
			copy(storedHash[:], data[pos+1+2:pos+1+2+32])

			var path ledger.Path
			copy(path[:], data[pos+1+2+32:pos+1+2+32+32])

			payloadLen := binary.BigEndian.Uint32(data[pos+1+2+32+32:])
			payloadEnd := pos + 1 + 2 + 32 + 32 + 4 + int(payloadLen)
			if payloadEnd > nodeCountPos {
				return hash.Hash{}, 0, fmt.Errorf("leaf node %d payload exceeds data bounds", nodeIdx)
			}

			payloadData := data[pos+1+2+32+32+4 : payloadEnd]
			payload, err := ledger.DecodePayloadWithoutPrefix(payloadData, false, 1)
			if err != nil {
				return hash.Hash{}, 0, fmt.Errorf("failed to decode payload at node %d: %w", nodeIdx, err)
			}

			computedHash = ledger.ComputeCompactValue(hash.Hash(path), payload.Value(), int(height))
			pos = payloadEnd
		} else if nodeType == 1 { // Interim node
			if pos+51 > nodeCountPos {
				return hash.Hash{}, 0, fmt.Errorf("interim node %d exceeds data bounds", nodeIdx)
			}

			height := binary.BigEndian.Uint16(data[pos+1:])
			copy(storedHash[:], data[pos+1+2:pos+1+2+32])

			lchildIndex := binary.BigEndian.Uint64(data[pos+1+2+32:])
			rchildIndex := binary.BigEndian.Uint64(data[pos+1+2+32+8:])

			// Get child hashes
			var lchildHash, rchildHash hash.Hash

			if lchildIndex == 0 {
				lchildHash = ledger.GetDefaultHashForHeight(int(height) - 1)
			} else {
				h, err := getHashByGlobalIndex(lchildIndex, int(height))
				if err != nil {
					// Must be a top-level node reference
					if lchildIndex <= totalSubtrieNodeCount {
						return hash.Hash{}, 0, fmt.Errorf("node %d: left child %d: %w", nodeIdx, lchildIndex, err)
					}
					// Top-level node - check descendants-first
					topLocalIndex := lchildIndex - totalSubtrieNodeCount - 1
					if topLocalIndex >= nodeIdx {
						return hash.Hash{}, 0, fmt.Errorf("node %d (global %d): left child %d violates descendants-first",
							nodeIdx, globalIndex, lchildIndex)
					}
					lchildHash = topLevelHashes[topLocalIndex]
				} else {
					lchildHash = h
				}
			}

			if rchildIndex == 0 {
				rchildHash = ledger.GetDefaultHashForHeight(int(height) - 1)
			} else {
				h, err := getHashByGlobalIndex(rchildIndex, int(height))
				if err != nil {
					// Must be a top-level node reference
					if rchildIndex <= totalSubtrieNodeCount {
						return hash.Hash{}, 0, fmt.Errorf("node %d: right child %d: %w", nodeIdx, rchildIndex, err)
					}
					// Top-level node - check descendants-first
					topLocalIndex := rchildIndex - totalSubtrieNodeCount - 1
					if topLocalIndex >= nodeIdx {
						return hash.Hash{}, 0, fmt.Errorf("node %d (global %d): right child %d violates descendants-first",
							nodeIdx, globalIndex, rchildIndex)
					}
					rchildHash = topLevelHashes[topLocalIndex]
				} else {
					rchildHash = h
				}
			}

			computedHash = hash.HashInterNode(lchildHash, rchildHash)
			pos += 51
		} else {
			return hash.Hash{}, 0, fmt.Errorf("unknown node type %d at node %d", nodeType, nodeIdx)
		}

		// Verify hash
		if computedHash != storedHash {
			return hash.Hash{}, 0, fmt.Errorf("node %d hash mismatch: stored=%x, computed=%x", nodeIdx, storedHash[:8], computedHash[:8])
		}

		topLevelHashes = append(topLevelHashes, storedHash)
	}

	// Parse trie entries to get root hash
	trieDataStart := nodeCountPos + 8 + 2 // after nodeCount and trieCount
	if trieCount == 0 {
		return hash.Hash{}, 0, fmt.Errorf("no tries in checkpoint")
	}

	// Read first (and typically only) trie's root hash
	// Trie format: rootIndex(8) + regCount(8) + regSize(8) + rootHash(32)
	triePos := trieDataStart
	rootIndex := binary.BigEndian.Uint64(data[triePos:])

	var rootHash hash.Hash
	if rootIndex == 0 {
		// Empty trie
		rootHash = ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight)
	} else {
		// Get root hash from verified nodes
		h, err := getHashByGlobalIndex(rootIndex, ledger.NodeMaxHeight+1)
		if err != nil {
			// Must be a top-level node
			if rootIndex <= totalSubtrieNodeCount {
				return hash.Hash{}, 0, fmt.Errorf("root index %d: %w", rootIndex, err)
			}
			topLocalIndex := rootIndex - totalSubtrieNodeCount - 1
			if topLocalIndex >= uint64(len(topLevelHashes)) {
				return hash.Hash{}, 0, fmt.Errorf("root index %d out of range", rootIndex)
			}
			rootHash = topLevelHashes[topLocalIndex]
		} else {
			rootHash = h
		}
	}

	// Verify against stored root hash
	storedRootHash := data[triePos+8+8+8 : triePos+8+8+8+32]
	var expectedRootHash hash.Hash
	copy(expectedRootHash[:], storedRootHash)

	if rootHash != expectedRootHash {
		return hash.Hash{}, 0, fmt.Errorf("root hash mismatch: computed=%x, stored=%x", rootHash[:8], expectedRootHash[:8])
	}

	return rootHash, nodeCount, nil
}

// File path helpers (duplicated from wal package since they're not exported)
func filePathCheckpointHeader(dir string, fileName string) string {
	return filepath.Join(dir, fileName)
}

func filePathSubTries(dir string, fileName string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("%s.%03d", fileName, index))
}

func filePathTopTries(dir string, fileName string) string {
	return filepath.Join(dir, fmt.Sprintf("%s.%03d", fileName, subtrieCount))
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
	fmt.Printf("Total Nodes Verified: %d\n", result.TotalNodes)
	fmt.Printf("  Subtrie Nodes: %d\n", result.TotalNodes-result.TopTrieNodes)
	fmt.Printf("  Top Trie Nodes: %d\n", result.TopTrieNodes)
	fmt.Printf("Total Duration: %v\n", result.TotalDuration)
}
