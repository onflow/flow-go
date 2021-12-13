package leader

import (
	"errors"
	"fmt"
	"math"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/model/flow"
)

const EstimatedSixMonthOfViews = 15000000 // 1 sec block time * 60 secs * 60 mins * 24 hours * 30 days * 6 months

// InvalidViewError is returned when a requested view is outside the pre-computed range.
type InvalidViewError struct {
	requestedView uint64 // the requested view
	firstView     uint64 // the first view we have pre-computed
	finalView     uint64 // the final view we have pre-computed
}

func (err InvalidViewError) Error() string {
	return fmt.Sprintf(
		"requested view (%d) outside of valid range [%d-%d]",
		err.requestedView, err.firstView, err.finalView,
	)
}

// IsInvalidViwError returns whether or not the input error is an invalid view error.
func IsInvalidViewError(err error) bool {
	return errors.As(err, &InvalidViewError{})
}

// LeaderSelection caches the pre-generated leader selections for a certain number of
// views starting from the epoch start view.
type LeaderSelection struct {

	// the ordered list of node IDs for all members of the current consensus committee
	memberIDs flow.IdentifierList

	// leaderIndexes caches pre-generated leader indices for the range
	// of views specified at construction, typically for an epoch
	//
	// The first value in this slice corresponds to the leader index at view
	// firstView, and so on
	leaderIndexes []uint16

	// The leader selection randomness varies for each epoch.
	// Leader selection only returns the correct leader selection for the corresponding epoch.
	// firstView specifies the start view of the current epoch
	firstView uint64
}

func (l LeaderSelection) FirstView() uint64 {
	return l.firstView
}

func (l LeaderSelection) FinalView() uint64 {
	return l.firstView + uint64(len(l.leaderIndexes)) - 1
}

// LeaderForView returns the node ID of the leader for a given view.
// Returns InvalidViewError if the view is outside the pre-computed range.
func (l LeaderSelection) LeaderForView(view uint64) (flow.Identifier, error) {
	if view < l.FirstView() {
		return flow.ZeroID, l.newInvalidViewError(view)
	}
	if view > l.FinalView() {
		return flow.ZeroID, l.newInvalidViewError(view)
	}

	viewIndex := int(view - l.firstView)      // index of leader index from view
	leaderIndex := l.leaderIndexes[viewIndex] // index of leader node ID from leader index
	leaderID := l.memberIDs[leaderIndex]      // leader node ID from leader index
	return leaderID, nil
}

func (l LeaderSelection) newInvalidViewError(view uint64) InvalidViewError {
	return InvalidViewError{
		requestedView: view,
		firstView:     l.FirstView(),
		finalView:     l.FinalView(),
	}
}

// ComputeLeaderSelectionFromSeed pre-generates a certain number of leader selections, and returns a
// leader selection instance for querying the leader indexes for certain views.
// firstView - the start view of the epoch, the generated leader selections start from this view.
// seed - the random seed for leader selection
// count - the number of leader selections to be pre-generated and cached.
// identities - the identities that contain the stake info, which is used as weight for the chance of
//							the identity to be selected as leader.
func ComputeLeaderSelectionFromSeed(firstView uint64, seed []byte, count int, identities flow.IdentityList) (*LeaderSelection, error) {

	if count < 1 {
		return nil, fmt.Errorf("number of views must be positive (got %d)", count)
	}

	weights := make([]uint64, 0, len(identities))
	for _, id := range identities {
		weights = append(weights, id.Stake)
	}

	leaders, err := WeightedRandomSelection(seed, count, weights)
	if err != nil {
		return nil, fmt.Errorf("could not select leader: %w", err)
	}

	return &LeaderSelection{
		memberIDs:     identities.NodeIDs(),
		leaderIndexes: leaders,
		firstView:     firstView,
	}, nil
}

// WeightedRandomSelection - given a seed and a given count, pre-generate the indexs of leader.
// The chance to be selected as leader is proportional to its weight.
// If an identity has 0 stake (weight is 0), it won't be selected as leader.
// This algorithm is essentially Fitness proportionate selection:
// See https://en.wikipedia.org/wiki/Fitness_proportionate_selection
func WeightedRandomSelection(seed []byte, count int, weights []uint64) ([]uint16, error) {
	// create random number generator from the seed
	rng, err := random.NewChacha20PRG(seed, []byte("leader_selec"))
	if err != nil {
		return nil, fmt.Errorf("can not create rng: %w", err)
	}

	if len(weights) == 0 {
		return nil, fmt.Errorf("weights is empty")
	}

	if len(weights) >= math.MaxUint16 {
		return nil, fmt.Errorf("number of possible leaders (%d) exceeds maximum (2^16-1)", len(weights))
	}

	// create an array of weight ranges for each identity.
	// an i-th identity is selected as the leader if the random number falls into its weight range.
	weightSums := make([]uint64, 0, len(weights))

	// cumulative sum of weights
	// after cumulating the weights, the sum is the total weight
	// total weight is used to specify the range of the random number.
	var cumsum uint64
	for _, weight := range weights {
		cumsum += weight
		weightSums = append(weightSums, cumsum)
	}

	if cumsum == 0 {
		return nil, fmt.Errorf("total weight must be greater than 0")
	}

	leaders := make([]uint16, 0, count)
	for i := 0; i < count; i++ {
		// pick a random number from 0 (inclusive) to cumsum (exclusive). Or [0, cumsum)
		randomness := rng.UintN(cumsum)

		// binary search to find the leader index by the random number
		leader := binarySearch(randomness, weightSums)

		leaders = append(leaders, uint16(leader))
	}
	return leaders, nil
}

// binary search to find the index of the first item in the given array that is
// strictly bigger to the given value.
// There are a few assumptions on inputs:
// - `arr` must be non-empty
// - items in `arr` must be in non-decreasing order
// - `value` must be less than the last item in `arr`
func binarySearch(value uint64, arr []uint64) int {
	return bsearch(0, len(arr)-1, value, arr)
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
