package payloadless

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/module"
)

// Forest holds several in-memory payloadless tries. As Forest is a storage-abstraction layer,
// we assume that all registers are addressed via paths of pre-defined uniform length.
//
// Unlike the full mtrie Forest, this variant stores only leaf hashes (HashLeaf(path, value))
// per register and not the underlying payload values. Reads therefore return leaf hashes,
// not values.
//
// Forest has a limit, the forestCapacity, on the number of tries it is able to store.
// If more tries are added than the capacity, the Least Recently Used trie is
// removed (evicted) from the Forest. THIS IS A ROUGH HEURISTIC as it might evict
// tries that are still needed. In fully matured Flow, we will have an
// explicit eviction policy.
//
// TODO: Storage Eviction Policy for Forest
// For the execution node: we only evict on sealing a result.
type Forest struct {
	// tries stores all MTries in the forest. It is NOT a CACHE in the conventional sense:
	// there is no mechanism to load a trie from disk in case of a cache miss. Missing a
	// needed trie in the forest might cause a fatal application logic error.
	tries          *TrieCache
	forestCapacity int
	onTreeEvicted  func(tree *MTrie)
	metrics        module.LedgerMetrics
}

// NewForest returns a new instance of memory forest.
//
// CAUTION on forestCapacity: the specified capacity MUST be SUFFICIENT to store all needed MTries in the forest.
// If more tries are added than the capacity, the Least Recently Added trie is removed (evicted) from the Forest (FIFO queue).
// Make sure you chose a sufficiently large forestCapacity, such that, when reaching the capacity, the
// Least Recently Added trie will never be needed again.
func NewForest(forestCapacity int, metrics module.LedgerMetrics, onTreeEvicted func(tree *MTrie)) (*Forest, error) {
	forest := &Forest{tries: NewTrieCache(uint(forestCapacity), onTreeEvicted),
		forestCapacity: forestCapacity,
		onTreeEvicted:  onTreeEvicted,
		metrics:        metrics,
	}

	// add trie with no allocated registers
	emptyTrie := NewEmptyMTrie()
	err := forest.AddTrie(emptyTrie)
	if err != nil {
		return nil, fmt.Errorf("adding empty trie to forest failed: %w", err)
	}
	return forest, nil
}

// HasPaths returns, for each input path, whether the path has an allocated register
// in the trie identified by `r.RootHash`. This replaces the full forest's ValueSizes
// method since the payloadless trie does not store payload byte sizes.
// TODO: can be optimized further if we don't care about changing the order of the input r.Paths
func (f *Forest) HasPaths(r *ledger.TrieRead) ([]bool, error) {

	if len(r.Paths) == 0 {
		return []bool{}, nil
	}

	// lookup the trie by rootHash
	trie, err := f.GetTrie(r.RootHash)
	if err != nil {
		return nil, err
	}

	// deduplicate paths:
	// Generally, we expect the VM to deduplicate reads and writes. Hence, the following is a pre-caution.
	// TODO: We could take out the following de-duplication logic
	//       Which increases the cost for duplicates but reduces complexity without duplicates.
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

	leafHashes := trie.UnsafeRead(deduplicatedPaths) // this sorts deduplicatedPaths IN-PLACE

	// reconstruct existence in the same key order that called the method
	exists := make([]bool, len(r.Paths))
	for i, p := range deduplicatedPaths {
		has := leafHashes[i] != nil
		for _, j := range pathOrgIndex[p] {
			exists[j] = has
		}
	}

	return exists, nil
}

// ReadSingleLeafHash reads the leaf hash for a single path. Returns nil if no
// leaf exists at that path or the leaf represents an unallocated register.
func (f *Forest) ReadSingleLeafHash(r *ledger.TrieReadSingleValue) (*hash.Hash, error) {
	// lookup the trie by rootHash
	trie, err := f.GetTrie(r.RootHash)
	if err != nil {
		return nil, err
	}

	return trie.ReadSingleLeafHash(r.Path), nil
}

