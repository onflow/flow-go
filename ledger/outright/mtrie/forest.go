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
	"github.com/dapperlabs/flow-go/ledger/outright/mtrie/proof"
	"github.com/dapperlabs/flow-go/ledger/outright/mtrie/trie"
	"github.com/dapperlabs/flow-go/utils/io"
)

// MForest is a a forest of in-memory tries. As MForest is a storage-abstraction layer,
// we assume that all registers are addressed via paths of pre-defined uniform length.
//
// MForest has a limit, the forestCapacity, on the number of tries it is able to store.
// If more tries are added than the capacity, the Least Recently Used trie is
// removed (evicted) from the MForest. THIS IS A ROUGH HEURISTIC as it might evict
// tries that are still needed. In fully matured Flow, we will have an
// explicit eviction policy.
//
// TODO: Storage Eviction Policy for MForrest
//       For the execution node: we only evict on sealing a result.
type MForest struct {
	// tries stores all MTries in the forest. It is NOT a CACHE in the conventional sense:
	// there is no mechanism to load a trie from disk in case of a cache miss. Missing a
	// needed trie in the forest might cause a fatal application logic error.
	tries          *lru.Cache
	dir            string
	forestCapacity int
	onTreeEvicted  func(tree *trie.MTrie) error
	pathByteSize   int // length [bytes] of register path
	metrics        module.LedgerMetrics
}

// NewMForest returns a new instance of memory forest.
//
// CAUTION on forestCapacity: the specified capacity MUST be SUFFICIENT to store all needed MTries in the forest.
// If more tries are added than the capacity, the Least Recently Used trie is removed (evicted) from the MForest.
// THIS IS A ROUGH HEURISTIC as it might evict tries that are still needed.
// Make sure you chose a sufficiently large forestCapacity, such that, when reaching the capacity, the
// Least Recently Used trie will never be needed again.
func NewMForest(pathByteSize int, trieStorageDir string, forestCapacity int, metrics module.LedgerMetrics, onTreeEvicted func(tree *trie.MTrie) error) (*MForest, error) {
	// init LRU cache as a SHORTCUT for a usage-related storage eviction policy
	var cache *lru.Cache
	var err error
	if onTreeEvicted != nil {
		cache, err = lru.NewWithEvict(forestCapacity, func(key interface{}, value interface{}) {
			trie, ok := value.(*trie.MTrie)
			if !ok {
				panic(fmt.Sprintf("cache contains item of type %T", value))
			}
			//TODO Log error
			_ = onTreeEvicted(trie)
		})
	} else {
		cache, err = lru.New(forestCapacity)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot create forest cache: %w", err)
	}

	// init Forrest and add an empty trie
	if pathByteSize < 1 {
		return nil, errors.New("trie's path size [in bytes] must be positive")
	}
	forest := &MForest{tries: cache,
		dir:            trieStorageDir,
		forestCapacity: forestCapacity,
		onTreeEvicted:  onTreeEvicted,
		pathByteSize:   pathByteSize,
		metrics:        metrics,
	}

	// add empty roothash
	emptyTrie, err := trie.NewEmptyMTrie(pathByteSize)
	if err != nil {
		return nil, fmt.Errorf("constructing empty trie for forest failed: %w", err)
	}

	err = forest.AddTrie(emptyTrie)
	if err != nil {
		return nil, fmt.Errorf("adding empty trie to forest failed: %w", err)
	}
	return forest, nil
}

// PathLength return the length [in bytes] the trie operates with.
// Concurrency safe (as Tries are immutable structures by convention)
func (f *MForest) PathLength() int { return f.pathByteSize }

// GetTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *MForest) GetTrie(rootHash []byte) (*trie.MTrie, error) {
	encRootHash := hex.EncodeToString(rootHash)

	// if in memory
	if ent, ok := f.tries.Get(encRootHash); ok {
		return ent.(*trie.MTrie), nil
	}

	// otherwise try to load from disk
	trie, err := f.LoadTrie(filepath.Join(f.dir, encRootHash))
	if err != nil {
		return nil, fmt.Errorf("trie with the given rootHash [%v] not found: %w", encRootHash, err)
	}

	return trie, nil
}

// GetTries returns list of currently cached tree root hashes
func (f *MForest) GetTries() ([]*trie.MTrie, error) {
	// ToDo needs concurrency safety
	keys := f.tries.Keys()
	tries := make([]*trie.MTrie, 0, len(keys))
	for _, key := range keys {
		t, ok := f.tries.Get(key)
		if !ok {
			return nil, errors.New("concurrent MForest modification")
		}
		tries = append(tries, t.(*trie.MTrie))
	}
	return tries, nil
}

