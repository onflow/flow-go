package payloadless

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// MTrie represents a perfect in-memory full binary Merkle tree with uniform height.
// For a detailed description of the storage model, please consult `mtrie/README.md`
//
// A MTrie is a thin wrapper around a trie's root Node. An MTrie implements the
// logic for forming MTrie-graphs from the elementary nodes. Specifically:
//   - how Nodes (graph vertices) form a Trie,
//   - how register values are read from the trie,
//   - how Merkle proofs are generated from a trie, and
//   - how a new Trie with updated values is generated.
//
// `MTrie`s are _immutable_ data structures. Updating register values is implemented through
// copy-on-write, which creates a new `MTrie`. For minimal memory consumption, all sub-tries
// that were not affected by the write operation are shared between the original MTrie
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
// An empty trie (root is nil) is not the same as a trie whose registers are all unallocated.
func (mt *MTrie) IsEmpty() bool {
	return mt.root == nil
}

// NewMTrie returns a Mtrie given the root.
//
// No error returns are expected during normal operation.
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
//
// CAUTION: the returned pointer aliases the trie node's internal leaf hash. Tries are immutable and
// shared copy-on-write across trie versions, so the caller MUST NOT modify the pointee; doing so would
// corrupt the node's cached hash. Callers needing a mutable hash must copy it (see the Forest layer,
// which returns a defensive copy). Do NOT MODIFY the returned hash!
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
//   - subtries that remain unchanged are referenced from the parent trie instead of copied.
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

