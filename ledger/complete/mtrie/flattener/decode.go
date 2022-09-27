package flattener

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// DecodeNodeData reads the node data from reader, and uses scratch as reusable buffer for holding data read from disk.
// it returns raw data so that they are allocated on stack instead of heap, useful when reading tons of nodes from a
// checkpoint file.
// any error returned means the file is malformated.
func DecodeNodeData(reader io.Reader, scratch []byte) (
	isLeaf bool,
	height uint16,
	lchildIndex uint64,
	rchildIndex uint64,
	path ledger.Path,
	payload *ledger.Payload,
	nodeHash hash.Hash,
	err error) {

	// minBufSize should be large enough for interim node and leaf node with small payload.
	// minBufSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected.  len(scratch) is 4096 by default, so minBufSize isn't likely to be used.
	const minBufSize = 1024

	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// fixLengthSize is the size of shared data of leaf node and interim node
	const fixLengthSize = encNodeTypeSize + encHeightSize + encHashSize

	_, err = io.ReadFull(reader, scratch[:fixLengthSize])
	if err != nil {
		return false, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to read fixed-length part of serialized node: %w", err)
	}

	pos := 0

	// Decode node type (1 byte)
	nType := scratch[pos]
	pos += encNodeTypeSize

	if nType != byte(leafNodeType) && nType != byte(interimNodeType) {
		return false, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to decode node type %d", nType)
	}

	// Decode height (2 bytes)
	height = binary.BigEndian.Uint16(scratch[pos:])
	pos += encHeightSize

	// Decode and create hash.Hash (32 bytes)
	nodeHash, err = hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return false, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to decode hash of serialized node: %w", err)
	}

	if nType == byte(leafNodeType) {

		// Read path (32 bytes)
		encPath := scratch[:encPathSize]
		_, err := io.ReadFull(reader, encPath)
		if err != nil {
			return false, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to read path of serialized node: %w", err)
		}

		// Decode and create ledger.Path.
		path, err := ledger.ToPath(encPath)
		if err != nil {
			return false, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to decode path of serialized node: %w", err)
		}

		// Read encoded payload data and create ledger.Payload.
		payload, err := readPayloadFromReader(reader, scratch)
		if err != nil {
			return false, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to read and decode payload of serialized node: %w", err)
		}

		return false, height, 0, 0, path, payload, nodeHash, nil
	}

	// Read interim node

	// Read left and right child index (16 bytes)
	_, err = io.ReadFull(reader, scratch[:encNodeIndexSize*2])
	if err != nil {
		return true, 0, 0, 0, ledger.DummyPath, nil, hash.DummyHash, fmt.Errorf("failed to read child index of serialized node: %w", err)
	}

	pos = 0

	// Decode left child index (8 bytes)
	lchildIndex = binary.BigEndian.Uint64(scratch[pos:])
	pos += encNodeIndexSize

	// Decode right child index (8 bytes)
	rchildIndex = binary.BigEndian.Uint64(scratch[pos:])

	return true, height, lchildIndex, rchildIndex, ledger.DummyPath, nil, nodeHash, nil
}

// DecodeTrieData reads the trie data and decode it.
// any error returned means the file is malformated
func DecodeTrieData(reader io.Reader, scratch []byte) (
	rootIndex uint64,
	regCount uint64,
	regSize uint64,
	rootHash hash.Hash,
	err error,
) {
	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}

	// Read encoded trie
	_, err = io.ReadFull(reader, scratch[:encodedTrieSize])
	if err != nil {
		return 0, 0, 0, hash.DummyHash, fmt.Errorf("failed to read serialized trie: %w", err)
	}

	pos := 0

	// Decode root node index
	rootIndex = binary.BigEndian.Uint64(scratch)
	pos += encNodeIndexSize

	// Decode trie reg count (8 bytes)
	regCount = binary.BigEndian.Uint64(scratch[pos:])
	pos += encRegCountSize

	// Decode trie reg size (8 bytes)
	regSize = binary.BigEndian.Uint64(scratch[pos:])
	pos += encRegSizeSize

	// Decode root node hash
	readRootHash, err := hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return 0, 0, 0, hash.DummyHash, fmt.Errorf("failed to decode hash of serialized trie: %w", err)
	}

	return rootIndex, regCount, regSize, readRootHash, nil
}
