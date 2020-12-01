package committees

import (
	"errors"
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
	state := new(protocolmock.ReadOnlyState)
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

	// counter for the current epoch
	epochCounter := uint64(1)

	// create mocks
	state := new(protocolmock.ReadOnlyState)
	snapshot := new(protocolmock.Snapshot)

	epoch1 := newMockEpoch(
		epochCounter,
		identities,
		1,
		100,
		unittest.SeedFixture(32),
	)
	epoch2 := newMockEpoch(
		epochCounter+1,
		identities,
		101,
		200,
		unittest.SeedFixture(32),
	)
	epoch3 := newMockEpoch(
		epochCounter+2,
		identities,
		201,
		300,
		unittest.SeedFixture(32),
	)
	epoch4 := newMockEpoch(
		epochCounter+3,
		identities,
		301,
		400,
		unittest.SeedFixture(32),
	)

	state.On("Final").Return(snapshot)
	epochs := mocks.NewEpochQuery(t, epochCounter, epoch1, epoch2, epoch3, epoch4)
	snapshot.On("Epochs").Return(epochs)

	committee, err := NewConsensusCommittee(state, me)
	require.Nil(t, err)

	// we should start with only current epoch (epoch 1) pre-computed
	// since there is no previous epoch
	assert.Equal(t, 1, len(committee.leaders))

	// when we first request a view within epoch 2, we should not remove
	// any old epochs since we have 2 (1 & 2)
	_, err = committee.LeaderForView(101)
	t.Run("first transition", func(t *testing.T) {
		assert.Nil(t, err)
		assert.Equal(t, 2, len(committee.leaders))
	})

	// transition so epoch 2 is current
	epochs.Transition()
	// request a view in epoch 3
	_, err = committee.LeaderForView(201)
	assert.Nil(t, err)

	// when we request a view within epoch 3, we should also not remove
	// any old epochs since we have 3 (the maximum)
	t.Run("second transition", func(t *testing.T) {

		// should have 3 epochs stored
		assert.Equal(t, 3, len(committee.leaders))

		// should have leader selection for epochs 1-3
		_, has1 := committee.leaders[epochCounter]
		_, has2 := committee.leaders[epochCounter+1]
		_, has3 := committee.leaders[epochCounter+2]
		assert.True(t, has1)
		assert.True(t, has2)
		assert.True(t, has3)
	})

	// transition so epoch 3 is current
	epochs.Transition()
	// request a view in epoch 4
	_, err = committee.LeaderForView(301)
	assert.Nil(t, err)

	t.Run("third transition", func(t *testing.T) {

		// should have 3 epochs stored (1 has been deleted)
		assert.Equal(t, 3, len(committee.leaders))

		// should have leader selection for epochs 2-4 (no 1)
		_, has1 := committee.leaders[epochCounter]
		_, has2 := committee.leaders[epochCounter+1]
		_, has3 := committee.leaders[epochCounter+2]
		_, has4 := committee.leaders[epochCounter+3]
		assert.False(t, has1)
		assert.True(t, has2)
		assert.True(t, has3)
		assert.True(t, has4)
	})
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
