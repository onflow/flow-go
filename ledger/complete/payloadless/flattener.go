package payloadless

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// Wire format for a payloadless trie node, encoded independently of the full mtrie
// flattener so payloadless checkpoints don't carry a payload wrapper.
//
// Leaf node layout:
//
//	type(1) | height(2) | hash(32) | path(32) | leafHashFlag(1) | leafHash(0 or 32)
//
// Interim node layout:
//
//	type(1) | height(2) | hash(32) | lchildIndex(8) | rchildIndex(8)
//
// Trie metadata layout:
//
//	rootIndex(8) | regCount(8) | rootHash(32)
const (
	encNodeTypeSize  = 1
	encHeightSize    = 2
	encHashSize      = hash.HashLen
	encPathSize      = ledger.PathLen
	encNodeIndexSize = 8
	encRegCountSize  = 8

	encLeafHashFlagSize = 1
	leafHashAbsent      = byte(0)
	leafHashPresent     = byte(1)

	leafNodeType    = byte(0)
	interimNodeType = byte(1)

	encodedLeafSizeMax    = encNodeTypeSize + encHeightSize + encHashSize + encPathSize + encLeafHashFlagSize + encHashSize
	encodedInterimSize    = encNodeTypeSize + encHeightSize + encHashSize + encNodeIndexSize*2
	encodedNodeHeaderSize = encNodeTypeSize + encHeightSize + encHashSize
	encodedTrieSize       = encNodeIndexSize + encRegCountSize + encHashSize

	// EncodedTrieSize is the on-disk byte size of one payloadless trie metadata record
	// (root index + allocated reg count + root hash). Exported so checkpoint readers can
	// compute tail-seek offsets without reading the full trie payload.
	EncodedTrieSize = encodedTrieSize
)

// EncodedTrie holds the fields recovered from a serialized trie record.
type EncodedTrie struct {
	RootIndex uint64
	RegCount  uint64
	RootHash  hash.Hash
}

// EncodeNode encodes a payloadless node into scratch (or a new buffer if scratch is
// too small) and returns the resulting slice. lchildIndex and rchildIndex are ignored
// for leaf nodes.
//
// WARNING: the returned slice may share storage with scratch; the caller must consume
// it before reusing scratch.
func EncodeNode(n *Node, lchildIndex, rchildIndex uint64, scratch []byte) []byte {
	if n.IsLeaf() {
		return encodeLeafNode(n, scratch)
	}
	return encodeInterimNode(n, lchildIndex, rchildIndex, scratch)
}

func encodeLeafNode(n *Node, scratch []byte) []byte {
	if len(scratch) < encodedLeafSizeMax {
		scratch = make([]byte, encodedLeafSizeMax)
	}
	pos := 0
	scratch[pos] = leafNodeType
	pos += encNodeTypeSize

	binary.BigEndian.PutUint16(scratch[pos:], uint16(n.Height()))
	pos += encHeightSize

	nodeHash := n.Hash()
	copy(scratch[pos:], nodeHash[:])
	pos += encHashSize

	p := n.Path()
	copy(scratch[pos:], p[:])
	pos += encPathSize

	if lh := n.LeafHash(); lh != nil {
		scratch[pos] = leafHashPresent
		pos += encLeafHashFlagSize
		copy(scratch[pos:], lh[:])
		pos += encHashSize
	} else {
		scratch[pos] = leafHashAbsent
		pos += encLeafHashFlagSize
	}
	return scratch[:pos]
}

func encodeInterimNode(n *Node, lchildIndex, rchildIndex uint64, scratch []byte) []byte {
	if len(scratch) < encodedInterimSize {
		scratch = make([]byte, encodedInterimSize)
	}
	pos := 0
	scratch[pos] = interimNodeType
	pos += encNodeTypeSize

	binary.BigEndian.PutUint16(scratch[pos:], uint16(n.Height()))
	pos += encHeightSize

	nodeHash := n.Hash()
	copy(scratch[pos:], nodeHash[:])
	pos += encHashSize

	binary.BigEndian.PutUint64(scratch[pos:], lchildIndex)
	pos += encNodeIndexSize
	binary.BigEndian.PutUint64(scratch[pos:], rchildIndex)
	pos += encNodeIndexSize
	return scratch[:pos]
}

