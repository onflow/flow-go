package trie

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/node"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
)

// MTrie is a fully in memory trie with option to persist to disk
// TODO: UPDATE to this DEFINITION:
//   * HEIGHT of a node v in a tree is the number of edges on the longest downward path
//    between v and a tree leaf. The height of a tree is the heights of its root.
type MTrie struct {
	root           *node.Node
	number         uint64
	maxHeight      int
	parentRootHash []byte
}

func NewEmptyMTrie(maxHeight int, number uint64, parentRootHash []byte) (*MTrie, error) {
	if (maxHeight-1)%8 != 0 {
		return nil, errors.New("key length of trie must be integer-multiple of 8")
	}
	return &MTrie{
		root:           node.NewEmptyTreeRoot(maxHeight - 1),
		number:         number,
		maxHeight:      maxHeight,
		parentRootHash: parentRootHash,
	}, nil
}

func (mt *MTrie) StringRootHash() string { return hex.EncodeToString(mt.root.Hash()) }
func (mt *MTrie) RootHash() []byte       { return mt.root.Hash() }
func (mt *MTrie) Number() uint64         { return mt.number }
func (mt *MTrie) ParentRootHash() []byte { return mt.parentRootHash }

// Height return the trie height. The height is identical to the key length [in bit].
func (mt *MTrie) Height() int { return mt.maxHeight - 1 }

func (mt *MTrie) String() string {
	trieStr := fmt.Sprintf("Trie number:%v hash:%v parent: %v\n", mt.number, mt.StringRootHash(), hex.EncodeToString(mt.parentRootHash))
	return trieStr + mt.root.FmtStr("", "")
}

func (mt *MTrie) UnsafeRead(keys [][]byte) ([][]byte, error) {
	return mt.read(mt.root, keys)
}

func (mt *MTrie) read(head *node.Node, keys [][]byte) ([][]byte, error) {
	// keys not found
	if head == nil {
		res := make([][]byte, 0, len(keys))
		for range keys {
			res = append(res, []byte{})
		}
		return res, nil
	}
	// reached a leaf node
	if head.Key() != nil {
		res := make([][]byte, 0)
		for _, k := range keys {
			if bytes.Equal(head.Key(), k) {
				res = append(res, head.Value())
			} else {
				res = append(res, []byte{})
			}
		}
		return res, nil
	}

	lkeys, rkeys, err := common.SplitSortedKeys(keys, mt.maxHeight-head.Height()-1)
	if err != nil {
		return nil, fmt.Errorf("can't read due to split key error: %w", err)
	}

	// TODO make this parallel
	values := make([][]byte, 0)
	if len(lkeys) > 0 {
		v, err := mt.read(head.LeftChild(), lkeys)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}

	if len(rkeys) > 0 {
		v, err := mt.read(head.RigthChild(), rkeys)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}
	return values, nil
}

// UnsafeUpdate returns constructs a new trie by updating the specified registers.
// Updates are performed in a copy-on-write manner:
//   * The original trie remains unchanged.
//   * identical subtries from the original trie are referenced instead of copied.
// UNSAFE: method requires the following conditions to be satisfied:
//   * keys are NOT duplicated
// TODO: move consistency checks from Forrest to here, so API is safe and self-contained
func (mt *MTrie) UnsafeUpdate(keys [][]byte, values [][]byte) (*MTrie, error) {
	parentRoot := mt.root
	updatedRoot, err := mt.update(keys, values, parentRoot.Height(), parentRoot)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}
	updatedTrie := &MTrie{
		root:           updatedRoot,
		number:         mt.number + 1,
		maxHeight:      mt.maxHeight,
		parentRootHash: mt.RootHash(),
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
func (mt *MTrie) update(keys [][]byte, values [][]byte, height int, parentNode *node.Node) (*node.Node, error) {
	if parentNode == nil { // parent Trie has no sub-trie for the set of key-value pairs => construct entire subtree
		return constructSubtrie(height, keys, mt.maxHeight-1, values)
	}
	if len(keys) == 0 { // We are not changing any values in this sub-trie => return parent trie
		return parentNode, nil
	}
	// from here on, we have parentNode != nil AND len(keys) > 0

	if parentNode.IsLeaf() { // parent node is a leaf, i.e. parent Trie only stores a single value in this sub-trie
		parentKey := parentNode.Key()  // Per definition, a leaf must have a key-value pair
		overrideExistingValue := false // true if and only if we are updating the parent Trie's leaf node value
		for _, k := range keys {
			if bytes.Equal(k, parentKey) {
				overrideExistingValue = true
				break
			}
		}
		if !overrideExistingValue {
			// TODO: copy keys and values when using in-place MergeSort for separating the keys
			keys = append(keys, parentKey)
			values = append(values, parentNode.Value())
		}
		return constructSubtrie(height, keys, mt.maxHeight-1, values)
	}

	// Split the keys and Values array so we can update the trie in parallel
	lkeys, lvalues, rkeys, rvalues, err := common.SplitKeyValues(keys, values, mt.maxHeight-1-height)
	if err != nil {
		return nil, fmt.Errorf("error spliting key Values: %w", err)
	}

	// TODO [runtime optimization]: do not branch if either lkeys or rkeys is empty
	var lChild, rChild *node.Node
	var lErr, rErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild, lErr = mt.update(lkeys, lvalues, height-1, parentNode.LeftChild())
	}()
	rChild, rErr = mt.update(rkeys, rvalues, height-1, parentNode.RigthChild())
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

	return node.NewInterimNode(height, lChild, rChild), nil
}

