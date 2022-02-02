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

const encodingDecodingVersion = uint16(0)

// EncodeNode encodes node.
// TODO: reuse buffer
func EncodeNode(n *node.Node, lchildIndex uint64, rchildIndex uint64) []byte {

	encPayload := encoding.EncodePayload(n.Payload())

	length := 2 + 2 + 8 + 8 + 2 + 8 + 2 + len(n.Path()) + 4 + len(encPayload) + 2 + len(n.Hash())

	buf := make([]byte, 0, length)

	// 2-bytes encoding version
	buf = utils.AppendUint16(buf, encodingDecodingVersion)

	// 2-bytes Big Endian uint16 height
	buf = utils.AppendUint16(buf, uint16(n.Height()))

	// 8-bytes Big Endian uint64 LIndex
	buf = utils.AppendUint64(buf, lchildIndex)

	// 8-bytes Big Endian uint64 RIndex
	buf = utils.AppendUint64(buf, rchildIndex)

	// 2-bytes Big Endian maxDepth
	buf = utils.AppendUint16(buf, n.MaxDepth())

	// 8-bytes Big Endian regCount
	buf = utils.AppendUint64(buf, n.RegCount())

	// 2-bytes Big Endian uint16 encoded path length and n-bytes encoded path
	path := n.Path()
	if path != nil {
		buf = utils.AppendShortData(buf, path[:])
	} else {
		buf = utils.AppendShortData(buf, nil)
	}

	// 4-bytes Big Endian uint32 encoded payload length and n-bytes encoded payload
	buf = utils.AppendLongData(buf, encPayload)

	// 2-bytes Big Endian uint16 hashValue length and n-bytes hashValue
	hash := n.Hash()
	buf = utils.AppendShortData(buf, hash[:])

	return buf
}

// ReadNode reconstructs a node from data read from reader.
// TODO: reuse read buffer
func ReadNode(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, error) {

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

// EncodeTrie encodes trie root node
// TODO: reuse buffer
func EncodeTrie(rootNode *node.Node, rootIndex uint64) []byte {
	// Get root hash
	var rootHash ledger.RootHash
	if rootNode == nil {
		rootHash = trie.EmptyTrieRootHash()
	} else {
		rootHash = ledger.RootHash(rootNode.Hash())
	}

	length := 2 + 8 + 2 + len(rootHash)
	buf := make([]byte, 0, length)
	// 2-bytes encoding version
	buf = utils.AppendUint16(buf, encodingDecodingVersion)

	// 8-bytes Big Endian uint64 RootIndex
	buf = utils.AppendUint64(buf, rootIndex)

	// 2-bytes Big Endian uint16 RootHash length and n-bytes RootHash
	buf = utils.AppendShortData(buf, rootHash[:])

	return buf
}

// ReadTrie reconstructs a trie from data read from reader.
func ReadTrie(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*trie.MTrie, error) {

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
