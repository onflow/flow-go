package payloadless

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// MTrie represents a perfect in-memory full binary Merkle tree with uniform height.
// For a detailed description of the storage model, please consult `mtrie/README.md`
//
// A MTrie is a thin wrapper around a the trie's root Node. An MTrie implements the
// logic for forming MTrie-graphs from the elementary nodes. Specifically:
//   - how Nodes (graph vertices) form a Trie,
//   - how register values are read from the trie,
//   - how Merkle proofs are generated from a trie, and
//   - how a new Trie with updated values is generated.
//
// `MTrie`s are _immutable_ data structures. Updating register values is implemented through
// copy-on-write, which creates a new `MTrie`. For minimal memory consumption, all sub-tries
// that where not affected by the write operation are shared between the original MTrie
// (before the register updates) and the updated MTrie (after the register writes).
//
// MTrie expects that for a specific path, the register's key never changes.
//
// DEFINITIONS and CONVENTIONS:
//   - HEIGHT of a node v in a tree is the number of edges on the longest downward path
//     between v and a tree leaf. The height of a tree is the height of its root.
//     The height of a Trie is always the height of the fully-expanded tree.
type MTrie struct {
	root     *Node
	regCount uint64 // number of registers allocated in the trie
}

// NewEmptyMTrie returns an empty Mtrie (root is nil)
func NewEmptyMTrie() *MTrie {
	return &MTrie{root: nil}
}

// IsEmpty checks if a trie is empty.
//
// An empty try doesn't mean a trie with no allocated registers.
func (mt *MTrie) IsEmpty() bool {
	return mt.root == nil
}

// NewMTrie returns a Mtrie given the root
func NewMTrie(root *Node, regCount uint64) (*MTrie, error) {
	if root != nil && root.Height() != ledger.NodeMaxHeight {
		return nil, fmt.Errorf("height of root node must be %d but is %d, hash: %s", ledger.NodeMaxHeight, root.Height(), root.Hash().String())
	}
	return &MTrie{
		root:     root,
		regCount: regCount,
	}, nil
}

// RootHash returns the trie's root hash.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) RootHash() ledger.RootHash {
	if mt.IsEmpty() {
		// case of an empty trie
		return EmptyTrieRootHash()
	}
	return ledger.RootHash(mt.root.Hash())
}

// AllocatedRegCount returns the number of allocated registers in the trie.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) AllocatedRegCount() uint64 {
	return mt.regCount
}

// RootNode returns the Trie's root Node
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) RootNode() *Node {
	return mt.root
}

// String returns the trie's string representation.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) String() string {
	if mt.IsEmpty() {
		return fmt.Sprintf("Empty Trie with default root hash: %v\n", mt.RootHash())
	}
	trieStr := fmt.Sprintf("Trie root hash: %v\n", mt.RootHash())
	return trieStr + mt.root.FmtStr("", "")
}

// ReadSingleLeafHash reads and returns the leaf hash for a single path.
// Returns nil if no leaf exists at the given path or if the leaf represents
// an unallocated register.
func (mt *MTrie) ReadSingleLeafHash(path ledger.Path) *hash.Hash {
	return readSingleLeafHash(path, mt.root)
}

// readSingleLeafHash reads and returns the leaf hash for a single path in subtree with `head` as root node.
func readSingleLeafHash(path ledger.Path, head *Node) *hash.Hash {
	pathBytes := path[:]

	if head == nil {
		return nil
	}

	depth := ledger.NodeMaxHeight - head.Height() // distance to the tree root

	// Traverse nodes following the path until a leaf node or nil node is reached.
	for !head.IsLeaf() {
		bit := bitutils.ReadBit(pathBytes, depth)
		if bit == 0 {
			head = head.LeftChild()
		} else {
			head = head.RightChild()
		}
		depth++
	}

	if head != nil && *head.Path() == path {
		return head.LeafHash()
	}

	return nil
}

// UnsafeRead reads leaf hashes for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// CAUTION: while reading the leaf hashes, `paths` is permuted IN-PLACE for optimized processing.
// Return:
//   - `leafHashes` []*hash.Hash
//     For each path, the corresponding leaf hash is written into leafHashes. AFTER
//     the read operation completes, the order of `path` and `leafHashes` are such that
//     for `path[i]` the corresponding leaf hash is referenced by `leafHashes[i]`.
//     A nil entry indicates that no leaf exists at that path or the leaf represents
//     an unallocated register.
//
// TODO move consistency checks from Forest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeRead(paths []ledger.Path) []*hash.Hash {
	leafHashes := make([]*hash.Hash, len(paths)) // pre-allocate slice for the result
	read(leafHashes, paths, mt.root)
	return leafHashes
}

