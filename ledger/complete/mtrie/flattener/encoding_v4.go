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

// This file contains decoding functions for checkpoint v4 for backwards compatibility.

const (
	encNodeTypeSizeV4      = 1
	encHeightSizeV4        = 2
	encMaxDepthSizeV4      = 2
	encRegCountSizeV4      = 8
	encHashSizeV4          = hash.HashLen
	encPathSizeV4          = ledger.PathLen
	encNodeIndexSizeV4     = 8
	encPayloadLengthSizeV4 = 4
)

// ReadNodeFromCheckpointV4 reconstructs a node from data read from reader.
// Scratch buffer is used to avoid allocs. It should be used directly instead
// of using append.  This function uses len(scratch) and ignores cap(scratch),
// so any extra capacity will not be utilized.
// If len(scratch) < 1024, then a new buffer will be allocated and used.
// Leaf node is encoded in the following format:
// - node type (1 byte)
// - height (2 bytes)
// - max depth (2 bytes)
// - reg count (8 bytes)
// - hash (32 bytes)
// - path (32 bytes)
// - payload (4 bytes + n bytes)
func ReadNodeFromCheckpointV4(reader io.Reader, scratch []byte, getNode getNodeFunc) (*node.Node, uint64, uint64, error) {

	// minBufSize should be large enough for interim node and leaf node with small payload.
	// minBufSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected.  len(scratch) is 4096 by default, so minBufSize isn't likely to be used.
	const minBufSize = 1024

	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// fixLengthSize is the size of shared data of leaf node and interim node
	const fixLengthSize = encNodeTypeSizeV4 +
		encHeightSizeV4 +
		encMaxDepthSizeV4 +
		encRegCountSizeV4 +
		encHashSizeV4

	_, err := io.ReadFull(reader, scratch[:fixLengthSize])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to read fixed-length part of serialized node: %w", err)
	}

	pos := 0

	// Decode node type (1 byte)
	nType := scratch[pos]
	pos += encNodeTypeSizeV4

	if nType != byte(leafNodeType) && nType != byte(interimNodeType) {
		return nil, 0, 0, fmt.Errorf("failed to decode node type %d", nType)
	}

	// Decode height (2 bytes)
	height := binary.BigEndian.Uint16(scratch[pos:])
	pos += encHeightSizeV4

	// Decode max depth (2 bytes)
	maxDepth := binary.BigEndian.Uint16(scratch[pos:])
	pos += encMaxDepthSizeV4

	// Decode reg count (8 bytes)
	regCount := binary.BigEndian.Uint64(scratch[pos:])
	pos += encRegCountSizeV4

	// Decode and create hash.Hash (32 bytes)
	nodeHash, err := hash.ToHash(scratch[pos : pos+encHashSizeV4])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode hash of serialized node: %w", err)
	}

	// maxDepth and regCount are removed from Node.
	_ = maxDepth
	_ = regCount

	if nType == byte(leafNodeType) {

		// Read path (32 bytes)
		encPath := scratch[:encPathSizeV4]
		_, err := io.ReadFull(reader, encPath)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read path of serialized node: %w", err)
		}

		// Decode and create ledger.Path.
		path, err := ledger.ToPath(encPath)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to decode path of serialized node: %w", err)
		}

		// Read encoded payload data and create ledger.Payload.
		payload, err := readPayloadV0FromReader(reader, scratch)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read and decode payload of serialized node: %w", err)
		}

		n := node.NewNode(int(height), nil, nil, path, payload, nodeHash)

		// Leaf node has 1 register and register size is payload size.
		return n, 1, uint64(payload.Size()), nil
	}

	// Read interim node

	// Read left and right child index (16 bytes)
	_, err = io.ReadFull(reader, scratch[:encNodeIndexSizeV4*2])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to read child index of serialized node: %w", err)
	}

	pos = 0

	// Decode left child index (8 bytes)
	lchildIndex := binary.BigEndian.Uint64(scratch[pos:])
	pos += encNodeIndexSizeV4

	// Decode right child index (8 bytes)
	rchildIndex := binary.BigEndian.Uint64(scratch[pos:])

	// Get left child node by node index
	lchild, lchildRegCount, lchildRegSize, err := getNode(lchildIndex)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to find left child node of serialized node: %w", err)
	}

	// Get right child node by node index
	rchild, rchildRegCount, rchildRegSize, err := getNode(rchildIndex)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to find right child node of serialized node: %w", err)
	}

	n := node.NewNode(int(height), lchild, rchild, ledger.DummyPath, nil, nodeHash)
	return n, lchildRegCount + rchildRegCount, lchildRegSize + rchildRegSize, nil
}

// ReadTrieFromCheckpointV4 reconstructs a trie from data read from reader.
func ReadTrieFromCheckpointV4(reader io.Reader, scratch []byte, getNode getNodeFunc) (*trie.MTrie, error) {

	// encodedTrieSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected (4096 by default).
	const encodedTrieSize = encNodeIndexSizeV4 + encHashSizeV4

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
	pos += encNodeIndexSizeV4

	// Decode root node hash
	readRootHash, err := hash.ToHash(scratch[pos : pos+encHashSizeV4])
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash of serialized trie: %w", err)
	}

	rootNode, regCount, regSize, err := getNode(rootIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find root node of serialized trie: %w", err)
	}

	mtrie, err := trie.NewMTrie(rootNode, regCount, regSize)
	if err != nil {
		return nil, fmt.Errorf("failed to restore serialized trie: %w", err)
	}

	rootHash := mtrie.RootHash()
	if !rootHash.Equals(ledger.RootHash(readRootHash)) {
		return nil, fmt.Errorf("failed to restore serialized trie: roothash doesn't match")
	}

	return mtrie, nil
}

// readPayloadV0FromReader reads and decodes payload from reader.
// Returned payload is a copy.
func readPayloadV0FromReader(reader io.Reader, scratch []byte) (*ledger.Payload, error) {

	if len(scratch) < encPayloadLengthSizeV4 {
		scratch = make([]byte, encPayloadLengthSizeV4)
	}

	// Read payload size
	_, err := io.ReadFull(reader, scratch[:encPayloadLengthSizeV4])
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
	payload, err := encoding.DecodePayloadWithoutPrefix(scratch, false, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	return payload, nil
}
