package wal

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// ErrCheckpointIntegrity indicates that a checkpoint's trie structure is corrupt:
// either an interim node references a child that has not been seen yet (a forward
// or out-of-range reference, violating the descendants-first ordering), or a node
// is not referenced by any parent interim node or trie root (an orphan node).
var ErrCheckpointIntegrity = errors.New("checkpoint integrity violation")

// CheckpointNode carries the decoded, per-node information passed to an
// [IterateNodeFunc] during a streaming iteration of a checkpoint. It is a
// lightweight view: no child pointers and no payload bytes are retained, so the
// caller can process arbitrarily large checkpoints without materializing the
// trie forest in memory.
type CheckpointNode struct {
	// Index is the 1-based global index of this node in the checkpoint's
	// descendants-first node sequence. It matches the index scheme used to
	// reference children: index 0 is reserved for the nil (empty) child.
	Index uint64

	// Height is the node's height in the trie.
	Height uint16

	// Hash is the node's hash.
	Hash hash.Hash

	// IsLeaf is true for leaf nodes and false for interim nodes.
	IsLeaf bool

	// IsDefault is true iff this node's hash equals the default hash for its
	// height, i.e. the sub-trie rooted at this node is completely unallocated.
	IsDefault bool

	// Path is the register storage path. Only meaningful for leaf nodes.
	Path ledger.Path

	// PayloadSize is the encoded payload size (in bytes) recorded in a V6 leaf
	// node's on-disk length prefix. It is 0 for interim nodes and for V7
	// (payloadless) leaf nodes, which do not store payloads.
	PayloadSize int

	// LeftChildIndex and RightChildIndex are the global indices of an interim
	// node's children; 0 means a nil (empty) child. Both are 0 for leaf nodes.
	LeftChildIndex  uint64
	RightChildIndex uint64
}

// IterateNodeFunc processes a single node during a checkpoint iteration. Nodes
// are delivered in descendants-first (post-order DFS) order, so every child of a
// node is delivered before the node itself.
//
// Returning an error aborts the iteration and the error is propagated out of
// [IterateCheckpointNodes].
type IterateNodeFunc func(*CheckpointNode) error

