package mtrie

import (
	"encoding/hex"
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/io"
)

// Forest holds several in-memory tries. As Forest is a storage-abstraction layer,
// we assume that all registers are addressed via paths of pre-defined uniform length.
//
// Forest has a limit, the forestCapacity, on the number of tries it is able to store.
// If more tries are added than the capacity, the Least Recently Used trie is
// removed (evicted) from the Forest. THIS IS A ROUGH HEURISTIC as it might evict
// tries that are still needed. In fully matured Flow, we will have an
// explicit eviction policy.
//
// TODO: Storage Eviction Policy for Forest
//       For the execution node: we only evict on sealing a result.
type Forest struct {
	// tries stores all MTries in the forest. It is NOT a CACHE in the conventional sense:
	// there is no mechanism to load a trie from disk in case of a cache miss. Missing a
	// needed trie in the forest might cause a fatal application logic error.
	tries          *lru.Cache
	dir            string
	forestCapacity int
	onTreeEvicted  func(tree *trie.MTrie) error
	metrics        module.LedgerMetrics
}

// NewForest returns a new instance of memory forest.
//
// CAUTION on forestCapacity: the specified capacity MUST be SUFFICIENT to store all needed MTries in the forest.
// If more tries are added than the capacity, the Least Recently Used trie is removed (evicted) from the Forest.
// THIS IS A ROUGH HEURISTIC as it might evict tries that are still needed.
// Make sure you chose a sufficiently large forestCapacity, such that, when reaching the capacity, the
// Least Recently Used trie will never be needed again.
func NewForest(trieStorageDir string, forestCapacity int, metrics module.LedgerMetrics, onTreeEvicted func(tree *trie.MTrie) error) (*Forest, error) {
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

	forest := &Forest{tries: cache,
		dir:            trieStorageDir,
		forestCapacity: forestCapacity,
		onTreeEvicted:  onTreeEvicted,
		metrics:        metrics,
	}

	// add empty roothash
	emptyTrie := trie.NewEmptyMTrie()

	err = forest.AddTrie(emptyTrie)
	if err != nil {
		return nil, fmt.Errorf("adding empty trie to forest failed: %w", err)
	}
	return forest, nil
}

// Read reads values for an slice of paths and returns values and error (if any)
// TODO: can be optimized further if we don't care about changing the order of the input r.Paths
func (f *Forest) Read(r *ledger.TrieRead) ([]*ledger.Payload, error) {

	if len(r.Paths) == 0 {
		return []*ledger.Payload{}, nil
	}

	// lookup the trie by rootHash
	trie, err := f.GetTrie(r.RootHash)
	if err != nil {
		return nil, err
	}

	// deduplicate keys
	deduplicatedPaths := make([]ledger.Path, 0)
	pathOrgIndex := make(map[string][]int)
	for i, path := range r.Paths {
		// only collect duplicated keys once
		if _, ok := pathOrgIndex[string(path[:])]; !ok { // deduplication here is optional
			deduplicatedPaths = append(deduplicatedPaths, path)
		}
		// append the index
		pathOrgIndex[string(path[:])] = append(pathOrgIndex[string(path[:])], i)
	}

	payloads := trie.UnsafeRead(deduplicatedPaths) // this sorts deduplicatedPaths

	totalPayloadSize := 0

	// reconstruct the payloads in the same key order that called the method
	orderedPayloads := make([]*ledger.Payload, len(r.Paths))
	for i, p := range deduplicatedPaths {
		for _, j := range pathOrgIndex[string(p[:])] {
			orderedPayloads[j] = payloads[i].DeepCopy()
			totalPayloadSize += payloads[i].Size()
		}
	}

	// TODO rename the metrics
	f.metrics.ReadValuesSize(uint64(totalPayloadSize))

	return orderedPayloads, nil
}

