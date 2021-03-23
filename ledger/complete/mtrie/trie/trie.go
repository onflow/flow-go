package trie

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

// MTrie represents a perfect in-memory full binary Merkle tree with uniform height.
// For a detailed description of the storage model, please consult `mtrie/README.md`
//
// A MTrie is a thin wrapper around a the trie's root Node. An MTrie implements the
// logic for forming MTrie-graphs from the elementary nodes. Specifically:
//   * how Nodes (graph vertices) form a Trie,
//   * how register values are read from the trie,
//   * how Merkle proofs are generated from a trie, and
//   * how a new Trie with updated values is generated.
//
// `MTrie`s are _immutable_ data structures. Updating register values is implemented through
// copy-on-write, which creates a new `MTrie`. For minimal memory consumption, all sub-tries
// that where not affected by the write operation are shared between the original MTrie
// (before the register updates) and the updated MTrie (after the register writes).
//
// DEFINITIONS and CONVENTIONS:
//   * HEIGHT of a node v in a tree is the number of edges on the longest downward path
//     between v and a tree leaf. The height of a tree is the height of its root.
//     The height of a Trie is always the height of the fully-expanded tree.
type MTrie struct {
	root         *node.Node
	pathByteSize int
}

// NewEmptyMTrie returns an empty Mtrie (root is an empty node)
func NewEmptyMTrie(pathByteSize int) (*MTrie, error) {
	if pathByteSize < 1 {
		return nil, errors.New("trie's path size [in bytes] must be positive")
	}
	height := pathByteSize * 8
	return &MTrie{
		root:         node.NewEmptyTreeRoot(height),
		pathByteSize: pathByteSize,
	}, nil
}

// NewMTrie returns a Mtrie given the root
func NewMTrie(root *node.Node) (*MTrie, error) {
	if root.Height()%8 != 0 {
		return nil, errors.New("height of root node must be integer-multiple of 8")
	}
	pathByteSize := root.Height() / 8
	return &MTrie{
		root:         root,
		pathByteSize: pathByteSize,
	}, nil
}

// Height returns the height of the trie, which
// is the height of its root node.
func (mt *MTrie) Height() int { return mt.root.Height() }

// StringRootHash returns the trie's Hex-encoded root hash.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) StringRootHash() string { return hex.EncodeToString(mt.root.Hash()) }

// RootHash returns the trie's root hash (i.e. the hash of the trie's root node).
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) RootHash() []byte { return mt.root.Hash() }

// PathLength return the length [in bytes] the trie operates with.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) PathLength() int { return mt.pathByteSize }

// AllocatedRegCount returns the number of allocated registers in the trie.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) AllocatedRegCount() uint64 { return mt.root.RegCount() }

// MaxDepth returns the length of the longest branch from root to leaf.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) MaxDepth() uint16 { return mt.root.MaxDepth() }

// RootNode returns the Trie's root Node
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) RootNode() *node.Node {
	return mt.root
}

// StringRootHash returns the trie's string representation.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) String() string {
	trieStr := fmt.Sprintf("Trie root hash: %v\n", mt.StringRootHash())
	return trieStr + mt.root.FmtStr("", "")
}

// UnsafeRead read payloads for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// CAUTION: while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
// Return:
//  * `payloads` []*ledger.Payload
//     For each path, the corresponding payload is written into payloads. AFTER
//     the read operation completes, the order of `path` and `payloads` are such that
//     for `path[i]` the corresponding register value is referenced by 0`payloads[i]`.
// TODO move consistency checks from Forest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeRead(paths []ledger.Path) []*ledger.Payload {
	// allocate the result buffer
	res := make([]*ledger.Payload, len(paths))
	mt.read(&res, mt.root, paths)
	return res
}

