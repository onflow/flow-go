package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// Encoded node field sizes shared by the V6 and V7 on-disk node formats. They
// mirror the (unexported) constants in the mtrie/flattener and payloadless
// flatteners; they are duplicated here because the streaming converter operates
// on the raw byte stream rather than through either flattener.
const (
	encNodeTypeSize      = 1
	encHeightSize        = 2
	encHashSize          = hash.HashLen
	encPathSize          = ledger.PathLen
	encNodeIndexSize     = 8
	encPayloadLengthSize = 4

	// fixedNodePrefixSize is the size of the leading bytes shared by every
	// encoded node (leaf or interim): node type + height + node hash.
	fixedNodePrefixSize = encNodeTypeSize + encHeightSize + encHashSize

	// leafNodeTypeByte and interimNodeTypeByte are the node-type tags. They are
	// identical in the V6 and V7 encodings, so an interim node's bytes can be
	// copied verbatim.
	leafNodeTypeByte    = byte(0)
	interimNodeTypeByte = byte(1)

	// payloadEncodingVersion is the payload encoding version used by the V6
	// leaf node encoding.
	payloadEncodingVersion = 1
)

// ConvertCheckpointV6ToV7Stream converts a V6 checkpoint at (inputDir, inputFileName)
// into a V7 (payloadless) checkpoint at (outputDir, outputFileName) by streaming
// each part file node-by-node, without ever materializing the full trie forest in
// memory.
//
// How it works:
//   - The V6 and V7 on-disk layouts are byte-identical except for (a) the version
//     bytes in every part file, (b) the leaf node encoding — V6 stores the full
//     payload, V7 stores a 32-byte leaf hash — and (c) the trie root records in the
//     top-trie part file, where V7 drops V6's 8-byte allocated-register-size field.
//     Interim nodes are byte-identical.
//   - Each of the 16 subtrie part files is a pure node stream: interim nodes are
//     copied verbatim and leaf nodes are projected to their payloadless form.
//   - The top-trie part file additionally re-encodes each trie root record to drop
//     the register-size field.
//   - Node count and ordering are unchanged by the conversion, so every interim
//     node's child indices remain valid without rewriting.
//   - Per-part-file CRC32 checksums are recomputed during the write and collected
//     into a freshly written V7 header.
//
// Peak memory is independent of checkpoint size: a single node plus reusable
// scratch buffers per part file. The 16 subtrie part files are converted in
// parallel using up to nWorker goroutines; valid range is [1, subtrieCount].
//
// Unlike [ConvertCheckpointV6ToV7], this function does not load the forest and
// therefore does not re-derive or cross-check trie root hashes. Node hashes are
// carried over verbatim from the V6 stream, so root hashes are structurally
// preserved.
//
// When verifyLeafHash is true, every allocated leaf is additionally checked: the
// node hash derived from (path, value) is compared against the leaf's V6 node hash
// read from disk, and a mismatch aborts the conversion. This is the streaming,
// per-leaf equivalent of the root-hash cross-check performed by the in-memory
// [ConvertCheckpointV6ToV7] (a wrong leaf hash would otherwise only surface as a
// root-hash mismatch when the forest is loaded). It adds a hash recomputation per
// allocated leaf but no extra memory.
//
// The output filename must carry the V7 suffix and no output part file may already
// exist; otherwise the call is rejected. On any failure, partially written output
// files are removed.
//
// No error returns are expected during normal operation; all error returns indicate
// a malformed input, a clobbering output, an IO failure, or a leaf-hash mismatch
// when verifyLeafHash is enabled.
func ConvertCheckpointV6ToV7Stream(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	logger zerolog.Logger,
	nWorker uint,
	verifyLeafHash bool,
) error {
	err := convertCheckpointV6ToV7Stream(inputDir, inputFileName, outputDir, outputFileName, logger, nWorker, verifyLeafHash)
	if err != nil {
		cleanupErr := deleteCheckpointFiles(outputDir, outputFileName)
		if cleanupErr != nil {
			return fmt.Errorf("fail to cleanup temp file %s, after running into error: %w", cleanupErr, err)
		}
		return err
	}
	return nil
}

