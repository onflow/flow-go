package committees

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/indices"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// test that LeaderForView returns a valid leader for the previous and current
// epoch and that it returns the appropriate sentinel for the next epoch if it
// is not yet ready
func TestConsensus_LeaderForView(t *testing.T) {

	identities := unittest.IdentityListFixture(10)
	me := identities[0].NodeID

	// the counter for the current epoch
	epochCounter := uint64(2)

	// create mocks
	state := new(protocolmock.State)
	snapshot := new(protocolmock.Snapshot)

	prevEpoch := newMockEpoch(
		epochCounter-1,
		identities,
		1,
		100,
		unittest.SeedFixture(32),
	)
	currEpoch := newMockEpoch(
		epochCounter,
		identities,
		101,
		200,
		unittest.SeedFixture(32),
	)

	state.On("Final").Return(snapshot)
	epochs := mocks.NewEpochQuery(t, 2, prevEpoch, currEpoch)
	snapshot.On("Epochs").Return(epochs)

	committee, err := NewConsensusCommittee(state, me)
	require.Nil(t, err)

	t.Run("next epoch not ready", func(t *testing.T) {
		t.Run("previous epoch", func(t *testing.T) {
			// get leader for view in previous epoch
			leaderID, err := committee.LeaderForView(50)
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("current epoch", func(t *testing.T) {
			// get leader for view in current epoch
			leaderID, err := committee.LeaderForView(150)
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("after current epoch", func(t *testing.T) {
			// get leader for view in next epoch when it is not set up yet
			_, err := committee.LeaderForView(250)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, protocol.ErrNextEpochNotSetup))
		})
	})

	// now, add a valid next epoch
	nextEpoch := newMockEpoch(
		epochCounter+1,
		identities,
		201,
		300,
		unittest.SeedFixture(32),
	)
	epochs.Add(nextEpoch)

	t.Run("next epoch ready", func(t *testing.T) {
		t.Run("previous epoch", func(t *testing.T) {
			// get leader for view in previous epoch
			leaderID, err := committee.LeaderForView(50)
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("current epoch", func(t *testing.T) {
			// get leader for view in current epoch
			leaderID, err := committee.LeaderForView(150)
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("next epoch", func(t *testing.T) {
			// get leader for view in next epoch after it has been set up
			leaderID, err := committee.LeaderForView(250)
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})
	})
}

func TestRemoveOldEpochs(t *testing.T) {

	identities := unittest.IdentityListFixture(10)
	me := identities[0].NodeID

	// keep track of epoch counter and views
	firstEpochCounter := uint64(1)
	currentEpochCounter := firstEpochCounter
	epochFinalView := uint64(100)

	epoch1 := newMockEpoch(currentEpochCounter, identities, 1, epochFinalView, unittest.SeedFixture(32))

	// create mocks
	state := new(protocolmock.State)
	snapshot := new(protocolmock.Snapshot)

	state.On("Final").Return(snapshot)
	epochQuery := mocks.NewEpochQuery(t, currentEpochCounter, epoch1)
	snapshot.On("Epochs").Return(epochQuery)

	committee, err := NewConsensusCommittee(state, me)
	require.Nil(t, err)

	// we should start with only current epoch (epoch 1) pre-computed
	// since there is no previous epoch
	assert.Equal(t, 1, len(committee.leaders))

	// test for 10 epochs
	for currentEpochCounter < 10 {

		// add another epoch
		firstView := epochFinalView + 1
		epochFinalView = epochFinalView + 100
		currentEpochCounter++
		nextEpoch := newMockEpoch(currentEpochCounter, identities, firstView, epochFinalView, unittest.SeedFixture(32))
		epochQuery.Add(nextEpoch)

		// query a view from the new epoch
		_, err = committee.LeaderForView(firstView)
		// transition to the next epoch
		epochQuery.Transition()

		t.Run(fmt.Sprintf("epoch %d", currentEpochCounter), func(t *testing.T) {
			// check we have the right number of epochs stored
			if currentEpochCounter <= 3 {
				assert.Equal(t, int(currentEpochCounter), len(committee.leaders))
			} else {
				assert.Equal(t, 3, len(committee.leaders))
			}

			// check we have the correct epochs stored
			for i := uint64(0); i < 3; i++ {
				counter := currentEpochCounter - i
				if counter < firstEpochCounter {
					break
				}
				_, exists := committee.leaders[counter]
				assert.True(t, exists, "missing epoch with counter %d max counter is %d", counter, currentEpochCounter)
			}
		})
	}
}

func newMockEpoch(
	counter uint64,
	identities flow.IdentityList,
	firstView uint64,
	finalView uint64,
	seed []byte,
) *protocolmock.Epoch {

	epoch := new(protocolmock.Epoch)
	epoch.On("Counter").Return(counter, nil)
	epoch.On("InitialIdentities").Return(identities, nil)
	epoch.On("FirstView").Return(firstView, nil)
	epoch.On("FinalView").Return(finalView, nil)

	var params []interface{}
	for _, ind := range indices.ProtocolConsensusLeaderSelection {
		params = append(params, ind)
	}
	epoch.On("Seed", params...).Return(seed, nil)

	return epoch
}
