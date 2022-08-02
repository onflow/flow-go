package mtrie

// test addition
// test under capacity
// test on capacity
// test across boundry

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTrieCache(t *testing.T) {
	const capacity = 10

	tc := NewTrieCache(capacity, nil)
	require.Equal(t, 0, tc.Count())

	tries := tc.Tries()
	require.Equal(t, 0, len(tries))
	require.Equal(t, 0, tc.Count())
	require.Equal(t, 0, len(tc.lookup))

	// savedTries contains all tries that are pushed to queue
	var savedTries []*trie.MTrie

	// Push tries to queue to fill out capacity
	for i := 0; i < capacity; i++ {
		trie, err := randomMTrie()
		require.NoError(t, err)

		tc.Push(trie)

		savedTries = append(savedTries, trie)

		tr := tc.Tries()
		require.Equal(t, savedTries, tr)
		require.Equal(t, len(savedTries), tc.Count())
		require.Equal(t, len(savedTries), len(tc.lookup))

		retTrie, found := tc.Get(trie.RootHash())
		require.Equal(t, retTrie, trie)
		require.True(t, found)

		// check last added trie functionality
		retTrie, ok := tc.LastAddedTrie()
		require.True(t, ok)
		require.Equal(t, retTrie, trie)
	}

	// Push more tries to queue to overwrite older elements
	for i := 0; i < capacity; i++ {
		trie, err := randomMTrie()
		require.NoError(t, err)

		tc.Push(trie)

		savedTries = append(savedTries, trie)

		tr := tc.Tries()
		require.Equal(t, capacity, len(tr))

		// After queue reaches capacity in previous loop,
		// queue overwrites older elements with new insertions,
		// and element count is its capacity value.
		// savedTries contains all elements inserted from previous loop and current loop, so
		// tr (queue snapshot) matches the last C elements in savedTries (where C is capacity).
		require.Equal(t, savedTries[len(savedTries)-capacity:], tr)
		require.Equal(t, capacity, tc.Count())
		require.Equal(t, capacity, len(tc.lookup))

		// check the trie is lookable
		retTrie, found := tc.Get(trie.RootHash())
		require.Equal(t, retTrie, trie)
		require.True(t, found)

		// check the last evicted value is not kept
		retTrie, found = tc.Get(savedTries[len(savedTries)-capacity-1].RootHash())
		require.Nil(t, retTrie)
		require.False(t, found)

		// check last added trie functionality
		retTrie, ok := tc.LastAddedTrie()
		require.True(t, ok)
		require.Equal(t, retTrie, trie)
	}

}

func TestPurge(t *testing.T) {
	const capacity = 5

	trie1, err := randomMTrie()
	require.NoError(t, err)
	trie2, err := randomMTrie()
	require.NoError(t, err)
	trie3, err := randomMTrie()
	require.NoError(t, err)

	called := 0
	tc := NewTrieCache(capacity, func(tree *trie.MTrie) {
		switch called {
		case 0:
			require.Equal(t, trie1, tree)
		case 1:
			require.Equal(t, trie2, tree)
		case 2:
			require.Equal(t, trie3, tree)
		}
		called++

	})
	tc.Push(trie1)
	tc.Push(trie2)
	tc.Push(trie3)

	tc.Purge()
	require.Equal(t, 0, tc.Count())
	require.Equal(t, 0, len(tc.lookup))

	require.Equal(t, 3, called)
}

func TestEvictCallBack(t *testing.T) {
	const capacity = 2

	trie1, err := randomMTrie()
	require.NoError(t, err)

	called := false
	tc := NewTrieCache(capacity, func(tree *trie.MTrie) {
		called = true
		require.Equal(t, trie1, tree)
	})
	tc.Push(trie1)

	trie2, err := randomMTrie()
	require.NoError(t, err)
	tc.Push(trie2)

	trie3, err := randomMTrie()
	require.NoError(t, err)
	tc.Push(trie3)

	require.True(t, called)
}