// ReadPayloadlessNode reconstructs a payloadless [Node] from data read from reader.
// For interim nodes, getNode is used to resolve the previously-decoded children by
// their assigned indices, in line with the Descendents-First-Relationship used during
// encoding.
//
// scratch is reused for header reads when large enough; if it is shorter than the
// minimum required buffer a fresh slice is allocated.
func ReadPayloadlessNode(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*Node, error)) (*Node, error) {
	const minBufSize = 256
	if len(scratch) < minBufSize {
		scratch = make([]byte, minBufSize)
	}

	// Read fixed-length header: type + height + hash
	if _, err := io.ReadFull(reader, scratch[:encodedNodeHeaderSize]); err != nil {
		return nil, fmt.Errorf("failed to read payloadless node header: %w", err)
	}
	pos := 0
	nType := scratch[pos]
	pos += encNodeTypeSize

	height := binary.BigEndian.Uint16(scratch[pos:])
	pos += encHeightSize

	nodeHash, err := hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payloadless node hash: %w", err)
	}

	switch nType {
	case leafNodeType:
		// Read path + leafHash flag.
		const leafTailLen = encPathSize + encLeafHashFlagSize
		if _, err := io.ReadFull(reader, scratch[:leafTailLen]); err != nil {
			return nil, fmt.Errorf("failed to read payloadless leaf path/flag: %w", err)
		}
		path, err := ledger.ToPath(scratch[:encPathSize])
		if err != nil {
			return nil, fmt.Errorf("failed to decode payloadless leaf path: %w", err)
		}
		flag := scratch[encPathSize]

		var leafHashPtr *hash.Hash
		switch flag {
		case leafHashPresent:
			if _, err := io.ReadFull(reader, scratch[:encHashSize]); err != nil {
				return nil, fmt.Errorf("failed to read payloadless leaf hash: %w", err)
			}
			lh, err := hash.ToHash(scratch[:encHashSize])
			if err != nil {
				return nil, fmt.Errorf("failed to decode payloadless leaf hash: %w", err)
			}
			leafHashPtr = &lh
		case leafHashAbsent:
			// leafHashPtr stays nil.
		default:
			return nil, fmt.Errorf("invalid payloadless leaf hash flag: %d", flag)
		}
		return NewNode(int(height), nil, nil, path, leafHashPtr, nodeHash), nil

	case interimNodeType:
		const idxLen = encNodeIndexSize * 2
		if _, err := io.ReadFull(reader, scratch[:idxLen]); err != nil {
			return nil, fmt.Errorf("failed to read payloadless interim child indices: %w", err)
		}
		lchildIdx := binary.BigEndian.Uint64(scratch[:encNodeIndexSize])
		rchildIdx := binary.BigEndian.Uint64(scratch[encNodeIndexSize:idxLen])

		lchild, err := getNode(lchildIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to find payloadless left child node: %w", err)
		}
		rchild, err := getNode(rchildIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to find payloadless right child node: %w", err)
		}
		return NewNode(int(height), lchild, rchild, ledger.DummyPath, nil, nodeHash), nil

	default:
		return nil, fmt.Errorf("failed to decode payloadless node type %d", nType)
	}
}

// EncodeTrie encodes a payloadless [MTrie] header (root pointer + reg count + root hash)
// into scratch and returns the resulting slice.
//
// WARNING: the returned slice may share storage with scratch.
func EncodeTrie(t *MTrie, rootIndex uint64, scratch []byte) []byte {
	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}
	pos := 0
	binary.BigEndian.PutUint64(scratch[pos:], rootIndex)
	pos += encNodeIndexSize

	binary.BigEndian.PutUint64(scratch[pos:], t.AllocatedRegCount())
	pos += encRegCountSize

	rootHash := t.RootHash()
	copy(scratch[pos:], rootHash[:])
	pos += encHashSize
	return scratch[:pos]
}

// ReadEncodedTrie reads only the trie metadata fields, leaving root-node resolution
// to the caller.
func ReadEncodedTrie(reader io.Reader, scratch []byte) (EncodedTrie, error) {
	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}
	if _, err := io.ReadFull(reader, scratch[:encodedTrieSize]); err != nil {
		return EncodedTrie{}, fmt.Errorf("failed to read payloadless trie metadata: %w", err)
	}
	rootIndex := binary.BigEndian.Uint64(scratch[:encNodeIndexSize])
	regCount := binary.BigEndian.Uint64(scratch[encNodeIndexSize : encNodeIndexSize+encRegCountSize])
	rootHashBytes := scratch[encNodeIndexSize+encRegCountSize : encodedTrieSize]
	rootHash, err := hash.ToHash(rootHashBytes)
	if err != nil {
		return EncodedTrie{}, fmt.Errorf("failed to decode payloadless trie root hash: %w", err)
	}
	return EncodedTrie{
		RootIndex: rootIndex,
		RegCount:  regCount,
		RootHash:  rootHash,
	}, nil
}

