package leader

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

var someSeed = []uint8{0x6A, 0x23, 0x41, 0xB7, 0x80, 0xE1, 0x64, 0x59,
	0x6A, 0x53, 0x40, 0xB7, 0x80, 0xE4, 0x64, 0x5C,
	0x66, 0x53, 0x41, 0xB7, 0x80, 0xE1, 0x64, 0x51,
	0xAA, 0x53, 0x40, 0xB7, 0x80, 0xE4, 0x64, 0x50}

// We test that leader selection works for a committee of size one
func TestSingleConsensusNode(t *testing.T) {

	identity := unittest.IdentityFixture(unittest.WithWeight(8))
	rng := prg(t, someSeed)
	selection, err := ComputeLeaderSelection(0, rng, 10, []*flow.Identity{identity})
	require.NoError(t, err)
	for i := uint64(0); i < 10; i++ {
		leaderID, err := selection.LeaderForView(i)
		require.NoError(t, err)
		require.Equal(t, identity.NodeID, leaderID)
	}
}

func prg(t testing.TB, seed []byte) random.Rand {
	rng, err := random.NewChacha20PRG(seed, []byte("random"))
	require.NoError(t, err)
	return rng
}

// Test given the same seed, the leader selection will produce the same selection
func TestDeterministic(t *testing.T) {

	const N_VIEWS = 100
	const N_NODES = 4

	identities := unittest.IdentityListFixture(N_NODES)
	for i, identity := range identities {
		identity.Weight = uint64(i + 1)
	}
	rng := prg(t, someSeed)

	leaders1, err := ComputeLeaderSelection(0, rng, N_VIEWS, identities)
	require.NoError(t, err)

	rng = prg(t, someSeed)

	leaders2, err := ComputeLeaderSelection(0, rng, N_VIEWS, identities)
	require.NoError(t, err)

	for i := 0; i < N_VIEWS; i++ {
		l1, err := leaders1.LeaderForView(uint64(i))
		require.NoError(t, err)

		l2, err := leaders2.LeaderForView(uint64(i))
		require.NoError(t, err)

		require.Equal(t, l1, l2)
	}
}

func TestInputValidation(t *testing.T) {

	rng := prg(t, someSeed)

	// should return an error if we request to compute leader selection for <1 views
	t.Run("epoch containing no views", func(t *testing.T) {
		count := 0
		_, err := ComputeLeaderSelection(0, rng, count, unittest.IdentityListFixture(4))
		assert.Error(t, err)
		count = -1
		_, err = ComputeLeaderSelection(0, rng, count, unittest.IdentityListFixture(4))
		assert.Error(t, err)
	})

	// epoch with no possible leaders should return an error
	t.Run("epoch without participants", func(t *testing.T) {
		identities := unittest.IdentityListFixture(0)
		_, err := ComputeLeaderSelection(0, rng, 100, identities)
		assert.Error(t, err)
	})
}

// test that requesting a view outside the given range returns an error
func TestViewOutOfRange(t *testing.T) {

	rng := prg(t, someSeed)

	firstView := uint64(100)
	finalView := uint64(200)

	identities := unittest.IdentityListFixture(4)
	leaders, err := ComputeLeaderSelection(firstView, rng, int(finalView-firstView+1), identities)
	require.Nil(t, err)

	// confirm the selection has first/final view we expect
	assert.Equal(t, firstView, leaders.FirstView())
	assert.Equal(t, finalView, leaders.FinalView())

	// boundary views should not return error
	t.Run("boundary views", func(t *testing.T) {
		_, err = leaders.LeaderForView(firstView)
		assert.Nil(t, err)
		_, err = leaders.LeaderForView(finalView)
		assert.Nil(t, err)
	})

	// views before first view should return error
	t.Run("before first view", func(t *testing.T) {
		before := firstView - 1 // 1 before first view
		_, err = leaders.LeaderForView(before)
		assert.Error(t, err)

		before = rand.Uint64() % firstView // random view before first view
		_, err = leaders.LeaderForView(before)
		assert.Error(t, err)
	})

	// views after final view should return error
	t.Run("after final view", func(t *testing.T) {
		after := finalView + 1 // 1 after final view
		_, err = leaders.LeaderForView(after)
		assert.Error(t, err)

		after = finalView + uint64(rand.Uint32()) + 1 // random view after final view
		_, err = leaders.LeaderForView(after)
		assert.Error(t, err)
	})
}

func TestDifferentSeedWillProduceDifferentSelection(t *testing.T) {

	const N_VIEWS = 100
	const N_NODES = 4

	identities := unittest.IdentityListFixture(N_NODES)
	for i, identity := range identities {
		identity.Weight = uint64(i)
	}

	rng1 := prg(t, someSeed)

	seed2 := make([]byte, 32)
	seed2[0] = 8
	rng2 := prg(t, seed2)

	leaders1, err := ComputeLeaderSelection(0, rng1, N_VIEWS, identities)
	require.NoError(t, err)

	leaders2, err := ComputeLeaderSelection(0, rng2, N_VIEWS, identities)
	require.NoError(t, err)

	diff := 0
	for view := 0; view < N_VIEWS; view++ {
		l1, err := leaders1.LeaderForView(uint64(view))
		require.NoError(t, err)

		l2, err := leaders2.LeaderForView(uint64(view))
		require.NoError(t, err)

		if l1 != l2 {
			diff++
		}
	}

	require.True(t, diff > 0)
}

