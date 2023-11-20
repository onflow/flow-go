package committees

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/mapfunc"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/prg"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestConsensusCommittee(t *testing.T) {
	suite.Run(t, new(ConsensusSuite))
}

type ConsensusSuite struct {
	suite.Suite

	// mocks
	state    *protocolmock.State
	snapshot *protocolmock.Snapshot
	params   *protocolmock.Params
	epochs   *mocks.EpochQuery

	// backend for mocked functions
	phase                  flow.EpochPhase
	epochFallbackTriggered bool
	currentEpochCounter    uint64
	myID                   flow.Identifier

	committee *Consensus
	cancel    context.CancelFunc
}

func (suite *ConsensusSuite) SetupTest() {
	suite.phase = flow.EpochPhaseStaking
	suite.epochFallbackTriggered = false
	suite.currentEpochCounter = 1
	suite.myID = unittest.IdentifierFixture()

	suite.state = new(protocolmock.State)
	suite.snapshot = new(protocolmock.Snapshot)
	suite.params = new(protocolmock.Params)
	suite.epochs = mocks.NewEpochQuery(suite.T(), suite.currentEpochCounter)

	suite.state.On("Final").Return(suite.snapshot)
	suite.state.On("Params").Return(suite.params)
	suite.params.On("EpochFallbackTriggered").Return(
		func() bool { return suite.epochFallbackTriggered },
		func() error { return nil },
	)
	suite.snapshot.On("Phase").Return(
		func() flow.EpochPhase { return suite.phase },
		func() error { return nil },
	)
	suite.snapshot.On("Epochs").Return(suite.epochs)
}

func (suite *ConsensusSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
	}
}

// CreateAndStartCommittee instantiates and starts the committee.
// Should be called only once per test, after initial epoch mocks are created.
// It spawns a goroutine to detect fatal errors from the committee's error channel.
func (suite *ConsensusSuite) CreateAndStartCommittee() {
	committee, err := NewConsensusCommittee(suite.state, suite.myID)
	require.NoError(suite.T(), err)
	ctx, cancel, errCh := irrecoverable.WithSignallerAndCancel(context.Background())
	committee.Start(ctx)
	go unittest.FailOnIrrecoverableError(suite.T(), ctx.Done(), errCh)

	suite.committee = committee
	suite.cancel = cancel
}

// CommitEpoch adds the epoch to the protocol state and mimics the protocol state
// behaviour when committing an epoch, by sending the protocol event to the committee.
func (suite *ConsensusSuite) CommitEpoch(epoch protocol.Epoch) {
	firstBlockOfCommittedPhase := unittest.BlockHeaderFixture()
	suite.state.On("AtBlockID", firstBlockOfCommittedPhase.ID()).Return(suite.snapshot)
	suite.epochs.Add(epoch)
	suite.committee.EpochCommittedPhaseStarted(1, firstBlockOfCommittedPhase)

	// get the first view, to test when the epoch has been processed
	firstView, err := epoch.FirstView()
	require.NoError(suite.T(), err)

	// wait for the protocol event to be processed (async)
	assert.Eventually(suite.T(), func() bool {
		_, err := suite.committee.IdentitiesByEpoch(firstView)
		return err == nil
	}, time.Second, time.Millisecond)
}

// AssertStoredEpochCounterRange asserts that the cached epochs are for exactly
// the given contiguous, inclusive counter range.
// Eg. for the input (2,4), the committee must have epochs cached with counters 2,3,4
func (suite *ConsensusSuite) AssertStoredEpochCounterRange(from, to uint64) {
	set := make(map[uint64]struct{})
	for i := from; i <= to; i++ {
		set[i] = struct{}{}
	}

	suite.committee.mu.RLock()
	defer suite.committee.mu.RUnlock()
	for epoch := range suite.committee.epochs {
		delete(set, epoch)
	}

	if !assert.Len(suite.T(), set, 0) {
		suite.T().Logf("%v should be empty, but isn't; expected epoch range [%d,%d]", set, from, to)
	}
}

// TestConstruction_CurrentEpoch tests construction with only a current epoch.
// Only the current epoch should be cached after construction.
func (suite *ConsensusSuite) TestConstruction_CurrentEpoch() {
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(curEpoch)

	suite.CreateAndStartCommittee()
	suite.Assert().Len(suite.committee.epochs, 1)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter)
}

