package committee

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

var someSeed = []uint8{0x6A, 0x23, 0x41, 0xB7, 0x80, 0xE1, 0x64, 0x59,
	0x6A, 0x53, 0x40, 0xB7, 0x80, 0xE4, 0x64, 0x5C,
	0x66, 0x53, 0x41, 0xB7, 0x80, 0xE1, 0x64, 0x51,
	0xAA, 0x53, 0x40, 0xB7, 0x80, 0xE4, 0x64, 0x50}

// Test binary search implementation
func TestBsearch(t *testing.T) {
	stakes := []uint64{1, 2, 3, 4}
	var sum uint64
	sums := make([]uint64, 0)
	for i := 0; i < len(stakes); i++ {
		sum += stakes[i]
		sums = append(sums, sum)
	}
	sel := make([]int, 0, 10)
	for i := 0; i < 10; i++ {
		index := binarySearch(uint64(i), sums)
		sel = append(sel, index)
	}
	require.Equal(t, []int{0, 1, 1, 2, 2, 2, 3, 3, 3, 3}, sel)
}

// compare the result of binary search with the brute force search,
// should be the same.
func TestBsearchWithNormalSearch(t *testing.T) {
	count := 100
	sums := make([]uint64, 0, count)
	sum := 0
	for i := 0; i < count; i++ {
		sum += i
		sums = append(sums, uint64(sum))
	}

	var value uint64
	total := sums[len(sums)-1]
	for value = 0; value < total; value++ {
		expected, err := bruteSearch(value, sums)
		require.NoError(t, err)

		actual := binarySearch(value, sums)
		require.NoError(t, err)

		require.Equal(t, expected, actual)
	}
}

func bruteSearch(value uint64, arr []uint64) (int, error) {
	// value ranges from [arr[0], arr[len(arr) -1 ]) exclusive
	for i, a := range arr {
		if a > value {
			return i, nil
		}
	}
	return 0, fmt.Errorf("not found")
}

// Test given the same seed, the leader selection will produce the same selection
func TestDeterminstic(t *testing.T) {
	identities := []*flow.Identity{
		&flow.Identity{Stake: 1},
		&flow.Identity{Stake: 2},
		&flow.Identity{Stake: 3},
		&flow.Identity{Stake: 4},
	}

	count := 100

	leaders1, err := ComputeLeaderSelectionFromSeed(0, someSeed, count, identities)
	require.NoError(t, err)

	leaders2, err := ComputeLeaderSelectionFromSeed(0, someSeed, count, identities)
	require.NoError(t, err)

	for i := 0; i < count; i++ {
		l1, err := leaders1.LeaderIndexForView(uint64(i))
		require.NoError(t, err)

		l2, err := leaders2.LeaderIndexForView(uint64(i))
		require.NoError(t, err)

		require.Equal(t, l1, l2)
	}
}

func TestDifferentSeedWillProduceDifferentSelection(t *testing.T) {
	identities := []*flow.Identity{
		&flow.Identity{Stake: 1},
		&flow.Identity{Stake: 2},
		&flow.Identity{Stake: 3},
		&flow.Identity{Stake: 4},
	}

	count := 100

	seed1 := make([]byte, 16)
	seed1[0] = 34

	seed2 := make([]byte, 16)
	seed2[0] = 8

	leaders1, err := ComputeLeaderSelectionFromSeed(0, seed1, count, identities)
	require.NoError(t, err)

	leaders2, err := ComputeLeaderSelectionFromSeed(0, seed2, count, identities)
	require.NoError(t, err)

	diff := 0
	for view := 0; view < count; view++ {
		l1, err := leaders1.LeaderIndexForView(uint64(view))
		require.NoError(t, err)

		l2, err := leaders2.LeaderIndexForView(uint64(view))
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
	count := 100000
	identities := []*flow.Identity{
		&flow.Identity{Stake: 1},
		&flow.Identity{Stake: 2},
		&flow.Identity{Stake: 3},
		&flow.Identity{Stake: 4},
	}

	leaders, err := ComputeLeaderSelectionFromSeed(0, someSeed, count, identities)
	require.NoError(t, err)

	selected := make([]uint64, 4)
	for view := 0; view < count; view++ {
		index, err := leaders.LeaderIndexForView(uint64(view))
		require.NoError(t, err)

		selected[index]++
	}

	fmt.Printf("selected for weights [1,2,3,4]: %v\n", selected)
	for i, selectedCount := range selected {
		target := uint64(count) * identities[i].Stake / 10

		var diff uint64
		if selectedCount > target {
			diff = selectedCount - target
		} else {
			diff = target - selectedCount
		}

		// difference should be less than 2%
		stdDiff := count / 10 * 2 / 100
		require.Less(t, diff, uint64(stdDiff))
	}
}

func BenchmarkLeaderSelection(b *testing.B) {
	nodes := 20
	identities := make([]*flow.Identity, 0, nodes)

	for i := 0; i < nodes; i++ {
		identities = append(identities, &flow.Identity{
			Stake: uint64(i),
		})
	}

	for n := 0; n < b.N; n++ {
		_, err := ComputeLeaderSelectionFromSeed(0, someSeed, 15000000, identities)

		require.NoError(b, err)
	}
}

func TestInvalidTotalWeight(t *testing.T) {
	identities := []*flow.Identity{
		&flow.Identity{Stake: 0},
		&flow.Identity{Stake: 0},
		&flow.Identity{Stake: 0},
		&flow.Identity{Stake: 0},
	}
	_, err := ComputeLeaderSelectionFromSeed(0, someSeed, 10, identities)
	require.Error(t, err)
}
