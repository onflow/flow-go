package payloadless

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

type nodeType byte

const (
	leafNodeType nodeType = iota
	interimNodeType
)

const (
	encNodeTypeSize     = 1
	encHeightSize       = 2
	encRegCountSize     = 8
	encHashSize         = hash.HashLen
	encPathSize         = ledger.PathLen
	encNodeIndexSize    = 8
	encLeafHashFlagSize = 1

	encodedTrieSize = encNodeIndexSize + encRegCountSize + encHashSize
	EncodedTrieSize = encodedTrieSize
)

const (
	leafHashAbsent  = byte(0)
	leafHashPresent = byte(1)
)

// encodeLeafNode encodes leaf node in the following format:
// - node type (1 byte)
// - height (2 bytes)
// - hash (32 bytes)
// - path (32 bytes)
// - leaf hash flag (1 byte: 0 = absent, 1 = present)
// - leaf hash (0 or 32 bytes, present only when flag is 1)
// Encoded leaf node size is between 68 and 100 bytes (assuming length of
// hash/path is 32 bytes).
// Scratch buffer is used to avoid allocs. It should be used directly instead
// of using append.  This function uses len(scratch) and ignores cap(scratch),
// so any extra capacity will not be utilized.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func encodeLeafNode(n *Node, scratch []byte) []byte {

	leafHash := n.LeafHash()
	encLeafHashSize := 0
	if leafHash != nil {
		encLeafHashSize = encHashSize
	}

	encodedNodeSize := encNodeTypeSize +
		encHeightSize +
		encHashSize +
		encPathSize +
		encLeafHashFlagSize +
		encLeafHashSize

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

	// Encode hash (32 bytes hashValue)
	h := n.Hash()
	copy(buf[pos:], h[:])
	pos += encHashSize

	// Encode path (32 bytes path)
	path := n.Path()
	copy(buf[pos:], path[:])
	pos += encPathSize

	// Encode leaf hash flag (1 byte) and optional leaf hash (0 or 32 bytes)
	if leafHash != nil {
		buf[pos] = leafHashPresent
		pos += encLeafHashFlagSize
		copy(buf[pos:], leafHash[:])
		pos += encHashSize
	} else {
		buf[pos] = leafHashAbsent
		pos += encLeafHashFlagSize
	}

	return buf[:pos]
}

