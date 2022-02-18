package flattener

import (
	"bytes"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// This file contains decoding functions for checkpoint v3 and earlier versions.
// These functions are for backwards compatibility, not optimized.

const encodingDecodingVersion = uint16(0)

// ReadNodeFromCheckpointV3AndEarlier reconstructs a node from data in checkpoint v3 and earlier versions.
// Encoded node in checkpoint v3 and earlier is in the following format:
// - version (2 bytes)
// - height (2 bytes)
// - lindex (8 bytes)
// - rindex (8 bytes)
// - max depth (2 bytes)
// - reg count (8 bytes)
// - path (2 bytes + 32 bytes)
// - payload (4 bytes + n bytes)
// - hash (2 bytes + 32 bytes)
func ReadNodeFromCheckpointV3AndEarlier(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, error) {

	// Read version (2 bytes)
	buf := make([]byte, 2)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read version of serialized node in v3: %w", err)
	}

	// Decode version
	version, _, err := utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode version of serialized node in v3: %w", err)
	}

	if version > encodingDecodingVersion {
		return nil, fmt.Errorf("found unsuported version %d (> %d) of serialized node in v3", version, encodingDecodingVersion)
	}

	// fixed-length data:
	//   height (2 bytes) +
	//   left child node index (8 bytes) +
	//   right child node index (8 bytes) +
	//   max depth (2 bytes) +
	//   reg count (8 bytes)
	buf = make([]byte, 2+8+8+2+8)

	// Read fixed-length part
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixed-length part of serialized node in v3: %w", err)
	}

	var height, maxDepth uint16
	var lchildIndex, rchildIndex, regCount uint64
	var path, hashValue, encPayload []byte

	// Decode height (2 bytes)
	height, buf, err = utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode height of serialized node in v3: %w", err)
	}

	// Decode left child index (8 bytes)
	lchildIndex, buf, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode left child index of serialized node in v3: %w", err)
	}

	// Decode right child index (8 bytes)
	rchildIndex, buf, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode right child index of serialized node in v3: %w", err)
	}

	// Decode max depth (2 bytes)
	maxDepth, buf, err = utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode max depth of serialized node in v3: %w", err)
	}

	// Decode reg count (8 bytes)
	regCount, _, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reg count of serialized node in v3: %w", err)
	}

	// Read path (2 bytes + 32 bytes)
	path, err = utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read path of serialized node in v3: %w", err)
	}

	// Read payload (4 bytes + n bytes)
	encPayload, err = utils.ReadLongDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read payload of serialized node in v3: %w", err)
	}

	// Read hash (2 bytes + 32 bytes)
	hashValue, err = utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read hash of serialized node in v3: %w", err)
	}

	// Create (and copy) hash from raw data.
	nodeHash, err := hash.ToHash(hashValue)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash of serialized node in v3: %w", err)
	}

	if len(path) > 0 {
		// Create (and copy) path from raw data.
		path, err := ledger.ToPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to decode path of serialized node in v3: %w", err)
		}

		// Decode payload (payload data isn't copied).
		payload, err := encoding.DecodePayload(encPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload of serialized node in v3: %w", err)
		}

		// Make a copy of payload
		var pl *ledger.Payload
		if payload != nil {
			pl = payload.DeepCopy()
		}

		n := node.NewNode(int(height), nil, nil, path, pl, nodeHash, maxDepth, regCount)
		return n, nil
	}

	// Get left child node by node index
	lchild, err := getNode(lchildIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find left child node of serialized node in v3: %w", err)
	}

	// Get right child node by node index
	rchild, err := getNode(rchildIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find right child node of serialized node in v3: %w", err)
	}

	n := node.NewNode(int(height), lchild, rchild, ledger.DummyPath, nil, nodeHash, maxDepth, regCount)
	return n, nil
}

// ReadTrieFromCheckpointV3AndEarlier reconstructs a trie from data in checkpoint v3 and earlier versions.
// Encoded trie in checkpoint v3 and earlier is in the following format:
// - version (2 bytes)
// - root node index (8 bytes)
// - root node hash (2 bytes + 32 bytes)
func ReadTrieFromCheckpointV3AndEarlier(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*trie.MTrie, error) {

	// Read version (2 bytes)
	buf := make([]byte, 2)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read version of serialized trie in v3: %w", err)
	}

	// Decode version
	version, _, err := utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode version of serialized trie in v3: %w", err)
	}

	if version > encodingDecodingVersion {
		return nil, fmt.Errorf("found unsuported version %d (> %d) of serialized trie in v3", version, encodingDecodingVersion)
	}

	// Read root index (8 bytes)
	buf = make([]byte, 8)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read root index of serialized trie in v3: %w", err)
	}

	// Decode root index
	rootIndex, _, err := utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode root index of serialized trie in v3: %w", err)
	}

	// Read root hash (2 bytes + 32 bytes)
	readRootHash, err := utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read root hash of serialized trie in v3: %w", err)
	}

	// Get node by index
	rootNode, err := getNode(rootIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find root node of serialized trie in v3: %w", err)
	}

	mtrie, err := trie.NewMTrie(rootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to restore serialized trie in v3: %w", err)
	}

	rootHash := mtrie.RootHash()
	if !bytes.Equal(readRootHash, rootHash[:]) {
		return nil, fmt.Errorf("failed to restore serialized trie in v3: roothash doesn't match")
	}

	return mtrie, nil
}
