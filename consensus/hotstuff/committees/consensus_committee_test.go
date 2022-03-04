package committees

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/seed"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// TestConsensus_InvalidSigner tests that the appropriate sentinel error is
// returned by the hotstuff.Committee implementation for non-existent or
// non-committee identities.
func TestConsensus_InvalidSigner(t *testing.T) {

	realIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	zeroWeightConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithWeight(0))
	ejectedConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithEjected(true))
	validNonConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	fakeID := unittest.IdentifierFixture()
	blockID := unittest.IdentifierFixture()

	state := new(protocolmock.State)
	snapshot := new(protocolmock.Snapshot)

	// create a mock epoch for leader selection setup in constructor
	currEpoch := newMockEpoch(
		1,
		unittest.IdentityListFixture(10),
		1,
		100,
		unittest.SeedFixture(seed.RandomSourceLength),
	)
	epochs := mocks.NewEpochQuery(t, 1, currEpoch)
	snapshot.On("Epochs").Return(epochs)

	state.On("Final").Return(snapshot)
	state.On("AtBlockID", blockID).Return(snapshot)

	snapshot.On("Identity", realIdentity.NodeID).Return(realIdentity, nil)
	snapshot.On("Identity", zeroWeightConsensusIdentity.NodeID).Return(zeroWeightConsensusIdentity, nil)
	snapshot.On("Identity", ejectedConsensusIdentity.NodeID).Return(ejectedConsensusIdentity, nil)
	snapshot.On("Identity", validNonConsensusIdentity.NodeID).Return(validNonConsensusIdentity, nil)
	snapshot.On("Identity", fakeID).Return(nil, protocol.IdentityNotFoundError{})

	com, err := NewConsensusCommittee(state, unittest.IdentifierFixture())
	require.NoError(t, err)

	t.Run("non-existent identity should return InvalidSignerError", func(t *testing.T) {
		_, err := com.Identity(blockID, fakeID)
		require.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("existent but non-committee-member identity should return InvalidSignerError", func(t *testing.T) {
		t.Run("zero-weight consensus node", func(t *testing.T) {
			_, err := com.Identity(blockID, zeroWeightConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("ejected consensus node", func(t *testing.T) {
			_, err := com.Identity(blockID, ejectedConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("otherwise valid non-consensus node", func(t *testing.T) {
			_, err := com.Identity(blockID, validNonConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})
	})

	t.Run("should be able to retrieve real identity", func(t *testing.T) {
		actual, err := com.Identity(blockID, realIdentity.NodeID)
		require.NoError(t, err)
		require.Equal(t, realIdentity, actual)
	})
}

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
		unittest.SeedFixture(seed.RandomSourceLength),
	)
	currEpoch := newMockEpoch(
		epochCounter,
		identities,
		101,
		200,
		unittest.SeedFixture(32),
	)

	state.On("Final").Return(snapshot)
	epochs := mocks.NewEpochQuery(t, epochCounter, prevEpoch, currEpoch)
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
			t.SkipNow()
			// REASON FOR SKIPPING TEST:
			// We have a temporary fallback to continue with the current consensus committee, if the
			// setup for the next epoch failed (aka emergency epoch chain continuation -- EECC).
			// This test covers with behaviour _without_ EECC and is therefore skipped.
			// The behaviour _with EECC_ is covered by the following test:
			// "after current epoch - with emergency epoch chain continuation"
			// TODO: for the mature implementation, remove EECC, enable this test, and remove the following test

			// get leader for view in next epoch when it is not set up yet
			_, err := committee.LeaderForView(250)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, protocol.ErrNextEpochNotSetup))
		})

		t.Run("after current epoch - with emergency epoch chain continuation", func(t *testing.T) {
			// This test covers the TEMPORARY emergency epoch chain continuation (EECC) fallback
			// TODO: for the mature implementation, remove this test,
			//       enable the previous test "after current epoch"

			// get leader for view in next epoch when it is not set up yet
			_, err := committee.LeaderForView(250)
			// emergency epoch chain continuation should kick in and return a valid leader
			assert.NoError(t, err)
		})
	})

	// now, add a valid next epoch
	nextEpoch := newMockEpoch(
		epochCounter+1,
		identities,
		201,
		300,
		unittest.SeedFixture(seed.RandomSourceLength),
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

	epoch1 := newMockEpoch(currentEpochCounter, identities, 1, epochFinalView, unittest.SeedFixture(seed.RandomSourceLength))

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
		nextEpoch := newMockEpoch(currentEpochCounter, identities, firstView, epochFinalView, unittest.SeedFixture(seed.RandomSourceLength))
		epochQuery.Add(nextEpoch)

		// query a view from the new epoch
		_, err = committee.LeaderForView(firstView)
		require.NoError(t, err)
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
	// return nil error to indicate the epoch is committed
	epoch.On("DKG").Return(nil, nil)

	epoch.On("RandomSource").Return(seed, nil)
	return epoch
}