// read reads all the registers in subtree with `head` as root node. For each
// `path[i]`, the corresponding leaf hash is written into `leafHashes[i]` for the same index `i`.
// CAUTION:
//   - while reading the leaf hashes, `paths` is permuted IN-PLACE for optimized processing.
//   - unchecked requirement: all paths must go through the `head` node
func read(leafHashes []*hash.Hash, paths []ledger.Path, head *Node) {
	// check for empty paths
	if len(paths) == 0 {
		return
	}

	// path not found
	if head == nil {
		// leafHashes entries remain nil
		return
	}

	// reached a leaf node
	if head.IsLeaf() {
		for i, p := range paths {
			if *head.Path() == p {
				leafHashes[i] = head.LeafHash()
			}
			// else: leafHashes[i] remains nil
		}
		return
	}

	// reached an interim node
	if len(paths) == 1 {
		// call readSingleLeafHash to skip partition and recursive calls when there is only one path
		leafHashes[0] = readSingleLeafHash(paths[0], head)
		return
	}

	// partition step to quick sort the paths:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := ledger.NodeMaxHeight - head.Height() // distance to the tree root
	partitionIndex := SplitPaths(paths, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lLeafHashes, rLeafHashes := leafHashes[:partitionIndex], leafHashes[partitionIndex:]

	// read values from left and right subtrees in parallel
	parallelRecursionThreshold := 32 // threshold to avoid the parallelization going too deep in the recursion
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		read(lLeafHashes, lpaths, head.LeftChild())
		read(rLeafHashes, rpaths, head.RightChild())
	} else {
		// concurrent read of left and right subtree
		wg := sync.WaitGroup{}
		wg.Go(func() {
			read(lLeafHashes, lpaths, head.LeftChild())
		})
		read(rLeafHashes, rpaths, head.RightChild())
		wg.Wait() // wait for all threads
	}
}

