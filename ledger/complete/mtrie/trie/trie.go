package trie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/node"
)

const (
	storageFormatVersion uint16 = 1
)

// MTrie is a fully in memory trie with option to persist to disk.
// Formally, an MTrie represents a perfect, full binary Merkle tree with uniform height.
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
//     between v and a tree leaf. The height of a tree is the heights of its root.
//     The height of a Trie is always the height of the fully-expanded tree.
type MTrie struct {
	root         *node.Node
	height       int
	pathByteSize int
}

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

// TODO move consistency checks from Forrest into Trie to obtain a safe, self-contained API
func (mt *MTrie) UnsafeRead(paths []ledger.Path) ([]ledger.Payload, error) {
	return mt.read(mt.root, paths)
}

func (mt *MTrie) read(head *node.Node, paths []ledger.Path) ([]ledger.Payload, error) {
	// path not found
	if head == nil {
		res := make([]ledger.Payload, 0, len(paths))
		for range paths {
			res = append(res, *ledger.EmptyPayload())
		}
		return res, nil
	}
	// reached a leaf node
	if head.IsLeaf() {
		res := make([]ledger.Payload, 0)
		for _, p := range paths {
			if bytes.Equal(head.Path(), p) {
				res = append(res, *head.Payload())
			} else {
				res = append(res, *ledger.EmptyPayload())
			}
		}
		return res, nil
	}

	lpaths, rpaths, err := common.SplitSortedPaths(paths, mt.height-head.Height())
	if err != nil {
		return nil, fmt.Errorf("can't read due to split path error: %w", err)
	}

	// TODO make this parallel
	values := make([]ledger.Payload, 0)
	if len(lpaths) > 0 {
		v, err := mt.read(head.LeftChild(), lpaths)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}

	if len(rpaths) > 0 {
		v, err := mt.read(head.RigthChild(), rpaths)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}
	return values, nil
}

// NewTrieWithUpdatedRegisters constructs a new trie containing all registers from the parent trie.
// The key-value pairs specify the registers whose values are supposed to hold updated values
// compared to the parent trie. Constructing the new trie is done in a COPY-ON-WRITE manner:
//   * The original trie remains unchanged.
//   * subtries that remain unchanged are from the parent trie instead of copied.
// UNSAFE: method requires the following conditions to be satisfied:
//   * keys are NOT duplicated
// TODO: move consistency checks from MForest to here, to make API is safe and self-contained
func NewTrieWithUpdatedRegisters(parentTrie *MTrie, updatedPaths []ledger.Path, updatedPayloads []ledger.Payload) (*MTrie, error) {
	parentRoot := parentTrie.root
	updatedRoot, err := update(parentTrie.height, parentRoot.Height(), parentRoot, updatedPaths, updatedPayloads)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	updatedTrie, err := NewMTrie(updatedRoot)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	return updatedTrie, nil
}

// update returns the head of updated sub-trie for the specified key-value pairs.
// UNSAFE: update requires the following conditions to be satisfied,
// but does not explicitly check them for performance reasons
//   * all keys AND the parent node share the same common prefix [0 : mt.maxHeight-1 - headHeight)
//     (excluding the bit at index headHeight)
//   * keys are NOT duplicated
// TODO: remove error return
func update(treeHeight int, nodeHeight int, parentNode *node.Node, paths []ledger.Path, payloads []ledger.Payload) (*node.Node, error) {
	if parentNode == nil { // parent Trie has no sub-trie for the set of paths => construct entire subtree
		return constructSubtrie(treeHeight, nodeHeight, paths, payloads)
	}

	if len(paths) == 0 { // We are not changing any values in this sub-trie => return parent trie
		return parentNode, nil
	}

	// from here on, we have parentNode != nil AND len(paths) > 0
	if parentNode.IsLeaf() { // parent node is a leaf, i.e. parent Trie only stores a single value in this sub-trie
		parentPath := parentNode.Path() // Per definition, a leaf must have a non-nil path
		overrideExistingValue := false  // true if and only if we are updating the parent Trie's leaf node value
		for _, p := range paths {
			if bytes.Equal(p, parentPath) {
				overrideExistingValue = true
				break
			}
		}
		if !overrideExistingValue {
			// TODO: copy payload when using in-place MergeSort for separating the payloads
			paths = append(paths, parentNode.Path())
			payloads = append(payloads, *parentNode.Payload())
		}
		return constructSubtrie(treeHeight, nodeHeight, paths, payloads)
	}

	// Split payloads so we can update the trie in parallel
	lpaths, lpayloads, rpaths, rpayloads, err := common.SplitByPath(paths, payloads, treeHeight-nodeHeight)
	if err != nil {
		return nil, fmt.Errorf("error spliting payloads by path: %w", err)
	}

	// TODO [runtime optimization]: do not branch if either lpayload or rpayload is empty
	var lChild, rChild *node.Node
	var lErr, rErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild, lErr = update(treeHeight, nodeHeight-1, parentNode.LeftChild(), lpaths, lpayloads)
	}()
	rChild, rErr = update(treeHeight, nodeHeight-1, parentNode.RigthChild(), rpaths, rpayloads)
	wg.Wait()
	if lErr != nil || rErr != nil {
		var merr *multierror.Error
		if lErr != nil {
			merr = multierror.Append(merr, lErr)
		}
		if rErr != nil {
			merr = multierror.Append(merr, rErr)
		}
		if err := merr.ErrorOrNil(); err != nil {
			return nil, fmt.Errorf("internal error while updating trie: %w", err)
		}
	}

	return node.NewInterimNode(nodeHeight, lChild, rChild), nil
}

