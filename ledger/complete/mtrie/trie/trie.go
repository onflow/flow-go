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
		return nil, fmt.Errorf("height of root node must be %d but is %d, hash: %s", ledger.NodeMaxHeight, root.Height(), root.Hash().String())
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
		return fmt.Sprintf("Empty Trie with default root hash: %v\n", mt.RootHash())
	}
	trieStr := fmt.Sprintf("Trie root hash: %v\n", mt.RootHash())
	return trieStr + mt.root.FmtStr("", "")
}

// UnsafeValueSizes returns payload value sizes for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// CAUTION: while getting payload value sizes, `paths` is permuted IN-PLACE for optimized processing.
// Return:
//   - `sizes` []int
//     For each path, the corresponding payload value size is written into sizes. AFTER
//     the size operation completes, the order of `path` and `sizes` are such that
//     for `path[i]` the corresponding register value size is referenced by `sizes[i]`.
//
// TODO move consistency checks from Forest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeValueSizes(paths []ledger.Path, payloadStorage ledger.PayloadStorage) ([]int, error) {
	sizes := make([]int, len(paths)) // pre-allocate slice for the result
	err := valueSizes(sizes, paths, mt.root, payloadStorage)
	if err != nil {
		return nil, err
	}
	return sizes, nil
}

// valueSizes returns value sizes of all the registers in `paths“ in subtree with `head` as root node.
// For each `path[i]`, the corresponding value size is written into `sizes[i]` for the same index `i`.
// CAUTION:
//   - while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
//   - unchecked requirement: all paths must go through the `head` node
func valueSizes(sizes []int, paths []ledger.Path, head *node.Node, payloadStorage ledger.PayloadStorage) error {
	// check for empty paths
	if len(paths) == 0 {
		return nil
	}

	// path not found
	if head == nil {
		return nil
	}

	// reached a leaf node
	if head.IsLeaf() {
		leafPath, leafPayload, err := payloadStorage.Get(head.ExpandedLeafHash())
		if err != nil {
			return fmt.Errorf("could not get payload from storage: %w", err)
		}
		for i, p := range paths {
			if leafPath == p {
				if leafPayload != nil {
					sizes[i] = leafPayload.Value().Size()
				}
				// NOTE: break isn't used here because precondition
				// doesn't require paths being deduplicated.
			}
		}
		return nil
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

		return valueSizes(sizes, paths, head, payloadStorage)
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
		err := valueSizes(lsizes, lpaths, head.LeftChild(), payloadStorage)
		if err != nil {
			return err
		}
		err = valueSizes(rsizes, rpaths, head.RightChild(), payloadStorage)
		if err != nil {
			return err
		}
	} else {
		// concurrent read of left and right subtree
		wg := sync.WaitGroup{}
		wg.Add(1)
		var goerr error
		go func() {
			goerr = valueSizes(lsizes, lpaths, head.LeftChild(), payloadStorage)
			wg.Done()
		}()
		err := valueSizes(rsizes, rpaths, head.RightChild(), payloadStorage)
		wg.Wait() // wait for all threads
		if err != nil {
			return err
		}
		if goerr != nil {
			return goerr
		}
	}
	return nil
}

// ReadSinglePayload reads and returns a payload for a single path from root
// it returns (ledger.EmptyPayload(), nil) if no payload was found for the given path
// it returns (payload, nil) if payload was found for the given path
// it returns (nil, err) for any exception
func (mt *MTrie) ReadSinglePayload(path ledger.Path, payloads ledger.PayloadStorage) (*ledger.Payload, error) {
	return readSinglePayload(path, mt.root, payloads)
}

func readSinglePayload(path ledger.Path, head *node.Node, payloadStorage ledger.PayloadStorage) (*ledger.Payload, error) {
	pathBytes := path[:]

	if head == nil {
		return ledger.EmptyPayload(), nil
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

	if head.IsDefaultNode() {
		return ledger.EmptyPayload(), nil
	}

	leafHash := head.ExpandedLeafHash()
	leafPath, payload, err := payloadStorage.Get(leafHash)
	if err != nil {
		return nil, fmt.Errorf("could not get payloads with hash %v from storage: %w", leafHash, err)
	}

	if leafPath != path {
		return ledger.EmptyPayload(), nil
	}

	return payload, nil
}

// UnsafeRead reads payloads for the given paths.
// UNSAFE: requires _all_ paths to have a length of mt.Height bits.
// CAUTION: while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
// Return:
//   - `payloads` []*ledger.Payload
//     For each path, the corresponding payload is written into payloads. AFTER
//     the read operation completes, the order of `path` and `payloads` are such that
//     for `path[i]` the corresponding register value is referenced by 0`payloads[i]`.
//
// TODO move consistency checks from Forest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeRead(paths []ledger.Path, payloadStorage ledger.PayloadStorage) ([]*ledger.Payload, error) {
	payloads := make([]*ledger.Payload, len(paths)) // pre-allocate slice for the result
	err := read(payloads, paths, mt.root, payloadStorage)
	return payloads, err
}

