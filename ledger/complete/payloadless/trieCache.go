package mtrie

import (
	"sync"

	"github.com/onflow/flow-go/ledger"
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
	tries         []*trie.MTrie
	lookup        map[ledger.RootHash]int // index to item
	lock          sync.RWMutex
	capacity      int
	tail          int // element index to write to
	count         int // number of elements (count <= capacity)
	onTreeEvicted OnTreeEvictedFunc
}

// NewTrieCache returns a new TrieCache with given capacity.
func NewTrieCache(capacity uint, onTreeEvicted OnTreeEvictedFunc) *TrieCache {
	return &TrieCache{
		tries:         make([]*trie.MTrie, capacity),
		lookup:        make(map[ledger.RootHash]int, capacity),
		lock:          sync.RWMutex{},
		capacity:      int(capacity),
		tail:          0,
		count:         0,
		onTreeEvicted: onTreeEvicted,
	}
}

// Purge removes all mtries stored in the buffer
func (tc *TrieCache) Purge() {
	tc.lock.Lock()
	defer tc.lock.Unlock()

	if tc.count == 0 {
		return
	}

	toEvict := 0
	for i := 0; i < tc.capacity; i++ {
		toEvict = (tc.tail + i) % tc.capacity
		if tc.onTreeEvicted != nil {
			if tc.tries[toEvict] != nil {
				tc.onTreeEvicted(tc.tries[toEvict])
			}
		}
		tc.tries[toEvict] = nil
	}
	tc.tail = 0
	tc.count = 0
	tc.lookup = make(map[ledger.RootHash]int, tc.capacity)
}

// Tries returns elements in queue, starting from the oldest element
// to the newest element.
func (tc *TrieCache) Tries() []*trie.MTrie {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	if tc.count == 0 {
		return nil
	}

	tries := make([]*trie.MTrie, tc.count)

	if tc.tail >= tc.count { // Data isn't wrapped around the slice.
		head := tc.tail - tc.count
		copy(tries, tc.tries[head:tc.tail])
	} else { // q.tail < q.count, data is wrapped around the slice.
		// This branch isn't used until TrieQueue supports Pop (removing oldest element).
		// At this time, there is no reason to implement Pop, so this branch is here to prevent future bug.
		head := tc.capacity - tc.count + tc.tail
		n := copy(tries, tc.tries[head:])
		copy(tries[n:], tc.tries[:tc.tail])
	}

	return tries
}

// Push pushes trie to queue.  If queue is full, it overwrites the oldest element.
func (tc *TrieCache) Push(t *trie.MTrie) {
	tc.lock.Lock()
	defer tc.lock.Unlock()

	// if its full
	if tc.count == tc.capacity {
		oldtrie := tc.tries[tc.tail]
		if tc.onTreeEvicted != nil {
			tc.onTreeEvicted(oldtrie)
		}
		delete(tc.lookup, oldtrie.RootHash())
		tc.count-- // so when we increment at the end of method we don't go beyond capacity
	}
	tc.tries[tc.tail] = t
	tc.lookup[t.RootHash()] = tc.tail
	tc.tail = (tc.tail + 1) % tc.capacity
	tc.count++
}

// LastAddedTrie returns the last trie added to the cache
func (tc *TrieCache) LastAddedTrie() *trie.MTrie {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	if tc.count == 0 {
		return nil
	}
	indx := tc.tail - 1
	if indx < 0 {
		indx = tc.capacity - 1
	}
	return tc.tries[indx]
}

// Get returns the trie by rootHash, if not exist will return nil and false
func (tc *TrieCache) Get(rootHash ledger.RootHash) (*trie.MTrie, bool) {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	idx, found := tc.lookup[rootHash]
	if !found {
		return nil, false
	}
	return tc.tries[idx], true
}

// Count returns number of items stored in the cache
func (tc *TrieCache) Count() int {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	return tc.count
}
