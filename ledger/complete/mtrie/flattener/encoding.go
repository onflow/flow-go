package flattener

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
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
// TODO: encode payload more efficiently.
// TODO: reuse buffer.
// TODO: reduce hash size from 2 bytes to 1 byte.
func encodeLeafNode(n *node.Node) []byte {

	hash := n.Hash()
	path := n.Path()
	encPayload := encoding.EncodePayloadWithoutPrefix(n.Payload())

	buf := make([]byte, 1+2+2+8+2+len(hash)+2+len(path)+4+len(encPayload))
	pos := 0

	// Encode node type (1 byte)
	buf[pos] = byte(leafNodeType)
	pos++

	// Encode height (2-bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], uint16(n.Height()))
	pos += 2

	// Encode max depth (2-bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], n.MaxDepth())
	pos += 2

	// Encode reg count (8-bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], n.RegCount())
	pos += 8

	// Encode hash (2-bytes Big Endian for hashValue length and n-bytes hashValue)
	binary.BigEndian.PutUint16(buf[pos:], uint16(len(hash)))
	pos += 2

	pos += copy(buf[pos:], hash[:])

	// Encode path (2-bytes Big Endian for path length and n-bytes path)
	binary.BigEndian.PutUint16(buf[pos:], uint16(len(path)))
	pos += 2

	pos += copy(buf[pos:], path[:])

	// Encode payload (4-bytes Big Endian for encoded payload length and n-bytes encoded payload)
	binary.BigEndian.PutUint32(buf[pos:], uint32(len(encPayload)))
	pos += 4

	copy(buf[pos:], encPayload)

	return buf
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
// TODO: reuse buffer.
// TODO: reduce hash size from 2 bytes to 1 byte.
func encodeInterimNode(n *node.Node, lchildIndex uint64, rchildIndex uint64) []byte {

	hash := n.Hash()

	buf := make([]byte, 1+2+2+8+8+8+2+len(hash))
	pos := 0

	// Encode node type (1-byte)
	buf[pos] = byte(interimNodeType)
	pos++

	// Encode height (2-bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], uint16(n.Height()))
	pos += 2

	// Encode max depth (2-bytes Big Endian)
	binary.BigEndian.PutUint16(buf[pos:], n.MaxDepth())
	pos += 2

	// Encode reg count (8-bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], n.RegCount())
	pos += 8

	// Encode left child index (8-bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], lchildIndex)
	pos += 8

	// Encode right child index (8-bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], rchildIndex)
	pos += 8

	// Encode hash (2-bytes Big Endian hashValue length and n-bytes hashValue)
	binary.BigEndian.PutUint16(buf[pos:], uint16(len(hash)))
	pos += 2

	copy(buf[pos:], hash[:])

	return buf
}

// EncodeNode encodes node.
func EncodeNode(n *node.Node, lchildIndex uint64, rchildIndex uint64) []byte {
	if n.IsLeaf() {
		return encodeLeafNode(n)
	}
	return encodeInterimNode(n, lchildIndex, rchildIndex)
}

// ReadNode reconstructs a node from data read from reader.
// TODO: reuse read buffer
func ReadNode(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*node.Node, error) {

	// bufSize is large enough to be used for:
	// - fixed-length data: node type (1 byte) + height (2 bytes) + max depth (2 bytes) + reg count (8 bytes), or
	// - child node indexes: 8 bytes * 2
	const bufSize = 16
	const fixLengthSize = 1 + 2 + 2 + 8

	// Read fixed-length part
	buf := make([]byte, bufSize)
	pos := 0

	_, err := io.ReadFull(reader, buf[:fixLengthSize])
	if err != nil {
		return nil, fmt.Errorf("failed to read serialized node, cannot read fixed-length part: %w", err)
	}

	// Read node type (1 byte)
	nType := buf[pos]
	pos++

	// Read height (2 bytes)
	height := binary.BigEndian.Uint16(buf[pos:])
	pos += 2

	// Read max depth (2 bytes)
	maxDepth := binary.BigEndian.Uint16(buf[pos:])
	pos += 2

	// Read reg count (8 bytes)
	regCount := binary.BigEndian.Uint64(buf[pos:])

	if nType == byte(leafNodeType) {

		// Read hash
		encHash, err := utils.ReadShortDataFromReader(reader)
		if err != nil {
			return nil, fmt.Errorf("cannot read hash: %w", err)
		}

		// Read path
		encPath, err := utils.ReadShortDataFromReader(reader)
		if err != nil {
			return nil, fmt.Errorf("cannot read path: %w", err)
		}

		// Read payload
		encPayload, err := utils.ReadLongDataFromReader(reader)
		if err != nil {
			return nil, fmt.Errorf("cannot read payload: %w", err)
		}

		// ToHash copies encHash
		nodeHash, err := hash.ToHash(encHash)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hash from checkpoint: %w", err)
		}

		// ToPath copies encPath
		path, err := ledger.ToPath(encPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decode path from checkpoint: %w", err)
		}

		// TODO: maybe optimize DecodePayload
		payload, err := encoding.DecodePayloadWithoutPrefix(encPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload from checkpoint: %w", err)
		}

		// make a copy of payload
		// TODO: copying may not be necessary
		var pl *ledger.Payload
		if payload != nil {
			pl = payload.DeepCopy()
		}

		node := node.NewNode(int(height), nil, nil, path, pl, nodeHash, maxDepth, regCount)
		return node, nil
	}

	// Read interim node

	pos = 0

	// Read left and right child index (8 bytes each)
	_, err = io.ReadFull(reader, buf[:16])
	if err != nil {
		return nil, fmt.Errorf("cannot read children index: %w", err)
	}

	// Read left child index (8 bytes)
	lchildIndex := binary.BigEndian.Uint64(buf[pos:])
	pos += 8

	// Read right child index (8 bytes)
	rchildIndex := binary.BigEndian.Uint64(buf[pos:])

	// Read hash
	hashValue, err := utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read hash data: %w", err)
	}

	// ToHash copies hashValue
	nodeHash, err := hash.ToHash(hashValue)
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
// TODO: reuse buffer
func EncodeTrie(rootNode *node.Node, rootIndex uint64) []byte {
	// Get root hash
	var rootHash ledger.RootHash
	if rootNode == nil {
		rootHash = trie.EmptyTrieRootHash()
	} else {
		rootHash = ledger.RootHash(rootNode.Hash())
	}

	length := 8 + 2 + len(rootHash)
	buf := make([]byte, length)
	pos := 0

	// 8-bytes Big Endian uint64 RootIndex
	binary.BigEndian.PutUint64(buf, rootIndex)
	pos += 8

	// Encode hash (2-bytes Big Endian for hashValue length and n-bytes hashValue)
	binary.BigEndian.PutUint16(buf[pos:], uint16(len(rootHash)))
	pos += 2

	copy(buf[pos:], rootHash[:])

	return buf
}

// ReadTrie reconstructs a trie from data read from reader.
func ReadTrie(reader io.Reader, getNode func(nodeIndex uint64) (*node.Node, error)) (*trie.MTrie, error) {

	// read root uint64 RootIndex
	buf := make([]byte, 8)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read fixed-legth part: %w", err)
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