// constructSubtrie returns the head of a newly-constructed sub-trie for the specified key-value pairs.
// UNSAFE: constructSubtrie requires the following conditions to be satisfied,
// but does not explicitly check them for performance reasons
//   * keys all share the same common prefix [0 : mt.maxHeight-1 - headHeight)
//     (excluding the bit at index headHeight)
//   * keys contains at least one element
//   * keys are NOT duplicated
// TODO: remove error return
func constructSubtrie(height int, keys [][]byte, keyLength int, values [][]byte) (*node.Node, error) {
	// no keys => default value, represented by nil node
	if len(keys) == 0 {
		return nil, nil
	}
	// If we are at a leaf node, we create the node
	if len(keys) == 1 {
		return node.NewLeaf(keys[0], values[0], height), nil
	}
	// from here on, we have: len(keys) > 1

	// Split the keys and Values array so we can update the trie in parallel
	lkeys, lvalues, rkeys, rvalues, err := common.SplitKeyValues(keys, values, keyLength-height)
	// Note: (keyLength-height) will never reach the value keyLength, i.e. we will never execute this code for height==0
	// This is because at height=0, we only have (at most) one key left, as keys are not duplicated
	// (by requirement of this function). But even if this condition is violated, the code will not return a faulty
	// but instead panic with Index Out Of Range error
	if err != nil {
		return nil, fmt.Errorf("error spliting key Values: %w", err)
	}

	// TODO [runtime optimization]: do not branch if either lkeys or rkeys is empty
	var lChild, rChild *node.Node
	var lErr, rErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lChild, lErr = constructSubtrie(height-1, lkeys, keyLength, lvalues)
	}()
	rChild, rErr = constructSubtrie(height-1, rkeys, keyLength, rvalues)
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

	return node.NewInterimNode(height, lChild, rChild), nil
}

func (mt *MTrie) UnsafeProofs(keys [][]byte, proofs []*proof.Proof) error {
	return mt.proofs(mt.root, keys, proofs)
}

func (mt *MTrie) proofs(head *node.Node, keys [][]byte, proofs []*proof.Proof) error {
	// we've reached the end of a trie
	// and key is not found (noninclusion proof)
	if head == nil {
		return nil
	}

	// we've reached a leaf that has a key
	if head.Key() != nil {
		// value matches (inclusion proof)
		if bytes.Equal(head.Key(), keys[0]) {
			proofs[0].Inclusion = true
		}
		return nil
	}

	// increment steps for all the proofs
	for _, p := range proofs {
		p.Steps++
	}
	// split keys based on the value of i-th bit (i = trie height - node height)
	lkeys, lproofs, rkeys, rproofs, err := proof.SplitKeyProofs(keys, proofs, mt.maxHeight-head.Height()-1)
	if err != nil {
		return fmt.Errorf("proof generation failed, split key error: %w", err)
	}

	if len(lkeys) > 0 {
		if rChild := head.RigthChild(); rChild != nil {
			nodeHash := rChild.Hash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(rChild.Height()))
			if !isDef { // in proofs, we only provide non-default value hashes
				for _, p := range lproofs {
					err := common.SetBit(p.Flags, mt.maxHeight-1-head.Height())
					if err != nil {
						return err
					}
					p.Values = append(p.Values, nodeHash)
				}
			}
		}
		err := mt.proofs(head.LeftChild(), lkeys, lproofs)
		if err != nil {
			return err
		}
	}

	if len(rkeys) > 0 {
		if lChild := head.LeftChild(); lChild != nil {
			nodeHash := lChild.Hash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(lChild.Height()))
			if !isDef { // in proofs, we only provide non-default value hashes
				for _, p := range rproofs {
					err := common.SetBit(p.Flags, mt.maxHeight-1-head.Height())
					if err != nil {
						return err
					}
					p.Values = append(p.Values, nodeHash)
				}
			}
		}
		err := mt.proofs(head.RigthChild(), rkeys, rproofs)
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
	return o.Number() == mt.Number() && o.Height() == mt.Height() && bytes.Equal(o.RootHash(), mt.RootHash())
}

