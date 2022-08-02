package mtrie

import (
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/common"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

type OnTreeEvictedFunc func(tree *trie.MTrie)

// TrieCache caches tries into memory, it acts as a fifo queue
// so when it reaches to the capacity it would evict the oldest trie
// from the cache.
//
// Under the hood it uses a circular buffer
// of mtrie pointers and a map of rootHash to cache index for fast lookup
type TrieCache struct {
	tries         *common.TrieQueue
	lookup        map[ledger.RootHash]*trie.MTrie // index to item
	lock          sync.RWMutex
	onTreeEvicted OnTreeEvictedFunc
}

// NewTrieCache takes a capacity and a callback that will be called when an item is evicted,
// and returns a new TrieCache with given capacity.
func NewTrieCache(capacity uint, onTreeEvicted OnTreeEvictedFunc) *TrieCache {
	notNil := onTreeEvicted
	if notNil == nil {
		notNil = func(*trie.MTrie) {} // no op
	}

	return &TrieCache{
		tries:         common.NewTrieQueue(capacity),
		lookup:        make(map[ledger.RootHash]*trie.MTrie, capacity),
		lock:          sync.RWMutex{},
		onTreeEvicted: notNil,
	}
}

// Purge removes all mtries stored in the buffer
func (tc *TrieCache) Purge() {
	tc.lock.Lock()
	defer tc.lock.Unlock()

	if tc.tries.Count() == 0 {
		return
	}

	toEvict := tc.tries.Tries()

	// update the state first
	capacity := tc.tries.Capacity()
	tc.tries = common.NewTrieQueue(uint(capacity))
	tc.lookup = make(map[ledger.RootHash]*trie.MTrie, capacity)

	// evicting all nodes after state is reset
	for _, trie := range toEvict {
		tc.onTreeEvicted(trie)
	}
}

// Tries returns elements in queue, starting from the oldest element
// to the newest element.
func (tc *TrieCache) Tries() []*trie.MTrie {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	return tc.tries.Tries()
}

// Push pushes trie to queue.  If queue is full, it overwrites the oldest element.
func (tc *TrieCache) Push(t *trie.MTrie) {
	tc.lock.Lock()
	defer tc.lock.Unlock()

	tc.lookup[t.RootHash()] = t
	old, ok := tc.tries.Push(t)
	if ok {
		// evict old node
		delete(tc.lookup, old.RootHash())
		tc.onTreeEvicted(old)
	}
}

// LastAddedTrie returns the last trie added to the cache
// It returns (nil, false) if trie cache has no trie
// It returns (trie, true) if there is last trie added to the cache
func (tc *TrieCache) LastAddedTrie() (*trie.MTrie, bool) {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	return tc.tries.LastAddedTrie()
}

// Get returns the trie by rootHash, if not exist will return nil and false
func (tc *TrieCache) Get(rootHash ledger.RootHash) (*trie.MTrie, bool) {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	trie, found := tc.lookup[rootHash]
	if !found {
		return nil, false
	}
	return trie, true
}

// Count returns number of items stored in the cache
func (tc *TrieCache) Count() int {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	return tc.tries.Count()
}
