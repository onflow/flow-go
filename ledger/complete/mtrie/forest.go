package mtrie

import (
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
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
	onTreeEvicted  func(tree *trie.MTrie)
	metrics        module.LedgerMetrics
}

// NewForest returns a new instance of memory forest.
//
// CAUTION on forestCapacity: the specified capacity MUST be SUFFICIENT to store all needed MTries in the forest.
// If more tries are added than the capacity, the Least Recently Used trie is removed (evicted) from the Forest.
// THIS IS A ROUGH HEURISTIC as it might evict tries that are still needed.
// Make sure you chose a sufficiently large forestCapacity, such that, when reaching the capacity, the
// Least Recently Used trie will never be needed again.
func NewForest(forestCapacity int, metrics module.LedgerMetrics, onTreeEvicted func(tree *trie.MTrie)) (*Forest, error) {
	// init LRU cache as a SHORTCUT for a usage-related storage eviction policy
	var cache *lru.Cache
	var err error
	if onTreeEvicted != nil {
		cache, err = lru.NewWithEvict(forestCapacity, func(key interface{}, value interface{}) {
			trie, ok := value.(*trie.MTrie)
			if !ok {
				panic(fmt.Sprintf("cache contains item of type %T", value))
			}
			onTreeEvicted(trie)
		})
	} else {
		cache, err = lru.New(forestCapacity)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot create forest cache: %w", err)
	}

	forest := &Forest{tries: cache,
		forestCapacity: forestCapacity,
		onTreeEvicted:  onTreeEvicted,
		metrics:        metrics,
	}

	// add trie with no allocated registers
	emptyTrie := trie.NewEmptyMTrie()
	err = forest.AddTrie(emptyTrie)
	if err != nil {
		return nil, fmt.Errorf("adding empty trie to forest failed: %w", err)
	}
	return forest, nil
}