// read reads all the registers in subtree with `head` as root node. For each
// `path[i]`, the corresponding payload is written into `payloads[i]` for the same index `i`.
// CAUTION:
//   - while reading the payloads, `paths` is permuted IN-PLACE for optimized processing.
//   - unchecked requirement: all paths must go through the `head` node
func read(
	payloads []*ledger.Payload,
	paths []ledger.Path,
	head *node.Node,
	payloadStorage ledger.PayloadStorage,
) error {
	// check for empty paths
	if len(paths) == 0 {
		return nil
	}

	// path not found
	if head == nil {
		for i := range paths {
			payloads[i] = ledger.EmptyPayload()
		}
		return nil
	}

	// reached a leaf node
	if head.IsLeaf() {
		leafHash := head.ExpandedLeafHash()
		leafPath, leafPayload, err := payloadStorage.Get(leafHash)
		if err != nil {
			return fmt.Errorf("could not get payload with hash %v: %w", leafHash, err)
		}
		for i, p := range paths {
			if leafPath == p {
				payloads[i] = leafPayload
			} else {
				payloads[i] = ledger.EmptyPayload()
			}
		}
		return nil
	}

	// reached an interim node
	if len(paths) == 1 {
		// call findLeafNode to skip partition and recursive calls when there is only one path
		payload, err := readSinglePayload(paths[0], head, payloadStorage)
		if err != nil {
			return fmt.Errorf("could not read single payload at path %v: %w", paths[0], err)
		}
		payloads[0] = payload
		return nil
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
		err := read(lpayloads, lpaths, head.LeftChild(), payloadStorage)
		if err != nil {
			return err
		}
		err = read(rpayloads, rpaths, head.RightChild(), payloadStorage)
		if err != nil {
			return err
		}
	} else {
		var lerr error
		// concurrent read of left and right subtree
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			lerr = read(lpayloads, lpaths, head.LeftChild(), payloadStorage)
			wg.Done()
		}()
		err := read(rpayloads, rpaths, head.RightChild(), payloadStorage)
		if err != nil {
			return err
		}
		wg.Wait() // wait for all threads
		if lerr != nil {
			return lerr
		}
	}

	return nil
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
// CAUTION: `updatedPaths` and `updatedPayloads` are permuted IN-PLACE for optimized processing.
// CAUTION: MTrie expects that for a specific path, the payload's key never changes.
// TODO: move consistency checks from MForest to here, to make API safe and self-contained
func NewTrieWithUpdatedRegisters(
	parentTrie *MTrie,
	updatedPaths []ledger.Path,
	updatedPayloads []ledger.Payload,
	prune bool,
	payloadStorage ledger.PayloadStorage,
) (*MTrie, uint16, error) {
	updatedRoot, regCountDelta, regSizeDelta, lowestHeightTouched, err := update(
		ledger.NodeMaxHeight,
		parentTrie.root,
		updatedPaths,
		updatedPayloads,
		nil,
		prune,
		payloadStorage,
	)

	if err != nil {
		return nil, 0, fmt.Errorf("could not update trie with payloads: %w", err)
	}

	updatedTrieRegCount := int64(parentTrie.AllocatedRegCount()) + regCountDelta
	updatedTrieRegSize := int64(parentTrie.AllocatedRegSize()) + regSizeDelta
	maxDepthTouched := uint16(ledger.NodeMaxHeight - lowestHeightTouched)

	updatedTrie, err := NewMTrie(updatedRoot, uint64(updatedTrieRegCount), uint64(updatedTrieRegSize))
	if err != nil {
		return nil, 0, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	// before returning the trie, add all payloads to storage
	// TODO: avoid re-computing hash
	// integrate into "update"
	leafNodeUpdates := toLeafNodeUpdates(updatedPaths, updatedPayloads)

	err = payloadStorage.Add(leafNodeUpdates)
	if err != nil {
		return nil, 0, fmt.Errorf("could not update storage with leaf node updates: %w", err)
	}

	return updatedTrie, maxDepthTouched, nil
}

// toLeafNodeUpdates converts the trie updates into the leaf nodes updates to be stored in storage
func toLeafNodeUpdates(paths []ledger.Path, payloads []ledger.Payload) []ledger.LeafNode {

	deduplicatedPaths := make([]ledger.Path, 0, len(paths))
	deduplicatedPayloads := make([]ledger.Payload, 0, len(paths))
	payloadMap := make(map[ledger.Path]int) // index into deduplicatedPaths, deduplicatedPayloads with register update
	for i, path := range paths {
		payload := payloads[i]
		// check if we already have encountered an update for the respective register
		if idx, ok := payloadMap[path]; ok {
			deduplicatedPayloads[idx] = payload
		} else {
			payloadMap[path] = len(deduplicatedPaths)
			deduplicatedPaths = append(deduplicatedPaths, path)
			deduplicatedPayloads = append(deduplicatedPayloads, payload)
		}
	}

	leafs := make([]ledger.LeafNode, 0, len(deduplicatedPayloads))

	for i, path := range deduplicatedPaths {
		payload := deduplicatedPayloads[i]
		// trie doesn't store nodes with empty payload, so we skip them, and never
		// add them to storage
		if payload.IsEmpty() {
			continue
		}
		hash := ledger.ComputeFullyExpandedLeafValue(path, &payload)
		leafs = append(leafs, ledger.LeafNode{
			Hash:    hash,
			Path:    path,
			Payload: payload,
		})
	}

	return leafs
}

// updateResult is a wrapper of return values from update().
// It's used to communicate values from goroutine.
type updateResult struct {
	child                  *node.Node
	allocatedRegCountDelta int64
	allocatedRegSizeDelta  int64
	lowestHeightTouched    int
	err                    error
}

// update traverses the subtree recursively and create new nodes with
// the updated payloads on the given paths
//
// it returns:
//   - new updated node or original node if nothing was updated
//   - allocated register count delta in subtrie (allocatedRegCountDelta)
//   - allocated register size delta in subtrie (allocatedRegSizeDelta)
//   - lowest height reached during recursive update in subtrie (lowestHeightTouched)
//
// update also compact a subtree into a single compact leaf node in the case where
// there is only 1 payload stored in the subtree.
//
// allocatedRegCountDelta and allocatedRegSizeDelta are used to compute updated
// trie's allocated register count and size.  lowestHeightTouched is used to
// compute max depth touched during update.
// CAUTION: while updating, `paths` and `payloads` are permuted IN-PLACE for optimized processing.
// UNSAFE: method requires the following conditions to be satisfied:
//   - paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
//   - paths are NOT duplicated
func update(
	nodeHeight int, // the height of the node during traversing the subtree
	currentNode *node.Node, // the current node on the travesing path, if it's nil it means the trie has no node on this path
	paths []ledger.Path, // the paths to update the payloads
	payloads []ledger.Payload, // the payloads to be updated at the given paths
	compactLeaf *node.Node, // a compact leaf node from its ancester, it could be nil
	prune bool, // prune is a flag for whether pruning nodes with empty payload. not pruning is useful for generating proof, expecially non-inclusion proof
	payloadStorage ledger.PayloadStorage, // for getting and setting the payloads stored on leaf nodes
) (n *node.Node, allocatedRegCountDelta int64, allocatedRegSizeDelta int64, lowestHeightTouched int, err error) {
	// No new path to update
	if len(paths) == 0 {
		if compactLeaf != nil {
			leafHash := compactLeaf.ExpandedLeafHash()
			leafPath, leafPayload, err := payloadStorage.Get(leafHash)
			if err != nil {
				return nil, 0, 0, 0, fmt.Errorf("could not get payload by leaf hash %v: %w", leafHash, err)
			}

			// if a compactLeaf from a higher height is still left,
			// then expand the compact leaf node to the current height by creating a new compact leaf
			// node with the same path and payload.
			// The old node shouldn't be recycled as it is still used by the tree copy before the update.
			n = node.NewLeaf(leafPath, leafPayload, nodeHeight)
			return n, 0, 0, nodeHeight, nil
		}
		// if no path to update and there is no compact leaf node on this path, we return
		// the current node regardless it exists or not.
		return currentNode, 0, 0, nodeHeight, nil
	}

	if len(paths) == 1 && currentNode.IsDefaultNode() && compactLeaf == nil {
		// if there is only 1 path to update, and the existing tree has no node on this path, also
		// no compact leaf node from its ancester, it means we are storing a payload on a new path,
		n = node.NewLeaf(paths[0], &payloads[0], nodeHeight)
		if payloads[0].IsEmpty() {
			// if we are storing an empty node, then no register is allocated
			// allocatedRegCountDelta and allocatedRegSizeDelta should both be 0
			return n, 0, 0, nodeHeight, nil
		}
		// if we are storing a non-empty node, we are allocating a new register
		return n, 1, int64(payloads[0].Size()), nodeHeight, nil
	}

	if currentNode != nil && currentNode.IsLeaf() { // if we're here then compactLeaf == nil
		// check if the current node path is among the updated paths
		if currentNode.IsDefaultNode() {
			compactLeaf = currentNode
		} else {

			// check if the current node path is among the updated paths
			found := false
			leafHash := currentNode.ExpandedLeafHash()

			currentPath, currentPayload, err := payloadStorage.Get(leafHash)
			if err != nil {
				return nil, 0, 0, 0, fmt.Errorf("could not get payload for current leaf hash %v: %w", leafHash, err)
			}

			for i, p := range paths {
				if p == currentPath {
					// the case where the recursion stops: only one path to update
					if len(paths) == 1 {
						if !currentPayload.ValueEquals(&payloads[i]) {
							// check if the only path to update has the same payload.
							// if payload is the same, we could skip the update to avoid creating duplicated node
							n = node.NewLeaf(paths[i], payloads[i].DeepCopy(), nodeHeight)

							allocatedRegCountDelta, allocatedRegSizeDelta =
								computeAllocatedRegDeltas(currentPayload, &payloads[i])

							return n, allocatedRegCountDelta, allocatedRegSizeDelta, nodeHeight, nil
						}
						// avoid creating a new node when the same payload is written
						return currentNode, 0, 0, nodeHeight, nil
					}
					// the case where the recursion carries on: len(paths)>1
					found = true

					allocatedRegCountDelta, allocatedRegSizeDelta =
						computeAllocatedRegDeltasFromHigherHeight(currentPayload)

					break
				}
			}
			if !found {
				// if the current node carries a path not included in the input path, then the current node
				// represents a compact leaf that needs to be carried down the recursion.
				compactLeaf = currentNode
			}
		}
	}

	// in the remaining code:
	//   - either len(paths) > 1
	//   - or len(paths) == 1 and compactLeaf!= nil
	//   - or len(paths) == 1 and currentNode != nil && !currentNode.IsLeaf()

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
		leafHash := compactLeaf.ExpandedLeafHash()
		leafPath, _, err := payloadStorage.Get(leafHash)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("could not get payload from storage by hash %v: %w", leafHash, err)
		}

		if bitutils.ReadBit(leafPath[:], depth) == 0 {
			lcompactLeaf = compactLeaf
		} else {
			rcompactLeaf = compactLeaf
		}
	}

	// set the node children
	var oldLeftChild, oldRightChild *node.Node
	if currentNode != nil {
		oldLeftChild = currentNode.LeftChild()
		oldRightChild = currentNode.RightChild()
	}

	// recurse over each branch
	var newLeftChild, newRightChild *node.Node
	var lRegCountDelta, rRegCountDelta int64
	var lRegSizeDelta, rRegSizeDelta int64
	var lLowestHeightTouched, rLowestHeightTouched int
	parallelRecursionThreshold := 16
	if len(lpaths) < parallelRecursionThreshold || len(rpaths) < parallelRecursionThreshold {
		// runtime optimization: if there are _no_ updates for either left or right sub-tree, proceed single-threaded
		newLeftChild, lRegCountDelta, lRegSizeDelta, lLowestHeightTouched, err = update(nodeHeight-1, oldLeftChild, lpaths, lpayloads, lcompactLeaf, prune, payloadStorage)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("could not update: %w", err)
		}
		newRightChild, rRegCountDelta, rRegSizeDelta, rLowestHeightTouched, err = update(nodeHeight-1, oldRightChild, rpaths, rpayloads, rcompactLeaf, prune, payloadStorage)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("could not update: %w", err)
		}
	} else {
		// runtime optimization: process the left child is a separate thread

		// Since we're receiving 4 values from goroutine, use a
		// struct and channel to reduce allocs/op.
		// Although WaitGroup approach can be faster than channel (esp. with 2+ goroutines),
		// we only use 1 goroutine here and need to communicate results from it. So using
		// channel is faster and uses fewer allocs/op in this case.
		results := make(chan updateResult, 1)
		go func(retChan chan<- updateResult) {
			child, regCountDelta, regSizeDelta, lowestHeightTouched, err := update(nodeHeight-1, oldLeftChild, lpaths, lpayloads, lcompactLeaf, prune, payloadStorage)
			retChan <- updateResult{child, regCountDelta, regSizeDelta, lowestHeightTouched, err}
		}(results)

		newRightChild, rRegCountDelta, rRegSizeDelta, rLowestHeightTouched, err = update(nodeHeight-1, oldRightChild, rpaths, rpayloads, rcompactLeaf, prune, payloadStorage)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("could not update: %w", err)
		}

		// Wait for results from goroutine.
		ret := <-results
		if ret.err != nil {
			return nil, 0, 0, 0, fmt.Errorf("could not update: %w", err)
		}
		newLeftChild, lRegCountDelta, lRegSizeDelta, lLowestHeightTouched = ret.child, ret.allocatedRegCountDelta, ret.allocatedRegSizeDelta, ret.lowestHeightTouched
	}

	allocatedRegCountDelta += lRegCountDelta + rRegCountDelta
	allocatedRegSizeDelta += lRegSizeDelta + rRegSizeDelta
	lowestHeightTouched = minInt(lLowestHeightTouched, rLowestHeightTouched)

	// mitigate storage exhaustion attack: avoids creating a new node when the exact same
	// payload is re-written at a register. CAUTION: we only check that the children are
	// unchanged. This is only sufficient for interim nodes (for leaf nodes, the children
	// might be unchanged, i.e. both nil, but the payload could have changed).
	// In case the current node was a leaf, we _cannot reuse_ it, because we potentially
	// updated registers in the sub-trie
	if !currentNode.IsLeaf() && newLeftChild == oldLeftChild && newRightChild == oldRightChild {
		return currentNode, 0, 0, lowestHeightTouched, nil
	}

	// if prune is on, then will check and create a compact leaf node if one child is nil, and the
	// other child is a leaf node
	if prune {
		n = node.NewInterimCompactifiedNode(nodeHeight, newLeftChild, newRightChild)
		return n, allocatedRegCountDelta, allocatedRegSizeDelta, lowestHeightTouched, nil
	}

	n = node.NewInterimNode(nodeHeight, newLeftChild, newRightChild)
	return n, allocatedRegCountDelta, allocatedRegSizeDelta, lowestHeightTouched, nil
}