// constructSubtrie returns the head of a newly-constructed sub-trie for the specified key-value pairs.
// UNSAFE: constructSubtrie requires the following conditions to be satisfied,
// but does not explicitly check them for performance reasons
//   * paths all share the same common prefix [0 : mt.maxHeight-1 - headHeight)
//     (excluding the bit at index headHeight)
//   * paths contains at least one element
//   * paths are NOT duplicated
// TODO: remove error return
func constructSubtrie(treeHeight int, nodeHeight int, paths []ledger.Path, payloads []ledger.Payload) (*node.Node, error) {
	// no inserts => default value, represented by nil node
	if len(paths) == 0 {
		return nil, nil
	}
	// If we are at a leaf node, we create the node
	if len(paths) == 1 {
		return node.NewLeaf(paths[0], &payloads[0], nodeHeight), nil
	}
	// from here on, we have: len(paths) > 1

	// Split updates by paths so we can update the trie in parallel
	lpaths, lpayloads, rpaths, rpayloads, err := common.SplitByPath(paths, payloads, treeHeight-nodeHeight)
	// Note: (pathLength-height) will never reach the value pathLength, i.e. we will never execute this code for height==0
	// This is because at height=0, we only have (at most) one path left, as paths are not duplicated
	// (by requirement of this function). But even if this condition is violated, the code will not return a faulty
	// but instead panic with Index Out Of Range error
	if err != nil {
		return nil, fmt.Errorf("error spliting paths: %w", err)
	}

	// TODO [runtime optimization]: do not branch if either lpaths or rpaths is empty
	var lChild, rChild *node.Node
	var lErr, rErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild, lErr = constructSubtrie(treeHeight, nodeHeight-1, lpaths, lpayloads)
	}()
	rChild, rErr = constructSubtrie(treeHeight, nodeHeight-1, rpaths, rpayloads)
	wg.Wait()
	if lErr != nil || rErr != nil {
		var merr *multierror.Error
		if lErr != nil {
			merr = multierror.Append(merr, lErr)
		}
		if rErr != nil {
			merr = multierror.Append(merr, rErr)
		}
		if err := merr.ErrorOrNil(); err != nil {
			return nil, fmt.Errorf("internal error while constructing sub-trie: %w", err)
		}
	}

	return node.NewInterimNode(nodeHeight, lChild, rChild), nil
}

func (mt *MTrie) UnsafeProofs(paths []ledger.Path, proofs []*ledger.Proof) error {
	return mt.proofs(mt.root, paths, proofs)
}

func (mt *MTrie) proofs(head *node.Node, paths []ledger.Path, proofs []*ledger.Proof) error {
	// we've reached the end of a trie
	// and path is not found (noninclusion proof)
	if head == nil {
		return nil
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
		return nil
	}

	// increment steps for all the proofs
	for _, p := range proofs {
		p.Steps++
	}
	// split paths based on the value of i-th bit (i = trie height - node height)
	lpaths, lproofs, rpaths, rproofs, err := common.SplitProofsByPath(paths, proofs, mt.height-head.Height())
	if err != nil {
		return fmt.Errorf("proof generation failed, path split error: %w", err)
	}

	if len(lpaths) > 0 {
		if rChild := head.RigthChild(); rChild != nil {
			nodeHash := rChild.Hash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(rChild.Height()))
			if !isDef { // in proofs, we only provide non-default value hashes
				for _, p := range lproofs {
					err := common.SetBit(p.Flags, mt.height-head.Height())
					if err != nil {
						return err
					}
					p.Interims = append(p.Interims, nodeHash)
				}
			}
		}
		err := mt.proofs(head.LeftChild(), lpaths, lproofs)
		if err != nil {
			return err
		}
	}

	if len(rpaths) > 0 {
		if lChild := head.LeftChild(); lChild != nil {
			nodeHash := lChild.Hash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(lChild.Height()))
			if !isDef { // in proofs, we only provide non-default value hashes
				for _, p := range rproofs {
					err := common.SetBit(p.Flags, mt.height-head.Height())
					if err != nil {
						return err
					}
					p.Interims = append(p.Interims, nodeHash)
				}
			}
		}
		err := mt.proofs(head.RigthChild(), rpaths, rproofs)
		if err != nil {
			return err
		}
	}
	return nil
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

// // Store stores the trie path-key-value triples to a file
// func (mt *MTrie) Store(path string) error {
// 	fi, err := os.Create(path)
// 	if err != nil {
// 		return err
// 	}
// 	defer fi.Close()
// 	writer := bufio.NewWriter(fi)
// 	defer writer.Flush()