// Store stores the trie key Values to a file
func (mt *MTrie) Store(path string) error {
	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()
	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	// first byte is the store format version
	_, err = writer.Write([]byte{byte(1)})
	if err != nil {
		return err
	}

	// then 8 bytes captures trie number
	b1 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b1, mt.number)
	_, err = writer.Write(b1)
	if err != nil {
		return err
	}

	// then 2 bytes capture the maxHeight
	b2 := make([]byte, 2)
	binary.LittleEndian.PutUint16(b2, uint16(mt.maxHeight))
	_, err = writer.Write(b2)
	if err != nil {
		return err
	}

	// next 32 bytes are parent rootHash
	_, err = writer.Write(mt.parentRootHash)
	if err != nil {
		return err
	}

	// next 32 bytes are trie rootHash
	_, err = writer.Write(mt.RootHash())
	if err != nil {
		return err
	}

	// repeated: x bytes key, 4bytes valueSize(Number of bytes value took), valueSize bytes value)
	err = mt.store(mt.root, writer)
	if err != nil {
		return err
	}

	return nil
}

func (mt *MTrie) store(n *node.Node, writer *bufio.Writer) error {
	if key := n.Key(); key != nil {
		_, err := writer.Write(key)
		if err != nil {
			return err
		}

		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(len(n.Value())))
		_, err = writer.Write(b)
		if err != nil {
			return err
		}

		_, err = writer.Write(n.Value())
		if err != nil {
			return err
		}
	}

	if lChild := n.LeftChild(); lChild != nil {
		err := mt.store(lChild, writer)
		if err != nil {
			return err
		}
	}

	if rChild := n.RigthChild(); rChild != nil {
		err := mt.store(rChild, writer)
		if err != nil {
			return err
		}
	}
	return nil
}

// Load loads a trie
func Load(path string) (*MTrie, error) {
	fi, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	// first byte is the store format version
	version := make([]byte, 1)
	_, err = fi.Read(version)
	if err != nil {
		return nil, err
	}
	if uint8(version[0]) != uint8(1) { // assert encoding version
		return nil, errors.New("trie store/load version doesn't match")
	}

	// next 8 bytes captures trie number
	trieNumberB := make([]byte, 8)
	_, err = fi.Read(trieNumberB)
	if err != nil {
		return nil, err
	}
	trieNumber := binary.LittleEndian.Uint64(trieNumberB)

	// next 2 bytes capture the maxHeight
	maxHeightB := make([]byte, 2)
	_, err = fi.Read(maxHeightB)
	if err != nil {
		return nil, err
	}
	maxHeight := binary.LittleEndian.Uint16(maxHeightB)
	if (maxHeight-1)%8 != 0 {
		return nil, errors.New("key length of trie must be integer-multiple of 8")
	}
	keyByteSize := (maxHeight - 1) / 8

	// next 32 bytes are parent rootHash
	parentRootHash := make([]byte, 32)
	_, err = fi.Read(parentRootHash)
	if err != nil {
		return nil, err
	}

	// next 32 bytes are rootHash
	expectedRootHash := make([]byte, 32)
	_, err = fi.Read(expectedRootHash)
	if err != nil {
		return nil, err
	}

	// repeated: x bytes key, 4bytes valueSize(Number of bytes value took), valueSize bytes value)
	keys := make([][]byte, 0)
	values := make([][]byte, 0)
	for {
		key := make([]byte, keyByteSize)
		_, err = fi.Read(key)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		valueSizeB := make([]byte, 8)
		_, err = fi.Read(valueSizeB)
		if err != nil {
			return nil, err
		}

		valueSize := binary.LittleEndian.Uint64(valueSizeB)
		value := make([]byte, valueSize)
		_, err = fi.Read(value)
		if err != nil {
			return nil, err
		}

		keys = append(keys, key)
		values = append(values, value)
	}

	// reconstruct trie
	trie, err := constructTrieFromKeyValuePairs(keys, int(maxHeight-1), values, trieNumber, parentRootHash)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(expectedRootHash, trie.RootHash()) {
		return nil, errors.New("root hash of reconstructed trie does not match")
	}

	return trie, nil
}

// constructTrieFromKeyValuePairs constructs a trie from the given key-value pairs.
// UNSAFE: function requires the following conditions to be satisfied, but does not explicitly check them:
//   * keys must have the same keyLength
func constructTrieFromKeyValuePairs(keys [][]byte, keyLength int, values [][]byte, number uint64, parentRootHash []byte) (*MTrie, error) {
	root, err := constructSubtrie(keyLength, keys, keyLength, values)
	if err != nil {
		return nil, fmt.Errorf("constructing trie from key-value pairs failed: %w", err)
	}

	return &MTrie{
		root:           root,
		number:         number,
		maxHeight:      keyLength + 1, // TODO: fix me when replacing maxHeight definition
		parentRootHash: parentRootHash,
	}, nil
}