// Update updates the Values for the registers and returns rootHash and error (if any).
// In case there are multiple updates to the same register, Update will persist the latest
// written value.
func (f *Forest) Update(u *ledger.TrieUpdate) (ledger.RootHash, error) {
	emptyHash := ledger.RootHash(hash.EmptyHash)

	parentTrie, err := f.GetTrie(u.RootHash)
	if err != nil {
		return emptyHash, err
	}

	if len(u.Paths) == 0 { // no key no change
		return u.RootHash, nil
	}

	// sort and deduplicate paths (we only consider the last occurrence, and ignore the rest)
	deduplicatedPaths := make([]ledger.Path, 0)
	payloadMap := make(map[string]ledger.Payload)
	totalPayloadSize := 0
	for i, path := range u.Paths {
		// check if doesn't exist
		if _, ok := payloadMap[string(path[:])]; !ok {
			deduplicatedPaths = append(deduplicatedPaths, path)
		}
		payloadMap[string(path[:])] = *u.Payloads[i]
		totalPayloadSize += u.Payloads[i].Size()
	}

	// TODO rename metrics names
	f.metrics.UpdateValuesSize(uint64(totalPayloadSize))

	payloads := make([]ledger.Payload, 0, len(deduplicatedPaths))
	for _, path := range deduplicatedPaths {
		payloads = append(payloads, payloadMap[string(path[:])])
	}

	newTrie, err := trie.NewTrieWithUpdatedRegisters(parentTrie, deduplicatedPaths, payloads)
	if err != nil {
		return emptyHash, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	f.metrics.LatestTrieRegCount(newTrie.AllocatedRegCount())
	f.metrics.LatestTrieRegCountDiff(newTrie.AllocatedRegCount() - parentTrie.AllocatedRegCount())
	f.metrics.LatestTrieMaxDepth(uint64(newTrie.MaxDepth()))
	f.metrics.LatestTrieMaxDepthDiff(uint64(newTrie.MaxDepth() - parentTrie.MaxDepth()))

	err = f.AddTrie(newTrie)
	if err != nil {
		return emptyHash, fmt.Errorf("adding updated trie to forest failed: %w", err)
	}

	return ledger.RootHash(newTrie.RootHash()), nil
}

// Proofs returns a batch proof for the given paths
func (f *Forest) Proofs(r *ledger.TrieRead) (*ledger.TrieBatchProof, error) {

	// no path, empty batchproof
	if len(r.Paths) == 0 {
		return ledger.NewTrieBatchProof(), nil
	}

	// look up for non existing paths
	retPayloads, err := f.Read(r)
	if err != nil {
		return nil, err
	}

	deduplicatedPaths := make([]ledger.Path, 0)
	notFoundPaths := make([]ledger.Path, 0)
	notFoundPayloads := make([]ledger.Payload, 0)
	pathOrgIndex := make(map[string][]int)
	for i, path := range r.Paths {
		// only collect duplicated keys once
		if _, ok := pathOrgIndex[string(path[:])]; !ok {
			deduplicatedPaths = append(deduplicatedPaths, path)
			pathOrgIndex[string(path[:])] = []int{i}

			// add it only once if is empty
			if retPayloads[i].IsEmpty() {
				notFoundPaths = append(notFoundPaths, path)
				notFoundPayloads = append(notFoundPayloads, *ledger.EmptyPayload())
			}
		} else {
			// handles duplicated keys
			pathOrgIndex[string(path[:])] = append(pathOrgIndex[string(path[:])], i)
		}
	}

	stateTrie, err := f.GetTrie(r.RootHash)
	if err != nil {
		return nil, err
	}

	// if we have to insert empty values
	if len(notFoundPaths) > 0 {

		newTrie, err := trie.NewTrieWithUpdatedRegisters(stateTrie, notFoundPaths, notFoundPayloads)
		if err != nil {
			return nil, err
		}

		// rootHash shouldn't change
		if newTrie.RootHash() != r.RootHash {
			return nil, fmt.Errorf("root hash has changed during the operation %s, %s", newTrie.RootHash(), r.RootHash)
		}
		stateTrie = newTrie
	}

	bp := ledger.NewTrieBatchProofWithEmptyProofs(len(deduplicatedPaths))

	for _, p := range bp.Proofs {
		p.Flags = make([]byte, ledger.PathLen)
		p.Inclusion = false
	}

	stateTrie.UnsafeProofs(deduplicatedPaths, bp.Proofs)

	// reconstruct the proofs in the same key order that called the method
	retbp := ledger.NewTrieBatchProofWithEmptyProofs(len(r.Paths))
	for i, p := range deduplicatedPaths {
		for _, j := range pathOrgIndex[string(p[:])] {
			retbp.Proofs[j] = bp.Proofs[i]
		}
	}

	return retbp, nil
}

// GetTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *Forest) GetTrie(rootHash ledger.RootHash) (*trie.MTrie, error) {
	encRootHash := hex.EncodeToString(rootHash[:])

	// if in memory
	if ent, ok := f.tries.Get(encRootHash); ok {
		return ent.(*trie.MTrie), nil
	}

	return nil, fmt.Errorf("trie with the given rootHash [%v] not found", encRootHash)
}

// GetTries returns list of currently cached tree root hashes
func (f *Forest) GetTries() ([]*trie.MTrie, error) {
	// ToDo needs concurrency safety
	keys := f.tries.Keys()
	tries := make([]*trie.MTrie, 0, len(keys))
	for _, key := range keys {
		t, ok := f.tries.Get(key)
		if !ok {
			return nil, errors.New("concurrent Forest modification")
		}
		tries = append(tries, t.(*trie.MTrie))
	}
	return tries, nil
}

// AddTries adds a trie to the forest
func (f *Forest) AddTries(newTries []*trie.MTrie) error {
	for _, t := range newTries {
		err := f.AddTrie(t)
		if err != nil {
			return fmt.Errorf("adding tries to forest failed: %w", err)
		}
	}
	return nil
}

// AddTrie adds a trie to the forest
func (f *Forest) AddTrie(newTrie *trie.MTrie) error {
	if newTrie == nil {
		return nil
	}

	// TODO: check Thread safety
	// TODO what is this string root hash
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
func (f *Forest) RemoveTrie(rootHash ledger.RootHash) {
	// TODO remove from the file as well
	encRootHash := hex.EncodeToString(rootHash[:])
	f.tries.Remove(encRootHash)
	f.metrics.ForestNumberOfTrees(uint64(f.tries.Len()))
}

// GetEmptyRootHash returns the rootHash of empty Trie
func (f *Forest) GetEmptyRootHash() ledger.RootHash {
	return trie.EmptyTrieRootHash()
}

// Size returns the number of active tries in this store
func (f *Forest) Size() int {
	return f.tries.Len()
}

// DiskSize returns the disk size of the directory used by the forest (in bytes)
func (f *Forest) DiskSize() (uint64, error) {
	return io.DirSize(f.dir)
}
