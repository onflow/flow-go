// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"crypto/sha256"
	"fmt"
	"sync"
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

// TestBackend_RunLimitChecking defines a backend with size limit of `limit`. It then
// starts adding `swarm`-many items concurrently to the backend each on a separate goroutine,
// where `swarm` > `limit`,
// and evaluates that size of the map stays within the limit.
func TestBackend_RunLimitChecking(t *testing.T) {
	const (
		limit = 10
		swarm = 20
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
		limit = 10
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
	err := pool.RegisterEjectionCallback(ensureEntityNotInMempool)
	require.NoError(t, err)

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

// TestBackend_ErrorOnRepeatedEjectionCallback verifies that the Backend errors is the
// ejection callback is set repeatedly
func TestBackend_ErrorOnRepeatedEjectionCallback(t *testing.T) {
	pool := NewBackend()
	err := pool.RegisterEjectionCallback(func(entity flow.Entity) {})
	require.NoError(t, err)

	err = pool.RegisterEjectionCallback(func(entity flow.Entity) {})
	require.Error(t, err)
}
