package trie

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
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
	root     *node.Node
	regCount uint64 // number of registers allocated in the trie
	regSize  uint64 // size of registers allocated in the trie
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
func NewMTrie(root *node.Node, regCount uint64, regSize uint64) (*MTrie, error) {
	if root != nil && root.Height() != ledger.NodeMaxHeight {
		return nil, fmt.Errorf("height of root node must be %d but is %d", ledger.NodeMaxHeight, root.Height())
	}
	return &MTrie{
		root:     root,
		regCount: regCount,
		regSize:  regSize,
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

// AllocatedRegSize returns the size of allocated registers in the trie.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) AllocatedRegSize() uint64 {
	return mt.regSize
}

// RootNode returns the Trie's root Node
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) RootNode() *node.Node {
	return mt.root
}

// String returns the trie's string representation.
// Concurrency safe (as Tries are immutable structures by convention)
func (mt *MTrie) String() string {
	if mt.IsEmpty() {
		return fmt.Sprintf("Empty Trie with default root hash: %x\n", mt.RootHash())
	}
	trieStr := fmt.Sprintf("Trie root hash: %x\n", mt.RootHash())
	return trieStr + mt.root.FmtStr("", "")
}

// UnsafeValueSizes returns payload value sizes for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// CAUTION: while getting payload value sizes, `paths` is permuted IN-PLACE for optimized processing.
// Return:
//  * `sizes` []int
//     For each path, the corresponding payload value size is written into sizes. AFTER
//     the size operation completes, the order of `path` and `sizes` are such that
//     for `path[i]` the corresponding register value size is referenced by `sizes[i]`.
// TODO move consistency checks from Forest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeValueSizes(paths []ledger.Path) []int {
	sizes := make([]int, len(paths)) // pre-allocate slice for the result
	valueSizes(sizes, paths, mt.root)
	return sizes
}

// valueSizes returns value sizes of all the registers in `paths`` in subtree with `head` as root node.
// For each `path[i]`, the corresponding value size is written into `sizes[i]` for the same index `i`.
// CAUTION:
//  * while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
//  * unchecked requirement: all paths must go through the `head` node
func valueSizes(sizes []int, paths []ledger.Path, head *node.Node) {
	// check for empty paths
	if len(paths) == 0 {
		return
	}

	// path not found
	if head == nil {
		return
	}

	// reached a leaf node
	if head.IsLeaf() {
		for i, p := range paths {
			if *head.Path() == p {
				payload := head.Payload()
				if payload != nil {
					sizes[i] = payload.Value.Size()
				}
				// NOTE: break isn't used here because precondition
				// doesn't require paths being deduplicated.
			}
		}
		return
	}

	// reached an interim node with only one path
	if len(paths) == 1 {
		path := paths[0][:]

		// traverse nodes following the path until a leaf node or nil node is reached.
		// "for" loop helps to skip partition and recursive call when there's only one path to follow.
		for {
			depth := ledger.NodeMaxHeight - head.Height() // distance to the tree root
			bit := bitutils.ReadBit(path, depth)
			if bit == 0 {
				head = head.LeftChild()
			} else {
				head = head.RightChild()
			}
			if head.IsLeaf() {
				break
			}
		}

		valueSizes(sizes, paths, head)
		return
	}

	// reached an interim node with more than one paths

	// partition step to quick sort the paths:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := ledger.NodeMaxHeight - head.Height() // distance to the tree root
	partitionIndex := SplitPaths(paths, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lsizes, rsizes := sizes[:partitionIndex], sizes[partitionIndex:]

	// read values from left and right subtrees in parallel
	parallelRecursionThreshold := 32 // threshold to avoid the parallelization going too deep in the recursion
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		valueSizes(lsizes, lpaths, head.LeftChild())
		valueSizes(rsizes, rpaths, head.RightChild())
	} else {
		// concurrent read of left and right subtree
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			valueSizes(lsizes, lpaths, head.LeftChild())
			wg.Done()
		}()
		valueSizes(rsizes, rpaths, head.RightChild())
		wg.Wait() // wait for all threads
	}
}

