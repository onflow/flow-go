package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIncorporatedResults(t *testing.T) {
	pool := NewIncorporatedResults(1000)

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