// read reads all the registers in subtree with `head` as root node. For each
// `path[i]`, the corresponding payload is written into `payloads[i]` for the same index `i`.
// CAUTION:
//  * while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
//  * unchecked requirement: all paths must go through the `head` node
func (mt *MTrie) read(head *node.Node, paths []ledger.Path, payloads []*ledger.Payload) {
	// check for empty paths
	if len(paths) == 0 {
		return
	}

	// path not found
	if head == nil {
		for i := range paths {
			payloads[i] = ledger.EmptyPayload()
		}
		return
	}
	// reached a leaf node
	if head.IsLeaf() {
		for i, p := range paths {
			if bytes.Equal(head.Path(), p) {
				payloads[i] = head.Payload()
			} else {
				payloads[i] = ledger.EmptyPayload()
			}
		}
		return
	}

	// partition step to quick sort the paths:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	heightIndex := mt.Height() - head.Height() // distance to the tree root
	partitionIndex := utils.SplitPaths(paths, heightIndex)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// read values from left and right subtrees in parallel
	parallelRecursionThreshold := 32 // threshold to avoid the parallelization going too deep in the recursion
	if len(lpaths) <= parallelRecursionThreshold {
		mt.read(head.LeftChild(), lpaths, lpayloads)
		mt.read(head.RightChild(), rpaths, rpayloads)
	} else {
		// concurrent read of left and right subtree
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			mt.read(head.LeftChild(), lpaths, lpayloads)
			wg.Done()
		}()
		mt.read(head.RightChild(), rpaths, rpayloads)
		wg.Wait() // wait for all threads
	}
}

// NewTrieWithUpdatedRegisters constructs a new trie containing all registers from the parent trie.
// The key-value pairs specify the registers whose values are supposed to hold updated values
// compared to the parent trie. Constructing the new trie is done in a COPY-ON-WRITE manner:
//   * The original trie remains unchanged.
//   * subtries that remain unchanged are from the parent trie instead of copied.
// UNSAFE: method requires the following conditions to be satisfied:
//   * keys are NOT duplicated
// CAUTION: `updatedPaths` and `updatedPayloads` are permuted IN-PLACE for optimized processing.
// TODO: move consistency checks from MForest to here, to make API safe and self-contained
func NewTrieWithUpdatedRegisters(parentTrie *MTrie, updatedPaths []ledger.Path, updatedPayloads []ledger.Payload) (*MTrie, error) {
	if len(updatedPaths) == 0 { // No new paths to write
		return parentTrie, nil
	}

	parentRoot := parentTrie.root
	updatedRoot := parentTrie.update(parentRoot.Height(), parentRoot, updatedPaths, updatedPayloads)
	if parentRoot == updatedRoot {
		return parentTrie, nil
	}
	updatedTrie, err := NewMTrie(updatedRoot)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	return updatedTrie, nil
}

// update traverses the subtree and updates the stored registers
// UNSAFE: method requires the following conditions to be satisfied:
//   * paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
//   * paths contains at least one element
//   * paths are NOT duplicated
func (parentTrie *MTrie) update(
	nodeHeight int, parentNode *node.Node,
	paths []ledger.Path, payloads []ledger.Payload,
) *node.Node {
	if parentNode == nil { // arrived at default node in parent tree, which needs to be expanded
		return constructSubtrie(parentTrie.Height(), nodeHeight, paths, payloads)
	}
	if parentNode.IsLeaf() { // arrived at compactified leaf of parent tree, which needs to be expanded
		return replaceLeaf(parentTrie.Height(), parentNode, parentNode.Height(), paths, payloads)
	}

	// Split paths and payloads to recurse:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := parentTrie.Height() - parentNode.Height() // distance to the tree root
	partitionIndex := utils.SplitByPath(paths, payloads, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// runtime optimization: if there are _no_ value for either left or right sub-tree, proceed single-threaded
	if len(lpaths) == 0 {
		rChild := parentTrie.update(nodeHeight-1, parentNode.RightChild(), rpaths, rpayloads)
		if parentNode.RightChild() == rChild {
			return parentNode
		}
		return node.NewInterimNode(parentNode.Height(), parentNode.LeftChild(), rChild)
	}
	if len(rpaths) == 0 {
		lChild := parentTrie.update(nodeHeight-1, parentNode.LeftChild(), lpaths, lpayloads)
		if parentNode.LeftChild() == lChild {
			return parentNode
		}
		return node.NewInterimNode(parentNode.Height(), lChild, parentNode.RightChild())
	}

	// recurse into left and right subtree
	var lChild, rChild *node.Node
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild = parentTrie.update(nodeHeight-1, parentNode.LeftChild(), lpaths, lpayloads)
	}()
	rChild = parentTrie.update(nodeHeight-1, parentNode.RightChild(), rpaths, rpayloads)
	wg.Wait()

	// mitigate storage exhaustion attack: avoids creating a new node when the exact same
	// payload is re-written at a register.
	if lChild == parentNode.LeftChild() && rChild == parentNode.RightChild() {
		return parentNode
	}
	return node.NewInterimNode(nodeHeight, lChild, rChild)
}

