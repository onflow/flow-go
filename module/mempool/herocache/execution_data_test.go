package herocache_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockExecutionDataPool(t *testing.T) {
	ed1 := unittest.BlockExecutionDatEntityFixture()
	ed2 := unittest.BlockExecutionDatEntityFixture()

	cache := herocache.NewBlockExecutionData(1000, unittest.Logger(), metrics.NewNoopCollector())

	t.Run("should be able to add first", func(t *testing.T) {
		added := cache.Add(ed1)
		assert.True(t, added)
	})

	t.Run("should be able to add second", func(t *testing.T) {
		added := cache.Add(ed2)
		assert.True(t, added)
	})

	t.Run("should be able to get size", func(t *testing.T) {
		size := cache.Size()
		assert.EqualValues(t, 2, size)
	})

	t.Run("should be able to get first by blockID", func(t *testing.T) {
		actual, exists := cache.ByID(ed1.BlockID)
		assert.True(t, exists)
		assert.Equal(t, ed1, actual)
	})

	t.Run("should be able to remove second by blockID", func(t *testing.T) {
		ok := cache.Remove(ed2.BlockID)
		assert.True(t, ok)
	})

	t.Run("should be able to retrieve all", func(t *testing.T) {
		items := cache.All()
		assert.Len(t, items, 1)
		assert.Equal(t, ed1, items[0])
	})

	t.Run("should be able to clear", func(t *testing.T) {
		assert.True(t, cache.Size() > 0)
		cache.Clear()
		assert.Equal(t, uint(0), cache.Size())
	})
}

// TestConcurrentWriteAndRead checks correctness of cache mempool under concurrent read and write.
func TestBlockExecutionDataConcurrentWriteAndRead(t *testing.T) {
	total := 100
	execDatas := unittest.BlockExecutionDatEntityListFixture(total)
	cache := herocache.NewBlockExecutionData(uint32(total), unittest.Logger(), metrics.NewNoopCollector())

	wg := sync.WaitGroup{}
	wg.Add(total)

	// storing all cache
	for i := 0; i < total; i++ {
		go func(ed *execution_data.BlockExecutionDataEntity) {
			require.True(t, cache.Add(ed))

			wg.Done()
		}(execDatas[i])
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not write all cache on time")
	require.Equal(t, cache.Size(), uint(total))

	wg.Add(total)
	// reading all cache
	for i := 0; i < total; i++ {
		go func(ed *execution_data.BlockExecutionDataEntity) {
			actual, ok := cache.ByID(ed.BlockID)
			require.True(t, ok)
			require.Equal(t, ed, actual)

			wg.Done()
		}(execDatas[i])
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not read all cache on time")
}

// TestAllReturnsInOrder checks All method of the HeroCache-based cache mempool returns all
// cache in the same order as they are returned.
func TestBlockExecutionDataAllReturnsInOrder(t *testing.T) {
	total := 100
	execDatas := unittest.BlockExecutionDatEntityListFixture(total)
	cache := herocache.NewBlockExecutionData(uint32(total), unittest.Logger(), metrics.NewNoopCollector())

	// storing all cache
	for i := 0; i < total; i++ {
		require.True(t, cache.Add(execDatas[i]))
		ed, ok := cache.ByID(execDatas[i].BlockID)
		require.True(t, ok)
		require.Equal(t, execDatas[i], ed)
	}

	// all cache must be retrieved in the same order as they are added
	all := cache.All()
	for i := 0; i < total; i++ {
		require.Equal(t, execDatas[i], all[i])
	}
}