func convertCheckpointV6ToV7Stream(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	logger zerolog.Logger,
	nWorker uint,
	verifyLeafHash bool,
) error {
	if nWorker == 0 || nWorker > subtrieCount {
		return fmt.Errorf("invalid nWorker %v, valid range is [1, %v]", nWorker, subtrieCount)
	}

	// Reject obvious filename misuse so converted files can coexist with the V6 source.
	if err := requireV7Filename(outputFileName); err != nil {
		return err
	}

	// Validate V6 input exists (header + part files).
	v6Header := filePathCheckpointHeader(inputDir, inputFileName)
	if _, err := os.Stat(v6Header); err != nil {
		return fmt.Errorf("V6 checkpoint header not found at %s: %w", v6Header, err)
	}
	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(v6Header, logger)
	if err != nil {
		return fmt.Errorf("could not read V6 checkpoint header: %w", err)
	}
	if err := allPartFileExist(inputDir, inputFileName, len(subtrieChecksums)); err != nil {
		return fmt.Errorf("V6 part files incomplete for %s/%s: %w", inputDir, inputFileName, err)
	}

	// Validate V7 output is not present (any of the part files).
	v7Existing, err := findCheckpointPartFiles(outputDir, outputFileName)
	if err != nil {
		return fmt.Errorf("could not check existing V7 output files: %w", err)
	}
	if len(v7Existing) != 0 {
		return fmt.Errorf("V7 output already exists: %v", v7Existing)
	}

	// Remove any leftover temp part files from a previously interrupted conversion
	// to this output; they are never reused and would otherwise accumulate.
	if err := removeStaleTempFiles(outputDir, outputFileName, logger); err != nil {
		return fmt.Errorf("could not remove stale temp files: %w", err)
	}

	logger.Info().
		Str("v6_dir", inputDir).
		Str("v6_file", inputFileName).
		Str("v7_dir", outputDir).
		Str("v7_file", outputFileName).
		Uint("nworker", nWorker).
		Bool("verify_leaf_hash", verifyLeafHash).
		Msg("starting streaming V6→V7 checkpoint conversion")

	// Convert the 16 subtrie part files concurrently, recomputing each checksum.
	newSubtrieChecksums, err := convertSubTriesV6ToV7StreamConcurrently(
		inputDir, inputFileName, outputDir, outputFileName, subtrieChecksums, logger, nWorker, verifyLeafHash)
	if err != nil {
		return fmt.Errorf("could not convert subtrie files: %w", err)
	}

	// Convert the top-trie part file.
	newTopTrieChecksum, err := convertTopTrieFileV6ToV7Stream(
		inputDir, inputFileName, outputDir, outputFileName, topTrieChecksum, logger, verifyLeafHash)
	if err != nil {
		return fmt.Errorf("could not convert top-trie file: %w", err)
	}

	// Write the V7 header referencing the freshly computed checksums.
	if err := storeCheckpointHeaderV7(newSubtrieChecksums, newTopTrieChecksum, outputDir, outputFileName, logger); err != nil {
		return fmt.Errorf("could not write V7 checkpoint header: %w", err)
	}

	logger.Info().Msg("stream V6→V7 checkpoint conversion complete")
	return nil
}

type streamSubtrieResult struct {
	index    int
	checksum uint32
	err      error
}

