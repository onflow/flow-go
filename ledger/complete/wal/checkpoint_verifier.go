package wal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// ErrCheckpointHashMismatch indicates that a node's stored (cached) hash does not
// match the hash recomputed from its content (leaf node) or its children (interim
// node). It signals a corrupt checkpoint.
var ErrCheckpointHashMismatch = errors.New("checkpoint hash verification failed")

// VerifyCheckpointHashes verifies the cryptographic integrity of every node in a
// checkpoint (V6 or V7) by recomputing each node's hash and comparing it against
// the hash stored alongside the node on disk:
//   - For a leaf node, the hash is recomputed from its content: the payload value
//     (V6) or the stored leaf hash (V7). This is the streaming equivalent of the
//     per-leaf check performed by [trie.MTrie.IsAValidTrie] /
//     [node.Node.VerifyCachedHash].
//   - For an interim node, the hash is recomputed as HashInterNode of its two
//     children's hashes (using the height-appropriate default hash for an empty
//     child).
//
// Nodes are streamed in descendants-first (post-order DFS) order, so every child's
// hash is verified and recorded before its parent is checked. A correct subtrie
// root hash therefore transitively attests the whole subtrie. The full forest is
// never materialized: only one 32-byte hash per node is retained (no node objects,
// no payloads), which is the improvement over loading the checkpoint and calling
// [trie.MTrie.IsAValidTrie].
//
// The 16 subtrie part files are verified concurrently using up to nWorker
// goroutines; nWorker must be in [1, 16]. The (small) top-trie part file is then
// verified single-threaded using the subtrie node hashes. Per-part-file CRC32
// checksums and magic/version bytes are validated while reading, matching the
// regular checkpoint readers.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointHashMismatch]: when a node's stored hash does not match its
//     recomputed hash.
//   - [ErrCheckpointIntegrity]: when an interim node references an out-of-range or
//     forward child index.
//   - [os.ErrNotExist] (wrapped): when a checkpoint part file is missing.
func VerifyCheckpointHashes(logger zerolog.Logger, dir string, fileName string, nWorker uint) error {
	if nWorker < 1 || nWorker > subtrieCount {
		return fmt.Errorf("invalid nWorker %d, valid range is [1, %d]", nWorker, subtrieCount)
	}

	headerPath := filePathCheckpointHeader(dir, fileName)

	version, err := readCheckpointHeaderVersion(headerPath)
	if err != nil {
		return fmt.Errorf("could not read checkpoint header version: %w", err)
	}
	isV7 := version == VersionV7

	var subtrieChecksums []uint32
	var topTrieChecksum uint32
	if isV7 {
		subtrieChecksums, topTrieChecksum, err = readCheckpointHeaderV7(headerPath, logger)
	} else {
		subtrieChecksums, topTrieChecksum, err = readCheckpointHeader(headerPath, logger)
	}
	if err != nil {
		return fmt.Errorf("could not read checkpoint header: %w", err)
	}

	if err := allPartFileExist(dir, fileName, len(subtrieChecksums)); err != nil {
		return fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	logger.Info().
		Int("version", int(version)).
		Int("subtrie_files", len(subtrieChecksums)).
		Uint("workers", nWorker).
		Msg("starting checkpoint hash verification")

	// Phase 1: verify the subtrie files concurrently, retaining each subtrie's
	// per-node hashes so the top trie can reference them.
	subtrieHashes, err := verifySubtriesConcurrently(logger, dir, fileName, subtrieChecksums, isV7, nWorker)
	if err != nil {
		return err
	}

	// Phase 2: verify the top trie using the subtrie node hashes.
	if err := verifyTopTrie(logger, dir, fileName, isV7, subtrieHashes, topTrieChecksum); err != nil {
		return fmt.Errorf("could not verify top trie: %w", err)
	}

	logger.Info().Msg("checkpoint hash verification succeeded")
	return nil
}

// minEncodedNodeSize is the smallest number of bytes any node occupies in a
// checkpoint part file (an interim node: type + height + hash + two child indices).
// It bounds how many nodes a footer count can plausibly describe for a given file
// size, so a corrupt count cannot drive an unbounded allocation.
const minEncodedNodeSize = fixedNodePrefixSize + 2*encNodeIndexSize

// verifySubtriesConcurrently verifies all subtrie part files using up to nWorker
// goroutines and returns, for each subtrie file (in index order), the slice of its
// node hashes indexed by the file-local node index (index 0 is an unused nil
// sentinel). Workers stop early once any subtrie has failed.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointHashMismatch], [ErrCheckpointIntegrity]: see [VerifyCheckpointHashes].
func verifySubtriesConcurrently(
	logger zerolog.Logger,
	dir string,
	fileName string,
	subtrieChecksums []uint32,
	isV7 bool,
	nWorker uint,
) ([][]hash.Hash, error) {
	numOfSubTries := len(subtrieChecksums)
	results := make([][]hash.Hash, numOfSubTries)
	errs := make([]error, numOfSubTries)

	jobs := make(chan int, numOfSubTries)
	for i := range subtrieChecksums {
		jobs <- i
	}
	close(jobs)

	// failed is set as soon as any subtrie fails so the remaining workers stop
	// draining the job channel instead of reading multi-GB part files to the end.
	var failed atomic.Bool
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for i := range jobs {
			if failed.Load() {
				return
			}
			hashes, err := verifySubtrie(logger, dir, fileName, i, subtrieChecksums[i], isV7)
			if err != nil {
				errs[i] = err
				failed.Store(true)
				return
			}
			results[i] = hashes
		}
	}

	for range nWorker {
		wg.Add(1)
		go worker()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("could not verify subtrie %d: %w", i, err)
		}
	}

	return results, nil
}