// 	// first two byte are the storage format version
// 	b1 := make([]byte, 2)
// 	binary.LittleEndian.PutUint16(b1, storageFormatVersion)
// 	_, err = writer.Write(b1)
// 	if err != nil {
// 		return err
// 	}

// 	// then 2 bytes capture the trie's path size
// 	b2 := make([]byte, 2)
// 	binary.LittleEndian.PutUint16(b2, uint16(mt.pathByteSize))
// 	_, err = writer.Write(b2)
// 	if err != nil {
// 		return err
// 	}

// 	// next 32 bytes are trie rootHash
// 	_, err = writer.Write(mt.RootHash())
// 	if err != nil {
// 		return err
// 	}

// 	// repeated: x bytes path, 8 bytes payload size (in bytes), payload bytes
// 	err = mt.store(mt.root, writer)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (mt *MTrie) store(n *node.Node, writer *bufio.Writer) error {
// 	if path := n.Path(); path != nil {
// 		_, err := writer.Write(n.Path())
// 		if err != nil {
// 			return err
// 		}

// 		// payload size
// 		pb := n.Payload().Encode()

// 		b2 := make([]byte, 8)
// 		binary.LittleEndian.PutUint64(b2, uint64(len(pb)))
// 		_, err = writer.Write(b2)
// 		if err != nil {
// 			return err
// 		}

// 		_, err = writer.Write(pb)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if lChild := n.LeftChild(); lChild != nil {
// 		err := mt.store(lChild, writer)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if rChild := n.RigthChild(); rChild != nil {
// 		err := mt.store(rChild, writer)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// // Load loads a trie
// func Load(filePath string) (*MTrie, error) {
// 	fi, err := os.Open(filePath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer fi.Close()

// 	// first two byte are the storage format version
// 	version := make([]byte, 2)
// 	_, err = fi.Read(version)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if binary.LittleEndian.Uint16(version) != storageFormatVersion { // assert encoding version
// 		return nil, errors.New("trie storage version doesn't match")
// 	}

// 	// next 2 bytes capture the trie's path size
// 	pathByteSizeB := make([]byte, 2)
// 	_, err = fi.Read(pathByteSizeB)
// 	if err != nil {
// 		return nil, err
// 	}
// 	pathByteSize := int(binary.LittleEndian.Uint16(pathByteSizeB))

// 	// next 32 bytes are rootHash
// 	expectedRootHash := make([]byte, 32)
// 	_, err = fi.Read(expectedRootHash)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// repeated: x bytes path, 8 bytes payload size (in bytes), payload bytes
// 	paths := make([][]byte, 0)
// 	payloads := make([]ledger.Payload, 0)
// 	for {
// 		path := make([]byte, pathByteSize)
// 		_, err = fi.Read(path)
// 		if err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			return nil, err
// 		}

// 		payloadSizeB := make([]byte, 8)
// 		_, err = fi.Read(payloadSizeB)
// 		if err != nil {
// 			return nil, err
// 		}

// 		keySize := binary.LittleEndian.Uint64(keySizeB)
// 		key := make([]byte, keySize)
// 		_, err = fi.Read(key)
// 		if err != nil {
// 			return nil, err
// 		}

// 		valueSizeB := make([]byte, 8)
// 		_, err = fi.Read(valueSizeB)
// 		if err != nil {
// 			return nil, err
// 		}

// 		valueSize := binary.LittleEndian.Uint64(valueSizeB)
// 		value := make([]byte, valueSize)
// 		_, err = fi.Read(value)
// 		if err != nil {
// 			return nil, err
// 		}

// 		paths = append(paths, path)
// 		keys = append(keys, key)
// 		values = append(values, value)
// 	}

// 	// reconstruct trie
// 	trie, err := constructTrieFromKeyValuePairs(pathByteSize, paths, keys, values)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if !bytes.Equal(expectedRootHash, trie.RootHash()) {
// 		return nil, errors.New("root hash of reconstructed trie does not match")
// 	}

// 	return trie, nil
// }

// // constructTrieFromKeyValuePairs constructs a trie from the given key-value pairs.
// // UNSAFE: function requires the following conditions to be satisfied, but does not explicitly check them:
// //   * keys must have the same PathLength
// func constructTrieFromKeyValuePairs(pathByteSize int, paths []ledger.Path, keys [][]byte, values [][]byte) (*MTrie, error) {
// 	treeHeight := 8 * pathByteSize
// 	root, err := constructSubtrie(treeHeight, treeHeight, paths, keys, values)
// 	if err != nil {
// 		return nil, fmt.Errorf("constructing trie from key-value pairs failed: %w", err)
// 	}

// 	updatedTrie, err := NewMTrie(root)
// 	if err != nil {
// 		return nil, fmt.Errorf("constructing trie from key-value pairs failed: %w", err)
// 	}
// 	return updatedTrie, nil
// }

// EmptyTrieRootHash returns the rootHash of an empty Trie for the specified path size [bytes]
func EmptyTrieRootHash(pathByteSize int) []byte {
	return node.NewEmptyTreeRoot(8 * pathByteSize).Hash()
}
