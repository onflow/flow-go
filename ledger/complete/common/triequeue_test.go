package common

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestEmptyTrieQueue(t *testing.T) {
	const capacity = 10

	q, err := NewTrieQueue(capacity)
	require.NoError(t, err)
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

		q, err := NewTrieQueueWithValues(capacity, initialValues)
		require.NoError(t, err)
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
	rand.Read(randomPath[:])

	var randomHashValue hash.Hash
	rand.Read(randomHashValue[:])

	root := node.NewNode(256, nil, nil, randomPath, nil, randomHashValue)

	return trie.NewMTrie(root, 1, 1)
}

func TestCapacity(t *testing.T) {
	for c := 0; c < 10; c++ {
		q1, err1 := NewTrieQueue(uint(c))
		q2, err2 := NewTrieQueueWithValues(uint(c), []*trie.MTrie{})
		if c == 0 {
			require.Error(t, err1)
			require.Error(t, err2)
		} else {
			require.NoError(t, err1)
			require.NoError(t, err2)
			require.Equal(t, c, q1.Capacity())
			require.Equal(t, c, q2.Capacity())
		}
	}
}

func TestLastLastAddedTrie(t *testing.T) {

	for n := 0; n < 10; n++ {
		q, err := NewTrieQueue(5)
		require.NoError(t, err)
		// prepare n tries
		tries := make([]*trie.MTrie, 0, n)
		for j := 0; j <= n; j++ {
			r, err := randomMTrie()
			require.NoError(t, err)
			tries = append(tries, r)
		}

		// pushing n tries in total
		for j := 0; j <= n; j++ {
			// verify LastAddedTrie
			old, ok := q.LastAddedTrie()
			if j == 0 {
				// before pushing, there is no last added trie
				require.False(t, ok)
				require.Nil(t, old)
			} else {
				require.True(t, ok)
				require.Equal(t, tries[j-1], old)
			}

			// push each trie to the queue
			old, ok = q.Push(tries[j])
			// verify Push
			if j < 5 {
				// no old item was evicted
				require.False(t, ok)
				require.Nil(t, old)
			} else {
				require.True(t, ok)
				require.Equal(t, tries[j-5], old)
			}
		}
	}
}
