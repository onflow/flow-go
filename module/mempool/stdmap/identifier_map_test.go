package stdmap

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestIdentiferMap(t *testing.T) {
	idMap, err := NewIdentifierMap(10)

	t.Run("creating new mempool", func(t *testing.T) {
		require.NoError(t, err)
	})

	key1 := unittest.IdentifierFixture()
	id1 := unittest.IdentifierFixture()
	t.Run("appending id to new key", func(t *testing.T) {
		err := idMap.Append(key1, id1)
		require.NoError(t, err)

		// checks the existence of id1 for key
		ids, ok := idMap.Get(key1)
		require.True(t, ok)
		require.Contains(t, ids, id1)
	})

	id2 := unittest.IdentifierFixture()
	t.Run("appending the second id", func(t *testing.T) {
		err := idMap.Append(key1, id2)
		require.NoError(t, err)

		// checks the existence of both id1 and id2 for key1
		ids, ok := idMap.Get(key1)
		require.True(t, ok)
		// both ids should exist
		assert.Contains(t, ids, id1)
		assert.Contains(t, ids, id2)
	})

	// tests against existence of another key, with a shared id (id1)
	key2 := unittest.IdentifierFixture()
	t.Run("appending shared id to another key", func(t *testing.T) {
		err := idMap.Append(key2, id1)
		require.NoError(t, err)

		// checks the existence of both id1 and id2 for key1
		ids, ok := idMap.Get(key1)
		require.True(t, ok)
		// both ids should exist
		assert.Contains(t, ids, id1)
		assert.Contains(t, ids, id2)

		// checks the existence of  id1 for key2
		ids, ok = idMap.Get(key2)
		require.True(t, ok)
		assert.Contains(t, ids, id1)
		assert.NotContains(t, ids, id2)
	})

	t.Run("getting all keys", func(t *testing.T) {
		keys, ok := idMap.Keys()

		// Keys should return all keys in mempool
		require.True(t, ok)
		assert.Contains(t, keys, key1)
		assert.Contains(t, keys, key2)
	})

	// tests against removing a key
	t.Run("removing key", func(t *testing.T) {
		ok := idMap.Rem(key1)
		require.True(t, ok)

		// getting removed key should return false
		ids, ok := idMap.Get(key1)
		require.False(t, ok)
		require.Nil(t, ids)

		// since key1 is removed, Has on it should return false
		require.False(t, idMap.Has(key1))

		// checks the existence of  id1 for key2
		// removing key1 should not alter key2
		ids, ok = idMap.Get(key2)
		require.True(t, ok)
		assert.Contains(t, ids, id1)
		assert.NotContains(t, ids, id2)

		// since key2 exists, Has on it should return true
		require.True(t, idMap.Has(key2))

		// Keys method should only return key2
		keys, ok := idMap.Keys()
		require.True(t, ok)
		assert.NotContains(t, keys, key1)
		assert.Contains(t, keys, key2)
	})

	// tests against appending an existing id for a key
	t.Run("duplicate id for a key", func(t *testing.T) {
		ids, ok := idMap.Get(key2)
		require.True(t, ok)
		assert.Contains(t, ids, id1)

		err := idMap.Append(key2, id1)
		require.NoError(t, err)
	})

	t.Run("removing id from a key test", func(t *testing.T) {
		// creates key3 and adds id1 and id2 to it.
		key3 := unittest.IdentifierFixture()
		err := idMap.Append(key3, id1)
		require.NoError(t, err)
		err = idMap.Append(key3, id2)
		require.NoError(t, err)

		// removes id1 and id2 from key3
		// removing id1
		err = idMap.RemIdFromKey(key3, id1)
		require.NoError(t, err)

		// key3 should still reside on idMap and id2 should be attached to it
		require.True(t, idMap.Has(key3))
		ids, ok := idMap.Get(key3)
		require.True(t, ok)
		require.Contains(t, ids, id2)

		// removing id2
		err = idMap.RemIdFromKey(key3, id2)
		require.NoError(t, err)

		// by removing id2 from key3, it is left out of id
		// so it should be automatically cleaned up
		require.False(t, idMap.Has(key3))

		ids, ok = idMap.Get(key3)
		require.False(t, ok)
		require.Empty(t, ids)

		// it however should not affect any other keys
		require.True(t, idMap.Has(key2))
	})
}

// TestRaceCondition is meant for running with `-race` flag.
// It performs Append, Has, Get, and RemIdFromKey methods of IdentifierMap concurrently
// each in a different goroutine.
// Running this test with `-race` flag detects and reports the existence of race condition if
// it is the case.
func TestRaceCondition(t *testing.T) {
	idMap, err := NewIdentifierMap(10)
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	key := unittest.IdentifierFixture()
	id := unittest.IdentifierFixture()
	wg.Add(4)

	go func() {
		defer wg.Done()
		require.NoError(t, idMap.Append(key, id))
	}()

	go func() {
		defer wg.Done()
		idMap.Has(key)
	}()

	go func() {
		defer wg.Done()
		idMap.Get(key)
	}()

	go func() {
		defer wg.Done()
		require.NoError(t, idMap.RemIdFromKey(key, id))
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "test could not finish on time")
}

// TestCapacity defines an identifier map with size limit of `limit`. It then
// starts adding `swarm`-many items concurrently to the map each on a separate goroutine,
// where `swarm` > `limit`,
// and evaluates that size of the map stays within the limit.
func TestCapacity(t *testing.T) {
	const (
		limit = 20
		swarm = 20
	)
	idMap, err := NewIdentifierMap(limit)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(swarm)

	for i := 0; i < swarm; i++ {
		go func() {
			// adds an item on a separate goroutine
			key := unittest.IdentifierFixture()
			id := unittest.IdentifierFixture()
			err := idMap.Append(key, id)
			require.NoError(t, err)

			// evaluates that the size remains in the permissible range
			require.True(t, idMap.Size() <= uint(limit),
				fmt.Sprintf("size violation: should be at most: %d, got: %d", limit, idMap.Size()))
			wg.Done()
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "test could not finish on time")
}
