package flattener

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

type nodeType byte

const (
	leafNodeType nodeType = iota
	interimNodeType
)

// encodeLeafNode encodes leaf node in the following format:
// - node type (1 byte)
// - height (2 bytes)
// - max depth (2 bytes)
// - reg count (8 bytes)
// - hash (2 bytes + 32 bytes)
// - path (2 bytes + 32 bytes)
// - payload (4 bytes + n bytes)
// Encoded leaf node size is 85 bytes (assuming length of hash/path is 32 bytes) +
// length of encoded payload size.
// Scratch buffer is used to avoid allocs.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
// TODO: reduce hash size from 2 bytes to 1 byte.
func encodeLeafNode(n *node.Node, scratch []byte) []byte {

	encPayloadSize := encoding.EncodedPayloadLengthWithoutPrefix(n.Payload())

	encodedNodeSize := 1 + 2 + 2 + 8 + 2 + hash.HashLen + 2 + ledger.PathLen + 4 + encPayloadSize

	if len(scratch) < encodedNodeSize {
		scratch = make([]byte, encodedNodeSize)
	}

	pos := 0

	// Encode node type (1 byte)
	scratch[pos] = byte(leafNodeType)
	pos++

	// Encode height (2-bytes Big Endian)
	binary.BigEndian.PutUint16(scratch[pos:], uint16(n.Height()))
	pos += 2

	// Encode max depth (2-bytes Big Endian)
	binary.BigEndian.PutUint16(scratch[pos:], n.MaxDepth())
	pos += 2

	// Encode reg count (8-bytes Big Endian)
	binary.BigEndian.PutUint64(scratch[pos:], n.RegCount())
	pos += 8

	// Encode hash (2-bytes Big Endian for hashValue length and n-bytes hashValue)
	hash := n.Hash()
	binary.BigEndian.PutUint16(scratch[pos:], uint16(len(hash)))
	pos += 2

	pos += copy(scratch[pos:], hash[:])

	// Encode path (2-bytes Big Endian for path length and n-bytes path)
	path := n.Path()
	binary.BigEndian.PutUint16(scratch[pos:], uint16(len(path)))
	pos += 2

	pos += copy(scratch[pos:], path[:])

	// Encode payload (4-bytes Big Endian for encoded payload length and n-bytes encoded payload)
	binary.BigEndian.PutUint32(scratch[pos:], uint32(encPayloadSize))
	pos += 4

	scratch = encoding.EncodeAndAppendPayloadWithoutPrefix(scratch[:pos], n.Payload())

	return scratch
}

// encodeInterimNode encodes interim node in the following format:
// - node type (1 byte)
// - height (2 bytes)
// - max depth (2 bytes)
// - reg count (8 bytes)
// - lchild index (8 bytes)
// - rchild index (8 bytes)
// - hash (2 bytes + 32 bytes)
// Encoded interim node size is 63 bytes (assuming length of hash is 32 bytes).
// Scratch buffer is used to avoid allocs.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
// TODO: reduce hash size from 2 bytes to 1 byte.
func encodeInterimNode(n *node.Node, lchildIndex uint64, rchildIndex uint64, scratch []byte) []byte {

	encodedNodeSize := 1 + 2 + 2 + 8 + 8 + 8 + 2 + hash.HashLen

	if len(scratch) < encodedNodeSize {
		scratch = make([]byte, encodedNodeSize)
	}

	pos := 0

	// Encode node type (1-byte)
	scratch[pos] = byte(interimNodeType)
	pos++

	// Encode height (2-bytes Big Endian)
	binary.BigEndian.PutUint16(scratch[pos:], uint16(n.Height()))
	pos += 2

	// Encode max depth (2-bytes Big Endian)
	binary.BigEndian.PutUint16(scratch[pos:], n.MaxDepth())
	pos += 2

	// Encode reg count (8-bytes Big Endian)
	binary.BigEndian.PutUint64(scratch[pos:], n.RegCount())
	pos += 8

	// Encode left child index (8-bytes Big Endian)
	binary.BigEndian.PutUint64(scratch[pos:], lchildIndex)
	pos += 8

	// Encode right child index (8-bytes Big Endian)
	binary.BigEndian.PutUint64(scratch[pos:], rchildIndex)
	pos += 8

	// Encode hash (2-bytes Big Endian hashValue length and n-bytes hashValue)
	binary.BigEndian.PutUint16(scratch[pos:], hash.HashLen)
	pos += 2

	h := n.Hash()
	pos += copy(scratch[pos:], h[:])

	return scratch[:pos]
}