// IterateCheckpointNodes streams every node of a checkpoint (V6 or V7), invoking
// fn once per node in descendants-first (post-order DFS) order, the same order in
// which nodes are written to disk. The whole checkpoint is never loaded into
// memory: each node is decoded from the raw byte stream and handed to fn without
// retaining child pointers or payloads.
//
// The version is detected from the checkpoint header file's version bytes; each
// part file's magic+version bytes are additionally validated while reading.
// Per-part-file CRC32 checksums are verified, matching the regular checkpoint
// readers.
//
// Counts produced by fn are over the unique nodes of the whole checkpoint forest
// (nodes shared between tries are stored, and therefore delivered, exactly once).
//
// While streaming, the trie structure is verified:
//   - every interim node must reference only already-seen, in-range children
//     (descendants-first ordering); and
//   - every node must be referenced by some parent interim node or trie root.
//
// To perform these checks without retaining nodes, the iterator keeps two bits
// per node (a "default node" bit and a "referenced" bit), i.e. O(nodeCount) bits
// of memory — far smaller than the nodes themselves, but not constant.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointIntegrity]: when an interim node references an unknown/forward
//     child, or when a node is not referenced by any parent or trie root.
//   - [os.ErrNotExist] (wrapped): when a checkpoint part file is missing.
func IterateCheckpointNodes(logger zerolog.Logger, dir string, fileName string, fn IterateNodeFunc) error {
	headerPath := filePathCheckpointHeader(dir, fileName)

	version, err := readCheckpointHeaderVersion(headerPath)
	if err != nil {
		return fmt.Errorf("could not read checkpoint header version: %w", err)
	}
	isV7 := version == VersionV7

	var subtrieChecksums []uint32
	if isV7 {
		subtrieChecksums, _, err = readCheckpointHeaderV7(headerPath, logger)
	} else {
		subtrieChecksums, _, err = readCheckpointHeader(headerPath, logger)
	}
	if err != nil {
		return fmt.Errorf("could not read checkpoint header: %w", err)
	}

	if err := allPartFileExist(dir, fileName, len(subtrieChecksums)); err != nil {
		return fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	// First pass: read only the part-file footers (at the file tails) to learn each
	// subtrie's node count. This yields the per-subtrie global-index offsets and the
	// total node count needed to size the integrity bitsets before streaming.
	offsets := make([]uint64, len(subtrieChecksums))
	var totalSub uint64
	for i := range subtrieChecksums {
		count, err := readSubtrieNodeCountFromFooter(logger, dir, fileName, i)
		if err != nil {
			return fmt.Errorf("could not read subtrie %d footer: %w", i, err)
		}
		offsets[i] = totalSub
		totalSub += count
	}

	topLevelNodesCount, err := readTopTrieNodeCountFromFooter(logger, dir, fileName)
	if err != nil {
		return fmt.Errorf("could not read top trie footer: %w", err)
	}

	total := totalSub + topLevelNodesCount

	logger.Info().
		Uint64("subtrie_nodes", totalSub).
		Uint64("top_level_nodes", topLevelNodesCount).
		Uint64("total_nodes", total).
		Msg("starting checkpoint node iteration")

	it := &checkpointIterator{
		fn:         fn,
		isDefault:  newBitset(total + 1),
		referenced: newBitset(total + 1),
		totalSub:   totalSub,
		total:      total,
		logProgress: logProgress(
			"iterating checkpoint nodes", int(total), logger),
	}

	// Second pass: stream the subtrie part files (sequentially), then the top-trie
	// part file. processCheckpointSubTrie(V7) validates the file header and verifies
	// the CRC32 checksum around the node stream we consume.
	for i := range subtrieChecksums {
		offset := offsets[i]
		process := func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4)
			for localIndex := uint64(1); localIndex <= nodesCount; localIndex++ {
				meta, err := readNodeMeta(reader, scratch, isV7)
				if err != nil {
					return fmt.Errorf("cannot read subtrie %d node %d: %w", i, localIndex, err)
				}
				globalIndex := offset + localIndex
				// Within a subtrie file, child indices are local to that file.
				lGlobal := subtrieChildToGlobal(meta.lChild, offset)
				rGlobal := subtrieChildToGlobal(meta.rChild, offset)
				if err := it.emit(meta, globalIndex, lGlobal, rGlobal); err != nil {
					return err
				}
			}
			return nil
		}

		if isV7 {
			err = processCheckpointSubTrieV7(dir, fileName, i, subtrieChecksums[i], logger, process)
		} else {
			err = processCheckpointSubTrie(dir, fileName, i, subtrieChecksums[i], logger, process)
		}
		if err != nil {
			return fmt.Errorf("could not iterate subtrie %d: %w", i, err)
		}
	}

	if err := it.iterateTopTrie(dir, fileName, isV7, logger); err != nil {
		return fmt.Errorf("could not iterate top trie: %w", err)
	}

	logger.Info().Uint64("total_nodes", total).Msg("finished streaming checkpoint nodes, verifying every node is referenced")

	// Every node must be referenced by a parent interim node or a trie root.
	for idx := uint64(1); idx <= total; idx++ {
		if !it.referenced.get(idx) {
			return fmt.Errorf("%w: node at global index %d is not referenced by any parent or trie root (orphan node)",
				ErrCheckpointIntegrity, idx)
		}
	}

	return nil
}

// checkpointIterator holds the shared state for a single streaming iteration: the
// caller's callback, the two integrity bitsets, and the global-index layout.
type checkpointIterator struct {
	fn          IterateNodeFunc
	isDefault   *bitset      // isDefault[i] set iff node i is a default node
	referenced  *bitset      // referenced[i] set iff node i is referenced by a parent or trie root
	totalSub    uint64       // total number of subtrie nodes; top-level node global indices start at totalSub+1
	total       uint64       // total number of nodes in the checkpoint
	logProgress func(uint64) // called once per node to log streaming progress (percentage + ETA)
}