// verifySubtrie verifies a single subtrie part file and returns its node hashes
// indexed by the file-local node index (index 0 is an unused nil sentinel).
//
// Expected error returns during normal operation:
//   - [ErrCheckpointHashMismatch], [ErrCheckpointIntegrity]: see [VerifyCheckpointHashes].
func verifySubtrie(
	logger zerolog.Logger,
	dir string,
	fileName string,
	index int,
	checksum uint32,
	isV7 bool,
) ([]hash.Hash, error) {
	var hashes []hash.Hash

	// The footer's node count is untrusted until the CRC is verified, so bound it by
	// the part file's size before using it to size the hash slice.
	partPath, _, err := filePathSubTries(dir, fileName, index)
	if err != nil {
		return nil, err
	}
	partInfo, err := os.Stat(partPath)
	if err != nil {
		return nil, fmt.Errorf("could not stat subtrie %d file: %w", index, err)
	}
	maxNodes := uint64(partInfo.Size()) / minEncodedNodeSize

	process := func(reader *Crc32Reader, nodesCount uint64) error {
		if nodesCount > maxNodes {
			return fmt.Errorf("%w: subtrie %d footer claims %d nodes, but file size %d allows at most %d",
				ErrCheckpointIntegrity, index, nodesCount, partInfo.Size(), maxNodes)
		}

		hashes = make([]hash.Hash, nodesCount+1) // +1: index 0 is the nil sentinel
		scratch := make([]byte, defaultBufioReadSize)

		logging := logProgress(fmt.Sprintf("verifying %d-th subtrie hashes", index), int(nodesCount), logger)

		for i := uint64(1); i <= nodesCount; i++ {
			meta, err := readNodeMeta(reader, scratch, isV7, true)
			if err != nil {
				return fmt.Errorf("cannot read subtrie %d node %d: %w", index, i, err)
			}

			// Within a subtrie file, child indices are local to that file.
			childHash := func(childIdx uint64) (hash.Hash, error) {
				if childIdx >= i {
					return hash.Hash{}, fmt.Errorf("%w: subtrie %d node %d references unknown/forward child %d",
						ErrCheckpointIntegrity, index, i, childIdx)
				}
				return hashes[childIdx], nil
			}

			if err := checkNodeHash(meta, isV7, childHash); err != nil {
				return err
			}

			hashes[i] = meta.hash
			logging(i)
		}
		return nil
	}

	if isV7 {
		err = processCheckpointSubTrieV7(dir, fileName, index, checksum, logger, process)
	} else {
		err = processCheckpointSubTrie(dir, fileName, index, checksum, logger, process)
	}
	if err != nil {
		return nil, err
	}

	return hashes, nil
}

