package mtrie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	lru "github.com/hashicorp/golang-lru"

	"github.com/dapperlabs/flow-go/module"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/node"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/utils/io"
)

// MForest is an in memory forest (collection of tries)
type MForest struct {
	tries         *lru.Cache // TODO: add godoc about meaning of LRU chache
	dir           string
	cacheSize     int
	maxHeight     int // height of the tree
	keyByteSize   int // acceptable number of bytes for key
	onTreeEvicted func(tree *trie.MTrie) error
	metrics       module.LedgerMetrics
}

// NewMForest returns a new instance of memory forest
func NewMForest(maxHeight int, trieStorageDir string, trieCacheSize int, metrics module.LedgerMetrics, onTreeEvicted func(tree *trie.MTrie) error) (*MForest, error) {

	var cache *lru.Cache
	var err error

	if onTreeEvicted != nil {
		cache, err = lru.NewWithEvict(trieCacheSize, func(key interface{}, value interface{}) {
			trie, ok := value.(*trie.MTrie)
			if !ok {
				panic(fmt.Sprintf("cache contains item of type %T", value))
			}
			//TODO Log error
			_ = onTreeEvicted(trie)
		})
	} else {
		cache, err = lru.New(trieCacheSize)
	}

	if err != nil {
		return nil, fmt.Errorf("cannot create forest cache: %w", err)
	}

	forest := &MForest{tries: cache,
		maxHeight:     maxHeight,
		dir:           trieStorageDir,
		cacheSize:     trieCacheSize,
		keyByteSize:   (maxHeight - 1) / 8,
		onTreeEvicted: onTreeEvicted,
		metrics:       metrics,
	}

	// add empty roothash
	emptyTrie, err := trie.NewEmptyMTrie(maxHeight, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("constructing empty trie for forest failed: %w", err)
	}

	err = forest.AddTrie(emptyTrie)
	if err != nil {
		return nil, fmt.Errorf("adding empty trie to forest failed: %w", err)
	}
	return forest, nil
}

// getTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *MForest) GetTrie(rootHash []byte) (*trie.MTrie, error) {
	encRootHash := hex.EncodeToString(rootHash)
	// if in the cache

	if ent, ok := f.tries.Get(encRootHash); ok {
		return ent.(*trie.MTrie), nil
	}

	// otherwise try to load from disk
	trie, err := f.LoadTrie(filepath.Join(f.dir, encRootHash))
	if err != nil {
		return nil, fmt.Errorf("trie with the given rootHash [%v] not found: %w", hex.EncodeToString(rootHash), err)
	}

	return trie, nil
}

// GetCachedRootHashes returns list of currently cached tree root hashes
func (f *MForest) GetTries() ([]*trie.MTrie, error) {
	// ToDo needs concurrency safety
	keys := f.tries.Keys()
	tries := make([]*trie.MTrie, 0, len(keys))
	for key := range keys {
		t, ok := f.tries.Get(key)
		if !ok {
			return nil, errors.New("concurrent MForest modification")
		}
		tries = append(tries, t.(*trie.MTrie))
	}
	return tries, nil
}

// addTrie adds a trie to the forest
func (f *MForest) AddTries(newTries []*trie.MTrie) error {
	for _, t := range newTries {
		err := f.AddTrie(t)
		if err != nil {
			fmt.Errorf("adding tries to forrest failed: %w", err)
		}
	}
	return nil
}

// addTrie adds a trie to the forest
func (f *MForest) AddTrie(newTrie *trie.MTrie) error {
	if newTrie == nil {
		return nil
	}
	expectedHeight := f.maxHeight - 1
	if newTrie.Height() != expectedHeight {
		return fmt.Errorf("forest holds tries of uniform height %d, but new trie has height %d", expectedHeight, newTrie.Height())
	}

	// TODO: check Thread safety
	hashString := newTrie.StringRootHash()
	if storedTrie, found := f.tries.Get(hashString); found {
		foo := storedTrie.(*trie.MTrie)
		if foo.Equals(newTrie) {
			return nil
		}
		return fmt.Errorf("forest already contains a tree with same root hash but other properties")
	}
	f.tries.Add(hashString, newTrie)
	f.metrics.ForestNumberOfTrees(uint64(f.tries.Len()))

	return nil
}

// RemoveTrie removes a trie to the forest
func (f *MForest) RemoveTrie(rootHash []byte) {
	// TODO remove from the file as well
	encRootHash := hex.EncodeToString(rootHash)
	f.tries.Remove(encRootHash)

	f.metrics.ForestNumberOfTrees(uint64(f.tries.Len()))
}

