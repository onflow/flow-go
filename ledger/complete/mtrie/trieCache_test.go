package mtrie

// test addition
// test under capacity
// test on capacity
// test across boundry

import (
	"crypto/rand"
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
		retTrie = tc.LastAddedTrie()
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
		retTrie = tc.LastAddedTrie()
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
	require.Equal(t, 0, tc.tail)
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
	_, err := rand.Read(randomPath[:])
	if err != nil {
		return nil, err
	}

	var randomHashValue hash.Hash
	_, err = rand.Read(randomHashValue[:])
	if err != nil {
		return nil, err
	}

	root := node.NewNode(256, nil, nil, randomPath, nil, randomHashValue)

	return trie.NewMTrie(root, 1, 1)
}