// emit verifies and records a fully-decoded node at the given global index (with
// child indices already converted to global indices, 0 meaning a nil child), then
// invokes the caller's callback.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointIntegrity]: when an interim node references a child whose
//     global index does not strictly precede this node (forward/unknown reference),
//     or references a default (completely unallocated) child.
func (it *checkpointIterator) emit(meta nodeMeta, globalIndex, lGlobal, rGlobal uint64) error {
	if !meta.isLeaf {
		// Descendants-first ordering: both children must have been seen already.
		// A nil child (index 0) trivially satisfies 0 < globalIndex.
		if lGlobal >= globalIndex || rGlobal >= globalIndex {
			return fmt.Errorf("%w: interim node at global index %d references an unknown/forward child (left=%d, right=%d)",
				ErrCheckpointIntegrity, globalIndex, lGlobal, rGlobal)
		}
		// A correctly compactified trie never stores a default (completely unallocated)
		// sub-trie as a referenced child: such children are collapsed to nil during
		// construction (see node.NewInterimCompactifiedNode). Because children are
		// emitted before their parent, their default status is already recorded in
		// it.isDefault. A referenced default child therefore indicates a malformed
		// (non-compactified) checkpoint trie.
		if lGlobal != 0 && it.isDefault.get(lGlobal) {
			return fmt.Errorf("%w: interim node at global index %d references a default (unallocated) left child %d",
				ErrCheckpointIntegrity, globalIndex, lGlobal)
		}
		if rGlobal != 0 && it.isDefault.get(rGlobal) {
			return fmt.Errorf("%w: interim node at global index %d references a default (unallocated) right child %d",
				ErrCheckpointIntegrity, globalIndex, rGlobal)
		}
		if lGlobal != 0 {
			it.referenced.set(lGlobal)
		}
		if rGlobal != 0 {
			it.referenced.set(rGlobal)
		}
	}

	isDef := meta.hash == ledger.GetDefaultHashForHeight(int(meta.height))
	if isDef {
		it.isDefault.set(globalIndex)
	}

	cn := CheckpointNode{
		Index:     globalIndex,
		Height:    meta.height,
		Hash:      meta.hash,
		IsLeaf:    meta.isLeaf,
		IsDefault: isDef,
	}
	if meta.isLeaf {
		cn.Path = meta.path
		cn.PayloadSize = meta.payloadSize
	} else {
		cn.LeftChildIndex = lGlobal
		cn.RightChildIndex = rGlobal
	}

	it.logProgress(globalIndex)

	return it.fn(&cn)
}