// NewTrieWithUpdatedRegisters constructs a new trie containing all registers from the parent trie,
// and returns:
//   - updated trie
//   - max depth touched during update (this isn't affected by prune flag)
//   - error
//
// The key-value pairs specify the registers whose values are supposed to hold updated values
// compared to the parent trie. Constructing the new trie is done in a COPY-ON-WRITE manner:
//   - The original trie remains unchanged.
//   - subtries that remain unchanged are from the parent trie instead of copied.
//
// UNSAFE: method requires the following conditions to be satisfied:
//   - keys are NOT duplicated
//   - requires _all_ paths to have a length of mt.Height bits.
//
// CAUTION: `updatedPaths` and `updatedValues` are permuted IN-PLACE for optimized processing.
// CAUTION: MTrie expects that for a specific path, the value's key never changes.
// TODO: move consistency checks from MForest to here, to make API safe and self-contained
func NewTrieWithUpdatedRegisters(
	parentTrie *MTrie,
	updatedPaths []ledger.Path,
	updatedValues [][]byte,
	prune bool,
) (*MTrie, uint16, error) {
	updatedRoot, allocatedRegCountDelta, lowestHeightTouched := update(
		ledger.NodeMaxHeight,
		parentTrie.root,
		updatedPaths,
		updatedValues,
		nil,
		prune,
	)

	updatedTrieRegCount := int64(parentTrie.AllocatedRegCount()) + allocatedRegCountDelta
	maxDepthTouched := uint16(ledger.NodeMaxHeight - lowestHeightTouched)

	updatedTrie, err := NewMTrie(updatedRoot, uint64(updatedTrieRegCount))
	if err != nil {
		return nil, 0, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	return updatedTrie, maxDepthTouched, nil
}

// updateResult is a wrapper of return values from update().
// It's used to communicate values from goroutine.
type updateResult struct {
	child                  *Node
	allocatedRegCountDelta int64
	lowestHeightTouched    int
}

// update traverses the subtree recursively and create new nodes with
// the updated values on the given paths
//
// it returns:
//   - new updated node or original node if nothing was updated
//   - allocated register count delta in subtrie (allocatedRegCountDelta)
//   - lowest height reached during recursive update in subtrie (lowestHeightTouched)
//
// update also compact a subtree into a single compact leaf node in the case where
// there is only 1 value stored in the subtree.
//
// allocatedRegCountDelta is used to compute updated trie's allocated register count.
// lowestHeightTouched is used to compute max depth touched during update.
// CAUTION: while updating, `paths` and `values` are permuted IN-PLACE for optimized processing.
// UNSAFE: method requires the following conditions to be satisfied:
//   - paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
//   - paths are NOT duplicated
func update(
	nodeHeight int, // the height of the node during traversing the subtree
	currentNode *Node, // the current node on the travesing path, if it's nil it means the trie has no node on this path
	paths []ledger.Path, // the paths to update the values
	values [][]byte, // the values to be updated at the given paths
	compactLeaf *Node, // a compact leaf node from its ancester, it could be nil
	prune bool, // prune is a flag for whether pruning nodes with empty values. not pruning is useful for generating proof, expecially non-inclusion proof
) (n *Node, allocatedRegCountDelta int64, lowestHeightTouched int) {
	// No new path to update
	if len(paths) == 0 {
		if compactLeaf != nil {
			// if a compactLeaf from a higher height is still left,
			// then expand the compact leaf node to the current height by creating a new compact leaf
			// node with the same path and value.
			// The old node shouldn't be recycled as it is still used by the tree copy before the update.
			if compactLeaf.leafHash != nil {
				n = NewLeafWithHash(compactLeaf.path, *compactLeaf.leafHash, nodeHeight)
			} else {
				n = NewLeaf(compactLeaf.path, nil, nodeHeight)
			}
			return n, 0, nodeHeight
		}
		// if no path to update and there is no compact leaf node on this path, we return
		// the current node regardless it exists or not.
		return currentNode, 0, nodeHeight
	}

	if len(paths) == 1 && currentNode == nil && compactLeaf == nil {
		// if there is only 1 path to update, and the existing tree has no node on this path, also
		// no compact leaf node from its ancester, it means we are storing a value on a new path,
		n = NewLeaf(paths[0], values[0], nodeHeight)
		if len(values[0]) == 0 {
			// if we are storing an empty value, then no register is allocated
			// allocatedRegCountDelta should be 0
			return n, 0, nodeHeight
		}
		// if we are storing a non-empty value, we are allocating a new register
		return n, 1, nodeHeight
	}

	if currentNode != nil && currentNode.IsLeaf() { // if we're here then compactLeaf == nil
		// check if the current node path is among the updated paths
		found := false
		currentPath := *currentNode.Path()
		for i, p := range paths {
			if p == currentPath {
				// the case where the recursion stops: only one path to update
				if len(paths) == 1 {
					// check if the only path to update has the same value.
					// if value is the same, we could skip the update to avoid creating duplicated node
					hadValue := currentNode.leafHash != nil
					hasValue := len(values[i]) > 0
					var newLeafHash hash.Hash
					if hasValue {
						newLeafHash = hash.HashLeaf(hash.Hash(paths[i]), values[i])
					}

					if hadValue == hasValue {
						// when value equals, if didn't have value before, then still no value after update;
						// if had value before, then the leaf hash is still the same after update,
						// so we can reuse the current node without creating a new one.
						if !hasValue || *currentNode.leafHash == newLeafHash {
							// avoid creating a new node when the same value is written
							return currentNode, 0, nodeHeight
						}
					}

					// the value is updated, we need to create a new leaf node with the updated value.
					// The old node shouldn't be recycled as it is still used by the trie before the update.
					if hasValue {
						n = NewLeafWithHash(paths[i], newLeafHash, nodeHeight)
					} else {
						n = NewLeaf(paths[i], nil, nodeHeight)
					}
					allocatedRegCountDelta = computeAllocatedRegCountDelta(hadValue, hasValue)
					return n, allocatedRegCountDelta, nodeHeight
				}
				// the case where the recursion carries on: len(paths)>1
				found = true
				allocatedRegCountDelta = computeAllocatedRegCountDeltaFromHigherHeight(currentNode.leafHash != nil)
				break
			}
		}
		if !found {
			// if the current node carries a path not included in the input path, then the current node
			// represents a compact leaf that needs to be carried down the recursion.
			compactLeaf = currentNode
		}
	}

	// in the remaining code:
	//   - either len(paths) > 1
	//   - or len(paths) == 1 and compactLeaf!= nil
	//   - or len(paths) == 1 and currentNode != nil && !currentNode.IsLeaf()

	// Split paths and values to recurse:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := ledger.NodeMaxHeight - nodeHeight // distance to the tree root
	partitionIndex := splitByPath(paths, values, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lvalues, rvalues := values[:partitionIndex], values[partitionIndex:]

	// check if there is a compact leaf that needs to get deep to height 0
	var lcompactLeaf, rcompactLeaf *Node
	if compactLeaf != nil {
		// if yes, check which branch it will go to.
		path := *compactLeaf.Path()
		if bitutils.ReadBit(path[:], depth) == 0 {
			lcompactLeaf = compactLeaf
		} else {
			rcompactLeaf = compactLeaf
		}
	}

	// set the node children
	var oldLeftChild, oldRightChild *Node
	if currentNode != nil {
		oldLeftChild = currentNode.LeftChild()
		oldRightChild = currentNode.RightChild()
	}

	// recurse over each branch
	var newLeftChild, newRightChild *Node
	var lRegCountDelta, rRegCountDelta int64
	var lLowestHeightTouched, rLowestHeightTouched int
	parallelRecursionThreshold := 16
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		// runtime optimization: if there are _no_ updates for either left or right sub-tree, proceed single-threaded
		newLeftChild, lRegCountDelta, lLowestHeightTouched = update(nodeHeight-1, oldLeftChild, lpaths, lvalues, lcompactLeaf, prune)
		newRightChild, rRegCountDelta, rLowestHeightTouched = update(nodeHeight-1, oldRightChild, rpaths, rvalues, rcompactLeaf, prune)
	} else {
		// runtime optimization: process the left child in a separate thread

		// Since we're receiving 3 values from goroutine, use a
		// struct and channel to reduce allocs/op.
		// Although WaitGroup approach can be faster than channel (esp. with 2+ goroutines),
		// we only use 1 goroutine here and need to communicate results from it. So using
		// channel is faster and uses fewer allocs/op in this case.
		results := make(chan updateResult, 1)
		go func(retChan chan<- updateResult) {
			child, regCountDelta, lowestHeightTouched := update(nodeHeight-1, oldLeftChild, lpaths, lvalues, lcompactLeaf, prune)
			retChan <- updateResult{child, regCountDelta, lowestHeightTouched}
		}(results)

		newRightChild, rRegCountDelta, rLowestHeightTouched = update(nodeHeight-1, oldRightChild, rpaths, rvalues, rcompactLeaf, prune)

		// Wait for results from goroutine.
		ret := <-results
		newLeftChild, lRegCountDelta, lLowestHeightTouched = ret.child, ret.allocatedRegCountDelta, ret.lowestHeightTouched
	}

	allocatedRegCountDelta += lRegCountDelta + rRegCountDelta
	lowestHeightTouched = minInt(lLowestHeightTouched, rLowestHeightTouched)

	// mitigate storage exhaustion attack: avoids creating a new node when the exact same
	// value is re-written at a register. CAUTION: we only check that the children are
	// unchanged. This is only sufficient for interim nodes (for leaf nodes, the children
	// might be unchanged, i.e. both nil, but the value could have changed).
	// In case the current node was a leaf, we _cannot reuse_ it, because we potentially
	// updated registers in the sub-trie
	if !currentNode.IsLeaf() && newLeftChild == oldLeftChild && newRightChild == oldRightChild {
		return currentNode, 0, lowestHeightTouched
	}

	// if prune is on, then will check and create a compact leaf node if one child is nil, and the
	// other child is a leaf node
	if prune {
		n = NewInterimCompactifiedNode(nodeHeight, newLeftChild, newRightChild)
		return n, allocatedRegCountDelta, lowestHeightTouched
	}

	n = NewInterimNode(nodeHeight, newLeftChild, newRightChild)
	return n, allocatedRegCountDelta, lowestHeightTouched
}

// computeAllocatedRegCountDeltaFromHigherHeight returns the delta
// needed to compute the allocated reg count when
// a value is updated or unallocated at a lower height.
func computeAllocatedRegCountDeltaFromHigherHeight(hadValue bool) (allocatedRegCountDelta int64) {
	if hadValue {
		// Allocated register will be updated or unallocated at lower height.
		allocatedRegCountDelta--
	}
	return
}

// computeAllocatedRegCountDelta returns the allocated reg count
// delta computed from the presence of the old and new value.
// PRECONDITION: hadValue != hasValue OR the stored value changed
func computeAllocatedRegCountDelta(hadValue, hasValue bool) (allocatedRegCountDelta int64) {
	allocatedRegCountDelta = 0
	if !hasValue {
		// Old value is present while new value is empty.
		// Allocated register will be unallocated.
		allocatedRegCountDelta = -1
	} else if !hadValue {
		// Old value is empty while new value is present.
		// Unallocated register will be allocated.
		allocatedRegCountDelta = 1
	}
	return
}

// UnsafeProofs provides proofs for the given paths.
//
// CAUTION: while updating, `paths` and `proofs` are permuted IN-PLACE for optimized processing.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// Paths in the input query don't have to be deduplicated, though deduplication would
// result in allocating less dynamic memory to store the proofs.
func (mt *MTrie) UnsafeProofs(paths []ledger.Path) *ledger.PayloadlessTrieBatchProof {
	batchProofs := ledger.NewPayloadlessTrieBatchProofWithEmptyProofs(len(paths))
	prove(mt.root, paths, batchProofs.Proofs)
	return batchProofs
}

// prove traverses the subtree and stores proofs for the given register paths in
// the provided `proofs` slice
// CAUTION: while updating, `paths` and `proofs` are permuted IN-PLACE for optimized processing.
// UNSAFE: method requires the following conditions to be satisfied:
//   - paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
func prove(head *Node, paths []ledger.Path, proofs []*ledger.PayloadlessTrieProof) {
	// check for empty paths
	if len(paths) == 0 {
		return
	}

	// we've reached the end of a trie
	// and path is not found (noninclusion proof)
	if head == nil {
		// by default, proofs are non-inclusion proofs
		return
	}

	// we've reached a leaf
	if head.IsLeaf() {
		for i, path := range paths {
			// value matches (inclusion proof)
			if *head.Path() == path {
				proofs[i].Path = *head.Path()
				proofs[i].LeafHash = head.LeafHash()
				proofs[i].Inclusion = true
			}
		}
		// by default, proofs are non-inclusion proofs
		return
	}

	// increment steps for all the proofs
	for _, p := range proofs {
		p.Steps++
	}

	// partition step to quick sort the paths:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := ledger.NodeMaxHeight - head.Height() // distance to the tree root
	partitionIndex := splitTrieProofsByPath(paths, proofs, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lproofs, rproofs := proofs[:partitionIndex], proofs[partitionIndex:]

	parallelRecursionThreshold := 64 // threshold to avoid the parallelization going too deep in the recursion
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		// runtime optimization: below the parallelRecursionThreshold, we proceed single-threaded
		addSiblingTrieHashToProofs(head.RightChild(), depth, lproofs)
		prove(head.LeftChild(), lpaths, lproofs)

		addSiblingTrieHashToProofs(head.LeftChild(), depth, rproofs)
		prove(head.RightChild(), rpaths, rproofs)
	} else {
		wg := sync.WaitGroup{}
		wg.Go(func() {
			addSiblingTrieHashToProofs(head.RightChild(), depth, lproofs)
			prove(head.LeftChild(), lpaths, lproofs)
		})

		addSiblingTrieHashToProofs(head.LeftChild(), depth, rproofs)
		prove(head.RightChild(), rpaths, rproofs)
		wg.Wait()
	}
}

// addSiblingTrieHashToProofs inspects the sibling Trie and adds its root hash
// to the proofs, if the trie contains non-empty registers (i.e. the
// siblingTrie has a non-default hash).
func addSiblingTrieHashToProofs(siblingTrie *Node, depth int, proofs []*ledger.PayloadlessTrieProof) {
	if siblingTrie == nil || len(proofs) == 0 {
		return
	}

	// This code is necessary, because we do not remove nodes from the trie
	// when a register is deleted. Instead, we just set the respective leaf's
	// payload to empty. While this will cause the lead's hash to become the
	// default hash, the node itself remains as part of the trie.
	// However, a proof has the convention that the hash of the sibling trie
	// should only be included, if it is _non-default_. Therefore, we can
	// neither use `siblingTrie == nil` nor `siblingTrie.RegisterCount == 0`,
	// as the sibling trie might contain leaves with default value (which are
	// still counted as occupied registers)
	// TODO: On update, prune subtries which only contain empty registers.
	//       Then, a child is nil if and only if the subtrie is empty.

	nodeHash := siblingTrie.Hash()
	isDef := nodeHash == ledger.GetDefaultHashForHeight(siblingTrie.Height())
	if !isDef { // in proofs, we only provide non-default value hashes
		for _, p := range proofs {
			bitutils.SetBit(p.Flags, depth)
			p.Interims = append(p.Interims, nodeHash)
		}
	}
}

// Equals compares two tries for equality.
// Tries are equal iff they store the same data (i.e. root hash matches)
// and their number and height are identical
func (mt *MTrie) Equals(o *MTrie) bool {
	if o == nil {
		return false
	}
	return o.RootHash() == mt.RootHash()
}

// DumpAsJSON dumps the trie leaf entries to a writer having each leaf as a json row.
// Each entry contains the leaf's path and its stored leaf hash.
func (mt *MTrie) DumpAsJSON(w io.Writer) error {

	// Use encoder to prevent building entire trie in memory
	enc := json.NewEncoder(w)

	err := dumpAsJSON(mt.root, enc)
	if err != nil {
		return err
	}

	return nil
}

// dumpLeafEntry is the JSON form of a leaf encoded by DumpAsJSON.
type dumpLeafEntry struct {
	Path     ledger.Path `json:"path"`
	LeafHash *hash.Hash  `json:"leafHash"`
}

// dumpAsJSON serializes the sub-trie with root n to json and feeds it into encoder
func dumpAsJSON(n *Node, encoder *json.Encoder) error {
	if n.IsLeaf() {
		if n != nil {
			err := encoder.Encode(dumpLeafEntry{Path: n.path, LeafHash: n.leafHash})
			if err != nil {
				return err
			}
		}
		return nil
	}

	if lChild := n.LeftChild(); lChild != nil {
		err := dumpAsJSON(lChild, encoder)
		if err != nil {
			return err
		}
	}

	if rChild := n.RightChild(); rChild != nil {
		err := dumpAsJSON(rChild, encoder)
		if err != nil {
			return err
		}
	}
	return nil
}

// EmptyTrieRootHash returns the rootHash of an empty Trie for the specified path size [bytes]
func EmptyTrieRootHash() ledger.RootHash {
	return ledger.RootHash(ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight))
}

