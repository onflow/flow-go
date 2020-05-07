package mtrie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// MForest is an in memory forest (collection of tries)
type MForest struct {
	tries       map[string]*MTrie
	maxHeight   int // Height of the tree
	keyByteSize int // acceptable number of bytes for key
}

// NewMForest returns a new instance of memory forest
func NewMForest(maxHeight int) *MForest {
	// add empty roothash
	tries := make(map[string]*MTrie)
	newRoot := newNode(maxHeight - 1)
	rootHash := GetDefaultHashForHeight(maxHeight - 1)
	tries[string(rootHash)] = NewMTrie(newRoot)
	return &MForest{tries: tries,
		maxHeight:   maxHeight,
		keyByteSize: (maxHeight - 1) / 8}
}

// GetEmptyRootHash returns the rootHash of empty forest
func (f *MForest) GetEmptyRootHash() []byte {
	newRoot := newNode(f.maxHeight - 1)
	rootHash := f.ComputeNodeHash(newRoot)
	return rootHash
}

// Read reads values for an slice of keys and returns values and error (if any)
func (f *MForest) Read(keys [][]byte, rootHash []byte) ([][]byte, error) {

	// lookup the trie by rootHash
	trie, ok := f.tries[string(rootHash)]
	if !ok {
		return nil, errors.New("root hash not found")
	}

	// sort keys and deduplicate keys
	sortedKeys := make([][]byte, 0)
	keyOrgIndex := make(map[string][]int)
	for i, key := range keys {
		// check key sizes
		if len(key) != f.keyByteSize {
			return nil, fmt.Errorf("key size doesn't match the trie height: %x", key)
		}
		// only collect dupplicated keys once
		if _, ok := keyOrgIndex[hex.EncodeToString(key)]; !ok {
			sortedKeys = append(sortedKeys, key)
			keyOrgIndex[hex.EncodeToString(key)] = []int{i}
		} else {
			// handles duplicated keys
			keyOrgIndex[hex.EncodeToString(key)] = append(keyOrgIndex[hex.EncodeToString(key)], i)
		}
	}

	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	values, err := f.read(trie, trie.root, sortedKeys)

	if err != nil {
		return nil, err
	}
	// reconstruct the values in the same key order that called the method
	orderedValues := make([][]byte, len(keys))
	for i, k := range sortedKeys {
		for _, j := range keyOrgIndex[hex.EncodeToString(k)] {
			orderedValues[j] = values[i]
		}
	}
	return orderedValues, nil
}

func (f *MForest) read(trie *MTrie, head *node, keys [][]byte) ([][]byte, error) {
	// not found
	if head == nil {
		return [][]byte{[]byte{}}, nil
	}
	// If we are at a leaf node, we create the node
	if len(keys) == 1 && head.lChild == nil && head.rChild == nil {
		// not found
		if head.key == nil || !bytes.Equal(head.key, keys[0]) {
			return [][]byte{[]byte{}}, nil
		}
		// found
		return [][]byte{head.value}, nil
	}

	lkeys, rkeys, err := SplitSortedKeys(keys, f.maxHeight-head.height-1)
	if err != nil {
		return nil, fmt.Errorf("can't read due to split key error: %v", err)
	}

	// TODO make this parallel
	values := make([][]byte, 0)
	if len(lkeys) > 0 {
		v, err := f.read(trie, head.lChild, lkeys)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}

	if len(rkeys) > 0 {
		v, err := f.read(trie, head.rChild, rkeys)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}
	return values, nil
}

// Update updates the values for the registers and returns rootHash and error (if any)
func (f *MForest) Update(keys [][]byte, values [][]byte, rootHash []byte) ([]byte, error) {

	// sort keys and deduplicate keys (we only consider the first occurance, and ignore the rest)
	sortedKeys := make([][]byte, 0)
	valueMap := make(map[string][]byte)
	for i, key := range keys {
		// check key sizes
		if len(key) != f.keyByteSize {
			return nil, fmt.Errorf("key size doesn't match the trie height: %x", key)
		}
		// check if doesn't exist
		if _, ok := valueMap[string(key)]; !ok {
			//do something here
			sortedKeys = append(sortedKeys, key)
			valueMap[string(key)] = values[i]
		}
	}

	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	sortedValues := make([][]byte, 0)
	for _, key := range sortedKeys {
		sortedValues = append(sortedValues, valueMap[string(key)])
	}

	// find trie
	trie, ok := f.tries[string(rootHash)]
	if !ok {
		return nil, errors.New("trie with the given rootHash not found")
	}

	newRoot := newNode(f.maxHeight - 1)
	newTrie := NewMTrie(newRoot)
	newTrie.SetParent(trie)

	err := f.update(newTrie, trie.root, newRoot, sortedKeys, sortedValues)
	if err != nil {
		return nil, err
	}

	newRootHash := f.ComputeNodeHash(newRoot)
	f.tries[string(newRootHash)] = newTrie
	return newRootHash, nil
}

