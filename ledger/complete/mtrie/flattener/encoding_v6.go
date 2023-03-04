package flattener

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

// ReadNodeV6 reconstructs a node from data read from reader.
// Scratch buffer is used to avoid allocs. It should be used directly instead
// of using append.  This function uses len(scratch) and ignores cap(scratch),
// so any extra capacity will not be utilized.
// If len(scratch) < 1024, then a new buffer will be allocated and used.
func ReadNodeV6(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, ledger.Path, *ledger.Payload, error) {

	// minBufSize should be large enough for interim node and leaf node with small payload.
	// minBufSize is a failsafe and is only used when len(scratch) is much smaller
	// than expected.  len(scratch) is 4096 by default, so minBufSize isn't likely to be used.
	const minBufSize = 1024

	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// fixLengthSize is the size of shared data of leaf node and interim node
	const fixLengthSize = encNodeTypeSize + encHeightSize + encHashSize

	_, err := io.ReadFull(reader, scratch[:fixLengthSize])
	if err != nil {
		return nil, ledger.DummyPath, nil, fmt.Errorf("failed to read fixed-length part of serialized node: %w", err)
	}

	pos := 0

	// Decode node type (1 byte)
	nType := scratch[pos]
	pos += encNodeTypeSize

	if nType != byte(leafNodeType) && nType != byte(interimNodeType) {
		return nil, ledger.DummyPath, nil, fmt.Errorf("failed to decode node type %d", nType)
	}

	// Decode height (2 bytes)
	height := binary.BigEndian.Uint16(scratch[pos:])
	pos += encHeightSize

	// Decode and create hash.Hash (32 bytes)
	nodeHash, err := hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return nil, ledger.DummyPath, nil, fmt.Errorf("failed to decode hash of serialized node: %w", err)
	}

	if nType == byte(leafNodeType) {

		// Read path (32 bytes)
		encPath := scratch[:encPathSize]
		_, err := io.ReadFull(reader, encPath)
		if err != nil {
			return nil, ledger.DummyPath, nil, fmt.Errorf("failed to read path of serialized node: %w", err)
		}

		// Decode and create ledger.Path.
		path, err := ledger.ToPath(encPath)
		if err != nil {
			return nil, ledger.DummyPath, nil, fmt.Errorf("failed to decode path of serialized node: %w", err)
		}

		// Read encoded payload data and create ledger.Payload.
		payload, err := readPayloadFromReader(reader, scratch)
		if err != nil {
			return nil, ledger.DummyPath, nil, fmt.Errorf("failed to read and decode payload of serialized node: %w", err)
		}

		node := node.NewLeafWithHash(path, payload, int(height), nodeHash)
		return node, path, payload, nil
	}

	// Read interim node

	// Read left and right child index (16 bytes)
	_, err = io.ReadFull(reader, scratch[:encNodeIndexSize*2])
	if err != nil {
		return nil, ledger.DummyPath, nil, fmt.Errorf("failed to read child index of serialized node: %w", err)
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
		return nil, ledger.DummyPath, nil, fmt.Errorf("failed to find left child node of serialized node: %w", err)
	}

	// Get right child node by node index
	rchild, err := getNode(rchildIndex)
	if err != nil {
		return nil, ledger.DummyPath, nil, fmt.Errorf("failed to find right child node of serialized node: %w", err)
	}

	n := node.NewInterimNodeWithHash(int(height), lchild, rchild, nodeHash)
	return n, ledger.DummyPath, nil, nil
}
