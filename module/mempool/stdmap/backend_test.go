// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type fake []byte

func (f fake) ID() flow.Identifier {
	return flow.HashToID(f)
}

func (f fake) Checksum() flow.Identifier {
	return flow.Identifier(sha256.Sum256(f))
}

func TestAddRem(t *testing.T) {
	item1 := fake("DEAD")
	item2 := fake("AGAIN")

	t.Run("should be able to add and rem", func(t *testing.T) {
		pool := NewBackend()
		added := pool.Add(item1)
		require.True(t, added)
		added = pool.Add(item2)
		require.True(t, added)

		t.Run("should be able to get size", func(t *testing.T) {
			size := pool.Size()
			assert.EqualValues(t, uint(2), size)
		})

		t.Run("should be able to get first", func(t *testing.T) {
			gotItem, exists := pool.ByID(item1.ID())
			assert.True(t, exists)
			assert.Equal(t, item1, gotItem)
		})

		t.Run("should be able to remove first", func(t *testing.T) {
			removed := pool.Rem(item1.ID())
			assert.True(t, removed)
			size := pool.Size()
			assert.EqualValues(t, uint(1), size)
		})

		t.Run("should be able to retrieve all", func(t *testing.T) {
			items := pool.All()
			require.Len(t, items, 1)
			assert.Equal(t, item2, items[0])
		})
	})
}

func TestAdjust(t *testing.T) {
	item1 := fake("DEAD")
	item2 := fake("AGAIN")

	t.Run("should not adjust if not exist", func(t *testing.T) {
		pool := NewBackend()
		_ = pool.Add(item1)

		// item2 doesn't exist
		updatedItem, updated := pool.Adjust(item2.ID(), func(old flow.Entity) flow.Entity {
			return item2
		})

		assert.False(t, updated)
		assert.Nil(t, updatedItem)

		_, found := pool.ByID(item2.ID())
		assert.False(t, found)
	})

	t.Run("should adjust if exists", func(t *testing.T) {
		pool := NewBackend()
		_ = pool.Add(item1)

		updatedItem, ok := pool.Adjust(item1.ID(), func(old flow.Entity) flow.Entity {
			// item 1 exist, got replaced with item2, the value was updated
			return item2
		})

		assert.True(t, ok)
		assert.Equal(t, updatedItem, item2)

		value2, found := pool.ByID(item2.ID())
		assert.True(t, found)
		assert.Equal(t, value2, item2)
	})
}

// Test that size mempool deduplicates based on ID
func Test_DeduplicationByID(t *testing.T) {
	item1 := fake("A")
	item2 := fake("A") // identical ID, but different instance
	assert.True(t, item1.ID() == item2.ID())

	pool := NewBackend()
	pool.Add(item1)
	pool.Add(item2)
	assert.Equal(t, uint(1), pool.Size())
}

// TestBackend_RunLimitChecking defines a backend with size limit of `limit`. It then
// starts adding `swarm`-many items concurrently to the backend each on a separate goroutine,
// where `swarm` > `limit`,
// and evaluates that size of the map stays within the limit.
func TestBackend_RunLimitChecking(t *testing.T) {
	const (
		limit = 150
		swarm = 150
	)
	pool := NewBackend(WithLimit(limit))

	wg := sync.WaitGroup{}
	wg.Add(swarm)

	for i := 0; i < swarm; i++ {
		go func(x int) {
			// creates and adds a fake item to the mempool
			item := fake(fmt.Sprintf("item%d", x))
			_ = pool.Run(func(backdata map[flow.Identifier]flow.Entity) error {
				backdata[item.ID()] = item
				return nil
			})

			// evaluates that the size remains in the permissible range
			require.True(t, pool.Size() <= uint(limit),
				fmt.Sprintf("size violation: should be at most: %d, got: %d", limit, pool.Size()))
			wg.Done()
		}(i)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "test could not finish on time")
}

// TestBackend_RegisterEjectionCallback verifies that the Backend calls the
// ejection callbacks whenever it ejects a stored entity due to size limitations.
func TestBackend_RegisterEjectionCallback(t *testing.T) {
	const (
		limit = 20
		swarm = 20
	)
	pool := NewBackend(WithLimit(limit))

	// on ejection callback: test whether ejected identity is no longer part of the mempool
	ensureEntityNotInMempool := func(entity flow.Entity) {
		id := entity.ID()
		go func() {
			e, found := pool.ByID(id)
			require.False(t, found)
			require.Nil(t, e)
		}()
		go func() {
			require.False(t, pool.Has(id))
		}()
	}
	pool.RegisterEjectionCallbacks(ensureEntityNotInMempool)

	wg := sync.WaitGroup{}
	wg.Add(swarm)
	for i := 0; i < swarm; i++ {
		go func(x int) {
			// creates and adds a fake item to the mempool
			item := fake(fmt.Sprintf("item%d", x))
			pool.Add(item)
			wg.Done()
		}(i)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "test could not finish on time")
	require.Equal(t, uint(limit), pool.Size(), "expected mempool to be at max capacity limit")
}

// TestBackend_Multiple_OnEjectionCallbacks verifies that the Backend
//  handles multiple ejection callbacks correctly
func TestBackend_Multiple_OnEjectionCallbacks(t *testing.T) {
	// ejection callback counts number of calls
	calls := uint64(0)
	callback := func(entity flow.Entity) {
		atomic.AddUint64(&calls, 1)
	}

	// construct backend
	const (
		limit = 30
	)
	pool := NewBackend(WithLimit(limit))
	pool.RegisterEjectionCallbacks(callback, callback)

	t.Run("fill mempool up to limit", func(t *testing.T) {
		addRandomEntities(t, pool, limit)
		require.Equal(t, uint(limit), pool.Size(), "expected mempool to be at max capacity limit")
		require.Equal(t, uint64(0), atomic.LoadUint64(&calls))
	})

	t.Run("add elements beyond limit", func(t *testing.T) {
		addRandomEntities(t, pool, 2) // as we registered callback _twice_, we should receive 2 calls per ejection
		require.Less(t, uint(limit), pool.Size(), "expected mempool to be at max capacity limit")
		require.Equal(t, uint64(0), atomic.LoadUint64(&calls))
	})

	t.Run("fill mempool up to limit", func(t *testing.T) {
		atomic.StoreUint64(&calls, uint64(0))
		pool.RegisterEjectionCallbacks(callback) // now we have registered the callback three times
		addRandomEntities(t, pool, 7)            // => we should receive 3 calls per ejection
		require.Less(t, uint(limit), pool.Size(), "expected mempool to be at max capacity limit")
		require.Equal(t, uint64(0), atomic.LoadUint64(&calls))
	})
}

func addRandomEntities(t *testing.T, backend *Backend, num int) {
	// add swarm-number of items to backend
	wg := sync.WaitGroup{}
	wg.Add(num)
	for ; num > 0; num-- {
		go func() {
			randID := unittest.IdentifierFixture()
			backend.Add(fake(randID[:])) // creates and adds a fake item to the mempool
			wg.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "failed to add elements in time")
}