// ReadPayloadlessTrie reconstructs a payloadless [MTrie] from data read from reader.
// It verifies the encoded root hash matches the resolved root node's hash.
func ReadPayloadlessTrie(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*Node, error)) (*MTrie, error) {
	enc, err := ReadEncodedTrie(reader, scratch)
	if err != nil {
		return nil, err
	}
	rootNode, err := getNode(enc.RootIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find root node of serialized payloadless trie: %w", err)
	}
	mtrie, err := NewMTrie(rootNode, enc.RegCount)
	if err != nil {
		return nil, fmt.Errorf("failed to restore serialized payloadless trie: %w", err)
	}
	if ledger.RootHash(enc.RootHash) != mtrie.RootHash() {
		return nil, fmt.Errorf("failed to restore serialized payloadless trie: roothash doesn't match")
	}
	return mtrie, nil
}

// NodeIterator yields the nodes of a payloadless trie in Descendents-First order,
// matching the contract of the full mtrie's [flattener.NodeIterator]: for any node
// at sequence index k, all its descendents have strictly smaller indices.
//
// When visitedNodes is non-nil, nodes already present in the map are skipped — this
// is how forest serialization avoids re-emitting sub-tries that are shared across
// tries.
//
// NOT safe for concurrent use when visitedNodes is shared with another iterator.
type NodeIterator struct {
	unprocessedRoot *Node
	stack           []*Node
	visitedNodes    map[*Node]uint64
}

// NewNodeIterator returns an iterator over a single payloadless trie's nodes.
// Safe for concurrent use because it does not consult any visitedNodes map.
func NewNodeIterator(n *Node) *NodeIterator {
	return NewUniqueNodeIterator(n, nil)
}

// NewUniqueNodeIterator returns an iterator that skips any node already present in
// visitedNodes, so forest traversal avoids re-emitting shared sub-tries.
//
// NOT safe for concurrent use because visitedNodes is read without synchronization.
func NewUniqueNodeIterator(n *Node, visitedNodes map[*Node]uint64) *NodeIterator {
	stackSize := ledger.NodeMaxHeight + 1
	i := &NodeIterator{
		stack:        make([]*Node, 0, stackSize),
		visitedNodes: visitedNodes,
	}
	i.unprocessedRoot = n
	return i
}

// Next advances the iterator to the next node in Descendents-First order.
// Returns true if [NodeIterator.Value] now points to a valid node, false at the end.
func (i *NodeIterator) Next() bool {
	if i.unprocessedRoot != nil {
		i.dig(i.unprocessedRoot)
		i.unprocessedRoot = nil
		return len(i.stack) > 0
	}
	n := i.pop()
	if len(i.stack) > 0 {
		parent := i.peek()
		if parent.LeftChild() == n {
			i.dig(parent.RightChild())
		}
		return true
	}
	return false
}

// Value returns the current node, or nil if the iterator is exhausted.
func (i *NodeIterator) Value() *Node {
	if len(i.stack) == 0 {
		return nil
	}
	return i.peek()
}

func (i *NodeIterator) pop() *Node {
	if len(i.stack) == 0 {
		return nil
	}
	headIdx := len(i.stack) - 1
	head := i.stack[headIdx]
	i.stack = i.stack[:headIdx]
	return head
}

func (i *NodeIterator) peek() *Node {
	return i.stack[len(i.stack)-1]
}

func (i *NodeIterator) dig(n *Node) {
	if n == nil {
		return
	}
	if _, found := i.visitedNodes[n]; found {
		return
	}
	for {
		i.stack = append(i.stack, n)
		if lChild := n.LeftChild(); lChild != nil {
			if _, found := i.visitedNodes[lChild]; !found {
				n = lChild
				continue
			}
		}
		if rChild := n.RightChild(); rChild != nil {
			if _, found := i.visitedNodes[rChild]; !found {
				n = rChild
				continue
			}
		}
		return
	}
}