// UnsafeRead reads payloads for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// CAUTION: while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
// Return:
//  * `payloads` []*ledger.Payload
//     For each path, the corresponding payload is written into payloads. AFTER
//     the read operation completes, the order of `path` and `payloads` are such that
//     for `path[i]` the corresponding register value is referenced by 0`payloads[i]`.
// TODO move consistency checks from Forest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeRead(paths []ledger.Path) []*ledger.Payload {
	payloads := make([]*ledger.Payload, len(paths)) // pre-allocate slice for the result
	read(payloads, paths, mt.root)
	return payloads
}

// read reads all the registers in subtree with `head` as root node. For each
// `path[i]`, the corresponding payload is written into `payloads[i]` for the same index `i`.
// CAUTION:
//  * while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
//  * unchecked requirement: all paths must go through the `head` node
func read(payloads []*ledger.Payload, paths []ledger.Path, head *node.Node) {
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
			if *head.Path() == p {
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
	depth := ledger.NodeMaxHeight - head.Height() // distance to the tree root
	partitionIndex := SplitPaths(paths, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// read values from left and right subtrees in parallel
	parallelRecursionThreshold := 32 // threshold to avoid the parallelization going too deep in the recursion
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		read(lpayloads, lpaths, head.LeftChild())
		read(rpayloads, rpaths, head.RightChild())
	} else {
		// concurrent read of left and right subtree
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			read(lpayloads, lpaths, head.LeftChild())
			wg.Done()
		}()
		read(rpayloads, rpaths, head.RightChild())
		wg.Wait() // wait for all threads
	}
}