// verifyTopTrie verifies the top-trie part file. Top-level node child indices are
// global, referencing either earlier top-level nodes or subtrie nodes (resolved
// from subtrieHashes). It also cross-checks each trie root record's stored hash
// against the hash of the node it references.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointHashMismatch], [ErrCheckpointIntegrity]: see [VerifyCheckpointHashes].
func verifyTopTrie(
	logger zerolog.Logger,
	dir string,
	fileName string,
	isV7 bool,
	subtrieHashes [][]hash.Hash,
	topTrieChecksum uint32,
) error {
	// Per-subtrie global-index offsets: subtrie i occupies global indices
	// (offsets[i], offsets[i]+count_i].
	offsets := make([]uint64, len(subtrieHashes))
	var totalSub uint64
	for i, hs := range subtrieHashes {
		offsets[i] = totalSub
		totalSub += uint64(len(hs) - 1) // -1 for the nil sentinel at index 0
	}

	// subtrieHashAt resolves a global index that falls within the subtrie range.
	subtrieHashAt := func(globalIdx uint64) (hash.Hash, error) {
		for i, start := range offsets {
			count := uint64(len(subtrieHashes[i]) - 1)
			if globalIdx > start && globalIdx <= start+count {
				return subtrieHashes[i][globalIdx-start], nil
			}
		}
		return hash.Hash{}, fmt.Errorf("%w: global index %d is not a valid subtrie node", ErrCheckpointIntegrity, globalIdx)
	}

	version := VersionV6
	if isV7 {
		version = VersionV7
	}

	topPath, _ := filePathTopTries(dir, fileName)
	return withFile(logger, topPath, func(file *os.File) error {
		if err := validateFileHeader(MagicBytesCheckpointToptrie, version, file); err != nil {
			return err
		}

		topLevelNodesCount, triesCount, expectedSum, err := readTopTriesFooter(file)
		if err != nil {
			return fmt.Errorf("could not read top tries footer: %w", err)
		}
		if topTrieChecksum != expectedSum {
			return fmt.Errorf("mismatch top trie checksum, header file has %v, toptrie file has %v",
				topTrieChecksum, expectedSum)
		}

		// The footer's node count is untrusted until the CRC is verified, so bound it
		// by the file size before using it to size the hash slice.
		topInfo, err := file.Stat()
		if err != nil {
			return fmt.Errorf("could not stat top trie file: %w", err)
		}
		if maxNodes := uint64(topInfo.Size()) / minEncodedNodeSize; topLevelNodesCount > maxNodes {
			return fmt.Errorf("%w: top trie footer claims %d top-level nodes, but file size %d allows at most %d",
				ErrCheckpointIntegrity, topLevelNodesCount, topInfo.Size(), maxNodes)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("could not seek to start of top trie file: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))
		if _, _, err := readFileHeader(reader); err != nil {
			return fmt.Errorf("could not read version for top trie: %w", err)
		}

		// Read and validate the subtrie node count carried in the top-trie file.
		buf := make([]byte, encNodeCountSize)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return fmt.Errorf("could not read subtrie node count: %w", err)
		}
		readSubtrieNodeCount, err := decodeNodeCount(buf)
		if err != nil {
			return fmt.Errorf("could not decode subtrie node count: %w", err)
		}
		if readSubtrieNodeCount != totalSub {
			return fmt.Errorf("mismatch subtrie node count, top trie file has %v, but subtrie files sum to %v",
				readSubtrieNodeCount, totalSub)
		}

		// topLevelHashes is indexed by top-level local index; global index = totalSub + localIndex.
		topLevelHashes := make([]hash.Hash, topLevelNodesCount+1)
		scratch := make([]byte, defaultBufioReadSize)

		for j := uint64(1); j <= topLevelNodesCount; j++ {
			meta, err := readNodeMeta(reader, scratch, isV7, true)
			if err != nil {
				return fmt.Errorf("cannot read top-level node %d: %w", j, err)
			}

			globalIndex := totalSub + j

			// Top-level child indices are global. They must reference an
			// already-seen node: a subtrie node, or an earlier top-level node.
			childHash := func(childIdx uint64) (hash.Hash, error) {
				if childIdx >= globalIndex {
					return hash.Hash{}, fmt.Errorf("%w: top-level node %d references unknown/forward child %d",
						ErrCheckpointIntegrity, globalIndex, childIdx)
				}
				if childIdx <= totalSub {
					return subtrieHashAt(childIdx)
				}
				return topLevelHashes[childIdx-totalSub], nil
			}

			if err := checkNodeHash(meta, isV7, childHash); err != nil {
				return err
			}

			topLevelHashes[j] = meta.hash
		}

		// resolveGlobal resolves any global node index to its verified hash.
		resolveGlobal := func(globalIdx uint64) (hash.Hash, error) {
			if globalIdx == 0 || globalIdx > totalSub+topLevelNodesCount {
				return hash.Hash{}, fmt.Errorf("%w: trie root references out-of-range node index %d", ErrCheckpointIntegrity, globalIdx)
			}
			if globalIdx <= totalSub {
				return subtrieHashAt(globalIdx)
			}
			return topLevelHashes[globalIdx-totalSub], nil
		}

		// Trie root records: cross-check each stored root hash against the hash of
		// the node it references.
		for i := range triesCount {
			var rootIndex uint64
			var storedRootHash hash.Hash
			if isV7 {
				enc, err := payloadless.ReadEncodedTrie(reader, scratch)
				if err != nil {
					return fmt.Errorf("cannot read trie root record %d: %w", i, err)
				}
				rootIndex, storedRootHash = enc.RootIndex, enc.RootHash
			} else {
				enc, err := flattener.ReadEncodedTrie(reader, scratch)
				if err != nil {
					return fmt.Errorf("cannot read trie root record %d: %w", i, err)
				}
				rootIndex, storedRootHash = enc.RootIndex, enc.RootHash
			}

			// rootIndex 0 means the empty trie; its root hash is the default hash at max height.
			if rootIndex == 0 {
				if storedRootHash != ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight) {
					return fmt.Errorf("%w: empty trie root record %d has non-default root hash", ErrCheckpointHashMismatch, i)
				}
				continue
			}

			nodeHash, err := resolveGlobal(rootIndex)
			if err != nil {
				return err
			}
			if nodeHash != storedRootHash {
				return fmt.Errorf("%w: trie root record %d hash does not match its root node %d",
					ErrCheckpointHashMismatch, i, rootIndex)
			}
		}

		// Consume the footer (node count + trie count) so the CRC covers it, then verify.
		if _, err := io.ReadFull(reader, scratch[:encNodeCountSize+encTrieCountSize]); err != nil {
			return fmt.Errorf("cannot read top trie footer: %w", err)
		}

		actualSum := reader.Crc32()
		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v", expectedSum, actualSum)
		}

		if _, err := io.ReadFull(reader, scratch[:crc32SumSize]); err != nil {
			return fmt.Errorf("could not read checksum from top trie file: %w", err)
		}

		if err := ensureReachedEOF(reader); err != nil {
			return fmt.Errorf("fail to read top trie file: %w", err)
		}

		return nil
	})
}

