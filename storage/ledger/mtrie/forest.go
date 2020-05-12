package mtrie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

// MForest is an in memory forest (collection of tries)
type MForest struct {
	tries       *lru.Cache
	dir         string
	cacheSize   int
	maxHeight   int // Height of the tree
	keyByteSize int // acceptable number of bytes for key
}

// NewMForest returns a new instance of memory forest
func NewMForest(maxHeight int, trieStorageDir string, trieCacheSize int) (*MForest, error) {

	evict := func(key interface{}, value interface{}) {
		trie, ok := value.(*MTrie)
		if !ok {
			panic(fmt.Sprintf("cache contains item of type %T", value))
		}
		go trie.Store(filepath.Join(trieStorageDir, hex.EncodeToString(trie.rootHash)))
	}
	cache, err := lru.NewWithEvict(trieCacheSize, evict)
	if err != nil {
		return nil, fmt.Errorf("cannot create forest cache: %w", err)
	}

	forest := &MForest{tries: cache,
		maxHeight:   maxHeight,
		dir:         trieStorageDir,
		cacheSize:   trieCacheSize,
		keyByteSize: (maxHeight - 1) / 8}

	// add empty roothash
	emptyTrie := NewMTrie(maxHeight)
	emptyTrie.number = uint64(0)
	emptyTrie.rootHash = GetDefaultHashForHeight(maxHeight - 1)

	forest.AddTrie(emptyTrie)
	return forest, nil
}

// GetTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *MForest) GetTrie(rootHash []byte) (*MTrie, error) {
	encRootHash := hex.EncodeToString(rootHash)
	// if in the cache

	if ent, ok := f.tries.Get(encRootHash); ok {
		return ent.(*MTrie), nil
	}

	// otherwise try to load from disk
	trie, err := f.LoadTrie(filepath.Join(f.dir, encRootHash))
	if err != nil {
		return nil, fmt.Errorf("trie with the given rootHash [%v] not found: %w", hex.EncodeToString(rootHash), err)
	}

	return trie, nil
}

// AddTrie adds a trie to the forest
func (f *MForest) AddTrie(trie *MTrie) error {
	// TODO check if not exist
	encoded := hex.EncodeToString(trie.rootHash)
	f.tries.Add(encoded, trie)
	return nil
}

// GetEmptyRootHash returns the rootHash of empty forest
func (f *MForest) GetEmptyRootHash() []byte {
	newRoot := newNode(f.maxHeight - 1)
	rootHash := f.ComputeNodeHash(newRoot, false)
	return rootHash
}

