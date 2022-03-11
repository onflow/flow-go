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

const (
	encNodeTypeSize      = 1
	encHeightSize        = 2
	encMaxDepthSize      = 2
	encRegCountSize      = 8
	encHashSize          = hash.HashLen
	encPathSize          = ledger.PathLen
	encNodeIndexSize     = 8
	encPayloadLengthSize = 4
)

// encodeLeafNode encodes leaf node in the following format:
// - node type (1 byte)
// - height (2 bytes)
// - max depth (2 bytes)
// - reg count (8 bytes)
// - hash (32 bytes)
// - path (32 bytes)
// - payload (4 bytes + n bytes)
// Encoded leaf node size is 81 bytes (assuming length of hash/path is 32 bytes) +
// length of encoded payload size.
// Scratch buffer is used to avoid allocs. It should be used directly instead
// of using append.  This function uses len(scratch) and ignores cap(scratch),
// so any extra capacity will not be utilized.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func encodeLeafNode(n *node.Node, scratch []byte) []byte {

	encPayloadSize := encoding.EncodedPayloadLengthWithoutPrefix(n.Payload())

	encodedNodeSize := encNodeTypeSize +
		encHeightSize +
		encMaxDepthSize +
		encRegCountSize +
		encHashSize +
		encPathSize +
		encPayloadLengthSize +
		encPayloadSize

	// buf uses received scratch buffer if it's large enough.
	// Otherwise, a new buffer is allocated.
	// buf is used directly so len(buf) must not be 0.
	// buf will be resliced to proper size before being returned from this function.
	buf := scratch
	if len(scratch) < encodedNodeSize {
		buf = make([]byte, encodedNodeSize)
	}

	pos := 0

	// Encode node type (1 byte)
	buf[pos] = byte(leafNodeType)
	pos += encNodeTypeSize

	// Encode height (2 bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], uint16(n.Height()))
	pos += encHeightSize

	// Encode max depth (2 bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], n.MaxDepth())
	pos += encMaxDepthSize

	// Encode reg count (8 bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], n.RegCount())
	pos += encRegCountSize

	// Encode hash (32 bytes hashValue)
	hash := n.Hash()
	copy(buf[pos:], hash[:])
	pos += encHashSize

	// Encode path (32 bytes path)
	path := n.Path()
	copy(buf[pos:], path[:])
	pos += encPathSize

	// Encode payload (4 bytes Big Endian for encoded payload length and n bytes encoded payload)
	binary.BigEndian.PutUint32(buf[pos:], uint32(encPayloadSize))
	pos += encPayloadLengthSize

	// EncodeAndAppendPayloadWithoutPrefix appends encoded payload to the resliced buf.
	// Returned buf is resliced to include appended payload.
	buf = encoding.EncodeAndAppendPayloadWithoutPrefix(buf[:pos], n.Payload())

	return buf
}

// encodeInterimNode encodes interim node in the following format:
// - node type (1 byte)
// - height (2 bytes)
// - max depth (2 bytes)
// - reg count (8 bytes)
// - hash (32 bytes)
// - lchild index (8 bytes)
// - rchild index (8 bytes)
// Encoded interim node size is 61 bytes (assuming length of hash is 32 bytes).
// Scratch buffer is used to avoid allocs. It should be used directly instead
// of using append.  This function uses len(scratch) and ignores cap(scratch),
// so any extra capacity will not be utilized.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func encodeInterimNode(n *node.Node, lchildIndex uint64, rchildIndex uint64, scratch []byte) []byte {

	const encodedNodeSize = encNodeTypeSize +
		encHeightSize +
		encMaxDepthSize +
		encRegCountSize +
		encHashSize +
		encNodeIndexSize +
		encNodeIndexSize

	// buf uses received scratch buffer if it's large enough.
	// Otherwise, a new buffer is allocated.
	// buf is used directly so len(buf) must not be 0.
	// buf will be resliced to proper size before being returned from this function.
	buf := scratch
	if len(scratch) < encodedNodeSize {
		buf = make([]byte, encodedNodeSize)
	}

	pos := 0

	// Encode node type (1 byte)
	buf[pos] = byte(interimNodeType)
	pos += encNodeTypeSize

	// Encode height (2 bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], uint16(n.Height()))
	pos += encHeightSize

	// Encode max depth (2 bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], n.MaxDepth())
	pos += encMaxDepthSize

	// Encode reg count (8 bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], n.RegCount())
	pos += encRegCountSize

	// Encode hash (32 bytes hashValue)
	h := n.Hash()
	copy(buf[pos:], h[:])
	pos += encHashSize

	// Encode left child index (8 bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], lchildIndex)
	pos += encNodeIndexSize

	// Encode right child index (8 bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], rchildIndex)
	pos += encNodeIndexSize

	return buf[:pos]
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
// Scratch buffer is used to avoid allocs. It should be used directly instead
// of using append.  This function uses len(scratch) and ignores cap(scratch),
// so any extra capacity will not be utilized.
// If len(scratch) < 1024, then a new buffer will be allocated and used.
func ReadNode(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, error) {

	// minBufSize should be large enough for interim node and leaf node with small payload.
	// minBufSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected.  len(scratch) is 4096 by default, so minBufSize isn't likely to be used.
	const minBufSize = 1024

	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// fixLengthSize is the size of shared data of leaf node and interim node
	const fixLengthSize = encNodeTypeSize +
		encHeightSize +
		encMaxDepthSize +
		encRegCountSize +
		encHashSize

	_, err := io.ReadFull(reader, scratch[:fixLengthSize])
	if err != nil {
		return nil, fmt.Errorf("failed to read fixed-length part of serialized node: %w", err)
	}

	pos := 0

	// Decode node type (1 byte)
	nType := scratch[pos]
	pos += encNodeTypeSize

	if nType != byte(leafNodeType) && nType != byte(interimNodeType) {
		return nil, fmt.Errorf("failed to decode node type %d", nType)
	}

	// Decode height (2 bytes)
	height := binary.BigEndian.Uint16(scratch[pos:])
	pos += encHeightSize

	// Decode max depth (2 bytes)
	maxDepth := binary.BigEndian.Uint16(scratch[pos:])
	pos += encMaxDepthSize

	// Decode reg count (8 bytes)
	regCount := binary.BigEndian.Uint64(scratch[pos:])
	pos += encRegCountSize

	// Decode and create hash.Hash (32 bytes)
	nodeHash, err := hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash of serialized node: %w", err)
	}

	if nType == byte(leafNodeType) {

		// Read path (32 bytes)
		encPath := scratch[:encPathSize]
		_, err := io.ReadFull(reader, encPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read path of serialized node: %w", err)
		}

		// Decode and create ledger.Path.
		path, err := ledger.ToPath(encPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decode path of serialized node: %w", err)
		}

		// Read encoded payload data and create ledger.Payload.
		payload, err := readPayloadFromReader(reader, scratch)
		if err != nil {
			return nil, fmt.Errorf("failed to read and decode payload of serialized node: %w", err)
		}

		node := node.NewNode(int(height), nil, nil, path, payload, nodeHash, maxDepth, regCount)
		return node, nil
	}

	// Read interim node

	// Read left and right child index (16 bytes)
	_, err = io.ReadFull(reader, scratch[:encNodeIndexSize*2])
	if err != nil {
		return nil, fmt.Errorf("failed to read child index of serialized node: %w", err)
	}

	pos = 0

	// Decode left child index (8 bytes)
	lchildIndex := binary.BigEndian.Uint64(scratch[pos:])
	pos += encNodeIndexSize

	// Decode right child index (8 bytes)
	rchildIndex := binary.BigEndian.Uint64(scratch[pos:])

	// Get left child node by node index
	lchild, err := getNode(lchildIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find left child node of serialized node: %w", err)
	}

	// Get right child node by node index
	rchild, err := getNode(rchildIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find right child node of serialized node: %w", err)
	}

	n := node.NewNode(int(height), lchild, rchild, ledger.DummyPath, nil, nodeHash, maxDepth, regCount)
	return n, nil
}

