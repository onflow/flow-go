package committees

import (
	"errors"
	"fmt"
	"math/rand"
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

// TestConsensus_IdentitiesByBlock tests retrieving committee members by block.
// * should use up-to-block committee information
// * should exclude non-committee members
func TestConsensus_IdentitiesByBlock(t *testing.T) {

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
		unittest.IdentityListFixture(10), // static initial identities should not be used
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
		_, err := com.IdentityByBlock(blockID, fakeID)
		require.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("existent but non-committee-member identity should return InvalidSignerError", func(t *testing.T) {
		t.Run("zero-weight consensus node", func(t *testing.T) {
			_, err := com.IdentityByBlock(blockID, zeroWeightConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("ejected consensus node", func(t *testing.T) {
			_, err := com.IdentityByBlock(blockID, ejectedConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("otherwise valid non-consensus node", func(t *testing.T) {
			_, err := com.IdentityByBlock(blockID, validNonConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})
	})

	t.Run("should be able to retrieve real identity", func(t *testing.T) {
		actual, err := com.IdentityByBlock(blockID, realIdentity.NodeID)
		require.NoError(t, err)
		require.Equal(t, realIdentity, actual)
	})
}

// TestConsensus_IdentitiesByEpoch tests that identities can be queried by epoch.
// * should use static epoch info (initial identities)
// * should exclude non-committee members
// * should correctly map views to epochs
// * should return ErrViewForUnknownEpoch sentinel for unknown epochs
func TestConsensus_IdentitiesByEpoch(t *testing.T) {

	// epoch 1 identities with varying conditions which would disqualify them
	// from committee participation
	realIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	zeroWeightConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithWeight(0))
	ejectedConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithEjected(true))
	validNonConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	epoch1Identities := flow.IdentityList{realIdentity, zeroWeightConsensusIdentity, ejectedConsensusIdentity, validNonConsensusIdentity}

	// a single epoch 2 identity
	epoch2Identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	epoch2Identities := flow.IdentityList{epoch2Identity}

	state := new(protocolmock.State)
	snapshot := new(protocolmock.Snapshot)
	state.On("Final").Return(snapshot)

	// create a mock epoch for leader selection setup in constructor
	epoch1 := newMockEpoch(
		1,
		epoch1Identities,
		1,
		100,
		unittest.SeedFixture(seed.RandomSourceLength),
	)
	// initially epoch 2 is not committed
	epoch2 := newMockEpoch(
		2,
		epoch2Identities,
		101,
		200,
		unittest.SeedFixture(seed.RandomSourceLength),
	)
	epochs := mocks.NewEpochQuery(t, 1, epoch1)
	snapshot.On("Epochs").Return(epochs)

	com, err := NewConsensusCommittee(state, unittest.IdentifierFixture())
	require.NoError(t, err)

	t.Run("only epoch 1 committed", func(t *testing.T) {
		t.Run("non-existent identity should return InvalidSignerError", func(t *testing.T) {
			_, err := com.IdentityByEpoch(randUint64(1, 100), unittest.IdentifierFixture())
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("existent but non-committee-member identity should return InvalidSignerError", func(t *testing.T) {
			t.Run("zero-weight consensus node", func(t *testing.T) {
				_, err := com.IdentityByEpoch(randUint64(1, 100), zeroWeightConsensusIdentity.NodeID)
				require.True(t, model.IsInvalidSignerError(err))
			})

			t.Run("ejected consensus node", func(t *testing.T) {
				_, err := com.IdentityByEpoch(randUint64(1, 100), ejectedConsensusIdentity.NodeID)
				require.True(t, model.IsInvalidSignerError(err))
			})

			t.Run("otherwise valid non-consensus node", func(t *testing.T) {
				_, err := com.IdentityByEpoch(randUint64(1, 100), validNonConsensusIdentity.NodeID)
				require.True(t, model.IsInvalidSignerError(err))
			})
		})

		t.Run("should be able to retrieve real identity", func(t *testing.T) {
			actual, err := com.IdentityByEpoch(randUint64(1, 100), realIdentity.NodeID)
			require.NoError(t, err)
			require.Equal(t, realIdentity, actual)
		})

		t.Run("should return ErrViewForUnknownEpoch for view outside existing epoch", func(t *testing.T) {
			_, err := com.IdentityByEpoch(randUint64(101, 1_000_000), epoch2Identity.NodeID)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrViewForUnknownEpoch))
		})
	})

	// commit epoch 2
	epochs.Add(epoch2)

	t.Run("epoch 1 and 2 committed", func(t *testing.T) {
		t.Run("should be able to retrieve epoch 1 identity in epoch 1", func(t *testing.T) {
			actual, err := com.IdentityByEpoch(randUint64(1, 100), realIdentity.NodeID)
			require.NoError(t, err)
			require.Equal(t, realIdentity, actual)
		})

		t.Run("should be unable to retrieve epoch 1 identity in epoch 2", func(t *testing.T) {
			_, err := com.IdentityByEpoch(randUint64(101, 200), realIdentity.NodeID)
			require.Error(t, err)
			fmt.Println(err)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("should be unable to retrieve epoch 2 identity in epoch 1", func(t *testing.T) {
			_, err := com.IdentityByEpoch(randUint64(1, 100), epoch2Identity.NodeID)
			require.Error(t, err)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("should be able to retrieve epoch 2 identity in epoch 2", func(t *testing.T) {
			actual, err := com.IdentityByEpoch(randUint64(101, 200), epoch2Identity.NodeID)
			require.NoError(t, err)
			require.Equal(t, epoch2Identity, actual)
		})

		t.Run("should return ErrViewForUnknownEpoch for view outside existing epochs", func(t *testing.T) {
			_, err := com.IdentityByEpoch(randUint64(201, 1_000_000), epoch2Identity.NodeID)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrViewForUnknownEpoch))
		})
	})
}

// TestConsensus_LeaderForView tests that LeaderForView returns a valid leader
// for the previous and current epoch and that it returns the appropriate
// sentinel for the next epoch if it is not yet ready
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
			leaderID, err := committee.LeaderForView(randUint64(1, 100))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("current epoch", func(t *testing.T) {
			// get leader for view in current epoch
			leaderID, err := committee.LeaderForView(randUint64(101, 200))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("after current epoch - should return ErrViewForUnknownEpoch", func(t *testing.T) {
			// get leader for view in next epoch when it is not set up yet
			_, err := committee.LeaderForView(randUint64(201, 300))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrViewForUnknownEpoch))
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
			leaderID, err := committee.LeaderForView(randUint64(1, 100))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("current epoch", func(t *testing.T) {
			// get leader for view in current epoch
			leaderID, err := committee.LeaderForView(randUint64(101, 200))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("next epoch", func(t *testing.T) {
			// get leader for view in next epoch after it has been set up
			leaderID, err := committee.LeaderForView(randUint64(201, 300))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("beyond known epochs", func(t *testing.T) {
			_, err := committee.LeaderForView(randUint64(301, 1_000_000))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrViewForUnknownEpoch))
		})
	})
}

// TestRemoveOldEpochs tests that old epochs are pruned
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
	assert.Equal(t, 1, len(committee.epochs))

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
				assert.Equal(t, int(currentEpochCounter), len(committee.epochs))
			} else {
				assert.Equal(t, 3, len(committee.epochs))
			}

			// check we have the correct epochs stored
			for i := uint64(0); i < 3; i++ {
				counter := currentEpochCounter - i
				if counter < firstEpochCounter {
					break
				}
				_, exists := committee.epochs[counter]
				assert.True(t, exists, "missing epoch with counter %d max counter is %d", counter, currentEpochCounter)
			}
		})
	}
}

func randUint64(min, max int) uint64 {
	return uint64(min + rand.Intn(max+1-min))
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