// given a random seed and certain weights, measure the chance each identity selected as leader.
// The number of time being selected as leader might not exactly match their weight, but also
// won't go too far from that.
func TestLeaderSelectionAreWeighted(t *testing.T) {
	rng := prg(t, someSeed)

	const N_VIEWS = 100000
	const N_NODES = 4

	identities := unittest.IdentityListFixture(N_NODES)
	for i, identity := range identities {
		identity.Weight = uint64(i + 1)
	}

	leaders, err := ComputeLeaderSelection(0, rng, N_VIEWS, identities)
	require.NoError(t, err)

	selected := make(map[flow.Identifier]uint64)
	for view := 0; view < N_VIEWS; view++ {
		nodeID, err := leaders.LeaderForView(uint64(view))
		require.NoError(t, err)

		selected[nodeID]++
	}

	fmt.Printf("selected for weights [1,2,3,4]: %v\n", selected)
	for nodeID, selectedCount := range selected {
		identity, ok := identities.ByNodeID(nodeID)
		require.True(t, ok)
		target := uint64(N_VIEWS) * identity.Weight / 10

		var diff uint64
		if selectedCount > target {
			diff = selectedCount - target
		} else {
			diff = target - selectedCount
		}

		// difference should be less than 2%
		stdDiff := N_VIEWS / 10 * 2 / 100
		require.Less(t, diff, uint64(stdDiff))
	}
}

func BenchmarkLeaderSelection(b *testing.B) {

	const N_VIEWS = 15000000
	const N_NODES = 20

	identities := make([]*flow.Identity, 0, N_NODES)
	for i := 0; i < N_NODES; i++ {
		identities = append(identities, unittest.IdentityFixture(unittest.WithWeight(uint64(i))))
	}
	rng := prg(b, someSeed)

	for n := 0; n < b.N; n++ {
		_, err := ComputeLeaderSelection(0, rng, N_VIEWS, identities)

		require.NoError(b, err)
	}
}

func TestInvalidTotalWeight(t *testing.T) {
	rng := prg(t, someSeed)
	identities := unittest.IdentityListFixture(4, unittest.WithWeight(0))
	_, err := ComputeLeaderSelection(0, rng, 10, identities)
	require.Error(t, err)
}

func TestZeroWeightNodeWillNotBeSelected(t *testing.T) {

	// create 2 RNGs from the same seed
	rng := prg(t, someSeed)
	rng_copy := prg(t, someSeed)

	// check that if there is some node with 0 weight, the selections for each view should be the same as
	// with no zero-weight nodes.
	t.Run("small dataset", func(t *testing.T) {
		const N_VIEWS = 100

		weightless := unittest.IdentityListFixture(5, unittest.WithWeight(0))
		weightful := unittest.IdentityListFixture(5)
		for i, identity := range weightful {
			identity.Weight = uint64(i + 1)
		}

		identities := append(weightless, weightful...)

		selectionFromAll, err := ComputeLeaderSelection(0, rng, N_VIEWS, identities)
		require.NoError(t, err)

		selectionFromWeightful, err := ComputeLeaderSelection(0, rng_copy, N_VIEWS, weightful)
		require.NoError(t, err)

		for i := 0; i < N_VIEWS; i++ {
			nodeIDFromAll, err := selectionFromAll.LeaderForView(uint64(i))
			require.NoError(t, err)

			nodeIDFromWeightful, err := selectionFromWeightful.LeaderForView(uint64(i))
			require.NoError(t, err)

			// the selection should be the same
			require.Equal(t, nodeIDFromAll, nodeIDFromWeightful)
		}
	})

	t.Run("fuzzy set", func(t *testing.T) {
		toolRng := prg(t, someSeed)

		// create 1002 nodes with all 0 weight
		identities := unittest.IdentityListFixture(1002, unittest.WithWeight(0))

		// create 2 nodes with 1 weight, and place them in between
		// index 233-777
		n := toolRng.UintN(777-233) + 233
		m := toolRng.UintN(777-233) + 233
		identities[n].Weight = 1
		identities[m].Weight = 1

		// the following code check the zero weight node should not be selected
		weightful := identities.Filter(filter.HasWeight(true))

		count := 1000
		selectionFromAll, err := ComputeLeaderSelection(0, rng, count, identities)
		require.NoError(t, err)

		selectionFromWeightful, err := ComputeLeaderSelection(0, rng_copy, count, weightful)
		require.NoError(t, err)

		for i := 0; i < count; i++ {
			nodeIDFromAll, err := selectionFromAll.LeaderForView(uint64(i))
			require.NoError(t, err)

			nodeIDFromWeightful, err := selectionFromWeightful.LeaderForView(uint64(i))
			require.NoError(t, err)

			// the selection should be the same
			require.Equal(t, nodeIDFromWeightful, nodeIDFromAll)
		}

		t.Run("if there is only 1 node has weight, then it will be always be the leader and the only leader", func(t *testing.T) {
			toolRng := prg(t, someSeed)

			identities := unittest.IdentityListFixture(1000, unittest.WithWeight(0))

			n := rng.UintN(1000)
			weight := n + 1
			identities[n].Weight = weight
			onlyNodeWithWeight := identities[n]

			selections, err := ComputeLeaderSelection(0, toolRng, 1000, identities)
			require.NoError(t, err)

			for i := 0; i < 1000; i++ {
				nodeID, err := selections.LeaderForView(uint64(i))
				require.NoError(t, err)
				require.Equal(t, onlyNodeWithWeight.NodeID, nodeID)
			}
		})
	})
}