// NewTrieWithUpdatedRegisters constructs a new trie containing all registers from the parent trie,
// and returns:
//   * updated trie
//   * max depth touched during update (this isn't affected by prune flag)
//   * error
// The key-value pairs specify the registers whose values are supposed to hold updated values
// compared to the parent trie. Constructing the new trie is done in a COPY-ON-WRITE manner:
//   * The original trie remains unchanged.
//   * subtries that remain unchanged are from the parent trie instead of copied.
// UNSAFE: method requires the following conditions to be satisfied:
//   * keys are NOT duplicated
//   * requires _all_ paths to have a length of mt.Height bits.
// CAUTION: `updatedPaths` and `updatedPayloads` are permuted IN-PLACE for optimized processing.
// TODO: move consistency checks from MForest to here, to make API safe and self-contained
func NewTrieWithUpdatedRegisters(
	parentTrie *MTrie,
	updatedPaths []ledger.Path,
	updatedPayloads []ledger.Payload,
	prune bool,
) (*MTrie, uint16, error) {
	updatedRoot, regCountDelta, regSizeDelta, lowestHeightTouched := update(
		ledger.NodeMaxHeight,
		parentTrie.root,
		updatedPaths,
		updatedPayloads,
		nil,
		prune,
	)

	updatedTrieRegCount := int64(parentTrie.AllocatedRegCount()) + regCountDelta
	updatedTrieRegSize := int64(parentTrie.AllocatedRegSize()) + regSizeDelta
	maxDepthTouched := uint16(ledger.NodeMaxHeight - lowestHeightTouched)

	updatedTrie, err := NewMTrie(updatedRoot, uint64(updatedTrieRegCount), uint64(updatedTrieRegSize))
	if err != nil {
		return nil, 0, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	return updatedTrie, maxDepthTouched, nil
}

// updateResult is a wrapper of return values from update().
// It's used to communicate values from goroutine.
type updateResult struct {
	child                  *node.Node
	allocatedRegCountDelta int64
	allocatedRegSizeDelta  int64
	lowestHeightTouched    int
}

// update traverses the subtree, updates the stored registers, and returns:
//   * new or original node (n)
//   * allocated register count delta in subtrie (allocatedRegCountDelta)
//   * allocated register size delta in subtrie (allocatedRegSizeDelta)
//   * lowest height reached during recursive update in subtrie (lowestHeightTouched)
// allocatedRegCountDelta and allocatedRegSizeDelta are used to compute updated
// trie's allocated register count and size.  lowestHeightTouched is used to
// compute max depth touched during update.
// CAUTION: while updating, `paths` and `payloads` are permuted IN-PLACE for optimized processing.
// UNSAFE: method requires the following conditions to be satisfied:
//   * paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
//   * paths are NOT duplicated
func update(
	nodeHeight int, parentNode *node.Node,
	paths []ledger.Path, payloads []ledger.Payload, compactLeaf *node.Node,
	prune bool,
) (n *node.Node, allocatedRegCountDelta int64, allocatedRegSizeDelta int64, lowestHeightTouched int) {
	// No new paths to write
	if len(paths) == 0 {
		// check is a compactLeaf from a higher height is still left.
		if compactLeaf != nil {
			// create a new node for the compact leaf path and payload. The old node shouldn't
			// be recycled as it is still used by the tree copy before the update.
			n = node.NewLeaf(*compactLeaf.Path(), compactLeaf.Payload(), nodeHeight)
			return n, 0, 0, nodeHeight
		}
		return parentNode, 0, 0, nodeHeight
	}

	if len(paths) == 1 && parentNode == nil && compactLeaf == nil {
		n = node.NewLeaf(paths[0], payloads[0].DeepCopy(), nodeHeight)
		if payloads[0].IsEmpty() {
			// Unallocated register doesn't affect allocatedRegCountDelta and allocatedRegSizeDelta.
			return n, 0, 0, nodeHeight
		}
		return n, 1, int64(payloads[0].Size()), nodeHeight
	}

	if parentNode != nil && parentNode.IsLeaf() { // if we're here then compactLeaf == nil
		// check if the parent node path is among the updated paths
		found := false
		parentPath := *parentNode.Path()
		for i, p := range paths {
			if p == parentPath {
				// the case where the recursion stops: only one path to update
				if len(paths) == 1 {
					if !parentNode.Payload().Equals(&payloads[i]) {
						n = node.NewLeaf(paths[i], payloads[i].DeepCopy(), nodeHeight)

						allocatedRegCountDelta, allocatedRegSizeDelta =
							allocatedRegDeltas(parentNode.Payload(), &payloads[i])

						return n, allocatedRegCountDelta, allocatedRegSizeDelta, nodeHeight
					}
					// avoid creating a new node when the same payload is written
					return parentNode, 0, 0, nodeHeight
				}
				// the case where the recursion carries on: len(paths)>1
				found = true

				allocatedRegCountDelta, allocatedRegSizeDelta =
					allocatedRegDeltasFromHigherHeight(parentNode.Payload())

				break
			}
		}
		if !found {
			// if the parent node carries a path not included in the input path, then the parent node
			// represents a compact leaf that needs to be carried down the recursion.
			compactLeaf = parentNode
		}
	}

	// in the remaining code: the registers to update are strictly larger than 1:
	//   - either len(paths)>1
	//   - or len(paths) == 1 and compactLeaf!= nil
	//   - or len(paths) == 1 and parentNode != nil && !parentNode.IsLeaf()

	// Split paths and payloads to recurse:
	// lpaths contains all paths that have `0` at the partitionIndex
	// rpaths contains all paths that have `1` at the partitionIndex
	depth := ledger.NodeMaxHeight - nodeHeight // distance to the tree root
	partitionIndex := splitByPath(paths, payloads, depth)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// check if there is a compact leaf that needs to get deep to height 0
	var lcompactLeaf, rcompactLeaf *node.Node
	if compactLeaf != nil {
		// if yes, check which branch it will go to.
		path := *compactLeaf.Path()
		if bitutils.ReadBit(path[:], depth) == 0 {
			lcompactLeaf = compactLeaf
		} else {
			rcompactLeaf = compactLeaf
		}
	}

	// set the parent node children
	var lchildParent, rchildParent *node.Node
	if parentNode != nil {
		lchildParent = parentNode.LeftChild()
		rchildParent = parentNode.RightChild()
	}

	// recurse over each branch
	var lChild, rChild *node.Node
	var lRegCountDelta, rRegCountDelta int64
	var lRegSizeDelta, rRegSizeDelta int64
	var lLowestHeightTouched, rLowestHeightTouched int
	parallelRecursionThreshold := 16
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		// runtime optimization: if there are _no_ updates for either left or right sub-tree, proceed single-threaded
		lChild, lRegCountDelta, lRegSizeDelta, lLowestHeightTouched = update(nodeHeight-1, lchildParent, lpaths, lpayloads, lcompactLeaf, prune)
		rChild, rRegCountDelta, rRegSizeDelta, rLowestHeightTouched = update(nodeHeight-1, rchildParent, rpaths, rpayloads, rcompactLeaf, prune)
	} else {
		// runtime optimization: process the left child is a separate thread

		// Since we're receiving 4 values from goroutine, use a
		// struct and channel to reduce allocs/op.
		// Although WaitGroup approach can be faster than channel (esp. with 2+ goroutines),
		// we only use 1 goroutine here and need to communicate results from it. So using
		// channel is faster and uses fewer allocs/op in this case.
		results := make(chan updateResult, 1)
		go func(retChan chan<- updateResult) {
			child, regCountDelta, regSizeDelta, lowestHeightTouched := update(nodeHeight-1, lchildParent, lpaths, lpayloads, lcompactLeaf, prune)
			retChan <- updateResult{child, regCountDelta, regSizeDelta, lowestHeightTouched}
		}(results)

		rChild, rRegCountDelta, rRegSizeDelta, rLowestHeightTouched = update(nodeHeight-1, rchildParent, rpaths, rpayloads, rcompactLeaf, prune)

		// Wait for results from goroutine.
		ret := <-results
		lChild, lRegCountDelta, lRegSizeDelta, lLowestHeightTouched = ret.child, ret.allocatedRegCountDelta, ret.allocatedRegSizeDelta, ret.lowestHeightTouched
	}

	allocatedRegCountDelta += lRegCountDelta + rRegCountDelta
	allocatedRegSizeDelta += lRegSizeDelta + rRegSizeDelta
	lowestHeightTouched = minInt(lLowestHeightTouched, rLowestHeightTouched)

	// mitigate storage exhaustion attack: avoids creating a new node when the exact same
	// payload is re-written at a register. CAUTION: we only check that the children are
	// unchanged. This is only sufficient for interim nodes (for leaf nodes, the children
	// might be unchanged, i.e. both nil, but the payload could have changed).
	if !parentNode.IsLeaf() && lChild == lchildParent && rChild == rchildParent {
		return parentNode, 0, 0, lowestHeightTouched
	}

	// In case the parent node was a leaf, we _cannot reuse_ it, because we potentially
	// updated registers in the sub-trie
	if prune {
		n = node.NewInterimCompactifiedNode(nodeHeight, lChild, rChild)
		return n, allocatedRegCountDelta, allocatedRegSizeDelta, lowestHeightTouched
	}

	n = node.NewInterimNode(nodeHeight, lChild, rChild)
	return n, allocatedRegCountDelta, allocatedRegSizeDelta, lowestHeightTouched
}