func (f *MForest) update(trie *MTrie, parent *node, head *node, keys [][]byte, values [][]byte) error {
	// parent has a key for this node (add key and insert)
	if parent.key != nil {
		alreadyExist := false
		// deduplicate
		for _, k := range keys {
			if bytes.Compare(k, parent.key) == 0 {
				alreadyExist = true
			}
		}
		if !alreadyExist {
			keys = append(keys, parent.key)
			values = append(values, parent.value)
		}

	}
	// If we are at a leaf node, we create the node
	if len(keys) == 1 && parent.lChild == nil && parent.rChild == nil {
		head.key = keys[0]
		head.value = values[0]
		head.hashValue = f.ComputeNodeHash(head)
		return nil
	}

	// Split the keys and values array so we can update the trie in parallel
	lkeys, lvalues, rkeys, rvalues, err := SplitKeyValues(keys, values, f.maxHeight-head.height-1)
	if err != nil {
		return fmt.Errorf("error spliting key values: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	var lupdate, rupdate *node
	var err1, err2 error
	go func() {
		defer wg.Done()
		// no change needed on the left side
		if len(lkeys) == 0 {
			lupdate = parent.lChild
		} else {
			newN := newNode(parent.height - 1)
			if parent.lChild != nil {
				err1 = f.update(trie, parent.lChild, newN, lkeys, lvalues)
			} else {
				err1 = f.update(trie, newNode(parent.height-1), newN, lkeys, lvalues)
			}
			// newN.hashValue = f.ComputeNodeHash(head)
			lupdate = newN
		}
	}()
	go func() {
		defer wg.Done()
		// no change needed on right side
		if len(rkeys) == 0 {
			rupdate = parent.rChild
		} else {
			newN := newNode(head.height - 1)
			if parent.rChild != nil {
				err2 = f.update(trie, parent.rChild, newN, rkeys, rvalues)
			} else {
				err2 = f.update(trie, newNode(head.height-1), newN, rkeys, rvalues)
			}
			rupdate = newN
		}
	}()
	wg.Wait()

	if err1 != nil {
		return err1
	}
	head.lChild = lupdate

	if err2 != nil {
		return err2
	}
	head.rChild = rupdate

	return nil
}

// Proofs returns a batch proof for the given keys
func (f *MForest) Proofs(keys [][]byte, rootHash []byte) (*BatchProof, error) {

	// look up for non exisitng keys
	notFoundKeys := make([][]byte, 0)
	notFoundValues := make([][]byte, 0)
	retValues, err := f.Read(keys, rootHash)
	if err != nil {
		return nil, err
	}
	for i, key := range keys {
		// TODO figure out nil vs empty slice
		// non exist
		if retValues[i] == nil || len(retValues[i]) == 0 {
			notFoundKeys = append(notFoundKeys, key)
			notFoundValues = append(notFoundValues, []byte{})
		}
	}

	if len(notFoundKeys) > 0 {
		newRootHash, err := f.Update(notFoundKeys, notFoundValues, rootHash)
		if err != nil {
			return nil, err
		}
		// rootHash shouldn't change
		if !bytes.Equal(newRootHash, rootHash) {
			return nil, errors.New("root hash has changed during the operation")
		}
	}

	trie, ok := f.tries[string(rootHash)]
	if !ok {
		return nil, errors.New("root hash not found")
	}

	flags := make([][]byte, len(keys))
	values := make([][][]byte, len(keys))
	inclusions := make([]bool, len(keys))
	steps := make([]uint8, len(keys))

	incl := true
	for i, key := range keys {
		// TODO flags can be optimized to use less space
		flag := make([]byte, f.keyByteSize)
		value := make([][]byte, 0)
		step := uint8(0)

		curr := trie.root

		for i := 0; i < f.maxHeight-1; i++ {
			if bytes.Equal(curr.key, key) {
				break
			}
			bitIsSet, err := IsBitSet(key, i)
			if err != nil {
				return nil, fmt.Errorf("error generating batch proof - error calling IsBitSet : %v", err)
			}
			if bitIsSet {
				if curr.lChild != nil {
					err := SetBit(flag, i)
					if err != nil {
						return nil, fmt.Errorf("error generating batch proof - error calling SetBit: %v", err)
					}
					value = append(value, f.ComputeNodeHash(curr.lChild))
				}
				curr = curr.rChild
				step++
			} else {
				if curr.rChild != nil {
					err := SetBit(flag, i)
					if err != nil {
						return nil, fmt.Errorf("error generating batch proof - error calling SetBit: %v", err)
					}
					value = append(value, f.ComputeNodeHash(curr.rChild))
				}
				curr = curr.lChild
				step++
			}
			if curr == nil {
				// TODO error ??
				incl = false
				break
			}

		}

		flags[i] = flag
		values[i] = value
		inclusions[i] = incl
		steps[i] = step
	}

	return NewBatchProof(flags, values, inclusions, steps), nil
}

// ComputeCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func (f *MForest) ComputeCompactValue(n *node) []byte {
	// if value is nil return default hash
	if len(n.value) == 0 {
		return GetDefaultHashForHeight(n.height)
	}
	computedHash := HashLeaf(n.key, n.value)

	for j := f.maxHeight - 2; j > f.maxHeight-n.height-2; j-- {
		bitIsSet, err := IsBitSet(n.key, j)
		// this won't happen ever
		if err != nil {
			panic(err)
		}
		if bitIsSet { // right branching
			computedHash = HashInterNode(GetDefaultHashForHeight(f.maxHeight-j-2), computedHash)
		} else { // left branching
			computedHash = HashInterNode(computedHash, GetDefaultHashForHeight(f.maxHeight-j-2))
		}
	}
	return computedHash
}

// ComputeNodeHash computes the hashValue for the given node
func (f *MForest) ComputeNodeHash(n *node) []byte {
	if n.hashValue != nil {
		return n.hashValue
	}
	// leaf node (this shouldn't happen)
	if n.lChild == nil && n.rChild == nil {
		if n.key != nil && n.value != nil {
			return f.ComputeCompactValue(n)
		}
		return GetDefaultHashForHeight(n.height)
	}
	// otherwise compute
	h1 := GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = f.ComputeNodeHash(n.lChild)
	}
	h2 := GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = f.ComputeNodeHash(n.rChild)
	}
	return HashInterNode(h1, h2)
}