// encodeInterimNode encodes interim node in the following format:
// - node type (1 byte)
// - height (2 bytes)
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
func encodeInterimNode(n *Node, lchildIndex uint64, rchildIndex uint64, scratch []byte) []byte {

	const encodedNodeSize = encNodeTypeSize +
		encHeightSize +
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
func EncodeNode(n *Node, lchildIndex uint64, rchildIndex uint64, scratch []byte) []byte {
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
func ReadNode(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*Node, error)) (*Node, error) {

	// minBufSize should be large enough for interim node and leaf node.
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

		// Read encoded leaf hash flag and optional leaf hash.
		leafHash, err := readLeafHashFromReader(reader, scratch)
		if err != nil {
			return nil, fmt.Errorf("failed to read and decode leaf hash of serialized node: %w", err)
		}

		node := NewNode(int(height), nil, nil, path, leafHash, nodeHash)
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

	n := NewNode(int(height), lchild, rchild, ledger.DummyPath, nil, nodeHash)
	return n, nil
}

type EncodedTrie struct {
	RootIndex uint64
	RegCount  uint64
	RootHash  hash.Hash
}

// EncodeTrie encodes trie in the following format:
// - root node index (8 byte)
// - allocated reg count (8 byte)
// - root node hash (32 bytes)
// Scratch buffer is used to avoid allocs.
// WARNING: The returned buffer is likely to share the same underlying array as
// the scratch buffer. Caller is responsible for copying or using returned buffer
// before scratch buffer is used again.
func EncodeTrie(trie *MTrie, rootIndex uint64, scratch []byte) []byte {
	buf := scratch
	if len(scratch) < encodedTrieSize {
		buf = make([]byte, encodedTrieSize)
	}

	pos := 0

	// Encode root node index (8 bytes Big Endian)
	binary.BigEndian.PutUint64(buf, rootIndex)
	pos += encNodeIndexSize

	// Encode trie reg count (8 bytes Big Endian)
	binary.BigEndian.PutUint64(buf[pos:], trie.AllocatedRegCount())
	pos += encRegCountSize

	// Encode hash (32-bytes hashValue)
	rootHash := trie.RootHash()
	copy(buf[pos:], rootHash[:])
	pos += encHashSize

	return buf[:pos]
}

func ReadEncodedTrie(reader io.Reader, scratch []byte) (EncodedTrie, error) {
	if len(scratch) < encodedTrieSize {
		scratch = make([]byte, encodedTrieSize)
	}

	// Read encoded trie
	_, err := io.ReadFull(reader, scratch[:encodedTrieSize])
	if err != nil {
		return EncodedTrie{}, fmt.Errorf("failed to read serialized trie: %w", err)
	}

	pos := 0

	// Decode root node index
	rootIndex := binary.BigEndian.Uint64(scratch)
	pos += encNodeIndexSize

	// Decode trie reg count (8 bytes)
	regCount := binary.BigEndian.Uint64(scratch[pos:])
	pos += encRegCountSize

	// Decode root node hash
	readRootHash, err := hash.ToHash(scratch[pos : pos+encHashSize])
	if err != nil {
		return EncodedTrie{}, fmt.Errorf("failed to decode hash of serialized trie: %w", err)
	}

	return EncodedTrie{
		RootIndex: rootIndex,
		RegCount:  regCount,
		RootHash:  readRootHash,
	}, nil
}

// ReadTrie reconstructs a trie from data read from reader.
func ReadTrie(reader io.Reader, scratch []byte, getNode func(nodeIndex uint64) (*Node, error)) (*MTrie, error) {
	encodedTrie, err := ReadEncodedTrie(reader, scratch)
	if err != nil {
		return nil, err
	}

	rootNode, err := getNode(encodedTrie.RootIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to find root node of serialized trie: %w", err)
	}

	mtrie, err := NewMTrie(rootNode, encodedTrie.RegCount)
	if err != nil {
		return nil, fmt.Errorf("failed to restore serialized trie: %w", err)
	}

	rootHash := mtrie.RootHash()
	if !rootHash.Equals(ledger.RootHash(encodedTrie.RootHash)) {
		return nil, fmt.Errorf("failed to restore serialized trie: roothash doesn't match")
	}

	return mtrie, nil
}

// readLeafHashFromReader reads and decodes the leaf hash flag and optional
// leaf hash from reader. Returns nil if the encoded flag indicates the leaf
// hash is absent.
func readLeafHashFromReader(reader io.Reader, scratch []byte) (*hash.Hash, error) {

	if len(scratch) < encLeafHashFlagSize {
		scratch = make([]byte, encLeafHashFlagSize)
	}

	// Read leaf hash flag (1 byte)
	_, err := io.ReadFull(reader, scratch[:encLeafHashFlagSize])
	if err != nil {
		return nil, fmt.Errorf("cannot read leaf hash flag: %w", err)
	}

	flag := scratch[0]
	switch flag {
	case leafHashAbsent:
		return nil, nil
	case leafHashPresent:
		if len(scratch) < encHashSize {
			scratch = make([]byte, encHashSize)
		}
		_, err := io.ReadFull(reader, scratch[:encHashSize])
		if err != nil {
			return nil, fmt.Errorf("cannot read leaf hash: %w", err)
		}
		leafHash, err := hash.ToHash(scratch[:encHashSize])
		if err != nil {
			return nil, fmt.Errorf("failed to decode leaf hash: %w", err)
		}
		return &leafHash, nil
	default:
		return nil, fmt.Errorf("invalid leaf hash flag: %d", flag)
	}
}

// NodeIterator is an iterator over the nodes in a trie.
// It guarantees a DESCENDANTS-FIRST-RELATIONSHIP in the sequence of nodes it generates:
//   - Consider the sequence of nodes, in the order they are generated by NodeIterator.
//     Let `node[k]` denote the node with index `k` in this sequence.
//   - Descendents-First-Relationship means that for any `node[k]`, all its descendents
//     have indices strictly smaller than k in the iterator's sequence.
//
// The Descendents-First-Relationship has the following important property:
// When re-building the Trie from the sequence of nodes, one can build the trie on the fly,
// as for each node, the children have been previously encountered.
type NodeIterator struct {
	// NodeIterator internal implementation
	// NodeIterator is initialized with an empty stack and the trie's root node assigned to
	// unprocessedRoot. On the FIRST call of Next(), the NodeIterator will traverse the trie
	// starting from the root in a depth-first search (DFS) order (prioritizing the left child
	// over the right, when descending). It pushed the nodes it encounters on the stack,
	// until it hits a leaf node (which then forms the head of the stack).
	// On each subsequent call of Next(), the NodeIterator always pops the head of the stack.
	// Let `n` be the node which was popped from the stack.
	// If the `n` has a parent, denominated as `p`, the parent is now the head of the stack.
	// Parent `p` can either have one or two children.
	//   * If the parent `p` has only one child, there is no other child of `p` to enumerate.
	//   * If the parent has two children:
	//       - if `n` is the left child, we haven't searched through `p.RightChild()`
	//         (as priority is given to the left child)
	//         => we search p.RightChild() and push nodes in DFS manner on the stack
	//            until we hit the first leaf node again
	// By induction, it follows that the head of the stack always contains a node,
	// whose descendents have already been recalled:
	//   * after the initial call of Next(), the head of the stack is a leaf node, which has
	//	   no children, it can be recalled without restriction.
	//   * When popping node `n` from the stack, its parent `p` (if it exists) is now the
	//     head of the stack.
	//     - If `p` has only one child, this child must be `n`.
	//       Therefore, by recalling `n`, we have recalled all ancestors of `p`.
	//     - If `n` is the right child, we haven already searched through all of `p`
	//       descendents (as the `p.LeftChild` must have been searched before)
	//       Therefore, by recalling `n`, we have recalled all ancestors of `p`
	// Hence, it follows that the head of the stack always satisfies the
	// Descendents-First-Relationship. As we search the trie in DFS manner, each
	// node of the trie is recalled (once). Hence, the algorithm iterates all
	// nodes of the MTrie while guaranteeing Descendents-First-Relationship.

	// unprocessedRoot contains the trie's root before the first call of Next().
	// Thereafter, it is set to nil (which prevents repeated iteration through the trie).
	// This has the advantage, that we gracefully handle tries whose root node is nil.
	unprocessedRoot *Node
	stack           []*Node
	// visitedNodes are nodes that were visited and can be skipped during
	// traversal through dig(). visitedNodes is used to optimize node traveral
	// IN FOREST by skipping nodes in shared sub-tries after they are visited,
	// because sub-tries are shared between tries (original MTrie before register updates
	// and updated MTrie after register writes).
	// NodeIterator only uses visitedNodes for read operation.
	// No special handling is needed if visitedNodes is nil.
	// WARNING: visitedNodes is not safe for concurrent use.
	visitedNodes map[*Node]uint64
}

// NewNodeIterator returns a node NodeIterator, which iterates through all nodes
// comprising the MTrie. The Iterator guarantees a DESCENDANTS-FIRST-RELATIONSHIP in
// the sequence of nodes it generates:
//   - Consider the sequence of nodes, in the order they are generated by NodeIterator.
//     Let `node[k]` denote the node with index `k` in this sequence.
//   - Descendents-First-Relationship means that for any `node[k]`, all its descendents
//     have indices strictly smaller than k in the iterator's sequence.
//
// The Descendents-First-Relationship has the following important property:
// When re-building the Trie from the sequence of nodes, one can build the trie on the fly,
// as for each node, the children have been previously encountered.
// NodeIterator created by NewNodeIterator is safe for concurrent use
// because visitedNodes is always nil in this case.
func NewNodeIterator(n *Node) *NodeIterator {
	return NewUniqueNodeIterator(n, nil)
}

// NewUniqueNodeIterator returns a node NodeIterator, which iterates through all unique nodes
// that weren't visited.  This should be used for forest node iteration to avoid repeatedly
// traversing shared sub-tries.
// The Iterator guarantees a DESCENDANTS-FIRST-RELATIONSHIP in the sequence of nodes it generates:
//   - Consider the sequence of nodes, in the order they are generated by NodeIterator.
//     Let `node[k]` denote the node with index `k` in this sequence.
//   - Descendents-First-Relationship means that for any `node[k]`, all its descendents
//     have indices strictly smaller than k in the iterator's sequence.
//
// The Descendents-First-Relationship has the following important property:
// When re-building the Trie from the sequence of nodes, one can build the trie on the fly,
// as for each node, the children have been previously encountered.
// WARNING: visitedNodes is not safe for concurrent use.
func NewUniqueNodeIterator(n *Node, visitedNodes map[*Node]uint64) *NodeIterator {
	// For a Trie with height H (measured by number of edges), the longest possible path
	// contains H+1 vertices.
	stackSize := ledger.NodeMaxHeight + 1
	i := &NodeIterator{
		stack:        make([]*Node, 0, stackSize),
		visitedNodes: visitedNodes,
	}
	i.unprocessedRoot = n
	return i
}

// Next moves the cursor to the next node in order for Value method to return it.
// It returns true if there is a next node to iterate, in which case the Value method will return the node.
// It returns false if there is no more node to iterate, in which case the Value method will return nil.
func (i *NodeIterator) Next() bool {
	if i.unprocessedRoot != nil {
		// initial call to Next() for a non-empty trie
		i.dig(i.unprocessedRoot)
		i.unprocessedRoot = nil
		return len(i.stack) > 0
	}

	// the current head of the stack, `n`, has been recalled
	// we now inspect n's parent and dig into the parent's right child, if necessary
	n := i.pop()
	if len(i.stack) > 0 {
		// If there are more elements on the stack, the next element on the stack is n's parent `p`.
		// Before we can recall `p`, we need to dig into the parent's right child, if we haven't
		// done so already. As we decent into the left child with priority, the only case where
		// we still need to dig into the right child is, if n is p's left child.
		parent := i.peek()
		if parent.LeftChild() == n {
			i.dig(parent.RightChild())
		}
		return true
	}
	return false // as len(i.stack) == 0, i.e. there are no more elements to recall
}

// Value will return the current node at the cursor.
// Note: you should call Next() before calling
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