// allocatedRegDeltasFromHigherHeight returns the deltas needed
// to compute the allocated reg count and reg size when a payload
// needs to be updated or unallocated at a lower height.
func allocatedRegDeltasFromHigherHeight(oldPayload *ledger.Payload) (allocatedRegCountDelta, allocatedRegSizeDelta int64) {
	if !oldPayload.IsEmpty() {
		// Allocated register will be updated or unallocated at lower height.
		allocatedRegCountDelta--
	}
	oldPayloadSize := oldPayload.Size()
	allocatedRegSizeDelta -= int64(oldPayloadSize)
	return
}

// allocatedRegDeltas returns the allocated reg count and reg size deltas
// computed from old payload and new payload.
// CAUTION: !oldPayload.Equals(newPayload)
func allocatedRegDeltas(oldPayload, newPayload *ledger.Payload) (allocatedRegCountDelta, allocatedRegSizeDelta int64) {
	allocatedRegCountDelta = 0
	if newPayload.IsEmpty() {
		// Old payload is not empty while new payload is empty.
		// Allocated register will be unallocated.
		allocatedRegCountDelta = -1
	} else if oldPayload.IsEmpty() {
		// Old payload is empty while new payload is not empty.
		// Unallocated register will be allocated.
		allocatedRegCountDelta = 1
	}

	oldPayloadSize := oldPayload.Size()
	newPayloadSize := newPayload.Size()
	allocatedRegSizeDelta = int64(newPayloadSize - oldPayloadSize)
	return
}

