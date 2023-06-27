package wal

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestEmptyTrieQueue(t *testing.T) {
	const capacity = 10

	q := NewTrieQueue(capacity)
	require.Equal(t, 0, q.Count())

	tries := q.Tries()
	require.Equal(t, 0, len(tries))
	require.Equal(t, 0, q.Count())

	// savedTries contains all tries that are pushed to queue
	var savedTries []*trie.MTrie

	// Push tries to queue to fill out capacity
	for i := 0; i < capacity; i++ {
		trie, err := randomMTrie()
		require.NoError(t, err)

		q.Push(trie)

		savedTries = append(savedTries, trie)

		tr := q.Tries()
		require.Equal(t, savedTries, tr)
		require.Equal(t, len(savedTries), q.Count())
	}

	// Push more tries to queue to overwrite older elements
	for i := 0; i < capacity; i++ {
		trie, err := randomMTrie()
		require.NoError(t, err)

		q.Push(trie)

		savedTries = append(savedTries, trie)

		tr := q.Tries()
		require.Equal(t, capacity, len(tr))

		// After queue reaches capacity in previous loop,
		// queue overwrites older elements with new insertions,
		// and element count is its capacity value.
		// savedTries contains all elements inserted from previous loop and current loop, so
		// tr (queue snapshot) matches the last C elements in savedTries (where C is capacity).
		require.Equal(t, savedTries[len(savedTries)-capacity:], tr)
		require.Equal(t, capacity, q.Count())
	}
}

func TestTrieQueueWithInitialValues(t *testing.T) {
	const capacity = 10

	// Test TrieQueue with initial values.  Numbers of initial values
	// are from 1 to capacity + 1.
	for initialValueCount := 1; initialValueCount <= capacity+1; initialValueCount++ {

		initialValues := make([]*trie.MTrie, initialValueCount)
		for i := 0; i < len(initialValues); i++ {
			tr, err := randomMTrie()
			require.NoError(t, err)
			initialValues[i] = tr
		}

		expectedCount := initialValueCount
		expectedTries := initialValues
		if initialValueCount > capacity {
			expectedCount = capacity
			expectedTries = initialValues[initialValueCount-capacity:]
		}

		q := NewTrieQueueWithValues(capacity, initialValues)
		require.Equal(t, expectedCount, q.Count())

		tries := q.Tries()
		require.Equal(t, expectedTries, tries)
		require.Equal(t, expectedCount, q.Count())

		// savedTries contains all tries that are pushed to queue, including initial values.
		savedTries := initialValues

		// Push tries to queue to fill out remaining capacity.
		if initialValueCount < capacity {
			for i := 0; i < capacity-initialValueCount; i++ {
				trie, err := randomMTrie()
				require.NoError(t, err)

				q.Push(trie)

				savedTries = append(savedTries, trie)

				tr := q.Tries()
				require.Equal(t, savedTries, tr)
				require.Equal(t, len(savedTries), q.Count())
			}
		}

		// Push more tries to queue to overwrite older elements.
		for i := 0; i < capacity; i++ {
			trie, err := randomMTrie()
			require.NoError(t, err)

			q.Push(trie)

			savedTries = append(savedTries, trie)

			tr := q.Tries()
			require.Equal(t, capacity, len(tr))
			require.Equal(t, savedTries[len(savedTries)-capacity:], tr)
			require.Equal(t, capacity, q.Count())
		}
	}
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
