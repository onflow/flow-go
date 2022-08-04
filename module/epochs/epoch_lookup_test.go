package epochs

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// newMockEpoch returns a mock epoch with the given fields set.
func newMockEpoch(counter, firstView, finalView uint64) *mockprotocol.Epoch {
	epoch := new(mockprotocol.Epoch)
	epoch.On("FirstView").Return(firstView, nil)
	epoch.On("FinalView").Return(finalView, nil)
	epoch.On("Counter").Return(counter, nil)
	return epoch
}

// setupMockState returns a mock protocol state with the given current epoch counter and epochs.
func setupMockState(t *testing.T, counter uint64, epochFallbackTriggered bool, epochs ...epochRange) *mockprotocol.State {
	epochQuery := mocks.NewEpochQuery(t, counter)
	phase := flow.EpochPhaseStaking
	for _, epoch := range epochs {
		mockEpoch := newMockEpoch(epoch.counter, epoch.firstView, epoch.finalView)
		epochQuery.Add(mockEpoch)
		// if we add a next epoch (counter 1 greater than current), then set phase to committed
		if epoch.counter == counter+1 {
			phase = flow.EpochPhaseCommitted
		}
	}

	snapshot := new(mockprotocol.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	snapshot.On("Phase").Return(phase, nil)

	params := new(mockprotocol.Params)
	params.On("EpochFallbackTriggered").Return(epochFallbackTriggered, nil)

	state := new(mockprotocol.State)
	state.On("Final").Return(snapshot)
	state.On("Params").Return(params)

	return state
}

// TestEpochLookup_EpochForViewWithFallback tests constructing and subsequently querying
// EpochLookup instances under various initial circumstances.
func TestEpochLookup_EpochForViewWithFallback(t *testing.T) {
	currentEpochCounter := uint64(1)
	prevEpoch := epochRange{counter: currentEpochCounter - 1, firstView: 100, finalView: 199}
	currEpoch := epochRange{counter: currentEpochCounter, firstView: 200, finalView: 299}
	nextEpoch := epochRange{counter: currentEpochCounter + 1, firstView: 300, finalView: 399}

	t.Run("current epoch", func(t *testing.T) {
		epochs := []epochRange{currEpoch}
		state := setupMockState(t, currentEpochCounter, false, epochs...)
		lookup, err := NewEpochLookup(state)
		require.NoError(t, err)
		testEpochForViewWithFallback(t, lookup, state, epochs...)
	})

	t.Run("previous/current epoch", func(t *testing.T) {
		epochs := []epochRange{prevEpoch, currEpoch}
		state := setupMockState(t, currentEpochCounter, false, epochs...)
		lookup, err := NewEpochLookup(state)
		require.NoError(t, err)
		testEpochForViewWithFallback(t, lookup, state, epochs...)
	})

	t.Run("current/next epoch", func(t *testing.T) {
		epochs := []epochRange{currEpoch, nextEpoch}
		state := setupMockState(t, currentEpochCounter, false, epochs...)
		lookup, err := NewEpochLookup(state)
		require.NoError(t, err)
		testEpochForViewWithFallback(t, lookup, state, epochs...)
	})

	t.Run("previous/current/next epoch", func(t *testing.T) {
		epochs := []epochRange{prevEpoch, currEpoch, nextEpoch}
		state := setupMockState(t, currentEpochCounter, false, epochs...)
		lookup, err := NewEpochLookup(state)
		require.NoError(t, err)
		testEpochForViewWithFallback(t, lookup, state, epochs...)
	})

	t.Run("epoch fallback triggered", func(t *testing.T) {
		epochs := []epochRange{prevEpoch, currEpoch, nextEpoch}
		state := setupMockState(t, currentEpochCounter, true, epochs...)

		lookup, err := NewEpochLookup(state)
		require.NoError(t, err)
		testEpochForViewWithFallback(t, lookup, state, epochs...)
	})
}

// testEpochForViewWithFallback accepts a constructed EpochLookup and state, and
// validates correctness by issuing various queries, using the input state and
// epochs as source of truth.
func testEpochForViewWithFallback(t *testing.T, lookup *EpochLookup, state protocol.State, epochs ...epochRange) {
	epochFallbackTriggered, err := state.Params().EpochFallbackTriggered()
	require.NoError(t, err)

	t.Run("should have set epoch fallback triggered correctly", func(t *testing.T) {
		assert.Equal(t, epochFallbackTriggered, lookup.epochFallbackIsTriggered.Load())
	})

	t.Run("should be able to query within any committed epoch", func(t *testing.T) {
		for _, epoch := range epochs {
			counter, err := lookup.EpochForViewWithFallback(randUint64(epoch.firstView, epoch.finalView))
			assert.NoError(t, err)
			assert.Equal(t, epoch.counter, counter)
		}
	})

	t.Run("should return ErrViewForUnknownEpoch below earliest epoch", func(t *testing.T) {
		_, err := lookup.EpochForViewWithFallback(randUint64(0, epochs[0].firstView-1))
		assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
	})

	// if epoch fallback is triggered, fallback to returning latest epoch counter
	// otherwise return ErrViewForUnknownEpoch
	if epochFallbackTriggered {
		t.Run("should use fallback logic for queries above latest epoch when epoch fallback is triggered", func(t *testing.T) {
			counter, err := lookup.EpochForViewWithFallback(epochs[len(epochs)-1].finalView + 1)
			assert.NoError(t, err)
			// should fallback to returning the counter for the latest epoch
			assert.Equal(t, epochs[len(epochs)-1].counter, counter)
		})
	} else {
		t.Run("should return ErrViewForUnknownEpoch for queries above latest epoch when epoch fallback is not triggered", func(t *testing.T) {
			_, err := lookup.EpochForViewWithFallback(epochs[len(epochs)-1].finalView + 1)
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
	}
}

// TODO test handling epoch events

// randUint64 returns a uint64 in [min,max]
func randUint64(min, max uint64) uint64 {
	return min + uint64(rand.Intn(int(max)+1-int(min)))
}