// update traverses the subtree recursively and creates new nodes with
// the updated values on the given paths
//
// it returns:
//   - new updated node or original node if nothing was updated
//   - allocated register count delta in subtrie (allocatedRegCountDelta)
//   - lowest height reached during recursive update in subtrie (lowestHeightTouched)
//
// update also compacts a subtree into a single compact leaf node in the case where
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
	currentNode *Node, // the current node on the traversing path, if it's nil it means the trie has no node on this path
	paths []ledger.Path, // the paths to update the values
	values [][]byte, // the values to be updated at the given paths
	compactLeaf *Node, // a compact leaf node from its ancestor, it could be nil
	prune bool, // prune flag specifies whether the update should prune nodes with empty values; not pruning is useful for generating proof, especially non-inclusion proof
) (n *Node, allocatedRegCountDelta int64, lowestHeightTouched int) {
	// IMPLEMENTATION Notes:
	// - This method proceeds recursively, essentially partitioning the remaining set of `paths` and corresponding `values` according to each bit of
	//   the path we are descending down. In essence, the base case of the recursion is when there is only a single path-value pair left to be updated
	//   in the trie. We *always create a leaf* in the recursion base case, no matter whether the leaf represents an unallocated or allocated register.
	//   Explicitly representing specific unallocated registers is an interim shortcut until we have specialized (more efficient) non-inclusion proofs
	//   implemented (at the moment, non-inclusion proofs fall back on inclusion proofs of explicitly represented default leaf nodes).
	// - When descending upwards again from the recursion, `NewInterimCompactifiedNode` takes care of the compaction if `prune` is true. When `prune`
	//   is enabled (unchanged for the entire update), it follows by induction that the trie always produces maximally compactified leaves for all
	//   registers written during the update (untouched registers may remain in the state before the update, potentially uncompactified).
	// - Important: we only track the number of *allocated* registers (the change thereof to be precise). Therefore, whenever a new leaf is created,
	//   we need to check if it represents an unallocated register and return the appropriate change of allocated register count.

	// [Recursion Base Case] empty update (len(paths) == 0), i.e. no register to write in this sub-trie.
	if len(paths) == 0 {
		if compactLeaf != nil { // this implies currentNode == nil per Lemma in mtrie/README.md
			// README case 3.a.ii: the sole leaf to create is the compactified leaf carried over from a higher height.
			// We re-level it to the current height by creating a new compact leaf node with the same path and value.
			// The old node isn't modified, as it is still used by the trie before the update. No matter whether
			// `compactLeaf` represents an unallocated or allocated register, the register count remains unchanged.
			n = NewRelevelledLeaf(compactLeaf, nodeHeight)
			return n, 0, nodeHeight
		}
		// README Case 0 (re-use): no path to update and no compact leaf carried over ⇒ no update at all, so we
		// re-use the existing sub-trie (mtrie/README.md § Update: "no update will be done and the original
		// sub-trie can be re-used"). We return `currentNode` regardless of whether it exists.
		return currentNode, 0, nodeHeight
	}

	// [Recursion Base Case] README case 3.a.i: currentNode == nil: A single register is written into a previously empty (e.g. pruned) sub-trie.
	if len(paths) == 1 && currentNode == nil && compactLeaf == nil {
		n = NewLeaf(paths[0], values[0], nodeHeight)
		allocatedRegCountDelta = computeAllocatedRegCountDelta(false, n.IsAllocatedRegisterLeaf())
		return n, allocatedRegCountDelta, nodeHeight
	}

	// Every remaining configuration has len(paths) ≥ 1. The code branches on currentNode next. By the Lemma (mtrie/README.md § Update), the
	// configuration currentNode ≠ nil AND compactLeaf ≠ nil cannot occur. So the configurations that remain, classified by currentNode, are:
	//   • currentNode is a leaf (⟹ compactLeaf == nil, by README's Lemma):                         README Case 2
	//   • currentNode is an interim node (⟹ compactLeaf == nil, by README's Lemma):                README Case 1
	//   • currentNode == nil (⟹ ≥2 leaves to create; as single-leaf case 3.a was covered above):   only README case 3.b remains

	// README Case 2: currentNode is a leaf (⟹ compactLeaf == nil per Lemma in mtrie/README.md).
	if currentNode != nil && currentNode.IsLeaf() {
		currentPath := *currentNode.Path()

		// [Recursion Base Case] README case 2.a.i: the single updated path coincides with `currentNode`'s path,
		// so we overwrite the register represented by the existing leaf in place.
		if len(paths) == 1 && (paths[0] == currentPath) {
			// In most cases, the new register value will be different from the old value, in which case we need to instantiate a new leaf
			// anyway. So we optimistically create the new leaf first. But if a posterior check reveals that the new leaf's hash is identical
			// to the old `currentNode`, we just return `currentNode` to avoid duplication (optimistically created leaf is garbage collected).
			n = NewLeaf(paths[0], values[0], nodeHeight)
			if n.hashValue == currentNode.hashValue {
				return currentNode, 0, nodeHeight
			}
			allocatedRegCountDelta = computeAllocatedRegCountDelta(currentNode.IsAllocatedRegisterLeaf(), n.IsAllocatedRegisterLeaf())
			return n, allocatedRegCountDelta, nodeHeight
		}

		// -- from here on, until the end of the method, we are handling the recursive cases --

		// [Recursive Case] README case 2.a.ii or 2.b, both FALL THROUGH to the SPLIT-AND-RECURSE section below
		if slices.Contains(paths, currentPath) { // `currentNode.path ∈ paths` and `len(paths) > 1`: README case 2.a.ii
			// The register at `currentNode`'s path is among the updated `paths`, so its value will be overwritten. Here we
			// only account for removing `currentNode`'s own contribution to the count; the new value is counted separately,
			// deeper in the recursion, when its leaf is (re)created. Dropping `currentNode` yields -1 if it held an
			// allocated register and 0 if it was a default (unallocated) leaf.
			allocatedRegCountDelta = computeAllocatedRegCountDelta(currentNode.IsAllocatedRegisterLeaf(), false) // drop `currentNode`
		} else { // `currentNode.path ∉ paths`: README case 2.b
			// `currentNode` carries a path that is not among the updated `paths`. Hence it represents a compact leaf
			// that must be carried down the recursion.
			compactLeaf = currentNode
			// INSIGHT [not implemented]: formally `currentNode` could be set to nil here. This proof (presumably relatively short)
			// should be written up in the readme, to keep the complexity out of the code.
		}
	}
	// CAUTION: in the prior code block implementing README case 2.b, we set compactLeaf = currentNode while
	// currentNode ≠ nil, so the Readme's Lemma no longer holds from here on. This is safe because currentNode
	// is a *leaf* in case 2.b, so its LeftChild()/RightChild() are nil (fetched below): the split descends into
	// empty children while compactLeaf carries the preserved register down.

	// Every remaining configuration has len(paths) ≥ 1. The configurations remaining to be handled are:
	//   case 1:  node ≠ nil and currentNode is an interim node, with at least one register to write into its sub-trie (len(paths) ≥ 1).
	//     We descend into currentNode's existing children to apply the update while preserving the rest.
	//   case 2.a.ii: currentNode is a leaf with currentNode.path ∈ paths and len(paths) > 1.
	//     currentNode's own register is among those updated, and at least one *other* register (different path) is
	//     written into the same sub-trie. So currentNode's leaf must be replaced by a sub-trie holding two or more
	//     distinct registers (built by the recursion).
	//   case 2.b: currentNode is a leaf with currentNode.path ∉ paths.
	//     At least one register is written here (len(paths) ≥ 1), but currentNode's own register is not among them
	//     (no updated path equals currentNode.path). So currentNode's leaf must be replaced by a sub-trie holding two
	//     or more distinct registers: currentNode's carried register plus the updated one(s) (built by the recursion).
	//   case 3.b: currentNode == nil with more than one register to create, since either
	//     - len(paths) == 1 and compactLeaf ≠ nil, or
	//     - len(paths) > 1.
	// Common to all these cases: the update continues by recursion. Either currentNode (a leaf or nil) is expanded into a
	// new sub-trie holding two or more distinct registers (cases 2.a.ii, 2.b, 3.b), or currentNode is already an interim
	// node and we recurse into its existing children (case 1).

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
		// runtime optimization: if there are only few updates for either left or right sub-tree, proceed single-threaded
		newLeftChild, lRegCountDelta, lLowestHeightTouched = update(nodeHeight-1, oldLeftChild, lpaths, lvalues, lcompactLeaf, prune)
		newRightChild, rRegCountDelta, rLowestHeightTouched = update(nodeHeight-1, oldRightChild, rpaths, rvalues, rcompactLeaf, prune)
	} else {
		// Runtime optimization: process the left child in a separate thread. This Recursive `update` call returns
		// 3 values, which we have to wait for. Although WaitGroup approach can be faster than channel (esp. with
		// 2+ goroutines), we only use 1 goroutine here and need to communicate results from it. So using
		// channel is faster and uses fewer allocs/op in this case.
		results := make(chan updateResult, 1) // channel capacity 1, so goroutine can push into channel and finish without being blocked
		go func(retChan chan<- updateResult) {
			child, regCountDelta, lowestHeightTouched := update(nodeHeight-1, oldLeftChild, lpaths, lvalues, lcompactLeaf, prune)
			retChan <- updateResult{child, regCountDelta, lowestHeightTouched}
		}(results)

		newRightChild, rRegCountDelta, rLowestHeightTouched = update(nodeHeight-1, oldRightChild, rpaths, rvalues, rcompactLeaf, prune)

		// Wait for results from goroutine processing left child
		ret := <-results
		newLeftChild, lRegCountDelta, lLowestHeightTouched = ret.child, ret.allocatedRegCountDelta, ret.lowestHeightTouched
	}

	allocatedRegCountDelta += lRegCountDelta + rRegCountDelta
	lowestHeightTouched = min(lLowestHeightTouched, rLowestHeightTouched)

	// mitigate storage exhaustion attack: avoids creating a new node when the exact same
	// value is re-written at a register. CAUTION: we only check that the children are
	// unchanged. This is only sufficient for interim nodes (for leaf nodes, the children
	// might be unchanged, i.e. both nil, but the value could have changed).
	// In case the current node was a leaf, we _cannot reuse_ it, because we potentially
	// updated registers in the sub-trie
	if !currentNode.IsLeaf() && newLeftChild == oldLeftChild && newRightChild == oldRightChild {
		return currentNode, 0, lowestHeightTouched
	}

	// -- from here on, until the end of the method, we are constructing the updated trie bottom up (increasing height) from the previously (recursively) updated children --

	// Instantiate the node replacing `currentNode` from the recursively-updated children. Each of newLeftChild / newRightChild is itself an update result:
	// nil, a compact leaf, or an interim node. If prune is on, `NewInterimCompactifiedNode` creates a compact leaf when one child is nil and the other is
	// a leaf node. When starting from a maximally compactified trie, by induction this process yields an updated trie that is also maximally compactified
	// (all leaves compactified).
	// However, if we operate on a sub-trie where some leaves are expanded or unallocated registers are explicitly represented, the resulting updated trie
	// may still contain leaves that could be pruned or compactified even when `prune == true`: a register deeper in an untouched, reused child stays as-is,
	// since `NewInterimCompactifiedNode` inspects only the immediate children (for efficiency) and does not traverse sub-tries unless there are register updates.
	if prune {
		n = NewInterimCompactifiedNode(nodeHeight, newLeftChild, newRightChild)
		return n, allocatedRegCountDelta, lowestHeightTouched
	}

	n = NewInterimNode(nodeHeight, newLeftChild, newRightChild)
	return n, allocatedRegCountDelta, lowestHeightTouched
}

// computeAllocatedRegCountDelta returns *change* in allocated reg count when updating a
// leaf to-and-from allocated register vs default leaf (representing an unallocated register).
func computeAllocatedRegCountDelta(wasAllocatedRegister, updatedToAllocatedRegister bool) (allocatedRegCountDelta int64) {
	if wasAllocatedRegister == updatedToAllocatedRegister {
		return 0
	}
	if updatedToAllocatedRegister {
		return 1 // previously unallocated register will be allocated.
	}
	return -1 // previously allocated register will be unallocated.
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
	// payload to empty. While this will cause the leaf's hash to become the
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
//
// CAUTION: each returned pointer aliases the corresponding trie node's internal leaf hash. Tries are
// immutable and shared copy-on-write across trie versions, so the caller MUST NOT modify any pointee;
// doing so would corrupt that node's cached hash. Callers needing mutable hashes must copy them. Do
// NOT MODIFY the returned hashes!
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
