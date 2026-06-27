package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

type v7ToV6SubtrieResult struct {
	index    int
	checksum uint32
	err      error
}

// convertSubTriesV7ToV6Concurrently streams all subtrieCount subtrie part files
// through the V7→V6 conversion using up to nWorker goroutines, and returns the
// recomputed per-file checksums in subtrie-index order.
//
// Each worker builds the payload pool for its partition, converts that subtrie
// part file, then releases the pool before taking the next job, so peak memory
// is bounded by nWorker partitions' payloads.
//
// No error returns are expected during normal operation.
func convertSubTriesV7ToV6Concurrently(
	v7Dir string,
	v7File string,
	outputDir string,
	outputFile string,
	v7SubtrieChecksums []uint32,
	src payloadSource,
	logger zerolog.Logger,
	nWorker uint,
) ([]uint32, error) {
	jobs := make(chan int, subtrieCount)
	for i := 0; i < subtrieCount; i++ {
		jobs <- i
	}
	close(jobs)

	// Buffered to subtrieCount so workers never block on send, even if the
	// collector returns early after the first error.
	results := make(chan v7ToV6SubtrieResult, subtrieCount)

	for w := 0; w < int(nWorker); w++ {
		go func() {
			for i := range jobs {
				sum, err := convertSubTrieFileV7ToV6(
					v7Dir, v7File, outputDir, outputFile, i, v7SubtrieChecksums[i], src, logger)
				results <- v7ToV6SubtrieResult{index: i, checksum: sum, err: err}
			}
		}()
	}

	checksums := make([]uint32, subtrieCount)
	for k := 0; k < subtrieCount; k++ {
		r := <-results
		if r.err != nil {
			return nil, fmt.Errorf("fail to convert %v-th subtrie: %w", r.index, r.err)
		}
		checksums[r.index] = r.checksum
	}
	return checksums, nil
}