// convertSubTriesV6ToV7StreamConcurrently streams all subtrieCount subtrie part
// files through the V6→V7 conversion using up to nWorker goroutines, and returns
// the recomputed per-file checksums in subtrie-index order.
func convertSubTriesV6ToV7StreamConcurrently(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	subtrieChecksums []uint32,
	logger zerolog.Logger,
	nWorker uint,
	verifyLeafHash bool,
) ([]uint32, error) {
	jobs := make(chan int, subtrieCount)
	for i := 0; i < subtrieCount; i++ {
		jobs <- i
	}
	close(jobs)

	// Buffered to subtrieCount so workers never block on send, even if the
	// collector returns early after the first error.
	results := make(chan streamSubtrieResult, subtrieCount)

	for w := 0; w < int(nWorker); w++ {
		go func() {
			for i := range jobs {
				sum, err := convertSubTrieFileV6ToV7Stream(
					inputDir, inputFileName, outputDir, outputFileName, i, subtrieChecksums[i], logger, verifyLeafHash)
				results <- streamSubtrieResult{index: i, checksum: sum, err: err}
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

// convertSubTrieFileV6ToV7Stream streams the subtrie part file at the given index,
// writing the converted V7 subtrie part file, and returns the recomputed checksum.
//
// expectedSum is the checksum recorded in the V6 header for this subtrie; it is
// verified against the checksum embedded in the V6 subtrie file before conversion.
func convertSubTrieFileV6ToV7Stream(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	index int,
	expectedSum uint32,
	logger zerolog.Logger,
	verifyLeafHash bool,
) (checksum uint32, errToReturn error) {
	inPath, _, err := filePathSubTries(inputDir, inputFileName, index)
	if err != nil {
		return 0, err
	}

	inFile, err := os.Open(inPath)
	if err != nil {
		return 0, fmt.Errorf("could not open subtrie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	nodeCount, embeddedSum, err := readSubTriesFooter(inFile)
	if err != nil {
		return 0, fmt.Errorf("could not read subtrie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return 0, fmt.Errorf("mismatch checksum in subtrie file %v: header has %v, file has %v",
			index, expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("could not seek to start of subtrie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointSubtrie, VersionV6, inFile); err != nil {
		return 0, fmt.Errorf("invalid subtrie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	closable, err := createWriterForSubtrie(outputDir, outputFileName, logger, index)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for subtrie: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)
	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointSubtrie, VersionV7)); err != nil {
		return 0, fmt.Errorf("cannot write version into subtrie file: %w", err)
	}

	logging := logProgress(fmt.Sprintf("converting %v-th sub trie (streaming)", index), int(nodeCount), logger)
	conv := newV6ToV7NodeConverter(verifyLeafHash)
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

// convertTopTrieFileV6ToV7Stream streams the top-trie part file, converting its
// top-level nodes and re-encoding each trie root record to drop V6's register-size
// field, and returns the recomputed checksum.
//
// expectedSum is the top-trie checksum recorded in the V6 header; it is verified
// against the checksum embedded in the V6 top-trie file before conversion.
func convertTopTrieFileV6ToV7Stream(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	expectedSum uint32,
	logger zerolog.Logger,
	verifyLeafHash bool,
) (checksum uint32, errToReturn error) {
	inPath, _ := filePathTopTries(inputDir, inputFileName)

	inFile, err := os.Open(inPath)
	if err != nil {
		return 0, fmt.Errorf("could not open top-trie file %v: %w", inPath, err)
	}
	defer func() {
		errToReturn = closeAndMergeError(inFile, errToReturn)
	}()

	topLevelNodesCount, triesCount, embeddedSum, err := readTopTriesFooter(inFile)
	if err != nil {
		return 0, fmt.Errorf("could not read top-trie footer: %w", err)
	}
	if embeddedSum != expectedSum {
		return 0, fmt.Errorf("mismatch top-trie checksum: header has %v, file has %v",
			expectedSum, embeddedSum)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("could not seek to start of top-trie file: %w", err)
	}
	if err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV6, inFile); err != nil {
		return 0, fmt.Errorf("invalid top-trie file header: %w", err)
	}
	reader := bufio.NewReaderSize(inFile, defaultBufioReadSize)

	// Read the subtrie node count and carry it over verbatim (unchanged by conversion).
	subtrieNodeCountBuf := make([]byte, encNodeCountSize)
	if _, err := io.ReadFull(reader, subtrieNodeCountBuf); err != nil {
		return 0, fmt.Errorf("could not read subtrie node count: %w", err)
	}

	closable, err := createWriterForTopTries(outputDir, outputFileName, logger)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for top tries: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)
	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointToptrie, VersionV7)); err != nil {
		return 0, fmt.Errorf("cannot write version into top-trie file: %w", err)
	}
	if _, err := writer.Write(subtrieNodeCountBuf); err != nil {
		return 0, fmt.Errorf("cannot write subtrie node count: %w", err)
	}

	// Convert the top-level nodes (above subtrieLevel).
	conv := newV6ToV7NodeConverter(verifyLeafHash)
	for i := uint64(0); i < topLevelNodesCount; i++ {
		if err := conv.convertNode(reader, writer); err != nil {
			return 0, fmt.Errorf("cannot convert top-level node %d: %w", i, err)
		}
	}

	// Re-encode each trie root record from V6 (index + regCount + regSize + hash)
	// to V7 (index + regCount + hash), dropping the register-size field.
	readScratch := make([]byte, flattener.EncodedTrieSize)
	trieBuf := make([]byte, payloadless.EncodedTrieSize)
	for i := uint16(0); i < triesCount; i++ {
		encTrie, err := flattener.ReadEncodedTrie(reader, readScratch)
		if err != nil {
			return 0, fmt.Errorf("cannot read trie root record %d: %w", i, err)
		}

		pos := 0
		binary.BigEndian.PutUint64(trieBuf[pos:], encTrie.RootIndex)
		pos += encNodeIndexSize
		binary.BigEndian.PutUint64(trieBuf[pos:], encTrie.RegCount)
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

// v6ToV7NodeConverter streams individual V6-encoded nodes into V7-encoded nodes,
// reusing internal scratch buffers across calls to avoid per-node allocations.
//
// NOT CONCURRENCY SAFE! A single converter must be used by one goroutine at a time.
type v6ToV7NodeConverter struct {
	prefix     []byte // node type + height + hash (fixedNodePrefixSize)
	childIndex []byte // interim left + right child indices
	path       []byte // leaf path
	lenBuf     []byte // leaf payload length prefix
	payload    []byte // leaf payload bytes (grows as needed)
	enc        []byte // scratch for the payloadless leaf encoding

	// verifyLeafHash, when true, makes convertLeaf compare each derived V7 leaf
	// node hash against the V6 leaf node hash read from disk.
	verifyLeafHash bool
}

// newV6ToV7NodeConverter returns a converter with preallocated scratch buffers.
func newV6ToV7NodeConverter(verifyLeafHash bool) *v6ToV7NodeConverter {
	return &v6ToV7NodeConverter{
		prefix:         make([]byte, fixedNodePrefixSize),
		childIndex:     make([]byte, 2*encNodeIndexSize),
		path:           make([]byte, encPathSize),
		lenBuf:         make([]byte, encPayloadLengthSize),
		payload:        make([]byte, 1024),
		enc:            make([]byte, 1024*4),
		verifyLeafHash: verifyLeafHash,
	}
}

// convertNode reads one V6-encoded node from reader and writes its V7 encoding to
// writer. Interim nodes are copied verbatim (their on-disk format is identical in
// V7); leaf nodes are projected via [FromV6LeafNode] and re-encoded with the
// payloadless flattener.
//
// No error returns are expected during normal operation; all error returns indicate
// a malformed input stream or an IO failure.
func (c *v6ToV7NodeConverter) convertNode(reader io.Reader, writer io.Writer) error {
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

// convertLeaf reads the remainder of a V6 leaf node (path + payload) from reader,
// having already consumed the shared prefix into c.prefix, and writes its V7
// payloadless encoding to writer.
//
// When c.verifyLeafHash is set, the V7 node hash derived from (path, value) is
// compared against the V6 node hash read from disk, and a mismatch is reported as
// an error.
//
// No error returns are expected during normal operation; all error returns indicate
// a malformed input stream, an IO failure, or (when verifying) a leaf-hash mismatch.
func (c *v6ToV7NodeConverter) convertLeaf(reader io.Reader, writer io.Writer) error {
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

	// Read payload length prefix (4 bytes) and payload bytes.
	if _, err := io.ReadFull(reader, c.lenBuf); err != nil {
		return fmt.Errorf("cannot read leaf payload length: %w", err)
	}
	size := binary.BigEndian.Uint32(c.lenBuf)
	if uint32(cap(c.payload)) < size {
		c.payload = make([]byte, size)
	}
	payloadBuf := c.payload[:size]
	if _, err := io.ReadFull(reader, payloadBuf); err != nil {
		return fmt.Errorf("cannot read leaf payload: %w", err)
	}

	// DecodePayloadWithoutPrefix with zeroCopy=false returns a copy, so reusing
	// payloadBuf on the next iteration is safe.
	payload, err := ledger.DecodePayloadWithoutPrefix(payloadBuf, false, payloadEncodingVersion)
	if err != nil {
		return fmt.Errorf("failed to decode leaf payload: %w", err)
	}

	// Reuse the tested V6→V7 leaf projection to keep a single source of truth for
	// the leaf-hash / empty-payload handling.
	v6leaf := node.NewNode(int(height), nil, nil, path, payload, nodeHash)
	v7leaf, err := FromV6LeafNode(v6leaf)
	if err != nil {
		return fmt.Errorf("cannot convert leaf node: %w", err)
	}

	// Optionally verify that the V7 node hash derived from (path, value) matches
	// the V6 node hash carried on disk. This is the per-leaf, streaming equivalent
	// of the forest-level root-hash cross-check in ConvertCheckpointV6ToV7.
	if c.verifyLeafHash {
		if derived := v7leaf.Hash(); derived != nodeHash {
			return fmt.Errorf("leaf hash verification failed for path %x: derived node hash %x does not match V6 node hash %x",
				path, derived, nodeHash)
		}
	}

	encoded := payloadless.EncodeNode(v7leaf, 0, 0, c.enc)
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("cannot write converted leaf node: %w", err)
	}
	return nil
}