// TestConstruction_PreviousEpoch tests construction with a previous epoch.
// Both current and previous epoch should be cached after construction.
func (suite *ConsensusSuite) TestConstruction_PreviousEpoch() {
	prevEpoch := newMockEpoch(suite.currentEpochCounter-1, unittest.IdentityListFixture(10), 1, 100, unittest.SeedFixture(32), true)
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(prevEpoch)
	suite.epochs.Add(curEpoch)

	suite.CreateAndStartCommittee()
	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter-1, suite.currentEpochCounter)
}

// TestConstruction_UncommittedNextEpoch tests construction with an uncommitted next epoch.
// Only the current epoch should be cached after construction.
func (suite *ConsensusSuite) TestConstruction_UncommittedNextEpoch() {
	suite.phase = flow.EpochPhaseSetup
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	nextEpoch := newMockEpoch(suite.currentEpochCounter+1, unittest.IdentityListFixture(10), 201, 300, unittest.SeedFixture(32), false)
	suite.epochs.Add(curEpoch)
	suite.epochs.Add(nextEpoch)

	suite.CreateAndStartCommittee()
	suite.Assert().Len(suite.committee.epochs, 1)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter)
}

// TestConstruction_CommittedNextEpoch tests construction with a committed next epoch.
// Both current and next epochs should be cached after construction.
func (suite *ConsensusSuite) TestConstruction_CommittedNextEpoch() {
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	nextEpoch := newMockEpoch(suite.currentEpochCounter+1, unittest.IdentityListFixture(10), 201, 300, unittest.SeedFixture(32), true)
	suite.epochs.Add(curEpoch)
	suite.epochs.Add(nextEpoch)
	suite.phase = flow.EpochPhaseCommitted

	suite.CreateAndStartCommittee()
	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter+1)
}

// TestConstruction_EpochFallbackTriggered tests construction when EECC has been triggered.
// Both current and the injected fallback epoch should be cached after construction.
func (suite *ConsensusSuite) TestConstruction_EpochFallbackTriggered() {
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(curEpoch)
	suite.epochFallbackTriggered = true

	suite.CreateAndStartCommittee()
	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter+1)
}

// TestProtocolEvents_CommittedEpoch tests that protocol events notifying of a newly
// committed epoch are handled correctly. A committed epoch should be cached, and
// repeated events should be no-ops.
func (suite *ConsensusSuite) TestProtocolEvents_CommittedEpoch() {
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(curEpoch)

	suite.CreateAndStartCommittee()

	nextEpoch := newMockEpoch(suite.currentEpochCounter+1, unittest.IdentityListFixture(10), 201, 300, unittest.SeedFixture(32), true)

	firstBlockOfCommittedPhase := unittest.BlockHeaderFixture()
	suite.state.On("AtBlockID", firstBlockOfCommittedPhase.ID()).Return(suite.snapshot)
	suite.epochs.Add(nextEpoch)
	suite.committee.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	// wait for the protocol event to be processed (async)
	assert.Eventually(suite.T(), func() bool {
		_, err := suite.committee.IdentitiesByEpoch(unittest.Uint64InRange(201, 300))
		return err == nil
	}, 30*time.Second, 50*time.Millisecond)

	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter+1)

	// should handle multiple deliveries of the protocol event
	suite.committee.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	suite.committee.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	suite.committee.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)

	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter+1)

}

// TestProtocolEvents_EpochFallback tests that protocol events notifying of epoch
// fallback are handled correctly. Epoch fallback triggering should result in a
// fallback epoch being injected, and repeated events should be no-ops.
func (suite *ConsensusSuite) TestProtocolEvents_EpochFallback() {
	curEpoch := newMockEpoch(suite.currentEpochCounter, unittest.IdentityListFixture(10), 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(curEpoch)

	suite.CreateAndStartCommittee()

	suite.committee.EpochEmergencyFallbackTriggered()
	// wait for the protocol event to be processed (async)
	require.Eventually(suite.T(), func() bool {
		_, err := suite.committee.IdentitiesByEpoch(unittest.Uint64InRange(201, 300))
		return err == nil
	}, 30*time.Second, 50*time.Millisecond)

	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter+1)

	// should handle multiple deliveries of the protocol event
	suite.committee.EpochEmergencyFallbackTriggered()
	suite.committee.EpochEmergencyFallbackTriggered()
	suite.committee.EpochEmergencyFallbackTriggered()

	suite.Assert().Len(suite.committee.epochs, 2)
	suite.AssertStoredEpochCounterRange(suite.currentEpochCounter, suite.currentEpochCounter+1)
}

