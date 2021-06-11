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

		is := &unittest.IncorporatedResultSealFactory{}
		seal := is.Fixture()

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

		is := &unittest.IncorporatedResultSealFactory{}
		seals := make([]*flow.IncorporatedResultSeal, 0, 100)
		for i := 0; i < 100; i++ {
			seals = append(seals, is.Fixture(func(s *flow.IncorporatedResultSeal) {
				s.Header.Height = uint64(i + 1)
			}))
		}

		for i := 0; i < 100; i++ {
			s := seals[i]
			ok, err := pool.Add(s)
			require.NoError(t, err)
			require.True(t, ok)
		}

		pool.PruneByHeight(5)
		for i := 0; i < 5; i++ {
			seal := seals[i]
			_, ok := pool.ByID(seal.ID())
			require.False(t, ok)
		}

		pool.PruneByHeight(10)
		for i := 0; i < 10; i++ {
			seal := seals[i]
			_, ok := pool.ByID(seal.ID())
			require.False(t, ok)
		}
	})
}
