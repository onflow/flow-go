package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIncorporatedResults(t *testing.T) {
	pool, err := NewIncorporatedResults(1000)
	require.NoError(t, err)

	ir1 := unittest.IncorporatedResultFixture()
	t.Run("Adding first incorporated result", func(t *testing.T) {
		ok, err := pool.Add(ir1)
		require.True(t, ok)
		require.NoError(t, err)

		// check the existence of incorporated result
		res, incorporatedResults, found := pool.ByResultID(ir1.Result.ID())
		require.True(t, found)
		require.Equal(t, ir1.Result, res)
		require.Contains(t, incorporatedResults, ir1.IncorporatedBlockID)
	})

	ir2 := &flow.IncorporatedResult{
		IncorporatedBlockID: unittest.IdentifierFixture(),
		Result:              ir1.Result,
	}
	t.Run("Adding second incorporated result for same result", func(t *testing.T) {
		ok, err := pool.Add(ir2)
		require.True(t, ok)
		require.NoError(t, err)

		// check the existence of incorporated result
		res, incorporatedResults, found := pool.ByResultID(ir2.Result.ID())
		require.True(t, found)
		require.Equal(t, ir1.Result, res)
		require.Contains(t, incorporatedResults, ir1.IncorporatedBlockID)
		require.Contains(t, incorporatedResults, ir2.IncorporatedBlockID)
	})

	ir3 := unittest.IncorporatedResultFixture()
	t.Run("Adding third incorporated result", func(t *testing.T) {
		ok, err := pool.Add(ir3)
		require.True(t, ok)
		require.NoError(t, err)

		// check the existence of incorporated result
		res, incorporatedResults, found := pool.ByResultID(ir3.Result.ID())
		require.True(t, found)
		require.Equal(t, ir3.Result, res)
		require.Contains(t, incorporatedResults, ir3.IncorporatedBlockID)
	})

	t.Run("Getting all incorporated results", func(t *testing.T) {
		all := pool.All()
		assert.Contains(t, all, ir1)
		assert.Contains(t, all, ir2)
		assert.Contains(t, all, ir3)
	})

	t.Run("Removing incorporated result", func(t *testing.T) {
		ok := pool.Rem(ir1)
		require.True(t, ok)

		res, incorporatedResults, found := pool.ByResultID(ir1.Result.ID())
		require.True(t, found)
		require.Equal(t, ir1.Result, res)
		require.Contains(t, incorporatedResults, ir2.IncorporatedBlockID)
	})
}

// Test that size gets decremented when items are automatically ejected
func TestIncorporatedResultsEjectSize(t *testing.T) {

	t.Run("check ejection of block with only a single result", func(t *testing.T) {
		pool, err := NewIncorporatedResults(10)
		require.NoError(t, err)

		// insert 20 items (10 above limit)
		for i := 0; i < 20; i++ {
			_, _ = pool.Add(unittest.IncorporatedResultFixture())
		}

		// 10 items should have been evicted, so size 10
		require.Equal(t, uint(10), pool.Size())
	})

	t.Run("check ejection of block with multiple results", func(t *testing.T) {
		var ejector EjectFunc = NewLRUEjector().Eject
		mempool, err := NewIncorporatedResults(10, WithEject(ejector))
		require.NoError(t, err)

		for b := 0; b < 10; b++ {
			result := unittest.ExecutionResultFixture()
			// insert 3 different IncorporatedResult for the same result
			for i := 0; i < 3; i++ {
				a := unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))
				_, _ = mempool.Add(a)
			}
			// The mempool stores all IncorporatedResult for the same result internally in one data structure.
			// Therefore, all results for the same result consume only capacity 1.
		}
		// mempool should now be at capacity limit
		require.Equal(t, uint(30), mempool.Size())

		// Adding another element should overflow the mempool;
		_, _ = mempool.Add(unittest.IncorporatedResult.Fixture())

		// The mempool stores all IncorporatedResult for the same result internally in one data structure.
		// Hence, eviction should lead to _all_ IncorporatedResult for a single result being dropped.
		//  * for this specific test, we use the LRU ejector
		//  * this ejector drops the result that was added earliest
		// Hence, we expect 3 IncorporatedResult for each of the results 1, 2, ..., 9
		// plus one result for result 10
		require.Equal(t, uint(9*3+1), mempool.Size())
	})
}