// computeAllocatedRegDeltasFromHigherHeight returns the deltas
// needed to compute the allocated reg count and reg size when
// a payload is updated or unallocated at a lower height.
func computeAllocatedRegDeltasFromHigherHeight(oldPayload *ledger.Payload) (allocatedRegCountDelta, allocatedRegSizeDelta int64) {
	if !oldPayload.IsEmpty() {
		// Allocated register will be updated or unallocated at lower height.
		allocatedRegCountDelta--
	}
	oldPayloadSize := oldPayload.Size()
	allocatedRegSizeDelta -= int64(oldPayloadSize)
	return
}

// computeAllocatedRegDeltas returns the allocated reg count
// and reg size deltas computed from old payload and new payload.
// PRECONDITION: !oldPayload.Equals(newPayload)
func computeAllocatedRegDeltas(oldPayload, newPayload *ledger.Payload) (allocatedRegCountDelta, allocatedRegSizeDelta int64) {
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
func (mt *MTrie) UnsafeProofs(paths []ledger.Path, payloadStorage ledger.PayloadStorage) (*ledger.TrieBatchProof, error) {
	// create non-inclusion proof by default, so that
	// if we can prove its inclusive then it's noninclusive
	batchProofs := ledger.NewTrieBatchProofWithEmptyProofs(paths)
	err := prove(mt.root, paths, batchProofs.Proofs, payloadStorage)
	if err != nil {
		return nil, fmt.Errorf("could not get proof: %w", err)
	}
	return batchProofs, nil
}

// prove traverses the subtree and stores proofs for the given register paths in
// the provided `proofs` slice
// CAUTION: while updating, `paths` and `proofs` are permuted IN-PLACE for optimized processing.
// UNSAFE: method requires the following conditions to be satisfied:
//   - paths all share the same common prefix [0 : mt.maxHeight-1 - nodeHeight)
//     (excluding the bit at index headHeight)
func prove(head *node.Node, paths []ledger.Path, proofs []*ledger.TrieProof, payloadStorage ledger.PayloadStorage) error {
	// check for empty paths
	if len(paths) == 0 {
		return nil
	}

	// we've reached the end of a trie
	// and path is not found (noninclusion proof)
	if head == nil {
		// by default, proofs are non-inclusion proofs
		return nil
	}

	// we've reached a leaf
	if head.IsLeaf() {
		if head.IsDefaultNode() {
			// if a leaf is a default node, means nothing is stored on this path,
			// we return proof for non-existing path
			for i, path := range paths {
				proofs[i].Path = path
				proofs[i].Payload = ledger.EmptyPayload()
				proofs[i].Inclusion = true
			}

			return nil
		}

		leafHash := head.ExpandedLeafHash()
		leafPath, leafPayload, err := payloadStorage.Get(leafHash)
		if err != nil {
			return fmt.Errorf("could not get payload by hash %v: %w", head.ExpandedLeafHash(), err)
		}
		for i, path := range paths {
			// value matches (inclusion proof)
			if leafPath == path {
				proofs[i].Path = leafPath
				proofs[i].Payload = leafPayload
				proofs[i].Inclusion = true
			}
		}
		// by default, proofs are non-inclusion proofs
		return nil
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
		err := prove(head.LeftChild(), lpaths, lproofs, payloadStorage)
		if err != nil {
			return err
		}

		addSiblingTrieHashToProofs(head.LeftChild(), depth, rproofs)
		err = prove(head.RightChild(), rpaths, rproofs, payloadStorage)
		if err != nil {
			return err
		}
	} else {
		wg := sync.WaitGroup{}
		wg.Add(1)
		var goerr error
		go func() {
			addSiblingTrieHashToProofs(head.RightChild(), depth, lproofs)
			goerr = prove(head.LeftChild(), lpaths, lproofs, payloadStorage)
			wg.Done()
		}()

		addSiblingTrieHashToProofs(head.LeftChild(), depth, rproofs)
		err := prove(head.RightChild(), rpaths, rproofs, payloadStorage)
		wg.Wait()
		if err != nil {
			return err
		}

		if goerr != nil {
			return err
		}
	}

	return nil
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
			err := encoder.Encode(n.ExpandedLeafHash())
			if err != nil {
				return err
			}
		}
		return nil
	}

	if newLeftChild := n.LeftChild(); newLeftChild != nil {
		err := dumpAsJSON(newLeftChild, encoder)
		if err != nil {
			return err
		}
	}

	if newRightChild := n.RightChild(); newRightChild != nil {
		err := dumpAsJSON(newRightChild, encoder)
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
func (mt *MTrie) AllPayloads(payloadStorage ledger.PayloadStorage) ([]ledger.Payload, error) {
	leafs := mt.AllLeafNodes()
	payloads := make([]ledger.Payload, 0, len(leafs))
	// TODO: batch request
	for _, leaf := range leafs {
		leafHash := leaf.ExpandedLeafHash()
		_, leafPayload, err := payloadStorage.Get(leafHash)
		if err != nil {
			return nil, fmt.Errorf("could not get payload by hash %v: %w", leafHash, err)
		}
		payloads = append(payloads, *leafPayload)
	}

	return payloads, nil
}

// AllLeafNodes returns all leaf nodes
func (mt *MTrie) AllLeafNodes() []*node.Node {
	return mt.root.AllLeafNodes()
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
//	For instance, if `paths` contains the following 3 paths, and bitIndex is `1`:
//	[[0,0,1,1], [0,1,0,1], [0,0,0,1]]
//	then `splitByPath` returns 2 and updates `paths` into:
//	[[0,0,1,1], [0,0,0,1], [0,1,0,1]]
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
