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
	height       int
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
		height:       height,
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
		height:       root.Height(),
	}, nil
}

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

// UnsafeRead read payloads for the given paths. It is called unsafe as it requires the
// paths to be sorted
// TODO move consistency checks from Forrest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeRead(paths []ledger.Path) []*ledger.Payload {
	res := make([]*ledger.Payload, 0, len(paths))
	mt.read(&res, mt.root, paths)
	return res
}

func (mt *MTrie) read(res *[]*ledger.Payload, head *node.Node, paths []ledger.Path) {
	// path not found
	if head == nil {
		for range paths {
			*res = append(*res, ledger.EmptyPayload())
		}
		return
	}
	// reached a leaf node
	if head.IsLeaf() {
		for _, p := range paths {
			if bytes.Equal(head.Path(), p) {
				*res = append(*res, head.Payload())
			} else {
				*res = append(*res, ledger.EmptyPayload())
			}
		}
		return
	}

	// partition step to quick sort the paths
	partitionIndex := utils.SplitPaths(paths, mt.height-head.Height())
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]

	// must start with the left first, as left payloads have to be appended first
	if len(lpaths) > 0 {
		mt.read(res, head.LeftChild(), lpaths)
	}

	if len(rpaths) > 0 {
		mt.read(res, head.RightChild(), rpaths)
	}
}

// NewTrieWithUpdatedRegisters constructs a new trie containing all registers from the parent trie.
// The key-value pairs specify the registers whose values are supposed to hold updated values
// compared to the parent trie. Constructing the new trie is done in a COPY-ON-WRITE manner:
//   * The original trie remains unchanged.
//   * subtries that remain unchanged are from the parent trie instead of copied.
// UNSAFE: method requires the following conditions to be satisfied:
//   * keys are NOT duplicated
// TODO: move consistency checks from MForest to here, to make API safe and self-contained
func NewTrieWithUpdatedRegisters(parentTrie *MTrie, updatedPaths []ledger.Path, updatedPayloads []ledger.Payload) (*MTrie, error) {
	parentRoot := parentTrie.root
	updatedRoot := parentTrie.update(parentRoot.Height(), parentRoot, updatedPaths, updatedPayloads, nil)
	updatedTrie, err := NewMTrie(updatedRoot)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	return updatedTrie, nil
}

func (parentTrie *MTrie) update(nodeHeight int, parentNode *node.Node,
	paths []ledger.Path, payloads []ledger.Payload, compactLeaf *node.Node) *node.Node {

	// No new paths to write
	if len(paths) == 0 {
		// check is a compactLeaf from a higher height is still left
		if compactLeaf != nil {
			return node.NewLeaf(compactLeaf.Path(), compactLeaf.Payload(), nodeHeight)
		}
		return parentNode
	}

	if len(paths) == 1 && parentNode == nil && compactLeaf == nil {
		return node.NewLeaf(paths[0], &payloads[0], nodeHeight)
	}

	if parentNode != nil && parentNode.IsLeaf() { // if we're here then compactLeaf == nil
		// check if the parent path node is among the updated paths
		parentPath := parentNode.Path()
		found := false
		for i, p := range paths {
			if bytes.Equal(p, parentPath) {
				// the case where the leaf can be reused
				if len(paths) == 1 {
					if !bytes.Equal(parentNode.Payload().Value, payloads[i].Value) {
						return node.NewLeaf(paths[i], &payloads[i], nodeHeight)
					}
					// avoid creating a new node when the same payload is written
					return parentNode
				}
				found = true
				break
			}
		}
		if !found {
			compactLeaf = parentNode
		}
	}

	// in the remaining code: len(paths)>1

	// Split paths and payloads to recurse
	partitionIndex := utils.SplitByPath(paths, payloads, parentTrie.height-nodeHeight)
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lpayloads, rpayloads := payloads[:partitionIndex], payloads[partitionIndex:]

	// check if there is a compact leaf that needs to get deep to height 0
	var lcompactLeaf, rcompactLeaf *node.Node
	if compactLeaf != nil {
		// if yes, check which branch it will go to.
		if utils.Bit(compactLeaf.Path(), parentTrie.height-nodeHeight) == 0 {
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
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild = parentTrie.update(nodeHeight-1, lchildParent, lpaths, lpayloads, lcompactLeaf)
	}()
	rChild = parentTrie.update(nodeHeight-1, rchildParent, rpaths, rpayloads, rcompactLeaf)
	wg.Wait()

	// mitigate storage exhaustion attack: avoids creating a new node when the exact same
	// payload is re-written at a register.
	if lChild == lchildParent && rChild == rchildParent {
		return parentNode
	}
	return node.NewInterimNode(nodeHeight, lChild, rChild)
}

// UnsafeProofs provides proofs for the given paths, this is called unsafe as
// it requires the input paths to be sorted in advance.
func (mt *MTrie) UnsafeProofs(paths []ledger.Path, proofs []*ledger.TrieProof) {
	mt.proofs(mt.root, paths, proofs)
}

func (mt *MTrie) proofs(head *node.Node, paths []ledger.Path, proofs []*ledger.TrieProof) {
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

	// partition step to quick sort the paths
	partitionIndex := utils.SplitTrieProofsByPath(paths, proofs, mt.height-head.Height())
	lpaths, rpaths := paths[:partitionIndex], paths[partitionIndex:]
	lproofs, rproofs := proofs[:partitionIndex], proofs[partitionIndex:]

	if len(lpaths) > 0 {
		if rChild := head.RightChild(); rChild != nil { // TODO: is that a sanity check?
			nodeHash := rChild.Hash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(rChild.Height())) // TODO: why not rChild.RegisterCount != 0?
			if !isDef {                                                                     // in proofs, we only provide non-default value hashes
				for _, p := range lproofs {
					utils.SetBit(p.Flags, mt.height-head.Height())
					p.Interims = append(p.Interims, nodeHash)
				}
			}
		}
		mt.proofs(head.LeftChild(), lpaths, lproofs)
	}

	if len(rpaths) > 0 {
		if lChild := head.LeftChild(); lChild != nil {
			nodeHash := lChild.Hash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(lChild.Height()))
			if !isDef { // in proofs, we only provide non-default value hashes
				for _, p := range rproofs {
					utils.SetBit(p.Flags, mt.height-head.Height())
					p.Interims = append(p.Interims, nodeHash)
				}
			}
		}
		mt.proofs(head.RightChild(), rpaths, rproofs)
	}
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
