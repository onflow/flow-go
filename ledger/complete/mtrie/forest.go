package mtrie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module"
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
	forestCapacity int
	onTreeEvicted  func(tree *trie.MTrie) error
	pathByteSize   int // length [bytes] of register path
	metrics        module.LedgerMetrics
}

// NewForest returns a new instance of memory forest.
//
// CAUTION on forestCapacity: the specified capacity MUST be SUFFICIENT to store all needed MTries in the forest.
// If more tries are added than the capacity, the Least Recently Used trie is removed (evicted) from the Forest.
// THIS IS A ROUGH HEURISTIC as it might evict tries that are still needed.
// Make sure you chose a sufficiently large forestCapacity, such that, when reaching the capacity, the
// Least Recently Used trie will never be needed again.
func NewForest(pathByteSize int, forestCapacity int, metrics module.LedgerMetrics, onTreeEvicted func(tree *trie.MTrie) error) (*Forest, error) {
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

	// init Forest and add an empty trie
	if pathByteSize < 1 {
		return nil, errors.New("trie's path size [in bytes] must be positive")
	}
	forest := &Forest{tries: cache,
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

	// deduplicate keys:
	// Generally, we expect the VM to deduplicate reads and writes. Hence, the following is a pre-caution.
	// TODO: We could take out the following de-duplication logic
	//       Which increases the cost for duplicates but reduces read complexity without duplicates.
	deduplicatedPaths := make([]ledger.Path, 0, len(r.Paths))
	pathOrgIndex := make(map[string][]int)
	for i, path := range r.Paths {
		// check key sizes
		if len(path) != f.pathByteSize {
			return nil, fmt.Errorf("path size doesn't match the trie height: %x", len(path))
		}
		// only collect duplicated keys once
		indices, ok := pathOrgIndex[string(path)]
		if !ok { // deduplication here is optional
			deduplicatedPaths = append(deduplicatedPaths, path)
		}
		// append the index
		pathOrgIndex[string(path)] = append(indices, i)
	}

	payloads := trie.UnsafeRead(deduplicatedPaths) // this sorts deduplicatedPaths IN-PLACE

	// reconstruct the payloads in the same key order that called the method
	orderedPayloads := make([]*ledger.Payload, len(r.Paths))
	totalPayloadSize := 0
	for i, p := range deduplicatedPaths {
		payload := payloads[i]
		indices := pathOrgIndex[string(p)]
		for _, j := range indices {
			orderedPayloads[j] = payload.DeepCopy()
		}
		totalPayloadSize += len(indices) * payload.Size()
	}
	// TODO rename the metrics
	f.metrics.ReadValuesSize(uint64(totalPayloadSize))

	return orderedPayloads, nil
}

// Update updates the Values for the registers and returns rootHash and error (if any).
// In case there are multiple updates to the same register, Update will persist the latest
// written value.
func (f *Forest) Update(u *ledger.TrieUpdate) (ledger.RootHash, error) {

	parentTrie, err := f.GetTrie(u.RootHash)
	if err != nil {
		return nil, err
	}

	if len(u.Paths) == 0 { // no key no change
		return u.RootHash, nil
	}

	// Deduplicate writes to the same register: we only retain the value of the last write
	// Generally, we expect the VM to deduplicate reads and writes.
	deduplicatedPaths := make([]ledger.Path, 0, len(u.Paths))
	deduplicatedPayloads := make([]ledger.Payload, 0, len(u.Paths))
	payloadMap := make(map[string]int) // index into deduplicatedPaths, deduplicatedPayloads with register update
	totalPayloadSize := 0
	for i, path := range u.Paths {
		// check path sizes
		if len(path) != f.pathByteSize {
			return nil, fmt.Errorf("path size doesn't match the trie height: %x", len(path))
		}
		payload := u.Payloads[i]
		// check if we already have encountered an update for the respective register
		if idx, ok := payloadMap[string(path)]; ok {
			oldPayload := deduplicatedPayloads[idx]
			deduplicatedPayloads[idx] = *payload
			totalPayloadSize += -oldPayload.Size() + payload.Size()
		} else {
			payloadMap[string(path)] = len(deduplicatedPaths)
			deduplicatedPaths = append(deduplicatedPaths, path)
			deduplicatedPayloads = append(deduplicatedPayloads, *u.Payloads[i])
			totalPayloadSize += payload.Size()
		}
	}

	// TODO rename metrics names
	f.metrics.UpdateValuesSize(uint64(totalPayloadSize))

	newTrie, err := trie.NewTrieWithUpdatedRegisters(parentTrie, deduplicatedPaths, deduplicatedPayloads)
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

	return ledger.RootHash(newTrie.RootHash()), nil
}

// Proofs returns a batch proof for the given paths.
//
// Proofs must provide proofs in an order not correlated to the path order in the query.
// In the current implementation, input paths in the TrieRead `r` are sorted in an ascendent order,
// The output proofs are provided following the order of the sorted paths.
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

	notFoundPaths := make([]ledger.Path, 0)
	notFoundPayloads := make([]ledger.Payload, 0)
	for i, path := range r.Paths {
		// check key sizes
		if len(path) != f.pathByteSize {
			return nil, fmt.Errorf("path size doesn't match the trie height: %x", len(path))
		}

		// add if empty
		if retPayloads[i].IsEmpty() {
			notFoundPaths = append(notFoundPaths, path)
			notFoundPayloads = append(notFoundPayloads, *ledger.EmptyPayload())
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
		if !bytes.Equal(newTrie.RootHash(), r.RootHash) {
			return nil, errors.New("root hash has changed during the operation")
		}
		stateTrie = newTrie
	}

	bp := stateTrie.UnsafeProofs(r.Paths, f.pathByteSize)
	return bp, nil
}

// PathLength return the length [in bytes] the trie operates with.
// Concurrency safe (as Tries are immutable structures by convention)
func (f *Forest) PathLength() int { return f.pathByteSize }

// GetTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *Forest) GetTrie(rootHash ledger.RootHash) (*trie.MTrie, error) {
	encRootHash := hex.EncodeToString(rootHash)

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
	if newTrie.PathLength() != f.pathByteSize {
		return fmt.Errorf("forest has path length %d, but new trie has path length %d", f.pathByteSize, newTrie.PathLength())
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
func (f *Forest) RemoveTrie(rootHash []byte) {
	// TODO remove from the file as well
	encRootHash := hex.EncodeToString(rootHash)
	f.tries.Remove(encRootHash)
	f.metrics.ForestNumberOfTrees(uint64(f.tries.Len()))
}

// GetEmptyRootHash returns the rootHash of empty Trie
func (f *Forest) GetEmptyRootHash() []byte {
	return trie.EmptyTrieRootHash(f.pathByteSize)
}

// Size returns the number of active tries in this store
func (f *Forest) Size() int {
	return f.tries.Len()
}
