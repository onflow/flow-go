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
// These functions are for backwards compatibility.

const encodingDecodingVersion = uint16(0)

// ReadNodeFromCheckpointV3AndEarlier reconstructs a node from data in checkpoint v3 and earlier versions.
func ReadNodeFromCheckpointV3AndEarlier(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, error) {

	// reading version
	buf := make([]byte, 2)
	read, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node, cannot read version part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("failed to read serialized node: not enough bytes read %d expected %d", read, len(buf))
	}

	version, _, err := utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node: %w", err)
	}

	if version > encodingDecodingVersion {
		return nil, fmt.Errorf("failed to read serialized node: unsuported version %d > %d", version, encodingDecodingVersion)
	}

	// reading fixed-length part
	buf = make([]byte, 2+8+8+2+8)

	read, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node, cannot read fixed-length part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("failed to read serialized node: not enough bytes read %d expected %d", read, len(buf))
	}

	var height, maxDepth uint16
	var lchildIndex, rchildIndex, regCount uint64
	var path, hashValue, encPayload []byte

	height, buf, err = utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node: %w", err)
	}

	lchildIndex, buf, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node: %w", err)
	}

	rchildIndex, buf, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node: %w", err)
	}

	maxDepth, buf, err = utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node: %w", err)
	}

	regCount, _, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node: %w", err)
	}

	path, err = utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read key data: %w", err)
	}

	encPayload, err = utils.ReadLongDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read value data: %w", err)
	}

	hashValue, err = utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read hashValue data: %w", err)
	}

	// Create (and copy) hash from raw data.
	nodeHash, err := hash.ToHash(hashValue)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash from checkpoint: %w", err)
	}

	if len(path) > 0 {
		// Create (and copy) path from raw data.
		path, err := ledger.ToPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to decode path from checkpoint: %w", err)
		}

		// Decode payload (payload data isn't copied).
		payload, err := encoding.DecodePayload(encPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload from checkpoint: %w", err)
		}

		// make a copy of payload
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

// ReadTrieFromCheckpointV3AndEarlier reconstructs a trie from data in checkpoint v3 and earlier versions.
func ReadTrieFromCheckpointV3AndEarlier(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*trie.MTrie, error) {

	// reading version
	buf := make([]byte, 2)
	read, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node, cannot read version part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	version, _, err := utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	if version > encodingDecodingVersion {
		return nil, fmt.Errorf("error reading storable node: unsuported version %d > %d", version, encodingDecodingVersion)
	}

	// read root uint64 RootIndex
	buf = make([]byte, 8)
	read, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read fixed-legth part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	rootIndex, _, err := utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read root index data: %w", err)
	}

	readRootHash, err := utils.ReadShortDataFromReader(reader)
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
	if !bytes.Equal(readRootHash, rootHash[:]) {
		return nil, fmt.Errorf("restoring trie failed: roothash doesn't match")
	}

	return mtrie, nil
}
