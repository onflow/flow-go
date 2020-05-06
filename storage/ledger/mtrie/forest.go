package mtrie

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// MForest is an in memory forest (collection of tries)
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
		return nil
	}

	// Split the keys and values array so we can update the trie in parallel
	lkeys, lvalues, rkeys, rvalues := SplitKeyValues(keys, values, f.maxHeight-head.height-1)
	if len(lkeys) != len(lvalues) {
		return errors.New("left Key/Value Length mismatch")
	}
	if len(rkeys) != len(rvalues) {
		return errors.New("right Key/Value Length mismatch")
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	var lupdate, rupdate *node
	var err1, err2 error
	if len(lkeys) > 0 && len(rkeys) > 0 && lkeys[0][0] == byte(uint8(138)) && rkeys[0][0] == byte(uint8(175)) {
		fmt.Println(lkeys, rkeys)
	}
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

// returns lkeys, lvalues, rkeys, rvalues
func SplitKeyValues(keys [][]byte, values [][]byte, bitIndex int) ([][]byte, [][]byte, [][]byte, [][]byte) {

	rkeys := make([][]byte, 0, len(keys))
	rvalues := make([][]byte, 0, len(values))
	lkeys := make([][]byte, 0, len(keys))
	lvalues := make([][]byte, 0, len(values))

	// splits keys at the smallest index i where keys[i] >= split
	for i, key := range keys {
		if utils.IsBitSet(key, bitIndex) {
			rkeys = append(rkeys, key)
			rvalues = append(rvalues, values[i])
		} else {
			lkeys = append(lkeys, key)
			lvalues = append(lvalues, values[i])
		}
	}
	return lkeys, lvalues, rkeys, rvalues
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