// checkNodeHash recomputes meta's hash and compares it against the stored hash.
// childHash resolves a (non-nil) child's already-verified hash; a nil child (index
// 0) is handled here using the height-appropriate default hash.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointHashMismatch]: when the recomputed hash does not match.
//   - [ErrCheckpointIntegrity]: when childHash reports an invalid child reference.
func checkNodeHash(meta nodeMeta, isV7 bool, childHash func(childIdx uint64) (hash.Hash, error)) error {
	var expected hash.Hash

	if meta.isLeaf {
		expected = leafExpectedHash(meta, isV7)
	} else {
		lh := ledger.GetDefaultHashForHeight(int(meta.height) - 1)
		if meta.lChild != 0 {
			h, err := childHash(meta.lChild)
			if err != nil {
				return err
			}
			lh = h
		}

		rh := ledger.GetDefaultHashForHeight(int(meta.height) - 1)
		if meta.rChild != 0 {
			h, err := childHash(meta.rChild)
			if err != nil {
				return err
			}
			rh = h
		}

		expected = hash.HashInterNode(lh, rh)
	}

	if expected != meta.hash {
		nodeKind := "interim"
		if meta.isLeaf {
			nodeKind = "leaf"
		}
		return fmt.Errorf("%w: %s node at height %d has stored hash %x but recomputed hash %x",
			ErrCheckpointHashMismatch, nodeKind, meta.height, meta.hash, expected)
	}

	return nil
}

// leafExpectedHash recomputes a leaf node's hash from its content.
//
// For V6, the hash is computed from the decoded payload value. For V7, the hash is
// computed from the stored leaf hash; a V7 leaf without a stored leaf hash is only
// valid if it is a default (unallocated) node, so the expected hash is the default
// hash for its height (any non-default V7 leaf missing its leaf hash will therefore
// fail the comparison in checkNodeHash).
func leafExpectedHash(meta nodeMeta, isV7 bool) hash.Hash {
	if !isV7 {
		return ledger.ComputeCompactValue(hash.Hash(meta.path), meta.value, int(meta.height))
	}
	if meta.hasLeafHash {
		return ledger.ComputeCompactValueFromLeafHash(hash.Hash(meta.path), meta.leafHash, int(meta.height))
	}
	return ledger.GetDefaultHashForHeight(int(meta.height))
}