// convertSubTrieFileV7ToV6 builds the payload pool for partition `index`, then
// streams the V7 subtrie part file at that index, writing the reconstructed V6
// subtrie part file, and returns the recomputed checksum.
//
// expectedSum is the checksum recorded in the V7 header for this subtrie; it is
// verified against the checksum embedded in the V7 subtrie file before conversion.
//
// No error returns are expected during normal operation.
func convertSubTrieFileV7ToV6(
	v7Dir string,
	v7File string,
	outputDir string,
	outputFile string,
	index int,
	expectedSum uint32,
	src payloadSource,
	logger zerolog.Logger,
) (checksum uint32, errToReturn error) {
	// Build the leaf-hash → payload pool for this partition. It is released when
	// this function returns, before the worker takes the next partition, so peak
	// memory is bounded by nWorker partitions' payloads.
	pool, err := buildPartitionPayloadPool(index, src, logger)
	if err != nil {
		return 0, fmt.Errorf("could not build payload pool for partition %d: %w", index, err)
	}

	inPath, _, err := filePathSubTries(v7Dir, v7File, index)
	if err != nil {
		return 0, err
	}

	inFile, err := os.Open(inPath)
	if err != nil {
		return 0, fmt.Errorf("could not open V7 subtrie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	nodeCount, embeddedSum, err := readSubTriesFooter(inFile)
	if err != nil {
		return 0, fmt.Errorf("could not read V7 subtrie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return 0, fmt.Errorf("mismatch checksum in V7 subtrie file %v: header has %v, file has %v",
			index, expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("could not seek to start of V7 subtrie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointSubtrie, VersionV7, inFile); err != nil {
		return 0, fmt.Errorf("invalid V7 subtrie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	closable, err := createWriterForSubtrie(outputDir, outputFile, logger, index)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for subtrie: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)
	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointSubtrie, VersionV6)); err != nil {
		return 0, fmt.Errorf("cannot write version into subtrie file: %w", err)
	}

	logging := logProgress(fmt.Sprintf("converting %v-th sub trie (V7→V6)", index), int(nodeCount), logger)
	conv := newV7ToV6NodeConverter(pool)
	for i := uint64(0); i < nodeCount; i++ {
		if err := conv.convertNode(reader, writer); err != nil {
			return 0, fmt.Errorf("cannot convert node %d of subtrie %d: %w", i, index, err)
		}
		logging(i)
	}

	sum, err := storeSubtrieFooter(nodeCount, writer)
	if err != nil {
		return 0, fmt.Errorf("could not store subtrie footer: %w", err)
	}
	return sum, nil
}

// convertTopTrieFileV7ToV6 streams the V7 top-trie part file, converting its
// top-level nodes (leaves sourced from topPool) and re-encoding each trie root
// record to add back V6's register-size field (written as 0, see below), and
// returns the recomputed checksum.
//
// regSize is metrics-only and not recoverable from a V7 checkpoint, so it is
// written as 0; a warning is logged once.
//
// expectedSum is the top-trie checksum recorded in the V7 header; it is verified
// against the checksum embedded in the V7 top-trie file before conversion.
//
// No error returns are expected during normal operation.
func convertTopTrieFileV7ToV6(
	v7Dir string,
	v7File string,
	outputDir string,
	outputFile string,
	expectedSum uint32,
	topPool map[hash.Hash]*ledger.Payload,
	logger zerolog.Logger,
) (checksum uint32, errToReturn error) {
	inPath, _ := filePathTopTries(v7Dir, v7File)

	inFile, err := os.Open(inPath)
	if err != nil {
		return 0, fmt.Errorf("could not open V7 top-trie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	topLevelNodesCount, triesCount, embeddedSum, err := readTopTriesFooter(inFile)
	if err != nil {
		return 0, fmt.Errorf("could not read V7 top-trie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return 0, fmt.Errorf("mismatch V7 top-trie checksum: header has %v, file has %v",
			expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("could not seek to start of V7 top-trie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV7, inFile); err != nil {
		return 0, fmt.Errorf("invalid V7 top-trie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	// Read the subtrie node count and carry it over verbatim (unchanged by conversion).
	subtrieNodeCountBuf := make([]byte, encNodeCountSize)
	if _, err := io.ReadFull(reader, subtrieNodeCountBuf); err != nil {
		return 0, fmt.Errorf("could not read subtrie node count: %w", err)
	}

	closable, err := createWriterForTopTries(outputDir, outputFile, logger)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for top tries: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)
	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointToptrie, VersionV6)); err != nil {
		return 0, fmt.Errorf("cannot write version into top-trie file: %w", err)
	}
	if _, err := writer.Write(subtrieNodeCountBuf); err != nil {
		return 0, fmt.Errorf("cannot write subtrie node count: %w", err)
	}

	// Convert the top-level nodes (above subtrieLevel).
	conv := newV7ToV6NodeConverter(topPool)
	for i := uint64(0); i < topLevelNodesCount; i++ {
		if err := conv.convertNode(reader, writer); err != nil {
			return 0, fmt.Errorf("cannot convert top-level node %d: %w", i, err)
		}
	}

	if triesCount > 0 {
		logger.Warn().Msgf("regSize (AllocatedRegSize) is not reconstructed from a V7 checkpoint; "+
			"writing 0 for all %d trie root record(s). This is a metrics-only field and does not "+
			"affect trie root hashes.", triesCount)
	}

	// Re-encode each trie root record from V7 (index + regCount + hash) to V6
	// (index + regCount + regSize + hash), adding back the register-size field as 0.
	readScratch := make([]byte, payloadless.EncodedTrieSize)
	trieBuf := make([]byte, flattener.EncodedTrieSize)
	for i := uint16(0); i < triesCount; i++ {
		encTrie, err := payloadless.ReadEncodedTrie(reader, readScratch)
		if err != nil {
			return 0, fmt.Errorf("cannot read trie root record %d: %w", i, err)
		}

		pos := 0
		binary.BigEndian.PutUint64(trieBuf[pos:], encTrie.RootIndex)
		pos += encNodeIndexSize
		binary.BigEndian.PutUint64(trieBuf[pos:], encTrie.RegCount)
		pos += encNodeIndexSize
		binary.BigEndian.PutUint64(trieBuf[pos:], 0) // regSize: not reconstructed
		pos += encNodeIndexSize
		copy(trieBuf[pos:], encTrie.RootHash[:])

		if _, err := writer.Write(trieBuf); err != nil {
			return 0, fmt.Errorf("cannot write converted trie root record %d: %w", i, err)
		}
	}

	sum, err := storeTopLevelTrieFooter(topLevelNodesCount, triesCount, writer)
	if err != nil {
		return 0, fmt.Errorf("could not store top-trie footer: %w", err)
	}
	return sum, nil
}

// v7ToV6NodeConverter streams individual V7-encoded nodes into V6-encoded nodes,
// reusing internal scratch buffers across calls to avoid per-node allocations.
// Leaf nodes are reconstructed by looking up their payload in `pool` by the
// stored leaf hash; the node hash is carried over verbatim.
//
// NOT CONCURRENCY SAFE! A single converter must be used by one goroutine at a time.
type v7ToV6NodeConverter struct {
	pool       map[hash.Hash]*ledger.Payload
	prefix     []byte // node type + height + hash (fixedNodePrefixSize)
	childIndex []byte // interim left + right child indices
	path       []byte // leaf path
	leafHash   []byte // leaf hash bytes
	enc        []byte // scratch for the V6 leaf encoding
}

// newV7ToV6NodeConverter returns a converter with preallocated scratch buffers.
func newV7ToV6NodeConverter(pool map[hash.Hash]*ledger.Payload) *v7ToV6NodeConverter {
	return &v7ToV6NodeConverter{
		pool:       pool,
		prefix:     make([]byte, fixedNodePrefixSize),
		childIndex: make([]byte, 2*encNodeIndexSize),
		path:       make([]byte, encPathSize),
		leafHash:   make([]byte, encHashSize),
		enc:        make([]byte, 1024*4),
	}
}

// convertNode reads one V7-encoded node from reader and writes its V6 encoding to
// writer. Interim nodes are copied verbatim (their on-disk format is identical in
// V6); leaf nodes have their payload re-sourced from the pool and are re-encoded
// with the V6 (full-payload) leaf format.
//
// Expected error returns during normal operation: none. An unmatched leaf hash
// is treated as an exception (the source data is incomplete or inconsistent).
func (c *v7ToV6NodeConverter) convertNode(reader io.Reader, writer io.Writer) error {
	if _, err := io.ReadFull(reader, c.prefix); err != nil {
		return fmt.Errorf("cannot read node prefix: %w", err)
	}

	switch c.prefix[0] {
	case interimNodeTypeByte:
		// Interim node: read the two child indices and copy the whole record verbatim.
		if _, err := io.ReadFull(reader, c.childIndex); err != nil {
			return fmt.Errorf("cannot read interim node child indices: %w", err)
		}
		if _, err := writer.Write(c.prefix); err != nil {
			return fmt.Errorf("cannot write interim node prefix: %w", err)
		}
		if _, err := writer.Write(c.childIndex); err != nil {
			return fmt.Errorf("cannot write interim node child indices: %w", err)
		}
		return nil

	case leafNodeTypeByte:
		return c.convertLeaf(reader, writer)

	default:
		return fmt.Errorf("failed to decode node type %d", c.prefix[0])
	}
}

// convertLeaf reads the remainder of a V7 leaf node (path + leaf-hash flag +
// optional leaf hash) from reader, having already consumed the shared prefix into
// c.prefix, looks up the matching payload, and writes the reconstructed V6 leaf
// node (with full payload) to writer.
//
// The node hash is carried over verbatim from the V7 stream; correctness of the
// match is guaranteed because the pool is keyed by HashLeaf(path, value), which
// is exactly the V7 leaf hash. Use checkpoint-verify-hash to independently
// re-derive and verify node hashes from the reconstructed payloads.
//
// Expected error returns during normal operation: none. A missing payload for a
// present leaf hash is treated as an exception.
func (c *v7ToV6NodeConverter) convertLeaf(reader io.Reader, writer io.Writer) error {
	height := binary.BigEndian.Uint16(c.prefix[encNodeTypeSize:])
	nodeHash, err := hash.ToHash(c.prefix[encNodeTypeSize+encHeightSize:])
	if err != nil {
		return fmt.Errorf("failed to decode leaf node hash: %w", err)
	}

	// Read path (32 bytes).
	if _, err := io.ReadFull(reader, c.path); err != nil {
		return fmt.Errorf("cannot read leaf path: %w", err)
	}
	path, err := ledger.ToPath(c.path)
	if err != nil {
		return fmt.Errorf("failed to decode leaf path: %w", err)
	}

	// Read the leaf-hash presence flag (1 byte).
	var flagBuf [encLeafHashFlagSize]byte
	if _, err := io.ReadFull(reader, flagBuf[:]); err != nil {
		return fmt.Errorf("cannot read leaf hash flag: %w", err)
	}

	var payload *ledger.Payload
	switch flagBuf[0] {
	case leafHashAbsentFlag:
		// Unallocated leaf: reconstruct an empty-payload V6 leaf with the
		// preserved (default-for-height) node hash.
		//
		// We must NOT use ledger.EmptyPayload() here: it leaves the encoded key
		// nil, which makes the V6 leaf encoding self-inconsistent (the length
		// prefix, derived from the decoded key, overcounts the bytes actually
		// written) and the resulting part file cannot be read back. An explicit
		// empty key round-trips correctly. Unallocated leaves are rare — pruned
		// checkpoints (the production norm) contain none — and the register has
		// no meaningful key, so an empty key is the faithful representation.
		payload = ledger.NewPayload(ledger.NewKey(nil), ledger.Value{})

	case leafHashPresentFlag:
		if _, err := io.ReadFull(reader, c.leafHash); err != nil {
			return fmt.Errorf("cannot read leaf hash: %w", err)
		}
		leafHash, err := hash.ToHash(c.leafHash)
		if err != nil {
			return fmt.Errorf("failed to decode leaf hash: %w", err)
		}
		payload = c.pool[leafHash]
		if payload == nil {
			return fmt.Errorf("no payload found for leaf hash %x at path %x; "+
				"the previous checkpoint and WAL range do not contain this register's value",
				leafHash, path)
		}

	default:
		return fmt.Errorf("invalid leaf hash flag: %d", flagBuf[0])
	}

	// Carry the node hash over verbatim (see method doc for the correctness argument).
	v6leaf := node.NewNode(int(height), nil, nil, path, payload, nodeHash)
	encoded := flattener.EncodeNode(v6leaf, 0, 0, c.enc)
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("cannot write reconstructed leaf node: %w", err)
	}
	return nil
}

// buildPartitionPayloadPool builds the leaf-hash → payload pool for the given
// partition (first path nibble) by scanning the previous checkpoint's subtrie
// part file for that partition and re-scanning the WAL segment range, keeping
// only pairs whose path falls in the partition.
//
// Memory is bounded by this single partition's payloads.
//
// TODO(perf): the WAL range (and the previous checkpoint's top-trie) is
// re-scanned once per partition (16 scans total) to keep peak memory minimal. A
// future optimization is to make a single WAL pass that splits updates into 16
// on-disk partition buckets, then read each bucket once. The on-disk persistence
// format for those buckets is not yet decided.
//
// No error returns are expected during normal operation.
func buildPartitionPayloadPool(
	partition int,
	src payloadSource,
	logger zerolog.Logger,
) (map[hash.Hash]*ledger.Payload, error) {
	pool := make(map[hash.Hash]*ledger.Payload)

	add := func(path ledger.Path, payload *ledger.Payload) {
		if payload.IsEmpty() {
			return
		}
		leafHash := hash.HashLeaf(hash.Hash(path), payload.Value())
		pool[leafHash] = payload
	}

	partitionFilteredAdd := func(path ledger.Path, payload *ledger.Payload) {
		if int(path[0]>>4) != partition {
			return
		}
		add(path, payload)
	}

	// Source A: the previous checkpoint's subtrie part file for this partition.
	// All of its leaves belong to this partition by construction.
	err := streamV6SubtrieLeaves(src.execDir, src.prevFile, partition,
		src.prevSubtrieChecksums[partition], func(path ledger.Path, payload *ledger.Payload) {
			add(path, payload)
		})
	if err != nil {
		return nil, fmt.Errorf("could not scan previous checkpoint subtrie %d: %w", partition, err)
	}

	// Source A': the previous checkpoint's top-trie part file. A register that was
	// a compactified leaf high in the previous trie (above the subtrie split) lives
	// in the top-trie file rather than in subtrie `partition`. A later trie may
	// de-compactify that same register into this partition's subtrie (e.g. once a
	// sibling register is added), so its payload must be sourced here too. The
	// top-trie file is small, so scanning it per partition is cheap.
	err = streamV6TopTrieLeaves(src.execDir, src.prevFile, src.prevTopTrieChecksum, partitionFilteredAdd)
	if err != nil {
		return nil, fmt.Errorf("could not scan previous checkpoint top-trie for partition %d: %w", partition, err)
	}

	// Source B: WAL updates in this partition.
	if src.walFrom <= src.walTo {
		err = scanWALUpdates(src.execDir, src.walFrom, src.walTo, partitionFilteredAdd)
		if err != nil {
			return nil, fmt.Errorf("could not scan WAL for partition %d: %w", partition, err)
		}
	}

	logger.Debug().Int("partition", partition).Int("pool_size", len(pool)).
		Msg("built partition payload pool")
	return pool, nil
}

// buildTopTriePayloadPool collects payloads for any leaf nodes stored in the V7
// top-trie part file (registers that sit above the subtrie split). Their paths
// may belong to any partition, so they are sourced by first collecting the set of
// leaf hashes referenced by the top-trie, then scanning all previous-checkpoint
// part files and the WAL range for matching payloads.
//
// For dense state (e.g. mainnet) the top-trie has no leaf nodes and this returns
// an empty pool without scanning any source.
//
// No error returns are expected during normal operation.
func buildTopTriePayloadPool(
	v7Dir string,
	v7File string,
	v7TopTrieChecksum uint32,
	src payloadSource,
	logger zerolog.Logger,
) (map[hash.Hash]*ledger.Payload, error) {
	needed, err := collectTopTrieLeafHashes(v7Dir, v7File, v7TopTrieChecksum)
	if err != nil {
		return nil, fmt.Errorf("could not collect top-trie leaf hashes: %w", err)
	}

	pool := make(map[hash.Hash]*ledger.Payload)
	if len(needed) == 0 {
		return pool, nil
	}

	logger.Info().Int("needed", len(needed)).
		Msg("V7 top-trie contains leaf nodes; scanning all sources for their payloads")

	add := func(path ledger.Path, payload *ledger.Payload) {
		if payload.IsEmpty() {
			return
		}
		leafHash := hash.HashLeaf(hash.Hash(path), payload.Value())
		if _, ok := needed[leafHash]; !ok {
			return
		}
		pool[leafHash] = payload
	}

	// Scan every previous-checkpoint subtrie part file.
	for i := 0; i < subtrieCount; i++ {
		err := streamV6SubtrieLeaves(src.execDir, src.prevFile, i, src.prevSubtrieChecksums[i], add)
		if err != nil {
			return nil, fmt.Errorf("could not scan previous checkpoint subtrie %d: %w", i, err)
		}
	}
	// Scan the previous-checkpoint top-trie part file leaves.
	if err := streamV6TopTrieLeaves(src.execDir, src.prevFile, src.prevTopTrieChecksum, add); err != nil {
		return nil, fmt.Errorf("could not scan previous checkpoint top-trie: %w", err)
	}
	// Scan the WAL range.
	if src.walFrom <= src.walTo {
		if err := scanWALUpdates(src.execDir, src.walFrom, src.walTo, add); err != nil {
			return nil, fmt.Errorf("could not scan WAL for top-trie leaves: %w", err)
		}
	}

	if len(pool) < len(needed) {
		return nil, fmt.Errorf("could not source all top-trie leaf payloads: found %d of %d",
			len(pool), len(needed))
	}
	return pool, nil
}

// collectTopTrieLeafHashes streams the V7 top-trie part file's top-level nodes
// and returns the set of leaf hashes for the leaf nodes among them. Leaf nodes
// with an absent leaf hash (unallocated) need no payload and are skipped.
//
// No error returns are expected during normal operation.
func collectTopTrieLeafHashes(
	v7Dir string,
	v7File string,
	expectedSum uint32,
) (set map[hash.Hash]struct{}, errToReturn error) {
	inPath, _ := filePathTopTries(v7Dir, v7File)
	inFile, err := os.Open(inPath)
	if err != nil {
		return nil, fmt.Errorf("could not open V7 top-trie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	topLevelNodesCount, _, embeddedSum, err := readTopTriesFooter(inFile)
	if err != nil {
		return nil, fmt.Errorf("could not read V7 top-trie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return nil, fmt.Errorf("mismatch V7 top-trie checksum: header has %v, file has %v", expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek to start of V7 top-trie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV7, inFile); err != nil {
		return nil, fmt.Errorf("invalid V7 top-trie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	// Skip the subtrie node count.
	if _, err := io.CopyN(io.Discard, reader, int64(encNodeCountSize)); err != nil {
		return nil, fmt.Errorf("could not skip subtrie node count: %w", err)
	}

	set = make(map[hash.Hash]struct{})
	prefix := make([]byte, fixedNodePrefixSize)
	childIndex := make([]byte, 2*encNodeIndexSize)
	pathBuf := make([]byte, encPathSize)
	leafHashBuf := make([]byte, encHashSize)
	var flagBuf [encLeafHashFlagSize]byte

	for i := uint64(0); i < topLevelNodesCount; i++ {
		if _, err := io.ReadFull(reader, prefix); err != nil {
			return nil, fmt.Errorf("cannot read node prefix: %w", err)
		}
		switch prefix[0] {
		case interimNodeTypeByte:
			if _, err := io.ReadFull(reader, childIndex); err != nil {
				return nil, fmt.Errorf("cannot read interim child indices: %w", err)
			}
		case leafNodeTypeByte:
			if _, err := io.ReadFull(reader, pathBuf); err != nil {
				return nil, fmt.Errorf("cannot read leaf path: %w", err)
			}
			if _, err := io.ReadFull(reader, flagBuf[:]); err != nil {
				return nil, fmt.Errorf("cannot read leaf hash flag: %w", err)
			}
			switch flagBuf[0] {
			case leafHashAbsentFlag:
				// unallocated, no payload needed
			case leafHashPresentFlag:
				if _, err := io.ReadFull(reader, leafHashBuf); err != nil {
					return nil, fmt.Errorf("cannot read leaf hash: %w", err)
				}
				leafHash, err := hash.ToHash(leafHashBuf)
				if err != nil {
					return nil, fmt.Errorf("failed to decode leaf hash: %w", err)
				}
				set[leafHash] = struct{}{}
			default:
				return nil, fmt.Errorf("invalid leaf hash flag: %d", flagBuf[0])
			}
		default:
			return nil, fmt.Errorf("failed to decode node type %d", prefix[0])
		}
	}

	return set, nil
}

// streamV6SubtrieLeaves opens the V6 subtrie part file at the given index and
// invokes cb for every leaf node, passing its path and decoded payload. Interim
// nodes are skipped. The embedded checksum is verified against expectedSum.
//
// No error returns are expected during normal operation.
func streamV6SubtrieLeaves(
	dir string,
	file string,
	index int,
	expectedSum uint32,
	cb func(path ledger.Path, payload *ledger.Payload),
) (errToReturn error) {
	inPath, _, err := filePathSubTries(dir, file, index)
	if err != nil {
		return err
	}
	inFile, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("could not open V6 subtrie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	nodeCount, embeddedSum, err := readSubTriesFooter(inFile)
	if err != nil {
		return fmt.Errorf("could not read V6 subtrie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return fmt.Errorf("mismatch checksum in V6 subtrie file %v: header has %v, file has %v",
			index, expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("could not seek to start of V6 subtrie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointSubtrie, VersionV6, inFile); err != nil {
		return fmt.Errorf("invalid V6 subtrie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	return streamV6LeafNodes(reader, nodeCount, cb)
}

// streamV6TopTrieLeaves opens the V6 top-trie part file and invokes cb for every
// leaf node among its top-level nodes. Interim nodes and trie root records are
// skipped. The embedded checksum is verified against expectedSum.
//
// No error returns are expected during normal operation.
func streamV6TopTrieLeaves(
	dir string,
	file string,
	expectedSum uint32,
	cb func(path ledger.Path, payload *ledger.Payload),
) (errToReturn error) {
	inPath, _ := filePathTopTries(dir, file)
	inFile, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("could not open V6 top-trie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	topLevelNodesCount, _, embeddedSum, err := readTopTriesFooter(inFile)
	if err != nil {
		return fmt.Errorf("could not read V6 top-trie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return fmt.Errorf("mismatch V6 top-trie checksum: header has %v, file has %v", expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("could not seek to start of V6 top-trie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV6, inFile); err != nil {
		return fmt.Errorf("invalid V6 top-trie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	// Skip the subtrie node count.
	if _, err := io.CopyN(io.Discard, reader, int64(encNodeCountSize)); err != nil {
		return fmt.Errorf("could not skip subtrie node count: %w", err)
	}

	return streamV6LeafNodes(reader, topLevelNodesCount, cb)
}

// streamV6LeafNodes reads nodeCount V6-encoded nodes from reader and invokes cb
// for each leaf node with its path and decoded payload. Interim nodes are skipped.
//
// No error returns are expected during normal operation.
func streamV6LeafNodes(
	reader io.Reader,
	nodeCount uint64,
	cb func(path ledger.Path, payload *ledger.Payload),
) error {
	prefix := make([]byte, fixedNodePrefixSize)
	childIndex := make([]byte, 2*encNodeIndexSize)
	pathBuf := make([]byte, encPathSize)
	lenBuf := make([]byte, encPayloadLengthSize)
	payloadBuf := make([]byte, 1024)

	for i := uint64(0); i < nodeCount; i++ {
		if _, err := io.ReadFull(reader, prefix); err != nil {
			return fmt.Errorf("cannot read node prefix: %w", err)
		}
		switch prefix[0] {
		case interimNodeTypeByte:
			if _, err := io.ReadFull(reader, childIndex); err != nil {
				return fmt.Errorf("cannot read interim child indices: %w", err)
			}
		case leafNodeTypeByte:
			if _, err := io.ReadFull(reader, pathBuf); err != nil {
				return fmt.Errorf("cannot read leaf path: %w", err)
			}
			path, err := ledger.ToPath(pathBuf)
			if err != nil {
				return fmt.Errorf("failed to decode leaf path: %w", err)
			}
			if _, err := io.ReadFull(reader, lenBuf); err != nil {
				return fmt.Errorf("cannot read leaf payload length: %w", err)
			}
			size := binary.BigEndian.Uint32(lenBuf)
			if uint32(cap(payloadBuf)) < size {
				payloadBuf = make([]byte, size)
			}
			buf := payloadBuf[:size]
			if _, err := io.ReadFull(reader, buf); err != nil {
				return fmt.Errorf("cannot read leaf payload: %w", err)
			}
			// zeroCopy=false returns a copy, so reusing payloadBuf next iteration is safe.
			payload, err := ledger.DecodePayloadWithoutPrefix(buf, false, payloadEncodingVersion)
			if err != nil {
				return fmt.Errorf("failed to decode leaf payload: %w", err)
			}
			cb(path, payload)
		default:
			return fmt.Errorf("failed to decode node type %d", prefix[0])
		}
	}
	return nil
}

// scanWALUpdates reads the WAL segment records in the inclusive range [from, to]
// and invokes cb for every (path, payload) pair in each update record. Delete
// records are ignored.
//
// Each payload passed to cb is a deep copy. This is required because
// [ledger.DecodeTrieUpdate] decodes payloads zero-copy over the WAL record
// buffer, which the underlying reader reuses on the next record; callers that
// retain the payload (as the pool builders do) would otherwise observe corruption.
//
// No error returns are expected during normal operation.
func scanWALUpdates(
	execDir string,
	from int,
	to int,
	cb func(path ledger.Path, payload *ledger.Payload),
) error {
	sr, err := prometheusWAL.NewSegmentsRangeReader(zerolog.Nop(), prometheusWAL.SegmentRange{
		Dir:   execDir,
		First: from,
		Last:  to,
	})
	if err != nil {
		return fmt.Errorf("cannot create WAL segment reader for [%d, %d]: %w", from, to, err)
	}
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)
	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode WAL record: %w", err)
		}
		if operation != WALUpdate {
			continue
		}
		for i, path := range update.Paths {
			// Deep copy: payloads are decoded zero-copy over the reusable WAL
			// record buffer, so a retained reference would be corrupted by the
			// next read.
			cb(path, update.Payloads[i].DeepCopy())
		}
	}
	if err := reader.Err(); err != nil {
		return fmt.Errorf("cannot read WAL: %w", err)
	}
	return nil
}

// verifyRootHashesMatch reads the trie root hashes from the reconstructed V6
// checkpoint and the source V7 checkpoint and verifies they are identical.
//
// No error returns are expected during normal operation.
func verifyRootHashesMatch(
	v7Dir string,
	v7File string,
	v6Dir string,
	v6File string,
	logger zerolog.Logger,
) error {
	v7Hashes, err := ReadTriesRootHashV7(logger, v7Dir, v7File)
	if err != nil {
		return fmt.Errorf("could not read V7 root hashes: %w", err)
	}
	v6Hashes, err := ReadTriesRootHash(logger, v6Dir, v6File)
	if err != nil {
		return fmt.Errorf("could not read reconstructed V6 root hashes: %w", err)
	}
	if len(v7Hashes) != len(v6Hashes) {
		return fmt.Errorf("trie count mismatch: V7 has %d, V6 has %d", len(v7Hashes), len(v6Hashes))
	}
	for i := range v7Hashes {
		if v7Hashes[i] != v6Hashes[i] {
			return fmt.Errorf("trie %d root hash mismatch: V7=%s V6=%s", i, v7Hashes[i], v6Hashes[i])
		}
	}
	logger.Info().Int("trie_count", len(v6Hashes)).Msg("verified reconstructed V6 root hashes match V7")
	return nil
}
