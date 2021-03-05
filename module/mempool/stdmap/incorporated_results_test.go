package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIncorporatedResults(t *testing.T) {
	t.Parallel()

	pool, err := NewIncorporatedResults(1000)
	require.NoError(t, err)

	ir1 := unittest.IncorporatedResult.Fixture()
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

	ir3 := unittest.IncorporatedResult.Fixture()
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
	t.Parallel()

	t.Run("check ejection of block with only a single result", func(t *testing.T) {
		pool, err := NewIncorporatedResults(10)
		require.NoError(t, err)

		// insert 20 items (10 above limit)
		for i := 0; i < 20; i++ {
			_, _ = pool.Add(unittest.IncorporatedResult.Fixture())
		}

		// 10 items should have been evicted, so size 10
		require.Equal(t, uint(10), pool.Size())
	})

	t.Run("check ejection of block with multiple results", func(t *testing.T) {
		// custom ejector which ejects only model.IncorporatedResultMap for given result
		result := unittest.ExecutionResultFixture()
		targetForEjection := result.ID()
		ejector := func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
			for id, entity := range entities {
				if id == targetForEjection {
					return id, entity
				}
			}
			panic("missing target ID")
		}

		// init mempool
		mempool, err := NewIncorporatedResults(10, WithEject(ejector))
		require.NoError(t, err)

		for r := 1; r <= 10; r++ {
			// insert 3 different IncorporatedResult for the same result
			addIncorporatedResults(t, mempool, result, 3)
			// The mempool stores all IncorporatedResult for the same result internally in one data structure.
			// Therefore, all IncorporatedResults for the same result consume only capacity 1.
			require.Equal(t, uint(3*r), mempool.Size())

			result = unittest.ExecutionResultFixture()
		}
		// mempool should now be at capacity limit: storing IncorporatedResults for 10 different base results
		require.Equal(t, uint(30), mempool.Size())

		// Adding an IncorporatedResult for a previously unknown base result should overflow the mempool:
		added, err := mempool.Add(unittest.IncorporatedResult.Fixture())
		require.True(t, added)
		require.NoError(t, err)

		// The mempool stores all IncorporatedResult for the same result internally in one data structure.
		// Hence, eviction should lead to _all_ IncorporatedResult for a single result being dropped.
		// For this specific test, we always evict the IncorporatedResult for the first base result.
		// Hence, we expect:
		//   * 0 IncorporatedResult for each of the results 1, as it was evicted
		//   * 3 IncorporatedResult for each of the results 2, 3, ..., 10
		//   * plus one result for result 11
		require.Equal(t, uint(9*3+1), mempool.Size())
	})
}

// addIncorporatedResults generates 3 different IncorporatedResults structures
// for the baseResult and adds those to the mempool
func addIncorporatedResults(t *testing.T, mempool *IncorporatedResults, baseResult *flow.ExecutionResult, num uint) {
	for ; num > 0; num-- {
		a := unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(baseResult))
		added, err := mempool.Add(a)
		require.True(t, added)
		require.NoError(t, err)
	}
}