// EncodeTrie encodes trie in the following format:
// - root node index (8 byte)
// - root node hash (32 bytes)
// Scratch buffer is used to avoid allocs.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func EncodeTrie(rootNode *node.Node, rootIndex uint64, scratch []byte) []byte {

	const encodedTrieSize = encNodeIndexSize + encHashSize

	// Get root hash
	var rootHash ledger.RootHash
	if rootNode == nil {
		rootHash = trie.EmptyTrieRootHash()
	} else {
		rootHash = ledger.RootHash(rootNode.Hash())
	}

	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}

	pos := 0

	// Encode root node index (8 bytes Big Endian)
	binary.BigEndian.PutUint64(scratch, rootIndex)
	pos += encNodeIndexSize

	// Encode hash (32-bytes hashValue)
	copy(scratch[pos:], rootHash[:])
	pos += encHashSize

	return scratch[:pos]
}

// ReadTrie reconstructs a trie from data read from reader.
func ReadTrie(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*node.Node, error)) (*trie.MTrie, error) {

	// encodedTrieSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected (4096 by default).
	const encodedTrieSize = encNodeIndexSize + encHashSize

	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}

	// Read encoded trie (8 + 32 bytes)
	_, err := io.ReadFull(reader, scratch[:encodedTrieSize])
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized trie: %w", err)
	}

	pos := 0

	// Decode root node index
	rootIndex := binary.BigEndian.Uint64(scratch)
	pos += encNodeIndexSize

	// Decode root node hash
	readRootHash, err := hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash of serialized trie: %w", err)
	}

	rootNode, err := getNode(rootIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find root node of serialized trie: %w", err)
	}

	mtrie, err := trie.NewMTrie(rootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to restore serialized trie: %w", err)
	}

	rootHash := mtrie.RootHash()
	if !rootHash.Equals(ledger.RootHash(readRootHash)) {
		return nil, fmt.Errorf("failed to restore serialized trie: roothash doesn't match")
	}

	return mtrie, nil
}

// readPayloadFromReader reads and decodes payload from reader.
// Returned payload is a copy.
func readPayloadFromReader(reader io.Reader, scratch []byte) (*ledger.Payload, error) {

	if len(scratch) < encPayloadLengthSize {
		scratch = make([]byte, encPayloadLengthSize)
	}

	// Read payload size
	_, err := io.ReadFull(reader, scratch[:encPayloadLengthSize])
	if err != nil {
		return nil, fmt.Errorf("cannot read payload length: %w", err)
	}

	// Decode payload size
	size := binary.BigEndian.Uint32(scratch)

	if len(scratch) < int(size) {
		scratch = make([]byte, size)
	} else {
		scratch = scratch[:size]
	}

	_, err = io.ReadFull(reader, scratch)
	if err != nil {
		return nil, fmt.Errorf("cannot read payload: %w", err)
	}

	// Decode and copy payload
	payload, err := encoding.DecodePayloadWithoutPrefix(scratch, false)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	return payload, nil
}
