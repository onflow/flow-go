package mtrie

import (
	"sync"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

type OnTreeEvictedFunc func(tree *trie.MTrie)

type Buffer struct {
	tries         []*trie.MTrie
	lookup        map[ledger.RootHash]int // index to item
	lock          sync.Mutex
	capacity      int
	tail          int // element index to write to
	count         int // number of elements (count <= capacity)
	onTreeEvicted OnTreeEvictedFunc
}

// NewBuffer returns a new Buffer with given capacity.
func NewBuffer(capacity uint, onTreeEvicted OnTreeEvictedFunc) *Buffer {
	return &Buffer{
		tries:         make([]*trie.MTrie, capacity),
		lookup:        make(map[ledger.RootHash]int, capacity),
		lock:          sync.Mutex{},
		capacity:      int(capacity),
		tail:          0,
		count:         0,
		onTreeEvicted: onTreeEvicted,
	}
}

// Purge removes all mtries stored in the buffer
func (b *Buffer) Purge() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.tail = 0
	b.count = 0
	b.tries = make([]*trie.MTrie, b.capacity)
	b.lookup = make(map[ledger.RootHash]int, b.capacity)
}

// Tries returns elements in queue, starting from the oldest element
// to the newest element.
func (b *Buffer) Tries() []*trie.MTrie {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.count == 0 {
		return nil
	}

	tries := make([]*trie.MTrie, b.count)

	if b.isFull() {
		// If queue is full, tail points to the oldest element.
		head := b.tail
		n := copy(tries, b.tries[head:])
		copy(tries[n:], b.tries[:b.tail])
	} else {
		if b.tail >= b.count { // Data isn't wrapped around the slice.
			head := b.tail - b.count
			copy(tries, b.tries[head:b.tail])
		} else { // q.tail < q.count, data is wrapped around the slice.
			// This branch isn't used until TrieQueue supports Pop (removing oldest element).
			// At this time, there is no reason to implement Pop, so this branch is here to prevent future bug.
			head := b.capacity - b.count + b.tail
			n := copy(tries, b.tries[head:])
			copy(tries[n:], b.tries[:b.tail])
		}
	}

	return tries
}

// Push pushes trie to queue.  If queue is full, it overwrites the oldest element.
func (b *Buffer) Push(t *trie.MTrie) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.isFull() {
		oldtrie := b.tries[b.tail]
		if b.onTreeEvicted != nil {
			b.onTreeEvicted(oldtrie)
		}
		delete(b.lookup, oldtrie.RootHash())
	}
	b.tries[b.tail] = t
	b.lookup[t.RootHash()] = b.tail
	b.tail = (b.tail + 1) % b.capacity
	if !b.isFull() {
		b.count++
	}
}

// LastAddedTrie returns the last trie added to the buffer
func (b *Buffer) LastAddedTrie() *trie.MTrie {
	if b.count == 0 {
		return nil
	}
	indx := b.tail - 1
	if indx < 0 {
		indx = b.capacity - 1
	}
	return b.tries[indx]
}

// Get returns the trie by rootHash, if not exist will return nil and false
func (b *Buffer) Get(rootHash ledger.RootHash) (*trie.MTrie, bool) {
	b.lock.Lock()
	defer b.lock.Unlock()

	idx, found := b.lookup[rootHash]
	if !found {
		return nil, false
	}
	return b.tries[idx], true
}

// Count returns number of items stored in the buffer
func (b *Buffer) Count() int {
	return b.count
}

func (b *Buffer) isFull() bool {
	return b.count == b.capacity
}