// EncodeNode encodes node.
// Scratch buffer is used to avoid allocs.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func EncodeNode(n *node.Node, lchildIndex uint64, rchildIndex uint64, scratch []byte) []byte {
	if n.IsLeaf() {
		return encodeLeafNode(n, scratch)
	}
	return encodeInterimNode(n, lchildIndex, rchildIndex, scratch)
}

// ReadNode reconstructs a node from data read from reader.
func ReadNode(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, error) {

	// minBufSize should be large enough for interim node and leaf node with small payload.
	// minBufSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected (4096 by default).
	const minBufSize = 1024

	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// fixed-length data: node type (1 byte) + height (2 bytes) + max depth (2 bytes) + reg count (8 bytes), or
	const fixLengthSize = 1 + 2 + 2 + 8

	// Read fixed-length part
	pos := 0

	_, err := io.ReadFull(reader, scratch[:fixLengthSize])
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node, cannot read fixed-length part: %w", err)
	}

	// Read node type (1 byte)
	nType := scratch[pos]
	pos++

	// Read height (2 bytes)
	height := binary.BigEndian.Uint16(scratch[pos:])
	pos += 2

	// Read max depth (2 bytes)
	maxDepth := binary.BigEndian.Uint16(scratch[pos:])
	pos += 2

	// Read reg count (8 bytes)
	regCount := binary.BigEndian.Uint64(scratch[pos:])

	if nType == byte(leafNodeType) {

		// Read encoded hash data from reader and create hash.Hash.
		nodeHash, err := readHashFromReader(reader, scratch)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hash from checkpoint: %w", err)
		}

		// Read encoded path data from reader and create ledger.Path.
		path, err := readPathFromReader(reader, scratch)
		if err != nil {
			return nil, fmt.Errorf("failed to decode path from checkpoint: %w", err)
		}

		// Read encoded payload data from reader and create ledger.Payload.
		payload, err := readPayloadFromReader(reader, scratch)
		if err != nil {
			return nil, fmt.Errorf("cannot read payload: %w", err)
		}

		node := node.NewNode(int(height), nil, nil, path, payload, nodeHash, maxDepth, regCount)
		return node, nil
	}

	// Read interim node

	pos = 0

	// Read left and right child index (8 bytes each)
	_, err = io.ReadFull(reader, scratch[:16])
	if err != nil {
		return nil, fmt.Errorf("cannot read children index: %w", err)
	}

	// Read left child index (8 bytes)
	lchildIndex := binary.BigEndian.Uint64(scratch[pos:])
	pos += 8

	// Read right child index (8 bytes)
	rchildIndex := binary.BigEndian.Uint64(scratch[pos:])

	// Read encoded hash data from reader and create hash.Hash
	nodeHash, err := readHashFromReader(reader, scratch)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash from checkpoint: %w", err)
	}

	// Get left child node by node index
	lchild, err := getNode(lchildIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find left child node: %w", err)
	}

	// Get right child node by node index
	rchild, err := getNode(rchildIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find right child node: %w", err)
	}

	n := node.NewNode(int(height), lchild, rchild, ledger.DummyPath, nil, nodeHash, maxDepth, regCount)
	return n, nil
}

// EncodeTrie encodes trie root node
// Scratch buffer is used to avoid allocs.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func EncodeTrie(rootNode *node.Node, rootIndex uint64, scratch []byte) []byte {

	// Get root hash
	var rootHash ledger.RootHash
	if rootNode == nil {
		rootHash = trie.EmptyTrieRootHash()
	} else {
		rootHash = ledger.RootHash(rootNode.Hash())
	}

	const encodedTrieSize = 8 + 2 + len(rootHash)

	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}

	pos := 0

	// 8-bytes Big Endian uint64 RootIndex
	binary.BigEndian.PutUint64(scratch, rootIndex)
	pos += 8

	// Encode hash (2-bytes Big Endian for hashValue length and n-bytes hashValue)
	binary.BigEndian.PutUint16(scratch[pos:], uint16(len(rootHash)))
	pos += 2

	pos += copy(scratch[pos:], rootHash[:])

	return scratch[:pos]
}