// replaceLeaf replaces the (compactified) leaf with an entire subtree containing
// paths with respective payloads. This method
// the head of a newly-constructed sub-trie for the specified key-value pairs.
// UNSAFE: replaceLeaf requires the following conditions to be satisfied,
// but does not explicitly check them for performance reasons
//   * leaf is not nil
//   * paths all share the same common prefix [0 : mt.maxHeight-1 - headHeight)
//     (excluding the bit at index headHeight)
//   * paths contains at least one element
//   * paths are NOT duplicated
func replaceLeaf(
	treeHeight int, leaf *node.Node,
	height int, paths []ledger.Path, payloads []ledger.Payload,
) *node.Node {
	// only the leaf remains for this subtree
	if len(paths) == 0 {
		if leaf.Height() == height {
			return leaf
		}
		return node.NewLeaf(leaf.Path(), leaf.Payload(), height)
	}
	if len(paths) == 1 {
		path := paths[0]
		payload := payloads[0]
		if bytes.Equal(path, leaf.Path()) { // override leaf's register
			if bytes.Equal(payload.Value, leaf.Payload().Value) && leaf.Height() == height {
				return leaf
			}
			return node.NewLeaf(path, &payload, height)
		}
		// write to _different_ register than leaf:
		// we allocate two slices here; however, this is negligible, as it happens only _once_ during a full update
		return constructSubtrie(treeHeight, height, []ledger.Path{path, leaf.Path()}, []ledger.Payload{payload, *leaf.Payload()})
	}

	// Split paths and payloads to recurse:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := treeHeight - height // distance to the tree root
	partitionIndex := utils.SplitByPath(paths, payloads, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// if yes, check which branch it will go to.
	var lChild, rChild *node.Node
	if utils.Bit(leaf.Path(), depth) == 0 { // leaf goes into the left subtree
		if len(rpaths) > 0 {
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				rChild = constructSubtrie(treeHeight, height-1, rpaths, rpayloads)
			}()
			lChild = replaceLeaf(treeHeight, leaf, height-1, lpaths, lpayloads)
			wg.Wait()
		} else {
			lChild = replaceLeaf(treeHeight, leaf, height-1, lpaths, lpayloads)
		}
	} else { // leaf goes into the right subtree
		if len(lpaths) > 0 {
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				lChild = constructSubtrie(treeHeight, height-1, lpaths, lpayloads)
			}()
			rChild = replaceLeaf(treeHeight, leaf, height-1, rpaths, rpayloads)
			wg.Wait()
		} else {
			rChild = replaceLeaf(treeHeight, leaf, height-1, rpaths, rpayloads)
		}
	}
	return node.NewInterimNode(height, lChild, rChild)
}

// constructSubtrie returns the head of a newly-constructed sub-trie for the specified key-value pairs.
// UNSAFE: constructSubtrie requires the following conditions to be satisfied,
// but does not explicitly check them for performance reasons
//   * paths all share the same common prefix [0 : mt.maxHeight-1 - headHeight)
//     (excluding the bit at index headHeight)
//   * paths contains at least one element
//   * paths are NOT duplicated
func constructSubtrie(treeHeight int, nodeHeight int, paths []ledger.Path, payloads []ledger.Payload) *node.Node {
	// If we are at a leaf node, we create the node
	if len(paths) == 1 {
		return node.NewLeaf(paths[0], &payloads[0], nodeHeight)
	}
	// from here on, we have: len(paths) > 1

	// Split paths and payloads to recurse:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := treeHeight - nodeHeight // distance to the tree root
	partitionIndex := utils.SplitByPath(paths, payloads, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// runtime optimization: if there are _no_ value for either left or right sub-tree, proceed single-threaded
	if len(lpaths) == 0 {
		rChild := constructSubtrie(treeHeight, nodeHeight-1, rpaths, rpayloads)
		return node.NewInterimNode(nodeHeight, nil, rChild)
	}
	if len(rpaths) == 0 {
		lChild := constructSubtrie(treeHeight, nodeHeight-1, lpaths, lpayloads)
		return node.NewInterimNode(nodeHeight, lChild, nil)
	}

	// we only reach this code, if we have values for the left _and_ right subtree;
	// concurrently construct left and right subtree
	var lChild, rChild *node.Node
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild = constructSubtrie(treeHeight, nodeHeight-1, lpaths, lpayloads)
	}()
	rChild = constructSubtrie(treeHeight, nodeHeight-1, rpaths, rpayloads)
	wg.Wait()
	return node.NewInterimNode(nodeHeight, lChild, rChild)
}