// TestIdentitiesByBlock tests retrieving committee members by block.
// * should use up-to-block committee information
// * should exclude non-committee members
func (suite *ConsensusSuite) TestIdentitiesByBlock() {
	t := suite.T()

	realIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	zeroWeightConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithParticipationStatus(flow.EpochParticipationStatusJoining))
	ejectedConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithParticipationStatus(flow.EpochParticipationStatusEjected))
	validNonConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	fakeID := unittest.IdentifierFixture()
	blockID := unittest.IdentifierFixture()

	// create a mock epoch for leader selection setup in constructor
	currEpoch := newMockEpoch(1, unittest.IdentityListFixture(10), 1, 100, unittest.SeedFixture(prg.RandomSourceLength), true)
	suite.epochs.Add(currEpoch)

	suite.state.On("AtBlockID", blockID).Return(suite.snapshot)
	suite.snapshot.On("Identity", realIdentity.NodeID).Return(realIdentity, nil)
	suite.snapshot.On("Identity", zeroWeightConsensusIdentity.NodeID).Return(zeroWeightConsensusIdentity, nil)
	suite.snapshot.On("Identity", ejectedConsensusIdentity.NodeID).Return(ejectedConsensusIdentity, nil)
	suite.snapshot.On("Identity", validNonConsensusIdentity.NodeID).Return(validNonConsensusIdentity, nil)
	suite.snapshot.On("Identity", fakeID).Return(nil, protocol.IdentityNotFoundError{})

	suite.CreateAndStartCommittee()

	t.Run("non-existent identity should return InvalidSignerError", func(t *testing.T) {
		_, err := suite.committee.IdentityByBlock(blockID, fakeID)
		require.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("existent but non-committee-member identity should return InvalidSignerError", func(t *testing.T) {
		t.Run("zero-weight consensus node", func(t *testing.T) {
			_, err := suite.committee.IdentityByBlock(blockID, zeroWeightConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("ejected consensus node", func(t *testing.T) {
			_, err := suite.committee.IdentityByBlock(blockID, ejectedConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("otherwise valid non-consensus node", func(t *testing.T) {
			_, err := suite.committee.IdentityByBlock(blockID, validNonConsensusIdentity.NodeID)
			require.True(t, model.IsInvalidSignerError(err))
		})
	})

	t.Run("should be able to retrieve real identity", func(t *testing.T) {
		actual, err := suite.committee.IdentityByBlock(blockID, realIdentity.NodeID)
		require.NoError(t, err)
		require.Equal(t, realIdentity, actual)
	})
	t.Run("should propagate unexpected errors", func(t *testing.T) {
		mockErr := errors.New("unexpected")
		suite.snapshot.On("Identity", mock.Anything).Return(nil, mockErr)
		_, err := suite.committee.IdentityByBlock(blockID, unittest.IdentifierFixture())
		assert.ErrorIs(t, err, mockErr)
	})
}

// TestIdentitiesByEpoch tests that identities can be queried by epoch.
// * should use static epoch info (initial identities)
// * should exclude non-committee members
// * should correctly map views to epochs
// * should return ErrViewForUnknownEpoch sentinel for unknown epochs
func (suite *ConsensusSuite) TestIdentitiesByEpoch() {
	t := suite.T()

	// epoch 1 identities with varying conditions which would disqualify them
	// from committee participation
	realIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	zeroWeightConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus),
		unittest.WithInitialWeight(0))
	ejectedConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus), unittest.WithParticipationStatus(flow.EpochParticipationStatusEjected))
	validNonConsensusIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	epoch1Identities := flow.IdentityList{realIdentity, zeroWeightConsensusIdentity, ejectedConsensusIdentity, validNonConsensusIdentity}

	// a single consensus node for epoch 2:
	epoch2Identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	epoch2Identities := flow.IdentityList{epoch2Identity}

	// create a mock epoch for leader selection setup in constructor
	epoch1 := newMockEpoch(suite.currentEpochCounter, epoch1Identities, 1, 100, unittest.SeedFixture(prg.RandomSourceLength), true)
	// initially epoch 2 is not committed
	epoch2 := newMockEpoch(suite.currentEpochCounter+1, epoch2Identities, 101, 200, unittest.SeedFixture(prg.RandomSourceLength), true)
	suite.epochs.Add(epoch1)

	suite.CreateAndStartCommittee()

	t.Run("only epoch 1 committed", func(t *testing.T) {
		t.Run("non-existent identity should return InvalidSignerError", func(t *testing.T) {
			_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(1, 100), unittest.IdentifierFixture())
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("existent but non-committee-member identity should return InvalidSignerError", func(t *testing.T) {
			t.Run("zero-weight consensus node", func(t *testing.T) {
				_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(1, 100), zeroWeightConsensusIdentity.NodeID)
				require.True(t, model.IsInvalidSignerError(err))
			})

			t.Run("otherwise valid non-consensus node", func(t *testing.T) {
				_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(1, 100), validNonConsensusIdentity.NodeID)
				require.True(t, model.IsInvalidSignerError(err))
			})
		})

		t.Run("should be able to retrieve real identity", func(t *testing.T) {
			actual, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(1, 100), realIdentity.NodeID)
			require.NoError(t, err)
			require.Equal(t, realIdentity.IdentitySkeleton, *actual)
		})

		t.Run("should return ErrViewForUnknownEpoch for view outside existing epoch", func(t *testing.T) {
			_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(101, 1_000_000), epoch2Identity.NodeID)
			require.Error(t, err)
			require.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
		})
	})

	// commit epoch 2
	suite.CommitEpoch(epoch2)

	t.Run("epoch 1 and 2 committed", func(t *testing.T) {
		t.Run("should be able to retrieve epoch 1 identity in epoch 1", func(t *testing.T) {
			actual, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(1, 100), realIdentity.NodeID)
			require.NoError(t, err)
			require.Equal(t, realIdentity.IdentitySkeleton, *actual)
		})

		t.Run("should be unable to retrieve epoch 1 identity in epoch 2", func(t *testing.T) {
			_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(101, 200), realIdentity.NodeID)
			require.Error(t, err)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("should be unable to retrieve epoch 2 identity in epoch 1", func(t *testing.T) {
			_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(1, 100), epoch2Identity.NodeID)
			require.Error(t, err)
			require.True(t, model.IsInvalidSignerError(err))
		})

		t.Run("should be able to retrieve epoch 2 identity in epoch 2", func(t *testing.T) {
			actual, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(101, 200), epoch2Identity.NodeID)
			require.NoError(t, err)
			require.Equal(t, epoch2Identity.IdentitySkeleton, *actual)
		})

		t.Run("should return ErrViewForUnknownEpoch for view outside existing epochs", func(t *testing.T) {
			_, err := suite.committee.IdentityByEpoch(unittest.Uint64InRange(201, 1_000_000), epoch2Identity.NodeID)
			require.Error(t, err)
			require.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
		})
	})

}

