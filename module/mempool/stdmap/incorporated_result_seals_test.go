package stdmap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIncorporatedResultSeals(t *testing.T) {
	t.Parallel()

	// after adding one receipt, should be able to query it back by previous result id
	// after removing, should not be able to query it back.
	t.Run("add remove get", func(t *testing.T) {
		pool := NewIncorporatedResultSeals()

		seal := unittest.IncorporatedResultSeal.Fixture()

		ok, err := pool.Add(seal)
		require.NoError(t, err)
		require.True(t, ok)

		actual, ok := pool.ByID(seal.ID())
		require.True(t, ok)
		require.Equal(t, seal, actual)

		deleted := pool.Rem(seal.ID())
		require.True(t, deleted)

		_, ok = pool.ByID(seal.ID())
		require.False(t, ok)
	})

	t.Run("add 100 prune by height", func(t *testing.T) {
		pool := NewIncorporatedResultSeals()

		seals := make([]*flow.IncorporatedResultSeal, 0, 100)
		for i := 0; i < 100; i++ {
			seal := unittest.IncorporatedResultSeal.Fixture(func(s *flow.IncorporatedResultSeal) {
				s.Header.Height = uint64(i + 1)
			})

			seals = append(seals, seal)
		}

		for i := 0; i < 100; i++ {
			s := seals[i]
			ok, err := pool.Add(s)
			require.NoError(t, err)
			require.True(t, ok)
		}

		err := pool.PruneUpToHeight(5)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			seal := seals[i]
			_, ok := pool.ByID(seal.ID())
			require.False(t, ok)
		}

		err = pool.PruneUpToHeight(10)
		require.NoError(t, err)
		for i := 0; i < 10; i++ {
			seal := seals[i]
			_, ok := pool.ByID(seal.ID())
			require.False(t, ok)
		}
	})
}