// ReadLeafHashes reads leaf hashes for a slice of paths and returns the leaf hashes
// in the same order as the input. A nil entry indicates the path has no allocated
// register in the trie.
// TODO: can be optimized further if we don't care about changing the order of the input r.Paths
func (f *Forest) ReadLeafHashes(r *ledger.TrieRead) ([]*hash.Hash, error) {

	if len(r.Paths) == 0 {
		return []*hash.Hash{}, nil
	}

	// lookup the trie by rootHash
	trie, err := f.GetTrie(r.RootHash)
	if err != nil {
		return nil, err
	}

	// call ReadSingleLeafHash if there is only one path
	if len(r.Paths) == 1 {
		return []*hash.Hash{trie.ReadSingleLeafHash(r.Paths[0])}, nil
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

	leafHashes := trie.UnsafeRead(deduplicatedPaths) // this sorts deduplicatedPaths IN-PLACE

	// reconstruct the leaf hashes in the same key order that called the method
	orderedLeafHashes := make([]*hash.Hash, len(r.Paths))
	for i, p := range deduplicatedPaths {
		lh := leafHashes[i]
		for _, j := range pathOrgIndex[p] {
			orderedLeafHashes[j] = lh
		}
	}

	return orderedLeafHashes, nil
}

// Update creates a new trie by updating values for registers in the parent trie,
// adds new trie to forest, and returns rootHash and error (if any).
// In case there are multiple updates to the same register, Update will persist
// the latest written value.
// Note: Update adds new trie to forest, unlike NewTrie().
//
// The input `u.Payloads` are interpreted by extracting only the value bytes; the
// payloadless trie does not store the payload key.
func (f *Forest) Update(u *ledger.TrieUpdate) (ledger.RootHash, error) {
	t, err := f.NewTrie(u)
	if err != nil {
		return ledger.RootHash(hash.DummyHash), err
	}

	err = f.AddTrie(t)
	if err != nil {
		return ledger.RootHash(hash.DummyHash), fmt.Errorf("adding updated trie to forest failed: %w", err)
	}

	return t.RootHash(), nil
}

// NewTrie creates a new trie by updating values for registers in the parent trie,
// and returns new trie and error (if any).
// In case there are multiple updates to the same register, NewTrie will persist
// the latest written value.
// Note: NewTrie doesn't add new trie to forest, unlike Update().
//
// Only the payload's value bytes are used; keys are discarded.
func (f *Forest) NewTrie(u *ledger.TrieUpdate) (*MTrie, error) {

	parentTrie, err := f.GetTrie(u.RootHash)
	if err != nil {
		return nil, err
	}

	if len(u.Paths) == 0 { // no key no change
		return parentTrie, nil
	}

	// Deduplicate writes to the same register: we only retain the value of the last write
	// Generally, we expect the VM to deduplicate reads and writes.
	deduplicatedPaths := make([]ledger.Path, 0, len(u.Paths))
	deduplicatedValues := make([][]byte, 0, len(u.Paths))
	valueMap := make(map[ledger.Path]int) // index into deduplicatedPaths, deduplicatedValues with register update
	for i, path := range u.Paths {
		value := []byte(u.Payloads[i].Value())
		// check if we already have encountered an update for the respective register
		if idx, ok := valueMap[path]; ok {
			deduplicatedValues[idx] = value
		} else {
			valueMap[path] = len(deduplicatedPaths)
			deduplicatedPaths = append(deduplicatedPaths, path)
			deduplicatedValues = append(deduplicatedValues, value)
		}
	}

	// Update metrics with number of updated registers.
	// TODO rename metrics names
	f.metrics.UpdateValuesNumber(uint64(len(deduplicatedValues)))

	// apply pruning on update
	applyPruning := true
	newTrie, maxDepthTouched, err := NewTrieWithUpdatedRegisters(parentTrie, deduplicatedPaths, deduplicatedValues, applyPruning)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	f.metrics.LatestTrieRegCount(newTrie.AllocatedRegCount())
	f.metrics.LatestTrieRegCountDiff(int64(newTrie.AllocatedRegCount() - parentTrie.AllocatedRegCount()))
	f.metrics.LatestTrieMaxDepthTouched(maxDepthTouched)

	return newTrie, nil
}

// Proofs returns a batch proof for the given paths.
//
// Proofs are generally _not_ provided in the register order of the query.
// In the current implementation, input paths in the TrieRead `r` are sorted in an ascendent order,
// The output proofs are provided following the order of the sorted paths.
//
// Returned proofs carry leaf hashes (HashLeaf(path, value)) rather than full payloads.
func (f *Forest) Proofs(r *ledger.TrieRead) (*ledger.PayloadlessTrieBatchProof, error) {

	// no path, empty batchproof
	if len(r.Paths) == 0 {
		return ledger.NewPayloadlessTrieBatchProof(), nil
	}

	// look up for non existing paths
	exists, err := f.HasPaths(r)
	if err != nil {
		return nil, err
	}

	notFoundPaths := make([]ledger.Path, 0)
	notFoundValues := make([][]byte, 0)
	for i, path := range r.Paths {
		// add if empty
		if !exists[i] {
			notFoundPaths = append(notFoundPaths, path)
			notFoundValues = append(notFoundValues, nil)
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
		newTrie, _, err := NewTrieWithUpdatedRegisters(stateTrie, notFoundPaths, notFoundValues, applyPruning)
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

// HasTrie returns true if trie exist at specific rootHash
func (f *Forest) HasTrie(rootHash ledger.RootHash) bool {
	_, found := f.tries.Get(rootHash)
	return found
}

// GetTrie returns trie at specific rootHash
// warning, use this function for read-only operation
func (f *Forest) GetTrie(rootHash ledger.RootHash) (*MTrie, error) {
	// if in memory
	if trie, found := f.tries.Get(rootHash); found {
		return trie, nil
	}
	return nil, fmt.Errorf("trie with the given rootHash %s not found", rootHash)
}

// GetTries returns list of currently cached tree root hashes
func (f *Forest) GetTries() ([]*MTrie, error) {
	return f.tries.Tries(), nil
}

// AddTries adds a trie to the forest
func (f *Forest) AddTries(newTries []*MTrie) error {
	for _, t := range newTries {
		err := f.AddTrie(t)
		if err != nil {
			return fmt.Errorf("adding tries to forest failed: %w", err)
		}
	}
	return nil
}

// AddTrie adds a trie to the forest
func (f *Forest) AddTrie(newTrie *MTrie) error {
	if newTrie == nil {
		return nil
	}

	// TODO: check Thread safety
	rootHash := newTrie.RootHash()
	if _, found := f.tries.Get(rootHash); found {
		// do no op
		return nil
	}
	f.tries.Push(newTrie)
	f.metrics.ForestNumberOfTrees(uint64(f.tries.Count()))

	return nil
}

// GetEmptyRootHash returns the rootHash of empty Trie
func (f *Forest) GetEmptyRootHash() ledger.RootHash {
	return EmptyTrieRootHash()
}

// MostRecentTouchedRootHash returns the rootHash of the most recently touched trie
func (f *Forest) MostRecentTouchedRootHash() (ledger.RootHash, error) {
	trie := f.tries.LastAddedTrie()
	if trie != nil {
		return trie.RootHash(), nil
	}
	return ledger.RootHash(hash.DummyHash), fmt.Errorf("no trie is stored in the forest")
}

// PurgeCacheExcept removes all tries in the memory except the one with the given root hash
func (f *Forest) PurgeCacheExcept(rootHash ledger.RootHash) error {
	trie, found := f.tries.Get(rootHash)
	if !found {
		return fmt.Errorf("trie with the given root hash not found")
	}
	f.tries.Purge()
	f.tries.Push(trie)
	return nil
}

// Size returns the number of active tries in this store
func (f *Forest) Size() int {
	return f.tries.Count()
}