// UnsafeProofs provides proofs for the given paths.
//
// CAUTION: while updating, `paths` and `proofs` are permuted IN-PLACE for optimized processing.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// Paths in the input query don't have to be deduplicated, though deduplication would
// result in allocating less dynamic memory to store the proofs.
func (mt *MTrie) UnsafeProofs(paths []ledger.Path) *ledger.TrieBatchProof {
	batchProofs := ledger.NewTrieBatchProofWithEmptyProofs(len(paths))
	prove(mt.root, paths, batchProofs.Proofs)
	return batchProofs
}

// prove traverses the subtree and stores proofs for the given register paths in
// the provided `proofs` slice
// CAUTION: while updating, `paths` and `proofs` are permuted IN-PLACE for optimized processing.
// UNSAFE: method requires the following conditions to be satisfied:
//   * paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
func prove(head *node.Node, paths []ledger.Path, proofs []*ledger.TrieProof) {
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
				proofs[i].Payload = head.Payload()
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
		wg.Add(1)
		go func() {
			addSiblingTrieHashToProofs(head.RightChild(), depth, lproofs)
			prove(head.LeftChild(), lpaths, lproofs)
			wg.Done()
		}()

		addSiblingTrieHashToProofs(head.LeftChild(), depth, rproofs)
		prove(head.RightChild(), rpaths, rproofs)
		wg.Wait()
	}
}

// addSiblingTrieHashToProofs inspects the sibling Trie and adds its root hash
// to the proofs, if the trie contains non-empty registers (i.e. the
// siblingTrie has a non-default hash).
func addSiblingTrieHashToProofs(siblingTrie *node.Node, depth int, proofs []*ledger.TrieProof) {
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

// DumpAsJSON dumps the trie key value pairs to a file having each key value pair as a json row
func (mt *MTrie) DumpAsJSON(w io.Writer) error {

	// Use encoder to prevent building entire trie in memory
	enc := json.NewEncoder(w)

	err := dumpAsJSON(mt.root, enc)
	if err != nil {
		return err
	}

	return nil
}

// dumpAsJSON serializes the sub-trie with root n to json and feeds it into encoder
func dumpAsJSON(n *node.Node, encoder *json.Encoder) error {
	if n.IsLeaf() {
		if n != nil {
			err := encoder.Encode(n.Payload())
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

// AllPayloads returns all payloads
func (mt *MTrie) AllPayloads() []ledger.Payload {
	return mt.root.AllPayloads()
}

// IsAValidTrie verifies the content of the trie for potential issues
func (mt *MTrie) IsAValidTrie() bool {
	// TODO add checks on the health of node max height ...
	return mt.root.VerifyCachedHash()
}

// splitByPath permutes the input paths to be partitioned into 2 parts. The first part contains paths with a zero bit
// at the input bitIndex, the second part contains paths with a one at the bitIndex. The index of partition
// is returned. The same permutation is applied to the payloads slice.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
//
//  For instance, if `paths` contains the following 3 paths, and bitIndex is `1`:
//  [[0,0,1,1], [0,1,0,1], [0,0,0,1]]
//  then `splitByPath` returns 2 and updates `paths` into:
//  [[0,0,1,1], [0,0,0,1], [0,1,0,1]]
func splitByPath(paths []ledger.Path, payloads []ledger.Payload, bitIndex int) int {
	i := 0
	for j, path := range paths {
		bit := bitutils.ReadBit(path[:], bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			payloads[i], payloads[j] = payloads[j], payloads[i]
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
func splitTrieProofsByPath(paths []ledger.Path, proofs []*ledger.TrieProof, bitIndex int) int {
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
