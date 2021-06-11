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
		pool := NewIncorporatedResultSeals(1000)

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
		pool := NewIncorporatedResultSeals(1000)

		seals := make([]*flow.IncorporatedResultSeal, 0, 100)
		for i := 0; i < 100; i++ {
			seal := unittest.IncorporatedResultSeal.Fixture(func(s *flow.IncorporatedResultSeal) {
				s.Header.Height = uint64(i)
			})

			seals = append(seals, seal)
		}

		for _, seal := range seals {
			ok, err := pool.Add(seal)
			require.NoError(t, err)
			require.True(t, ok, "seal at height %d was not added", seal.Header.Height)
		}
		verifyPresent(t, pool, seals...)

		err := pool.PruneUpToHeight(5)
		require.NoError(t, err)
		verifyAbsent(t, pool, seals[:5]...)
		verifyPresent(t, pool, seals[5:]...)

		err = pool.PruneUpToHeight(10)
		require.NoError(t, err)
		verifyAbsent(t, pool, seals[:10]...)
		verifyPresent(t, pool, seals[10:]...)
	})

	t.Run("prune with some nonexisting heights", func(t *testing.T) {
		pool := NewIncorporatedResultSeals(1000)

		// create seals, but NOT for heights 2 and 3
		seals := make([]*flow.IncorporatedResultSeal, 0, 6)
		for _, h := range []uint64{0, 1, 4, 5, 6, 7} {
			seal := unittest.IncorporatedResultSeal.Fixture(func(s *flow.IncorporatedResultSeal) {
				s.Header.Height = h
			})
			seals = append(seals, seal)
			ok, err := pool.Add(seal)
			require.NoError(t, err)
			require.True(t, ok)
		}

		err := pool.PruneUpToHeight(5)
		require.NoError(t, err)
		verifyAbsent(t, pool, seals[:3]...)
		verifyPresent(t, pool, seals[3:]...)
	})

	t.Run("test ejection", func(t *testing.T) {
		pool := NewIncorporatedResultSeals(3)

		seals := make([]*flow.IncorporatedResultSeal, 0, 6)
		for _, h := range []uint64{7, 10, 5, 12, 8} {
			seal := unittest.IncorporatedResultSeal.Fixture(func(s *flow.IncorporatedResultSeal) {
				s.Header.Height = h
			})
			seals = append(seals, seal)
			ok, err := pool.Add(seal)
			require.NoError(t, err)
			require.True(t, ok)
		}

		verifyPresent(t, pool, seals[0], seals[2], seals[4])
		verifyAbsent(t, pool, seals[1], seals[3])
	})
}

func verifyPresent(t *testing.T, pool *IncorporatedResultSeals, seals ...*flow.IncorporatedResultSeal) {
	for _, seal := range seals {
		_, ok := pool.ByID(seal.ID())
		require.True(t, ok, "seal at height %d should be in mempool", seal.Header.Height)
	}
}

func verifyAbsent(t *testing.T, pool *IncorporatedResultSeals, seals ...*flow.IncorporatedResultSeal) {
	for _, seal := range seals {
		_, ok := pool.ByID(seal.ID())
		require.False(t, ok, "seal at height %d should not be in mempool", seal.Header.Height)
	}
}