// iterateTopTrie streams the top-trie part file: the subtrie-node count, then the
// top-level nodes (whose child indices are global), then the trie root records
// (each referencing its root node by global index). It mirrors readTopLevelTries
// (V6) / readTopLevelTriesV7 (V7) but extracts only per-node metadata and verifies
// the CRC32 checksum.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointIntegrity]: see [checkpointIterator.emit] and trie-root range checks.
func (it *checkpointIterator) iterateTopTrie(dir string, fileName string, isV7 bool, logger zerolog.Logger) error {
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
		if readSubtrieNodeCount != it.totalSub {
			return fmt.Errorf("mismatch subtrie node count, top trie file has %v, but subtrie footers sum to %v",
				readSubtrieNodeCount, it.totalSub)
		}

		scratch := make([]byte, 1024*4)

		// Top-level nodes: child indices are already global (0 = nil child).
		for j := uint64(1); j <= topLevelNodesCount; j++ {
			meta, err := readNodeMeta(reader, scratch, isV7)
			if err != nil {
				return fmt.Errorf("cannot read top-level node %d: %w", j, err)
			}
			globalIndex := it.totalSub + j
			if err := it.emit(meta, globalIndex, meta.lChild, meta.rChild); err != nil {
				return err
			}
		}

		// Trie root records: each references its root node by global index.
		for i := uint16(0); i < triesCount; i++ {
			var rootIndex uint64
			if isV7 {
				enc, err := payloadless.ReadEncodedTrie(reader, scratch)
				if err != nil {
					return fmt.Errorf("cannot read trie root record %d: %w", i, err)
				}
				rootIndex = enc.RootIndex
			} else {
				enc, err := flattener.ReadEncodedTrie(reader, scratch)
				if err != nil {
					return fmt.Errorf("cannot read trie root record %d: %w", i, err)
				}
				rootIndex = enc.RootIndex
			}
			if rootIndex > it.total {
				return fmt.Errorf("%w: trie root record %d references out-of-range node index %d (total %d)",
					ErrCheckpointIntegrity, i, rootIndex, it.total)
			}
			if rootIndex != 0 {
				it.referenced.set(rootIndex)
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

// nodeMeta holds the per-node fields decoded from the raw checkpoint byte stream.
// For interim nodes, lChild/rChild are the child indices exactly as stored (local
// to the subtrie file, or global in the top-trie file); the caller converts them
// as needed. For leaf nodes, lChild/rChild are 0.
type nodeMeta struct {
	isLeaf      bool
	height      uint16
	hash        hash.Hash
	path        ledger.Path
	payloadSize int
	lChild      uint64
	rChild      uint64
}

// readNodeMeta decodes one node from reader, extracting only the fields needed for
// iteration and integrity checking. It does NOT construct a node or resolve child
// references. Leaf payload bytes (V6) and optional leaf hashes (V7) are consumed
// from the reader — so the wrapping CRC32 reader still sees them — but discarded.
//
// scratch is a reusable buffer; if it is smaller than 1024 bytes a new buffer is
// allocated. The same scratch may be reused across calls.
//
// No error returns are expected during normal operation; all error returns indicate
// a malformed input stream or an IO failure.
func readNodeMeta(reader io.Reader, scratch []byte, isV7 bool) (nodeMeta, error) {
	const minBufSize = 1024
	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	if _, err := io.ReadFull(reader, scratch[:fixedNodePrefixSize]); err != nil {
		return nodeMeta{}, fmt.Errorf("cannot read node prefix: %w", err)
	}

	nType := scratch[0]
	height := binary.BigEndian.Uint16(scratch[encNodeTypeSize:])
	nodeHash, err := hash.ToHash(scratch[encNodeTypeSize+encHeightSize : fixedNodePrefixSize])
	if err != nil {
		return nodeMeta{}, fmt.Errorf("failed to decode node hash: %w", err)
	}

	switch nType {
	case interimNodeTypeByte:
		if _, err := io.ReadFull(reader, scratch[:2*encNodeIndexSize]); err != nil {
			return nodeMeta{}, fmt.Errorf("cannot read interim node child indices: %w", err)
		}
		return nodeMeta{
			isLeaf: false,
			height: height,
			hash:   nodeHash,
			lChild: binary.BigEndian.Uint64(scratch[:encNodeIndexSize]),
			rChild: binary.BigEndian.Uint64(scratch[encNodeIndexSize : 2*encNodeIndexSize]),
		}, nil

	case leafNodeTypeByte:
		if _, err := io.ReadFull(reader, scratch[:encPathSize]); err != nil {
			return nodeMeta{}, fmt.Errorf("cannot read leaf path: %w", err)
		}
		path, err := ledger.ToPath(scratch[:encPathSize])
		if err != nil {
			return nodeMeta{}, fmt.Errorf("failed to decode leaf path: %w", err)
		}

		meta := nodeMeta{isLeaf: true, height: height, hash: nodeHash, path: path}

		if isV7 {
			// V7 leaf: 1-byte leaf-hash flag, then an optional 32-byte leaf hash.
			if _, err := io.ReadFull(reader, scratch[:encLeafHashFlagSize]); err != nil {
				return nodeMeta{}, fmt.Errorf("cannot read leaf hash flag: %w", err)
			}
			switch scratch[0] {
			case 0: // leaf hash absent
			case 1: // leaf hash present: consume and discard 32 bytes
				if _, err := io.ReadFull(reader, scratch[:encHashSize]); err != nil {
					return nodeMeta{}, fmt.Errorf("cannot read leaf hash: %w", err)
				}
			default:
				return nodeMeta{}, fmt.Errorf("invalid leaf hash flag: %d", scratch[0])
			}
			// V7 leaves store no payload; payloadSize stays 0.
		} else {
			// V6 leaf: 4-byte encoded payload length, then that many payload bytes.
			if _, err := io.ReadFull(reader, scratch[:encPayloadLengthSize]); err != nil {
				return nodeMeta{}, fmt.Errorf("cannot read leaf payload length: %w", err)
			}
			size := binary.BigEndian.Uint32(scratch[:encPayloadLengthSize])
			meta.payloadSize = int(size)
			// Consume the payload through the reader (so the CRC sees it) without retaining it.
			if _, err := io.CopyN(io.Discard, reader, int64(size)); err != nil {
				return nodeMeta{}, fmt.Errorf("cannot read leaf payload: %w", err)
			}
		}

		return meta, nil

	default:
		return nodeMeta{}, fmt.Errorf("failed to decode node type %d", nType)
	}
}

// subtrieChildToGlobal converts a subtrie-file-local child index into the global
// index used by the integrity bitsets. A local index of 0 (nil child) maps to the
// global nil index 0.
func subtrieChildToGlobal(localChild uint64, offset uint64) uint64 {
	if localChild == 0 {
		return 0
	}
	return offset + localChild
}

// readCheckpointHeaderVersion opens the checkpoint header file and reads its
// magic+version bytes, returning the checkpoint version. It validates the magic
// bytes but performs no checksum verification (the per-version header reader does
// that during the main pass).
//
// No error returns are expected during normal operation.
func readCheckpointHeaderVersion(headerPath string) (uint16, error) {
	f, err := os.Open(headerPath)
	if err != nil {
		return 0, fmt.Errorf("could not open header file: %w", err)
	}
	defer f.Close()

	magic, version, err := readFileHeader(f)
	if err != nil {
		return 0, fmt.Errorf("could not read header magic and version: %w", err)
	}
	if magic != MagicBytesCheckpointHeader {
		return 0, fmt.Errorf("wrong magic bytes for checkpoint header, expect %#x, got %#x",
			MagicBytesCheckpointHeader, magic)
	}
	return version, nil
}

// readSubtrieNodeCountFromFooter opens the subtrie part file at the given index and
// reads its node count from the footer at the file tail (without scanning the nodes).
//
// No error returns are expected during normal operation.
func readSubtrieNodeCountFromFooter(logger zerolog.Logger, dir string, fileName string, index int) (uint64, error) {
	filepath, _, err := filePathSubTries(dir, fileName, index)
	if err != nil {
		return 0, err
	}
	var count uint64
	err = withFile(logger, filepath, func(f *os.File) error {
		c, _, err := readSubTriesFooter(f)
		if err != nil {
			return err
		}
		count = c
		return nil
	})
	return count, err
}

// readTopTrieNodeCountFromFooter opens the top-trie part file and reads its
// top-level node count from the footer at the file tail.
//
// No error returns are expected during normal operation.
func readTopTrieNodeCountFromFooter(logger zerolog.Logger, dir string, fileName string) (uint64, error) {
	filepath, _ := filePathTopTries(dir, fileName)
	var count uint64
	err := withFile(logger, filepath, func(f *os.File) error {
		c, _, _, err := readTopTriesFooter(f)
		if err != nil {
			return err
		}
		count = c
		return nil
	})
	return count, err
}

// bitset is a compact fixed-size set of bit flags indexed by node global index.
// It uses one bit per element (8x smaller than a []bool), which matters when the
// element count is the checkpoint's node count.
//
// NOT CONCURRENCY SAFE!
type bitset struct {
	words []uint64
}

// newBitset returns a bitset able to hold indices in the range [0, n).
func newBitset(n uint64) *bitset {
	return &bitset{words: make([]uint64, (n+63)/64)}
}

// set marks the bit at index i.
func (b *bitset) set(i uint64) {
	b.words[i>>6] |= 1 << (i & 63)
}

// get reports whether the bit at index i is set.
func (b *bitset) get(i uint64) bool {
	return b.words[i>>6]&(1<<(i&63)) != 0
}