// AddTries adds a trie to the forest
func (f *MForest) AddTries(newTries []*trie.MTrie) error {
	for _, t := range newTries {
		err := f.AddTrie(t)
		if err != nil {
			return fmt.Errorf("adding tries to forrest failed: %w", err)
		}
	}
	return nil
}

// AddTrie adds a trie to the forest
func (f *MForest) AddTrie(newTrie *trie.MTrie) error {
	if newTrie == nil {
		return nil
	}
	if newTrie.PathLength() != f.pathByteSize {
		return fmt.Errorf("forest has path length %d, but new trie has path length %d", f.pathByteSize, newTrie.PathLength())
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
	return trie.EmptyTrieRootHash(f.pathByteSize)
}

// Read reads values for an slice of paths and returns values and error (if any)
func (f *MForest) Read(rootHash []byte, paths [][]byte) ([][]byte, error) {

	// no key no change
	if len(paths) == 0 {
		return [][]byte{}, nil
	}

	// lookup the trie by rootHash
	trie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	// sort paths and deduplicate keys
	sortedPaths := make([][]byte, 0)
	pathOrgIndex := make(map[string][]int)
	for i, path := range paths {
		// check key sizes
		if len(path) != f.pathByteSize {
			return nil, fmt.Errorf("path size doesn't match the trie height: %x", len(path))
		}
		// only collect duplicated keys once
		if _, ok := pathOrgIndex[string(path)]; !ok {
			sortedPaths = append(sortedPaths, path)
			pathOrgIndex[string(path)] = []int{i}
		} else {
			// handles duplicated keys
			pathOrgIndex[string(path)] = append(pathOrgIndex[string(path)], i)
		}
	}

	sort.Slice(sortedPaths, func(i, j int) bool {
		return bytes.Compare(sortedPaths[i], sortedPaths[j]) < 0
	})

	values, err := trie.UnsafeRead(sortedPaths)

	if err != nil {
		return nil, err
	}

	totalValuesSize := 0

	// reconstruct the values in the same key order that called the method
	orderedValues := make([][]byte, len(paths))
	for i, p := range sortedPaths {
		for _, j := range pathOrgIndex[string(p)] {
			orderedValues[j] = values[i]
			totalValuesSize += len(values[i])
		}
	}

	f.metrics.ReadValuesSize(uint64(totalValuesSize))

	return orderedValues, nil
}

// Update updates the Values for the registers and returns rootHash and error (if any).
// In case there are multiple updates to the same register, Update will persist the latest
// written value.
func (f *MForest) Update(rootHash []byte, paths [][]byte, keys [][]byte, values [][]byte) (*trie.MTrie, error) {
	parentTrie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 { // no key no change
		return parentTrie, nil
	}

	// sort and deduplicate paths (we only consider the last occurrence, and ignore the rest)
	sortedPaths := make([][]byte, 0)
	valueMap := make(map[string][]byte)
	keyMap := make(map[string][]byte)
	totalValuesSize := 0
	for i, path := range paths {
		// check path sizes
		if len(path) != f.pathByteSize {
			return nil, fmt.Errorf("path size doesn't match the trie height: %x", len(path))
		}
		// check if doesn't exist
		if _, ok := valueMap[string(path)]; !ok {
			sortedPaths = append(sortedPaths, path)
		}
		valueMap[string(path)] = values[i]
		keyMap[string(path)] = keys[i]
		totalValuesSize += len(values[i])
	}

	f.metrics.UpdateValuesSize(uint64(totalValuesSize))

	// TODO we might be able to remove this
	sort.Slice(sortedPaths, func(i, j int) bool {
		return bytes.Compare(sortedPaths[i], sortedPaths[j]) < 0
	})

	sortedKeys := make([][]byte, 0, len(sortedPaths))
	sortedValues := make([][]byte, 0, len(sortedPaths))
	for _, path := range sortedPaths {
		sortedValues = append(sortedValues, valueMap[string(path)])
		sortedKeys = append(sortedKeys, keyMap[string(path)])
	}

	newTrie, err := trie.NewTrieWithUpdatedRegisters(parentTrie, sortedPaths, sortedKeys, sortedValues)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	f.metrics.LatestTrieRegCount(newTrie.AllocatedRegCount())
	f.metrics.LatestTrieRegCountDiff(newTrie.AllocatedRegCount() - parentTrie.AllocatedRegCount())
	f.metrics.LatestTrieMaxDepth(uint64(newTrie.MaxDepth()))
	f.metrics.LatestTrieMaxDepthDiff(uint64(newTrie.MaxDepth() - parentTrie.MaxDepth()))

	err = f.AddTrie(newTrie)
	if err != nil {
		return nil, fmt.Errorf("adding updated trie to forest failed: %w", err)
	}
	// go func() {
	// 	_ = newTrie.Store(filepath.Join(f.dir, hex.EncodeToString(newRootHash)))
	// }()
	return newTrie, nil
}

// Proofs returns a batch proof for the given paths
func (f *MForest) Proofs(rootHash []byte, paths [][]byte) (*proof.BatchProof, error) {

	// no path, empty batchproof
	if len(paths) == 0 {
		return proof.NewBatchProof(), nil
	}

	// look up for non existing paths
	retValues, err := f.Read(rootHash, paths)
	if err != nil {
		return nil, err
	}

	sortedPaths := make([][]byte, 0)
	notFoundPaths := make([][]byte, 0)
	notFoundKeys := make([][]byte, 0)
	notFoundValues := make([][]byte, 0)
	pathOrgIndex := make(map[string][]int)
	for i, path := range paths {
		// check key sizes
		if len(path) != f.pathByteSize {
			return nil, fmt.Errorf("path size doesn't match the trie height: %x", len(path))
		}
		// only collect duplicated keys once
		if _, ok := pathOrgIndex[string(path)]; !ok {
			sortedPaths = append(sortedPaths, path)
			pathOrgIndex[string(path)] = []int{i}

			// add it only once
			if len(retValues[i]) == 0 {
				notFoundPaths = append(notFoundPaths, path)
				notFoundKeys = append(notFoundKeys, []byte{})
				notFoundValues = append(notFoundValues, []byte{})
			}
		} else {
			// handles duplicated keys
			pathOrgIndex[string(path)] = append(pathOrgIndex[string(path)], i)
		}

	}

	stateTrie, err := f.GetTrie(rootHash)
	if err != nil {
		return nil, err
	}

	// if we have to insert empty values
	if len(notFoundPaths) > 0 {

		sort.Slice(notFoundPaths, func(i, j int) bool {
			return bytes.Compare(notFoundPaths[i], notFoundPaths[j]) < 0
		})

		newTrie, err := trie.NewTrieWithUpdatedRegisters(stateTrie, notFoundPaths, notFoundKeys, notFoundValues)
		if err != nil {
			return nil, err
		}

		// rootHash shouldn't change
		if !bytes.Equal(newTrie.RootHash(), rootHash) {
			return nil, errors.New("root hash has changed during the operation")
		}
		stateTrie = newTrie
	}

	sort.Slice(sortedPaths, func(i, j int) bool {
		return bytes.Compare(sortedPaths[i], sortedPaths[j]) < 0
	})

	bp := proof.NewBatchProofWithEmptyProofs(len(sortedPaths))

	for _, p := range bp.Proofs {
		p.Flags = make([]byte, f.pathByteSize)
		p.Inclusion = false
	}

	err = stateTrie.UnsafeProofs(sortedPaths, bp.Proofs)
	if err != nil {
		return nil, err
	}

	// reconstruct the proofs in the same key order that called the method
	retbp := proof.NewBatchProofWithEmptyProofs(len(paths))
	for i, p := range sortedPaths {
		for _, j := range pathOrgIndex[string(p)] {
			retbp.Proofs[j] = bp.Proofs[i]
		}
	}

	return retbp, nil
}

// StoreTrie stores a trie on disk
func (f *MForest) StoreTrie(rootHash []byte, filepath string) error {
	trie, err := f.GetTrie(rootHash)
	if err != nil {
		return err
	}
	return trie.Store(filepath)
}

// LoadTrie loads a trie from the disk
func (f *MForest) LoadTrie(filepath string) (*trie.MTrie, error) {
	newTrie, err := trie.Load(filepath)
	if err != nil {
		return nil, fmt.Errorf("loading trie from '%s' failed: %w", filepath, err)
	}
	err = f.AddTrie(newTrie)
	if err != nil {
		return nil, fmt.Errorf("adding loaded trie from '%s' to forest failed: %w", filepath, err)
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