// TestThresholds tests that the weight threshold methods return the
// correct thresholds for the previous and current epoch and that it returns the
// appropriate sentinel for the next epoch if it is not yet ready.
//
// There are 3 epochs in this test case, each with the same identities but different
// weights.
func (suite *ConsensusSuite) TestThresholds() {
	t := suite.T()

	identities := unittest.IdentityListFixture(10)

	prevEpoch := newMockEpoch(suite.currentEpochCounter-1, identities.Map(mapfunc.WithInitialWeight(100)), 1, 100, unittest.SeedFixture(prg.RandomSourceLength), true)
	currEpoch := newMockEpoch(suite.currentEpochCounter, identities.Map(mapfunc.WithInitialWeight(200)), 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(prevEpoch)
	suite.epochs.Add(currEpoch)

	suite.CreateAndStartCommittee()

	t.Run("next epoch not ready", func(t *testing.T) {
		t.Run("previous epoch", func(t *testing.T) {
			threshold, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(1, 100))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToBuildQC(1000), threshold)
			threshold, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(1, 100))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToTimeout(1000), threshold)
		})

		t.Run("current epoch", func(t *testing.T) {
			threshold, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(101, 200))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToBuildQC(2000), threshold)
			threshold, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(101, 200))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToTimeout(2000), threshold)
		})

		t.Run("after current epoch - should return ErrViewForUnknownEpoch", func(t *testing.T) {
			// get threshold for view in next epoch when it is not set up yet
			_, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(201, 300))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
			_, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(201, 300))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
		})
	})

	// now, add a valid next epoch
	nextEpoch := newMockEpoch(suite.currentEpochCounter+1, identities.Map(mapfunc.WithInitialWeight(300)), 201, 300, unittest.SeedFixture(prg.RandomSourceLength), true)
	suite.CommitEpoch(nextEpoch)

	t.Run("next epoch ready", func(t *testing.T) {
		t.Run("previous epoch", func(t *testing.T) {
			threshold, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(1, 100))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToBuildQC(1000), threshold)
			threshold, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(1, 100))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToTimeout(1000), threshold)
		})

		t.Run("current epoch", func(t *testing.T) {
			threshold, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(101, 200))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToBuildQC(2000), threshold)
			threshold, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(101, 200))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToTimeout(2000), threshold)
		})

		t.Run("next epoch", func(t *testing.T) {
			threshold, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(201, 300))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToBuildQC(3000), threshold)
			threshold, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(201, 300))
			require.Nil(t, err)
			assert.Equal(t, WeightThresholdToTimeout(3000), threshold)
		})

		t.Run("beyond known epochs", func(t *testing.T) {
			// get threshold for view in next epoch when it is not set up yet
			_, err := suite.committee.QuorumThresholdForView(unittest.Uint64InRange(301, 10_000))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
			_, err = suite.committee.TimeoutThresholdForView(unittest.Uint64InRange(301, 10_000))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
		})
	})
}

