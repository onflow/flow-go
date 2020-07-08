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

// WeightedRandomSelection - given a seed and a given count, pre-generate the indexs of leader.
// The chance to be selected as leader is proportional to its weight.
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
	weightSum := make([]uint64, 0, len(weights))
	var sum uint64
	for _, weight := range weights {
		sum += weight
		weightSum = append(weightSum, sum)
	}

	// after accumulating the weights, the last item is the total weight
	// total weight is used to specify the range of the random number.
	totalWeight := weightSum[len(weightSum)-1]

	leaders := make([]int, 0, count)
	for i := 0; i < count; i++ {
		// make a random number by specifying the range.
		// the range should be from 0 (inclusive) to totalWeight (exclusive). Or [0, totalWeight)
		randomness, err := rng.IntN(int(totalWeight))
		if err != nil {
			return nil, fmt.Errorf("could not generate randomness: %w", err)
		}

		// binary search to find the leader index by the random number
		leader, err := binarySearch(uint64(randomness), weightSum)
		if err != nil {
			return nil, fmt.Errorf("could not generate leader index: %w", err)
		}

		leaders = append(leaders, leader)
	}
	return leaders, nil
}

// binary search to find the index of the first item in the given array that is
// bigger or equal to the given value.
func binarySearch(value uint64, arr []uint64) (int, error) {
	if len(arr) == 0 {
		return 0, fmt.Errorf("arr is empty")
	}

	if value >= arr[len(arr)-1] {
		return 0, fmt.Errorf("value (%v) exceeds the search range: [0, %v)", value, arr[len(arr)-1])
	}

	return bsearch(0, len(arr)-1, value, arr), nil
}

func bsearch(left, right int, value uint64, arr []uint64) int {
	if left == right {
		return left
	}

	mid := (left + right) / 2
	if value < arr[mid] {
		return bsearch(left, mid, value, arr)
	}
	return bsearch(mid+1, right, value, arr)
}