// Read reads values for an slice of keys and returns values and error (if any)
func (f *MForest) Read(keys [][]byte, rootHash []byte) ([][]byte, error) {

	// no key no change
	if len(keys) == 0 {
		return [][]byte{}, nil
	}

	// lookup the trie by rootHash
	trie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
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
	// keys not found
	if head == nil {
		res := make([][]byte, 0, len(keys))
		for _ = range keys {
			res = append(res, []byte{})
		}
		return res, nil
	}
	// reached a leaf node
	if head.key != nil {
		res := make([][]byte, 0)
		for _, k := range keys {
			if bytes.Equal(head.key, k) {
				res = append(res, head.value)
			} else {
				res = append(res, []byte{})
			}
		}
		return res, nil
	}

	lkeys, rkeys, err := SplitSortedKeys(keys, f.maxHeight-head.height-1)
	if err != nil {
		return nil, fmt.Errorf("can't read due to split key error: %w", err)
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

	// no key no change
	if len(keys) == 0 {
		return rootHash, nil
	}

	// sort keys and deduplicate keys (we only consider the last occurrence, and ignore the rest)
	sortedKeys := make([][]byte, 0)
	valueMap := make(map[string][]byte)
	for i, key := range keys {
		// check key sizes
		if len(key) != f.keyByteSize {
			return nil, fmt.Errorf("key size doesn't match the trie height: %x", key)
		}
		// check if doesn't exist
		if _, ok := valueMap[hex.EncodeToString(key)]; !ok {
			//do something here
			sortedKeys = append(sortedKeys, key)
			valueMap[hex.EncodeToString(key)] = values[i]
		} else {
			valueMap[hex.EncodeToString(key)] = values[i]
		}
	}

	// TODO we might be able to remove this
	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	sortedValues := make([][]byte, 0, len(sortedKeys))
	for _, key := range sortedKeys {
		sortedValues = append(sortedValues, valueMap[hex.EncodeToString(key)])
	}

	trie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	newTrie := NewMTrie(f.maxHeight)
	newTrie.parentRootHash = f.GetNodeHash(trie.root)
	newTrie.number = trie.number + 1

	err = f.update(newTrie, trie.root, newTrie.root, sortedKeys, sortedValues)
	if err != nil {
		return nil, err
	}

	f.PopulateNodeHashValues(newTrie.root)
	newRootHash := f.GetNodeHash(newTrie.root)
	newTrie.rootHash = newRootHash
	err = f.AddTrie(newTrie)
	if err != nil {
		return nil, err
	}
	return newRootHash, nil
}

func (f *MForest) update(trie *MTrie, parent *node, head *node, keys [][]byte, values [][]byte) error {
	// parent has a key for this node (add key and insert)
	if parent.key != nil {
		alreadyExist := false
		// deduplicate
		for _, k := range keys {
			if bytes.Equal(k, parent.key) {
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
		// ????
		head.hashValue = f.GetNodeHash(head)
		return nil
	}

	// Split the keys and values array so we can update the trie in parallel
	lkeys, lvalues, rkeys, rvalues, err := SplitKeyValues(keys, values, f.maxHeight-head.height-1)
	if err != nil {
		return fmt.Errorf("error spliting key values: %w", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	var lupdate, rupdate *node
	var err1, err2 error
	go func() {
		defer wg.Done()
		// no change needed on the left side,
		if len(lkeys) == 0 {
			// reuse the node from previous trie
			lupdate = parent.lChild
		} else {
			newN := newNode(parent.height - 1)
			if parent.lChild != nil {
				err1 = f.update(trie, parent.lChild, newN, lkeys, lvalues)
			} else {
				err1 = f.update(trie, newNode(parent.height-1), newN, lkeys, lvalues)
			}
			f.PopulateNodeHashValues(newN)
			lupdate = newN
		}
	}()
	go func() {
		defer wg.Done()
		// no change needed on right side
		if len(rkeys) == 0 {
			// reuse the node from previous trie
			rupdate = parent.rChild
		} else {
			newN := newNode(head.height - 1)
			if parent.rChild != nil {
				err2 = f.update(trie, parent.rChild, newN, rkeys, rvalues)
			} else {
				err2 = f.update(trie, newNode(head.height-1), newN, rkeys, rvalues)
			}
			f.PopulateNodeHashValues(newN)
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

	// no key, empty batchproof
	if len(keys) == 0 {
		return NewBatchProof(), nil
	}

	// look up for non exisitng keys
	notFoundKeys := make([][]byte, 0)
	notFoundValues := make([][]byte, 0)
	retValues, err := f.Read(keys, rootHash)
	if err != nil {
		return nil, err
	}

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

			// add it only once
			if len(retValues[i]) == 0 {
				notFoundKeys = append(notFoundKeys, key)
				notFoundValues = append(notFoundValues, []byte{})
			}
		} else {
			// handles duplicated keys
			keyOrgIndex[hex.EncodeToString(key)] = append(keyOrgIndex[hex.EncodeToString(key)], i)
		}

	}

	trie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	// if we have to insert empty values
	if len(notFoundKeys) > 0 {
		newTrie := NewMTrie(f.maxHeight)
		newRoot := newTrie.root

		sort.Slice(notFoundKeys, func(i, j int) bool {
			return bytes.Compare(notFoundKeys[i], notFoundKeys[j]) < 0
		})

		err = f.update(newTrie, trie.root, newRoot, notFoundKeys, notFoundValues)
		if err != nil {
			return nil, err
		}

		// rootHash shouldn't change
		if !bytes.Equal(f.GetNodeHash(newRoot), rootHash) {
			return nil, errors.New("root hash has changed during the operation")
		}
		trie = newTrie
	}

	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	bp := NewBatchProofWithEmptyProofs(len(sortedKeys))

	for _, p := range bp.Proofs {
		p.flags = make([]byte, f.keyByteSize)
		p.inclusion = false
	}

	err = f.proofs(trie.root, sortedKeys, bp.Proofs)
	if err != nil {
		return nil, err
	}

	// reconstruct the proofs in the same key order that called the method
	retbp := NewBatchProofWithEmptyProofs(len(keys))
	for i, k := range sortedKeys {
		for _, j := range keyOrgIndex[hex.EncodeToString(k)] {
			retbp.Proofs[j] = bp.Proofs[i]
		}
	}

	return retbp, nil
}

func (f *MForest) proofs(head *node, keys [][]byte, proofs []*Proof) error {
	// we've reached the end of a trie
	// and key is not found (noninclusion proof)
	if head == nil {
		return nil
	}

	// we've reached a leaf that has a key
	if head.key != nil {
		// value matches (inclusion proof)
		if bytes.Equal(head.key, keys[0]) {
			proofs[0].inclusion = true
		}
		return nil
	}

	// increment steps for all the proofs
	for _, p := range proofs {
		p.steps++
	}
	// split keys based on the value of i-th bit (i = trie height - node height)
	lkeys, lproofs, rkeys, rproofs, err := SplitKeyProofs(keys, proofs, f.maxHeight-head.height-1)
	if err != nil {
		return fmt.Errorf("proof generation failed, split key error: %w", err)
	}

	if len(lkeys) > 0 {
		if head.rChild != nil {
			nodeHash := f.GetNodeHash(head.rChild)
			isDef := bytes.Equal(nodeHash, GetDefaultHashForHeight(head.rChild.height))
			for _, p := range lproofs {
				// we skip default values
				if !isDef {
					err := SetBit(p.flags, f.maxHeight-head.height-1)
					if err != nil {
						return err
					}
					p.values = append(p.values, nodeHash)
				}
			}
		}
		err := f.proofs(head.lChild, lkeys, lproofs)
		if err != nil {
			return err
		}
	}

	if len(rkeys) > 0 {
		if head.lChild != nil {
			nodeHash := f.GetNodeHash(head.lChild)
			isDef := bytes.Equal(nodeHash, GetDefaultHashForHeight(head.lChild.height))
			for _, p := range rproofs {
				// we skip default values
				if !isDef {
					err := SetBit(p.flags, f.maxHeight-head.height-1)
					if err != nil {
						return err
					}
					p.values = append(p.values, nodeHash)
				}
			}
		}
		err := f.proofs(head.rChild, rkeys, rproofs)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func (f *MForest) GetCompactValue(n *node) []byte {
	return ComputeCompactValue(n.key, n.value, n.height, f.maxHeight)
}

// GetNodeHash computes the hashValue for the given node
func (f *MForest) GetNodeHash(n *node) []byte {
	if n.hashValue != nil {
		return n.hashValue
	}
	return f.ComputeNodeHash(n, false)
}

// ComputeNodeHash computes the hashValue for the given node
// if forced it set it won't trust hash values of children and
// recomputes it.
func (f *MForest) ComputeNodeHash(n *node, forced bool) []byte {
	// leaf node (this shouldn't happen)
	if n.lChild == nil && n.rChild == nil {
		if len(n.value) > 0 {
			return f.GetCompactValue(n)
		}
		return GetDefaultHashForHeight(n.height)
	}
	// otherwise compute
	h1 := GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		if forced {
			h1 = f.ComputeNodeHash(n.lChild, forced)
		} else {
			h1 = f.GetNodeHash(n.lChild)
		}
	}
	h2 := GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		if forced {
			h2 = f.ComputeNodeHash(n.rChild, forced)
		} else {
			h2 = f.GetNodeHash(n.rChild)
		}
	}
	return HashInterNode(h1, h2)
}

// PopulateNodeHashValues recursively update nodes with
// the hash values for intermediate nodes (leafs already has values after update)
// we only use this function to speed up proof generation,
// for less memory usage we can skip this function
func (f *MForest) PopulateNodeHashValues(n *node) []byte {
	if n.hashValue != nil {
		return n.hashValue
	}

	// otherwise compute
	h1 := GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = f.PopulateNodeHashValues(n.lChild)
	}
	h2 := GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = f.PopulateNodeHashValues(n.rChild)
	}
	n.hashValue = HashInterNode(h1, h2)

	return n.hashValue
}

// StoreTrie stores a trie on disk
func (f *MForest) StoreTrie(rootHash []byte, path string) error {
	trie, err := f.GetTrie(rootHash)
	if err != nil {
		return err
	}
	return trie.Store(path)
}

// LoadTrie loads a trie from the disk
func (f *MForest) LoadTrie(path string) (*MTrie, error) {
	trie := NewMTrie(f.maxHeight)
	err := trie.Load(path)
	if err != nil {
		return nil, err
	}
	f.PopulateNodeHashValues(trie.root)
	if !bytes.Equal(trie.rootHash, f.GetNodeHash(trie.root)) {
		return nil, errors.New("error loading a trie, rootHash doesn't match")
	}
	err = f.AddTrie(trie)
	if err != nil {
		return nil, err
	}

	return trie, nil
}