// ValueSizes returns value sizes for a slice of paths and error (if any)
// TODO: can be optimized further if we don't care about changing the order of the input r.Paths
func (f *Forest) ValueSizes(r *ledger.TrieRead) ([]int, error) {

	if len(r.Paths) == 0 {
		return []int{}, nil
	}

	// lookup the trie by rootHash
	trie, err := f.GetTrie(r.RootHash)
	if err != nil {
		return nil, err
	}

	// deduplicate paths:
	// Generally, we expect the VM to deduplicate reads and writes. Hence, the following is a pre-caution.
	// TODO: We could take out the following de-duplication logic
	//       Which increases the cost for duplicates but reduces ValueSizes complexity without duplicates.
	deduplicatedPaths := make([]ledger.Path, 0, len(r.Paths))
	pathOrgIndex := make(map[ledger.Path][]int)
	for i, path := range r.Paths {
		// only collect duplicated paths once
		indices, ok := pathOrgIndex[path]
		if !ok { // deduplication here is optional
			deduplicatedPaths = append(deduplicatedPaths, path)
		}
		// append the index
		pathOrgIndex[path] = append(indices, i)
	}

	sizes := trie.UnsafeValueSizes(deduplicatedPaths) // this sorts deduplicatedPaths IN-PLACE

	// reconstruct value sizes in the same key order that called the method
	orderedValueSizes := make([]int, len(r.Paths))
	totalValueSize := 0
	for i, p := range deduplicatedPaths {
		size := sizes[i]
		indices := pathOrgIndex[p]
		for _, j := range indices {
			orderedValueSizes[j] = size
		}
		totalValueSize += len(indices) * size
	}
	// TODO rename the metrics
	f.metrics.ReadValuesSize(uint64(totalValueSize))

	return orderedValueSizes, nil
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
	pathOrgIndex := make(map[ledger.Path][]int)
	for i, path := range r.Paths {
		// only collect duplicated keys once
		indices, ok := pathOrgIndex[path]
		if !ok { // deduplication here is optional
			deduplicatedPaths = append(deduplicatedPaths, path)
		}
		// append the index
		pathOrgIndex[path] = append(indices, i)
	}

	payloads := trie.UnsafeRead(deduplicatedPaths) // this sorts deduplicatedPaths IN-PLACE

	// reconstruct the payloads in the same key order that called the method
	orderedPayloads := make([]*ledger.Payload, len(r.Paths))
	totalPayloadSize := 0
	for i, p := range deduplicatedPaths {
		payload := payloads[i]
		indices := pathOrgIndex[p]
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
	emptyHash := ledger.RootHash(hash.DummyHash)

	parentTrie, err := f.GetTrie(u.RootHash)
	if err != nil {
		return emptyHash, err
	}

	if len(u.Paths) == 0 { // no key no change
		return u.RootHash, nil
	}

	// Deduplicate writes to the same register: we only retain the value of the last write
	// Generally, we expect the VM to deduplicate reads and writes.
	deduplicatedPaths := make([]ledger.Path, 0, len(u.Paths))
	deduplicatedPayloads := make([]ledger.Payload, 0, len(u.Paths))
	payloadMap := make(map[ledger.Path]int) // index into deduplicatedPaths, deduplicatedPayloads with register update
	totalPayloadSize := 0
	for i, path := range u.Paths {
		payload := u.Payloads[i]
		// check if we already have encountered an update for the respective register
		if idx, ok := payloadMap[path]; ok {
			oldPayload := deduplicatedPayloads[idx]
			deduplicatedPayloads[idx] = *payload
			totalPayloadSize += -oldPayload.Size() + payload.Size()
		} else {
			payloadMap[path] = len(deduplicatedPaths)
			deduplicatedPaths = append(deduplicatedPaths, path)
			deduplicatedPayloads = append(deduplicatedPayloads, *u.Payloads[i])
			totalPayloadSize += payload.Size()
		}
	}

	// TODO rename metrics names
	f.metrics.UpdateValuesSize(uint64(totalPayloadSize))

	// apply pruning on update
	applyPruning := true
	newTrie, err := trie.NewTrieWithUpdatedRegisters(parentTrie, deduplicatedPaths, deduplicatedPayloads, applyPruning)
	if err != nil {
		return emptyHash, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	f.metrics.LatestTrieRegCount(newTrie.AllocatedRegCount())
	f.metrics.LatestTrieRegCountDiff(int64(newTrie.AllocatedRegCount() - parentTrie.AllocatedRegCount()))
	f.metrics.LatestTrieMaxDepth(uint64(newTrie.MaxDepth()))
	f.metrics.LatestTrieMaxDepthDiff(int64(newTrie.MaxDepth() - parentTrie.MaxDepth()))

	err = f.AddTrie(newTrie)
	if err != nil {
		return emptyHash, fmt.Errorf("adding updated trie to forest failed: %w", err)
	}

	return newTrie.RootHash(), nil
}

// Proofs returns a batch proof for the given paths.
//
// Proves are generally _not_ provided in the register order of the query.
// In the current implementation, input paths in the TrieRead `r` are sorted in an ascendent order,
// The output proofs are provided following the order of the sorted paths.
func (f *Forest) Proofs(r *ledger.TrieRead) (*ledger.TrieBatchProof, error) {

	// no path, empty batchproof
	if len(r.Paths) == 0 {
		return ledger.NewTrieBatchProof(), nil
	}

	// look up for non existing paths
	retValueSizes, err := f.ValueSizes(r)
	if err != nil {
		return nil, err
	}

	notFoundPaths := make([]ledger.Path, 0)
	notFoundPayloads := make([]ledger.Payload, 0)
	for i, path := range r.Paths {
		// add if empty
		if retValueSizes[i] == 0 {
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
		// for proofs, we have to set the pruning to false,
		// currently batch proofs are only consists of inclusion proofs
		// so for non-inclusion proofs we expand the trie with nil value and use an inclusion proof
		// instead. if pruning is enabled it would break this trick and return the exact trie.
		applyPruning := false
		newTrie, err := trie.NewTrieWithUpdatedRegisters(stateTrie, notFoundPaths, notFoundPayloads, applyPruning)
		if err != nil {
			return nil, err
		}

		// rootHash shouldn't change
		if newTrie.RootHash() != r.RootHash {
			return nil, fmt.Errorf("root hash has changed during the operation %x, %x", newTrie.RootHash(), r.RootHash)
		}
		stateTrie = newTrie
	}

	bp := stateTrie.UnsafeProofs(r.Paths)
	return bp, nil
}

// GetTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *Forest) GetTrie(rootHash ledger.RootHash) (*trie.MTrie, error) {
	// if in memory
	if ent, found := f.tries.Get(rootHash); found {
		trie, ok := ent.(*trie.MTrie)
		if !ok {
			return nil, fmt.Errorf("forest contains an element of a wrong type")
		}
		return trie, nil
	}
	return nil, fmt.Errorf("trie with the given rootHash %s not found", rootHash)
}

// GetTries returns list of currently cached tree root hashes
func (f *Forest) GetTries() ([]*trie.MTrie, error) {
	// ToDo needs concurrency safety
	keys := f.tries.Keys()
	tries := make([]*trie.MTrie, len(keys))
	for i, key := range keys {
		t, ok := f.tries.Get(key)
		if !ok {
			return nil, errors.New("concurrent Forest modification")
		}
		trie, ok := t.(*trie.MTrie)
		if !ok {
			return nil, errors.New("forest contains an element of a wrong type")
		}
		tries[i] = trie
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
	rootHash := newTrie.RootHash()
	if storedTrie, found := f.tries.Get(rootHash); found {
		trie, ok := storedTrie.(*trie.MTrie)
		if !ok {
			return fmt.Errorf("forest contains an element of a wrong type")
		}
		if trie.Equals(newTrie) {
			return nil
		}
		return fmt.Errorf("forest already contains a tree with same root hash but other properties")
	}
	f.tries.Add(rootHash, newTrie)
	f.metrics.ForestNumberOfTrees(uint64(f.tries.Len()))

	return nil
}

// RemoveTrie removes a trie to the forest
func (f *Forest) RemoveTrie(rootHash ledger.RootHash) {
	// TODO remove from the file as well
	f.tries.Remove(rootHash)
	f.metrics.ForestNumberOfTrees(uint64(f.tries.Len()))
}

// GetEmptyRootHash returns the rootHash of empty Trie
func (f *Forest) GetEmptyRootHash() ledger.RootHash {
	return trie.EmptyTrieRootHash()
}

// MostRecentTouchedRootHash returns the rootHash of the most recently touched trie
func (f *Forest) MostRecentTouchedRootHash() (ledger.RootHash, error) {
	keys := f.tries.Keys()
	if len(keys) > 0 {
		return keys[len(keys)-1].(ledger.RootHash), nil
	}
	return ledger.RootHash(hash.DummyHash), fmt.Errorf("no trie is stored in the forest")
}

// Size returns the number of active tries in this store
func (f *Forest) Size() int {
	return f.tries.Len()
}