// AllLeafHashes returns all leaf hashes stored in the trie. Empty leaves
// (unallocated registers) are skipped.
func (mt *MTrie) AllLeafHashes() []*hash.Hash {
	return mt.root.AllLeafHashes()
}

// IsAValidTrie verifies the content of the trie for potential issues
func (mt *MTrie) IsAValidTrie() bool {
	// TODO add checks on the health of node max height ...
	return mt.root.VerifyCachedHash()
}

// splitByPath permutes the input paths to be partitioned into 2 parts. The first part contains paths with a zero bit
// at the input bitIndex, the second part contains paths with a one at the bitIndex. The index of partition
// is returned. The same permutation is applied to the values slice.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
//
//	For instance, if `paths` contains the following 3 paths, and bitIndex is `1`:
//	[[0,0,1,1], [0,1,0,1], [0,0,0,1]]
//	then `splitByPath` returns 2 and updates `paths` into:
//	[[0,0,1,1], [0,0,0,1], [0,1,0,1]]
func splitByPath(paths []ledger.Path, values [][]byte, bitIndex int) int {
	i := 0
	for j, path := range paths {
		bit := bitutils.ReadBit(path[:], bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			values[i], values[j] = values[j], values[i]
			i++
		}
	}
	return i
}

// SplitPaths permutes the input paths to be partitioned into 2 parts. The first part contains paths with a zero bit
// at the input bitIndex, the second part contains paths with a one at the bitIndex. The index of partition
// is returned.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func SplitPaths(paths []ledger.Path, bitIndex int) int {
	i := 0
	for j, path := range paths {
		bit := bitutils.ReadBit(path[:], bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			i++
		}
	}
	return i
}

// splitTrieProofsByPath permutes the input paths to be partitioned into 2 parts. The first part contains paths
// with a zero bit at the input bitIndex, the second part contains paths with a one at the bitIndex. The index
// of partition is returned. The same permutation is applied to the proofs slice.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func splitTrieProofsByPath(paths []ledger.Path, proofs []*ledger.PayloadlessTrieProof, bitIndex int) int {
	i := 0
	for j, path := range paths {
		bit := bitutils.ReadBit(path[:], bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			proofs[i], proofs[j] = proofs[j], proofs[i]
			i++
		}
	}
	return i
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TraverseNodes traverses all nodes of the trie in DFS order
func TraverseNodes(trie *MTrie, processNode func(*Node) error) error {
	return traverseRecursive(trie.root, processNode)
}

func traverseRecursive(n *Node, processNode func(*Node) error) error {
	if n == nil {
		return nil
	}

	err := processNode(n)
	if err != nil {
		return err
	}

	err = traverseRecursive(n.LeftChild(), processNode)
	if err != nil {
		return err
	}

	err = traverseRecursive(n.RightChild(), processNode)
	if err != nil {
		return err
	}

	return nil
}
