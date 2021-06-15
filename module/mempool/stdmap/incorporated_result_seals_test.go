package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"pgregory.net/rapid"
)

// icrSealsMachine is a description of a state machine for testing IncorporatedresultSeals
type icrSealsMachine struct {
	icrs  *IncorporatedResultSeals       // icrSeals being tested
	state []*flow.IncorporatedResultSeal // model of the icrSeals
	limit uint
}

// Init is an action for initializing  a icrSeals instance.
func (m *icrSealsMachine) Init(t *rapid.T) {
	n := uint(rapid.IntRange(1, 1000).Draw(t, "n").(int))
	m.icrs = NewIncorporatedResultSeals(n)
	m.limit = n
}

// Add is a conditional action which adds an item to the icrSeals.
func (m *icrSealsMachine) Add(t *rapid.T) {
	i := rapid.Int().Draw(t, "i").(int)

	seal := unittest.IncorporatedResultSeal.Fixture(func(s *flow.IncorporatedResultSeal) {
		s.Header.Height = uint64(i)
	})

	_, err := m.icrs.Add(seal)
	require.NoError(t, err)

	unmet := true
	for _, v := range m.state {
		if v.ID() == seal.ID() {
			unmet = false
		}
	}

	if unmet && seal.Header.Height >= m.icrs.lowestHeight {
		m.state = append(m.state, seal)
		if len(m.state) > int(m.limit) {
			// we remove one of the max height elements if we're beyond the mempool limit
			max_height := m.icrs.lowestHeight
			for _, v := range m.state {
				if v.Header.Height > max_height {
					max_height = v.Header.Height
				}
			}
			filtered_state := make([]*flow.IncorporatedResultSeal, 0)
			for _, v := range m.state {
				if v.Header.Height < max_height {
					filtered_state = append(filtered_state, v)
				} else {
					_, still_there := m.icrs.ByID(v.ID())
					if still_there {
						filtered_state = append(filtered_state, v)
					}
				}
			}
			m.state = filtered_state
		}
	}
}

func (m *icrSealsMachine) Get(t *rapid.T) {
	n := len(m.state)
	// skip if the store is empty
	if n > 0 {
		i := rapid.IntRange(0, n-1).Draw(t, "i").(int)

		s := m.state[i]
		actual, ok := m.icrs.ByID(s.ID())
		require.True(t, ok)
		require.Equal(t, s, actual)
	}
}

func (m *icrSealsMachine) Rem(t *rapid.T) {
	n := len(m.state)
	// skip if the store is empty
	if n > 0 {
		i := rapid.IntRange(0, n-1).Draw(t, "i").(int)

		s := m.state[i]
		ok := m.icrs.Rem(s.ID())
		require.True(t, ok)

		// remove m[i], we don't care about ordering here
		m.state[n-1], m.state[i] = m.state[i], m.state[n-1]
		m.state = m.state[:n-1]
	}
}

// Check runs after every action and verifies that all required invariants hold.
func (m *icrSealsMachine) Check(t *rapid.T) {
	if int(m.icrs.Size()) != len(m.state) {
		t.Fatalf("store size mismatch: %v vs expected %v, %v", m.icrs.Size(), len(m.state), m.icrs.All())
	}
	assert.ElementsMatch(t, m.icrs.All(), m.state)
}

// Run the icrSeals state machine and test it against its model
func TestIcrs(t *testing.T) {
	rapid.Check(t, rapid.Run(&icrSealsMachine{}))
}

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