// ReadTrie reconstructs a trie from data read from reader.
func ReadTrie(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*node.Node, error)) (*trie.MTrie, error) {

	// minBufSize should be large enough for encoded trie (42 bytes).
	// minBufSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected (4096 by default).
	const minBufSize = 42

	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// read root index (8 bytes)
	_, err := io.ReadFull(reader, scratch[:8])
	if err != nil {
		return nil, fmt.Errorf("cannot read root index data: %w", err)
	}

	rootIndex := binary.BigEndian.Uint64(scratch)

	readRootHash, err := readHashFromReader(reader, scratch)
	if err != nil {
		return nil, fmt.Errorf("cannot read roothash data: %w", err)
	}

	rootNode, err := getNode(rootIndex)
	if err != nil {
		return nil, fmt.Errorf("cannot find root node: %w", err)
	}

	mtrie, err := trie.NewMTrie(rootNode)
	if err != nil {
		return nil, fmt.Errorf("restoring trie failed: %w", err)
	}

	rootHash := mtrie.RootHash()
	if !rootHash.Equals(ledger.RootHash(readRootHash)) {
		return nil, fmt.Errorf("restoring trie failed: roothash doesn't match")
	}

	return mtrie, nil
}

func readHashFromReader(reader io.Reader, scratch []byte) (hash.Hash, error) {

	const encHashBufSize = 2 + hash.HashLen

	if len(scratch) < encHashBufSize {
		scratch = make([]byte, encHashBufSize)
	} else {
		scratch = scratch[:encHashBufSize]
	}

	_, err := io.ReadFull(reader, scratch)
	if err != nil {
		return hash.DummyHash, fmt.Errorf("cannot read hash: %w", err)
	}

	sizeBuf, encHashBuf := scratch[:2], scratch[2:]

	size := binary.BigEndian.Uint16(sizeBuf)
	if size != hash.HashLen {
		return hash.DummyHash, fmt.Errorf("encoded hash size is wrong: want %d bytes, got %d bytes", hash.HashLen, size)
	}

	// hash.ToHash copies data
	return hash.ToHash(encHashBuf)
}

func readPathFromReader(reader io.Reader, scratch []byte) (ledger.Path, error) {

	const encPathBufSize = 2 + ledger.PathLen

	if len(scratch) < encPathBufSize {
		scratch = make([]byte, encPathBufSize)
	} else {
		scratch = scratch[:encPathBufSize]
	}

	_, err := io.ReadFull(reader, scratch)
	if err != nil {
		return ledger.DummyPath, fmt.Errorf("cannot read path: %w", err)
	}

	sizeBuf, encPathBuf := scratch[:2], scratch[2:]

	size := binary.BigEndian.Uint16(sizeBuf)
	if size != ledger.PathLen {
		return ledger.DummyPath, fmt.Errorf("encoded path size is wrong: want %d bytes, got %d bytes", ledger.PathLen, size)
	}

	// ToPath copies encPath
	return ledger.ToPath(encPathBuf)
}

func readPayloadFromReader(reader io.Reader, scratch []byte) (*ledger.Payload, error) {

	if len(scratch) < 4 {
		scratch = make([]byte, 4)
	}

	// Read payload size
	_, err := io.ReadFull(reader, scratch[:4])
	if err != nil {
		return nil, fmt.Errorf("cannot read long data length: %w", err)
	}

	size := binary.BigEndian.Uint32(scratch)

	if len(scratch) < int(size) {
		scratch = make([]byte, size)
	} else {
		scratch = scratch[:size]
	}

	_, err = io.ReadFull(reader, scratch)
	if err != nil {
		return nil, fmt.Errorf("cannot read long data: %w", err)
	}

	// Decode and copy payload
	payload, err := encoding.DecodePayloadWithoutPrefix(scratch, false)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload from checkpoint: %w", err)
	}

	return payload, nil
}