// UnsafeProofs provides proofs for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
func (mt *MTrie) UnsafeProofs(paths []ledger.Path, proofs []*ledger.TrieProof) {
	mt.proofs(mt.root, paths, proofs)
}

func (mt *MTrie) proofs(head *node.Node, paths []ledger.Path, proofs []*ledger.TrieProof) {
	// check for empty paths
	if len(paths) == 0 {
		return
	}

	// we've reached the end of a trie
	// and path is not found (noninclusion proof)
	if head == nil {
		return
	}

	// we've reached a leaf
	if head.IsLeaf() {
		// value matches (inclusion proof)
		if bytes.Equal(head.Path(), paths[0]) {
			proofs[0].Path = head.Path()
			proofs[0].Payload = head.Payload()
			proofs[0].Inclusion = true
		}
		// TODO: insert ERROR if len(paths) != 1
		return
	}

	// increment steps for all the proofs
	for _, p := range proofs {
		p.Steps++
	}

	// partition step to quick sort the paths:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	heightIndex := mt.Height() - head.Height() // distance to the tree root
	partitionIndex := utils.SplitTrieProofsByPath(paths, proofs, heightIndex)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lproofs, rproofs := proofs[:partitionIndex], proofs[partitionIndex:]

	wg := sync.WaitGroup{}
	parallelRecursionThreshold := 128 // thresold to avoid the parallelization going too deep in the recursion

	if rChild := head.RightChild(); rChild != nil { // TODO: is that a sanity check?
		nodeHash := rChild.Hash()
		isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(rChild.Height())) // TODO: why not rChild.RegisterCount != 0?
		if !isDef {                                                                     // in proofs, we only provide non-default value hashes
			for _, p := range lproofs {
				utils.SetBit(p.Flags, heightIndex)
				p.Interims = append(p.Interims, nodeHash)
			}
		}
	}
	if len(lpaths) > parallelRecursionThreshold {
		wg.Add(1)
		go func() {
			mt.proofs(head.LeftChild(), lpaths, lproofs)
			wg.Done()
		}()
	} else {
		mt.proofs(head.LeftChild(), lpaths, lproofs)
	}

	if lChild := head.LeftChild(); lChild != nil {
		nodeHash := lChild.Hash()
		isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(lChild.Height()))
		if !isDef { // in proofs, we only provide non-default value hashes
			for _, p := range rproofs {
				utils.SetBit(p.Flags, heightIndex)
				p.Interims = append(p.Interims, nodeHash)
			}
		}
	}
	mt.proofs(head.RightChild(), rpaths, rproofs)
	wg.Wait()
}

// Equals compares two tries for equality.
// Tries are equal iff they store the same data (i.e. root hash matches)
// and their number and height are identical
func (mt *MTrie) Equals(o *MTrie) bool {
	if o == nil {
		return false
	}
	return o.PathLength() == mt.PathLength() && bytes.Equal(o.RootHash(), mt.RootHash())
}

// DumpAsJSON dumps the trie key value pairs to a file having each key value pair as a json row
func (mt *MTrie) DumpAsJSON(w io.Writer) error {

	// Use encoder to prevent building entire trie in memory
	enc := json.NewEncoder(w)

	err := mt.dumpAsJSON(mt.root, enc)
	if err != nil {
		return err
	}

	return nil
}

func (mt *MTrie) dumpAsJSON(n *node.Node, encoder *json.Encoder) error {
	if n.IsLeaf() {
		err := encoder.Encode(n.Payload())
		if err != nil {
			return err
		}
		return nil
	}

	if lChild := n.LeftChild(); lChild != nil {
		err := mt.dumpAsJSON(lChild, encoder)
		if err != nil {
			return err
		}
	}

	if rChild := n.RightChild(); rChild != nil {
		err := mt.dumpAsJSON(rChild, encoder)
		if err != nil {
			return err
		}
	}
	return nil
}

// EmptyTrieRootHash returns the rootHash of an empty Trie for the specified path size [bytes]
func EmptyTrieRootHash(pathByteSize int) []byte {
	return node.NewEmptyTreeRoot(8 * pathByteSize).Hash()
}

// AllPayloads returns all payloads
func (mt *MTrie) AllPayloads() []ledger.Payload {
	return mt.root.AllPayloads()
}

// IsAValidTrie verifies the content of the trie for potential issues
func (mt *MTrie) IsAValidTrie() bool {
	// TODO add checks on the health of node max height ...
	return mt.root.VerifyCachedHash()
}