// GetEmptyRootHash returns the rootHash of empty Trie
func (f *MForest) GetEmptyRootHash() []byte {
	return node.NewEmptyTreeRoot(f.maxHeight - 1).Hash()
}

// Read reads values for an slice of keys and returns values and error (if any)
func (f *MForest) Read(rootHash []byte, keys [][]byte) ([][]byte, error) {

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

	values, err := trie.UnsafeRead(sortedKeys)

	if err != nil {
		return nil, err
	}

	totalValuesSize := 0

	// reconstruct the values in the same key order that called the method
	orderedValues := make([][]byte, len(keys))
	for i, k := range sortedKeys {
		for _, j := range keyOrgIndex[hex.EncodeToString(k)] {
			orderedValues[j] = values[i]
			totalValuesSize += len(values[i])
		}
	}

	f.metrics.ReadValuesSize(uint64(totalValuesSize))

	return orderedValues, nil
}

// Update updates the Values for the registers and returns rootHash and error (if any)
func (f *MForest) Update(rootHash []byte, keys [][]byte, values [][]byte) (*trie.MTrie, error) {
	parentTrie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 { // no key no change
		return parentTrie, nil
	}

	// sort keys and deduplicate keys (we only consider the last occurrence, and ignore the rest)
	sortedKeys := make([][]byte, 0)
	valueMap := make(map[string][]byte)
	totalValuesSize := 0
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
		totalValuesSize += len(values[i])
	}

	f.metrics.UpdateValuesSize(uint64(totalValuesSize))

	// TODO we might be able to remove this
	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	sortedValues := make([][]byte, 0, len(sortedKeys))
	for _, key := range sortedKeys {
		sortedValues = append(sortedValues, valueMap[hex.EncodeToString(key)])
	}

	newTrie, err := trie.NewTrieWithUpdatedRegisters(parentTrie, sortedKeys, sortedValues)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	err = f.AddTrie(newTrie)
	if err != nil {
		return nil, fmt.Errorf("adding updated trie to forest failed: %w", err)
	}
	// go func() {
	// 	_ = newTrie.Store(filepath.Join(f.dir, hex.EncodeToString(newRootHash)))
	// }()
	return newTrie, nil
}

// Proofs returns a batch proof for the given keys
func (f *MForest) Proofs(rootHash []byte, keys [][]byte) (*proof.BatchProof, error) {

	// no key, empty batchproof
	if len(keys) == 0 {
		return proof.NewBatchProof(), nil
	}

	// look up for non exisitng keys
	notFoundKeys := make([][]byte, 0)
	notFoundValues := make([][]byte, 0)
	retValues, err := f.Read(rootHash, keys)
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
		// only collect duplicated keys once
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

	stateTrie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	// if we have to insert empty values
	if len(notFoundKeys) > 0 {
		sort.Slice(notFoundKeys, func(i, j int) bool {
			return bytes.Compare(notFoundKeys[i], notFoundKeys[j]) < 0
		})

		newTrie, err := trie.NewTrieWithUpdatedRegisters(stateTrie, notFoundKeys, notFoundValues)
		if err != nil {
			return nil, err
		}

		// rootHash shouldn't change
		if !bytes.Equal(newTrie.RootHash(), rootHash) {
			return nil, errors.New("root hash has changed during the operation")
		}
		stateTrie = newTrie
	}

	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	bp := proof.NewBatchProofWithEmptyProofs(len(sortedKeys))

	for _, p := range bp.Proofs {
		p.Flags = make([]byte, f.keyByteSize)
		p.Inclusion = false
	}

	err = stateTrie.UnsafeProofs(sortedKeys, bp.Proofs)
	if err != nil {
		return nil, err
	}

	// reconstruct the proofs in the same key order that called the method
	retbp := proof.NewBatchProofWithEmptyProofs(len(keys))
	for i, k := range sortedKeys {
		for _, j := range keyOrgIndex[hex.EncodeToString(k)] {
			retbp.Proofs[j] = bp.Proofs[i]
		}
	}

	return retbp, nil
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
func (f *MForest) LoadTrie(path string) (*trie.MTrie, error) {
	newTrie, err := trie.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading trie from '%s' failed: %w", path, err)
	}
	err = f.AddTrie(newTrie)
	if err != nil {
		return nil, fmt.Errorf("adding loaded trie from '%s' to forest failed: %w", path, err)
	}

	return newTrie, nil
}

// Size returns the number of active tries in this store
func (f *MForest) Size() int {
	return f.tries.Len()
}

// DiskSize returns the disk size of the directory used by the forest (in bytes)
func (f *MForest) DiskSize() (int64, error) {
	return io.DirSize(f.dir)
}
