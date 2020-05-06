package mtrie

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

type MForest struct {
	tries       map[string]*MTrie
	maxHeight   int // Height of the tree
	keyByteSize int // acceptable number of bytes for key
}

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

func (f *MForest) GetEmptyRootHash() []byte {
	newRoot := newNode(f.maxHeight - 1)
	rootHash := f.ComputeNodeHash(newRoot)
	return rootHash
}

// returns keys, values, error
func (f *MForest) Read(keys [][]byte, rootHash []byte) ([][]byte, [][]byte, error) {
	trie, ok := f.tries[string(rootHash)]
	if !ok {
		return nil, nil, errors.New("root hash not found")
	}

	// sort keys and deduplicate keys
	sortedKeys := make([][]byte, 0)
	keyOrgIndex := make(map[string]int)
	for i, key := range keys {
		// check key sizes
		if len(key) != f.keyByteSize {
			return nil, nil, fmt.Errorf("key size doesn't match the trie height: %x", key)
		}
		// check if doesn't exist
		if _, ok := keyOrgIndex[string(key)]; !ok {
			//do something here
			sortedKeys = append(sortedKeys, key)
			keyOrgIndex[string(key)] = i
		}
	}

	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	values, err := f.read(trie, trie.root, sortedKeys)
	return sortedKeys, values, err
}

func (f *MForest) read(trie *MTrie, head *node, keys [][]byte) ([][]byte, error) {

	// If we are at a leaf node, we create the node
	if len(keys) == 1 && head.lChild == nil && head.rChild == nil {
		// not found
		if head.key == nil || !bytes.Equal(head.key, keys[0]) {
			return [][]byte{[]byte{}}, nil
		}
		// found
		return [][]byte{head.value}, nil
	}

	// Split the keys so we can lookup the trie in parallel
	lkeys, rkeys, _ := utils.SplitKeys(keys, f.maxHeight-head.height-1)

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

// returns new state commitment, error
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
	newTrie.parent = trie

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
		nKeys := make([][]byte, 0, len(keys)+1)
		nValues := make([][]byte, 0, len(values)+1)
		for i, k := range keys {
			comp := bytes.Compare(k, parent.key)
			if comp == 0 {
				nKeys = keys
				nValues = values
				break
			}
			if comp > 0 {
				nKeys = append(nKeys, keys[:i]...)
				nKeys = append(nKeys, [][]byte{parent.key}...)
				nKeys = append(nKeys, keys[i:]...)
				nValues = append(nValues, values[:i]...)
				nValues = append(nValues, [][]byte{parent.value}...)
				nValues = append(nValues, values[i:]...)
				break
			}
		}
		// key bigger than all elements
		if len(nKeys) == 0 {
			keys = append(keys, parent.key)
			values = append(values, parent.value)
		} else {
			keys = nKeys
			values = nValues
		}
	}

	// If we are at a leaf node, we create the node
	if len(keys) == 1 && parent.lChild == nil && parent.rChild == nil {
		head.key = keys[0]
		// TODO at hash time compute the hash including the nodes under
		head.value = values[0]
		return nil
	}

	// Split the keys and values array so we can update the trie in parallel
	lkeys, rkeys, splitIndex := utils.SplitKeys(keys, f.maxHeight-head.height-1)
	lvalues, rvalues := values[:splitIndex], values[splitIndex:]
	if len(lkeys) != len(lvalues) {
		return errors.New("left Key/Value Length mismatch")
	}
	if len(rkeys) != len(rvalues) {
		return errors.New("right Key/Value Length mismatch")
	}

	// no change needed on right side
	if len(rkeys) == 0 {
		if parent != nil {
			head.rChild = parent.rChild
		}
	} else {
		newN := newNode(head.height - 1)
		if parent.rChild != nil {
			err := f.update(trie, parent.rChild, newN, rkeys, rvalues)
			if err != nil {
				return err
			}
		} else {
			err := f.update(trie, newN, newN, rkeys, rvalues)
			if err != nil {
				return err
			}
		}
		head.rChild = newN
	}

	// no change needed on the left side
	if len(lkeys) == 0 {
		head.lChild = parent.lChild
	} else {
		newN := newNode(parent.height - 1)
		if parent.lChild != nil {
			err := f.update(trie, parent.lChild, newN, lkeys, lvalues)
			if err != nil {
				return err
			}
		} else {
			err := f.update(trie, newN, newN, lkeys, lvalues)
			if err != nil {
				return err
			}
		}
		head.lChild = newN
	}

	return nil
}

// ComputeCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func (f *MForest) ComputeCompactValue(n *node) []byte {
	// if value is nil return default hash
	if len(n.value) == 0 {
		return GetDefaultHashForHeight(n.height)
	}
	computedHash := HashLeaf(n.key, n.value)

	for j := f.maxHeight - 2; j > f.maxHeight-n.height-2; j-- {
		if utils.IsBitSet(n.key, j) { // right branching
			computedHash = HashInterNode(GetDefaultHashForHeight(f.maxHeight-j-2), computedHash)
		} else { // left branching
			computedHash = HashInterNode(computedHash, GetDefaultHashForHeight(f.maxHeight-j-2))
		}
	}
	return computedHash
}

func (f *MForest) ComputeNodeHash(n *node) []byte {
	// leaf node
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