func TestConcurrentAccess(t *testing.T) {

	const worker = 50
	const capacity = 100 // large enough to not worry evicts

	tc := NewTrieCache(capacity, nil)

	unittest.Concurrently(worker, func(i int) {
		trie, err := randomMTrie()
		require.NoError(t, err)
		tc.Push(trie)

		ret, found := tc.Get(trie.RootHash())
		require.True(t, found)
		require.Equal(t, trie, ret)
	})

	require.Equal(t, worker, tc.Count())
}

func randomMTrie() (*trie.MTrie, error) {
	var randomPath ledger.Path
	rand.Read(randomPath[:])

	var randomHashValue hash.Hash
	rand.Read(randomHashValue[:])

	root := node.NewNode(256, nil, nil, randomPath, nil, randomHashValue)

	return trie.NewMTrie(root, 1, 1)
}

func TestTrieCacheUpdate(t *testing.T) {
	for n := 0; n < 10; n++ {
		// prepare n tries to be pushed to the cache
		tries := make([]*trie.MTrie, 0, n)
		for j := 0; j < n; j++ {
			r, err := randomMTrie()
			require.NoError(t, err)
			tries = append(tries, r)
		}

		// prepare the trie cache to be tested
		evicted := make([]*trie.MTrie, 0)
		onEvicted := func(m *trie.MTrie) {
			evicted = append(evicted, m)
		}

		c := NewTrieCache(5, onEvicted)

		// pushing n tries in total
		for j := 0; j < n; j++ {
			// verify LastAddedTrie
			old, ok := c.LastAddedTrie()
			if j == 0 {
				// before pushing, there is no last added trie
				require.False(t, ok)
				require.Nil(t, old)
			} else {
				require.True(t, ok)
				require.Equal(t, tries[j-1], old)
			}

			// push each trie to the queue
			c.Push(tries[j])
			// verify Push
			if j < 5 {
				// no old item was evicted
				require.Equal(t, []*trie.MTrie{}, evicted)
				require.Equal(t, j+1, c.Count())
			} else {
				// check evicted
				require.Len(t, evicted, j-5+1)
				require.Equal(t, tries[:j-5+1], evicted)
				require.Equal(t, 5, c.Count())
			}

			// verify Get method, verify that each pushed item that hasn't been ejected must exist
			// since n items needs to be pushed and j items have been pushed,
			// no matter how many to push, if there are no more than 5 (capacity) have been pushed,
			// then all pushed j items, must exist, the rest are not
			if j < 5 {
				for g, p := range tries {
					found, ok := c.Get(p.RootHash())
					if g <= j {
						require.True(t, ok, fmt.Sprintf("%v (n) tries to push, %v (j)-th tries pushed, %v-th trie to veirfy", n, j, g))
						require.Equal(t, p, found)
					} else {
						require.False(t, ok, fmt.Sprintf("%v (n) tries to push, %v (j)-th tries pushed, %v-th trie to veirfy", n, j, g))
						require.Nil(t, found)
					}
				}
			} else {

				// if there are more than 5 (capacity) was pushed, then some item must have been pushed,
				// only the last 5 items still exist.

				for g, p := range tries {
					found, ok := c.Get(p.RootHash())
					hasBeenPushed := g <= j
					isLast5 := g > j-5
					if hasBeenPushed && isLast5 {
						require.True(t, ok, fmt.Sprintf("%v (n) tries to push, %v (j)-th tries pushed, %v-th trie to veirfy", n, j, g))
						require.Equal(t, p, found)
					} else {
						require.False(t, ok, fmt.Sprintf("%v (n) tries to push, %v (j)-th tries pushed, %v-th trie to veirfy", n, j, g))
						require.Nil(t, found)
					}
				}
			}
		}

		// verify purging the cache
		toBeEvicted := c.Tries()
		evicted = make([]*trie.MTrie, 0) // reset before Purge
		c.Purge()
		require.Equal(t, 0, c.Count())
		if len(toBeEvicted) == 0 {
			require.Len(t, evicted, 0)
		} else {
			require.Equal(t, toBeEvicted, evicted)
		}
	}
}
