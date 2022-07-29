package mtrie

// test addition
// test under capacity
// test on capacity
// test across boundry

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestBuffer(t *testing.T) {
	const capacity = 10

	b := NewBuffer(capacity, nil)
	require.Equal(t, 0, b.Count())

	tries := b.Tries()
	require.Equal(t, 0, len(tries))
	require.Equal(t, 0, b.Count())
	require.Equal(t, 0, len(b.lookup))

	// savedTries contains all tries that are pushed to queue
	var savedTries []*trie.MTrie

	// Push tries to queue to fill out capacity
	for i := 0; i < capacity; i++ {
		trie, err := randomMTrie()
		require.NoError(t, err)

		b.Push(trie)

		savedTries = append(savedTries, trie)

		tr := b.Tries()
		require.Equal(t, savedTries, tr)
		require.Equal(t, len(savedTries), b.Count())
		require.Equal(t, len(savedTries), len(b.lookup))

		retTrie, found := b.Get(trie.RootHash())
		require.Equal(t, retTrie, trie)
		require.True(t, found)

		// check last added trie functionality
		retTrie = b.LastAddedTrie()
		require.Equal(t, retTrie, trie)
	}

	// Push more tries to queue to overwrite older elements
	for i := 0; i < capacity; i++ {
		trie, err := randomMTrie()
		require.NoError(t, err)

		b.Push(trie)

		savedTries = append(savedTries, trie)

		tr := b.Tries()
		require.Equal(t, capacity, len(tr))

		// After queue reaches capacity in previous loop,
		// queue overwrites older elements with new insertions,
		// and element count is its capacity value.
		// savedTries contains all elements inserted from previous loop and current loop, so
		// tr (queue snapshot) matches the last C elements in savedTries (where C is capacity).
		require.Equal(t, savedTries[len(savedTries)-capacity:], tr)
		require.Equal(t, capacity, b.Count())
		require.Equal(t, capacity, len(b.lookup))

		// check the trie is lookable
		retTrie, found := b.Get(trie.RootHash())
		require.Equal(t, retTrie, trie)
		require.True(t, found)

		// check the last evicted value is not kept
		retTrie, found = b.Get(savedTries[len(savedTries)-capacity-1].RootHash())
		require.Nil(t, retTrie)
		require.False(t, found)

		// check last added trie functionality
		retTrie = b.LastAddedTrie()
		require.Equal(t, retTrie, trie)
	}

	// test purge functionality
	b.Purge()
	require.Equal(t, 0, b.Count())
	require.Equal(t, 0, b.tail)
	require.Equal(t, 0, len(b.lookup))
}

func TestEvictCallBack(t *testing.T) {
	const capacity = 2

	trie1, err := randomMTrie()
	require.NoError(t, err)

	called := false
	b := NewBuffer(capacity, func(tree *trie.MTrie) {
		called = true
		require.Equal(t, trie1, tree)
	})
	b.Push(trie1)

	trie2, err := randomMTrie()
	require.NoError(t, err)
	b.Push(trie2)

	trie3, err := randomMTrie()
	require.NoError(t, err)
	b.Push(trie3)

	require.True(t, called)
}

func randomMTrie() (*trie.MTrie, error) {
	var randomPath ledger.Path
	rand.Read(randomPath[:])

	var randomHashValue hash.Hash
	rand.Read(randomHashValue[:])

	root := node.NewNode(256, nil, nil, randomPath, nil, randomHashValue)

	return trie.NewMTrie(root, 1, 1)
}
