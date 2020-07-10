package committee

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/flow"
)

// LeaderSelection caches the pre-generated leader selections for a certain number of
// views starting from the epoch start view.
type LeaderSelection struct {
	// leaderIndexesForView caches leader selections that were pre-generated for a
	// certain number of views.
	leaderIndexesForView []int
	// The leader selection randomness varies for each epoch.
	// Leader selection only returns the correct leader selection for the corresponding epoch.
	// epochStartView specifies the start view of the current epoch
	epochStartView uint64
}

// LeaderIndexForView returns the leader index for given view.
// If the view is smaller than the epochStartView, an error will be returned.
func (l LeaderSelection) LeaderIndexForView(view uint64) (int, error) {
	if view < l.epochStartView {
		return 0, fmt.Errorf("view (%v) is smaller than the epochStartView (%v)", view, l.epochStartView)
	}

	index := int(view - l.epochStartView)
	if index >= len(l.leaderIndexesForView) {
		return 0, fmt.Errorf("view out of cached range: %v", view)
	}
	return l.leaderIndexesForView[index], nil
}

// ComputeLeaderSelectionFromSeed pre-generates a certain number of leader selections, and returns a
// leader selection instance for querying the leader indexes for certain views.
// epochStartView - the start view of the epoch, the generated leader selections start from this view.
// seed - the random seed for leader selection
// count - the number of leader selections to be pre-generated and cached.
// identities - the identities that contain the stake info, which is used as weight for the chance of
// 							the identity to be selected as leader.
func ComputeLeaderSelectionFromSeed(epochStartView uint64, seed []byte, count int, identities flow.IdentityList) (*LeaderSelection, error) {
	weights := make([]uint64, 0, len(identities))
	for _, id := range identities {
		weights = append(weights, id.Stake)
	}

	leaders, err := WeightedRandomSelection(seed, count, weights)
	if err != nil {
		return nil, fmt.Errorf("could not select leader: %w", err)
	}

	return &LeaderSelection{
		leaderIndexesForView: leaders,
		epochStartView:       epochStartView,
	}, nil
}

type indexedCumSum struct {
	index  int
	cumsum uint64
}

func (i indexedCumSum) value() uint64 {
	return i.cumsum
}

// WeightedRandomSelection - given a seed and a given count, pre-generate the indexs of leader.
// The chance to be selected as leader is proportional to its weight.
// This algorithm is essentially Fitness proportionate selection:
// See https://en.wikipedia.org/wiki/Fitness_proportionate_selection
func WeightedRandomSelection(seed []byte, count int, weights []uint64) ([]int, error) {
	// create random number generator from the seed
	rng, err := random.NewRand(seed)
	if err != nil {
		return nil, fmt.Errorf("can not create rng: %w", err)
	}

	if len(weights) == 0 {
		return nil, fmt.Errorf("weights is empty")
	}

	// create an array of weight ranges for each identity.
	// an i-th identity is selected as the leader if the random number falls into its weight range.
	weightSums := make([]*indexedCumSum, 0, len(weights))

	// cumulative sum of weights
	// after cumulating the weights, the sum is the total weight
	// total weight is used to specify the range of the random number.
	var cumsum uint64
	for i, weight := range weights {
		if weight > 0 {
			cumsum += weight
			weightSums = append(weightSums, &indexedCumSum{
				index:  i,
				cumsum: cumsum,
			})
		}
	}

	if cumsum == 0 {
		return nil, fmt.Errorf("total weight must be greater than 0")
	}

	leaders := make([]int, 0, count)
	for i := 0; i < count; i++ {
		// pick a random number from 0 (inclusive) to cumsum (exclusive). Or [0, cumsum)
		randomness, err := rng.IntN(int(cumsum))
		if err != nil {
			return nil, fmt.Errorf("could not generate randomness: %w", err)
		}

		// binary search to find the leader index by the random number
		itemIndex := binarySearch(uint64(randomness), weightSums)
		leader := weightSums[itemIndex].index

		leaders = append(leaders, leader)
	}
	return leaders, nil
}

// binary search to find the index of the first item in the given array that is
// strictly bigger to the given value
// There are a few assumptions on inputs:
// - `arr` must be non-empty
// - items in `arr` must be in increasing order
// - `value` must be less than the last item in `arr`
func binarySearch(value uint64, arr []*indexedCumSum) int {
	return bsearch(0, len(arr)-1, value, arr)
}

func bsearch(left, right int, value uint64, arr []*indexedCumSum) int {
	if left == right {
		return left
	}

	mid := (left + right) / 2
	if value < arr[mid].value() {
		return bsearch(left, mid, value, arr)
	}
	return bsearch(mid+1, right, value, arr)
}