// TestLeaderForView tests that LeaderForView returns a valid leader
// for the previous and current epoch and that it returns the appropriate
// sentinel for the next epoch if it is not yet ready
func (suite *ConsensusSuite) TestLeaderForView() {
	t := suite.T()

	identities := unittest.IdentityListFixture(10)

	prevEpoch := newMockEpoch(suite.currentEpochCounter-1, identities, 1, 100, unittest.SeedFixture(prg.RandomSourceLength), true)
	currEpoch := newMockEpoch(suite.currentEpochCounter, identities, 101, 200, unittest.SeedFixture(32), true)
	suite.epochs.Add(currEpoch)
	suite.epochs.Add(prevEpoch)

	suite.CreateAndStartCommittee()

	t.Run("next epoch not ready", func(t *testing.T) {
		t.Run("previous epoch", func(t *testing.T) {
			// get leader for view in previous epoch
			leaderID, err := suite.committee.LeaderForView(unittest.Uint64InRange(1, 100))
			assert.NoError(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("current epoch", func(t *testing.T) {
			// get leader for view in current epoch
			leaderID, err := suite.committee.LeaderForView(unittest.Uint64InRange(101, 200))
			assert.NoError(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("after current epoch - should return ErrViewForUnknownEpoch", func(t *testing.T) {
			// get leader for view in next epoch when it is not set up yet
			_, err := suite.committee.LeaderForView(unittest.Uint64InRange(201, 300))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
		})
	})

	// now, add a valid next epoch
	nextEpoch := newMockEpoch(suite.currentEpochCounter+1, identities, 201, 300, unittest.SeedFixture(prg.RandomSourceLength), true)
	suite.CommitEpoch(nextEpoch)

	t.Run("next epoch ready", func(t *testing.T) {
		t.Run("previous epoch", func(t *testing.T) {
			// get leader for view in previous epoch
			leaderID, err := suite.committee.LeaderForView(unittest.Uint64InRange(1, 100))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("current epoch", func(t *testing.T) {
			// get leader for view in current epoch
			leaderID, err := suite.committee.LeaderForView(unittest.Uint64InRange(101, 200))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("next epoch", func(t *testing.T) {
			// get leader for view in next epoch after it has been set up
			leaderID, err := suite.committee.LeaderForView(unittest.Uint64InRange(201, 300))
			require.Nil(t, err)
			_, exists := identities.ByNodeID(leaderID)
			assert.True(t, exists)
		})

		t.Run("beyond known epochs", func(t *testing.T) {
			_, err := suite.committee.LeaderForView(unittest.Uint64InRange(301, 1_000_000))
			assert.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrViewForUnknownEpoch))
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

	epoch1 := newMockEpoch(currentEpochCounter, identities, 1, epochFinalView, unittest.SeedFixture(prg.RandomSourceLength), true)

	// create mocks
	state := new(protocolmock.State)
	snapshot := new(protocolmock.Snapshot)
	params := new(protocolmock.Params)
	state.On("Final").Return(snapshot)
	state.On("Params").Return(params)
	params.On("EpochFallbackTriggered").Return(false, nil)

	epochQuery := mocks.NewEpochQuery(t, currentEpochCounter, epoch1)
	snapshot.On("Epochs").Return(epochQuery)
	currentEpochPhase := flow.EpochPhaseStaking
	snapshot.On("Phase").Return(
		func() flow.EpochPhase { return currentEpochPhase },
		func() error { return nil },
	)

	com, err := NewConsensusCommittee(state, me)
	require.Nil(t, err)

	ctx, cancel, errCh := irrecoverable.WithSignallerAndCancel(context.Background())
	com.Start(ctx)
	go unittest.FailOnIrrecoverableError(t, ctx.Done(), errCh)
	defer cancel()

	// we should start with only current epoch (epoch 1) pre-computed
	// since there is no previous epoch
	assert.Equal(t, 1, len(com.epochs))

	// test for 10 epochs
	for currentEpochCounter < 10 {

		// add another epoch
		firstView := epochFinalView + 1
		epochFinalView = epochFinalView + 100
		currentEpochCounter++
		nextEpoch := newMockEpoch(currentEpochCounter, identities, firstView, epochFinalView, unittest.SeedFixture(prg.RandomSourceLength), true)
		epochQuery.Add(nextEpoch)

		currentEpochPhase = flow.EpochPhaseCommitted
		firstBlockOfCommittedPhase := unittest.BlockHeaderFixture()
		state.On("AtBlockID", firstBlockOfCommittedPhase.ID()).Return(snapshot)
		com.EpochCommittedPhaseStarted(currentEpochCounter, firstBlockOfCommittedPhase)
		// wait for the protocol event to be processed (async)
		require.Eventually(t, func() bool {
			_, err := com.IdentityByEpoch(unittest.Uint64InRange(firstView, epochFinalView), unittest.IdentifierFixture())
			return !errors.Is(err, model.ErrViewForUnknownEpoch)
		}, time.Second, time.Millisecond)

		// query a view from the new epoch
		_, err = com.LeaderForView(firstView)
		require.NoError(t, err)
		// transition to the next epoch
		epochQuery.Transition()

		t.Run(fmt.Sprintf("epoch %d", currentEpochCounter), func(t *testing.T) {
			// check we have the right number of epochs stored
			if currentEpochCounter <= 3 {
				assert.Equal(t, int(currentEpochCounter), len(com.epochs))
			} else {
				assert.Equal(t, 3, len(com.epochs))
			}

			// check we have the correct epochs stored
			for i := uint64(0); i < 3; i++ {
				counter := currentEpochCounter - i
				if counter < firstEpochCounter {
					break
				}
				_, exists := com.epochs[counter]
				assert.True(t, exists, "missing epoch with counter %d max counter is %d", counter, currentEpochCounter)
			}
		})
	}
}

// newMockEpoch returns a new mocked epoch with the given fields
func newMockEpoch(counter uint64, identities flow.IdentityList, firstView uint64, finalView uint64, seed []byte, committed bool) *protocolmock.Epoch {

	epoch := new(protocolmock.Epoch)
	epoch.On("Counter").Return(counter, nil)
	epoch.On("InitialIdentities").Return(identities.ToSkeleton(), nil)
	epoch.On("FirstView").Return(firstView, nil)
	epoch.On("FinalView").Return(finalView, nil)
	if committed {
		// return nil error to indicate the epoch is committed
		epoch.On("DKG").Return(nil, nil)
	} else {
		epoch.On("DKG").Return(nil, protocol.ErrNextEpochNotCommitted)
	}

	epoch.On("RandomSource").Return(seed, nil)
	return epoch
}
