package epochs

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	mockstate "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochFallbackStateMachine(t *testing.T) {
	suite.Run(t, new(EpochFallbackStateMachineSuite))
}

// ProtocolStateMachineSuite is a dedicated test suite for testing happy path state machine.
type EpochFallbackStateMachineSuite struct {
	BaseStateMachineSuite
	params *mockstate.GlobalParams

	stateMachine *FallbackStateMachine
}

func (s *EpochFallbackStateMachineSuite) SetupTest() {
	s.BaseStateMachineSuite.SetupTest()

	s.params = mockstate.NewGlobalParams(s.T())
	s.params.On("EpochCommitSafetyThreshold").Return(uint64(200))
	s.parentProtocolState.EpochFallbackTriggered = true

	var err error
	s.stateMachine, err = NewFallbackStateMachine(s.params, s.consumer, s.candidate.View, s.parentProtocolState.Copy())
	require.NoError(s.T(), err)
}

// ProcessEpochSetupIsNoop ensures that processing epoch setup event is noop.
func (s *EpochFallbackStateMachineSuite) TestProcessEpochSetupIsNoop() {
	setup := unittest.EpochSetupFixture()
	s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
	s.consumer.On("OnInvalidServiceEvent", setup.ServiceEvent(),
		mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	applied, err := s.stateMachine.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)
	require.False(s.T(), applied)
	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), s.parentProtocolState.ID(), updatedState.ID())
	require.Equal(s.T(), updatedState.ID(), stateID)
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID())
}

// ProcessEpochCommitIsNoop ensures that processing epoch commit event is noop.
func (s *EpochFallbackStateMachineSuite) TestProcessEpochCommitIsNoop() {
	commit := unittest.EpochCommitFixture()
	s.consumer.On("OnServiceEventReceived", commit.ServiceEvent()).Once()
	s.consumer.On("OnInvalidServiceEvent", commit.ServiceEvent(),
		mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	applied, err := s.stateMachine.ProcessEpochCommit(commit)
	require.NoError(s.T(), err)
	require.False(s.T(), applied)
	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), s.parentProtocolState.ID(), updatedState.ID())
	require.Equal(s.T(), updatedState.ID(), stateID)
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID())
}

// TestProcessEpochRecover ensures that after processing EpochRecover event, the state machine initializes
// correctly the next epoch with expected values. Tests happy path scenario where the next epoch has been set up correctly.
func (s *EpochFallbackStateMachineSuite) TestProcessEpochRecover() {
	nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
		setup.Participants = nextEpochParticipants.ToSkeleton()
		setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
		setup.FinalView = setup.FirstView + 10_000
	})
	s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
	s.consumer.On("OnServiceEventProcessed", epochRecover.ServiceEvent()).Once()
	processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
	require.NoError(s.T(), err)
	require.True(s.T(), processed)
	updatedState, updatedStateID, hasChanges := s.stateMachine.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), updatedState.ID(), updatedStateID, "state ID should be equal to updated state ID")

	expectedState := &flow.EpochMinStateEntry{
		PreviousEpoch: s.parentProtocolState.PreviousEpoch.Copy(),
		CurrentEpoch:  s.parentProtocolState.CurrentEpoch,
		NextEpoch: &flow.EpochStateContainer{
			SetupID:          epochRecover.EpochSetup.ID(),
			CommitID:         epochRecover.EpochCommit.ID(),
			ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(nextEpochParticipants),
			EpochExtensions:  nil,
		},
		EpochFallbackTriggered: false,
	}
	require.Equal(s.T(), expectedState, updatedState.EpochMinStateEntry, "updatedState should be equal to expected one")
}

// TestProcessInvalidEpochRecover tests that processing epoch recover event which is invalid or is not compatible with current
// protocol state returns a sentinel error.
func (s *EpochFallbackStateMachineSuite) TestProcessInvalidEpochRecover() {
	nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	mockConsumer := func(epochRecover *flow.EpochRecover) {
		s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
		s.consumer.On("OnInvalidServiceEvent", epochRecover.ServiceEvent(),
			mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	}
	s.Run("invalid-first-view", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 2 // invalid view
			setup.FinalView = setup.FirstView + 10_000
		})
		mockConsumer(epochRecover)
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("invalid-first-view_ignores-epoch-extension", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})

		parentProtocolState := s.parentProtocolState.Copy()
		parentProtocolState.CurrentEpoch.EpochExtensions = []flow.EpochExtension{
			{
				FirstView:     s.parentProtocolState.CurrentEpochSetup.FinalView + 2, // invalid view for extension
				FinalView:     s.parentProtocolState.CurrentEpochSetup.FinalView + 1 + 10_000,
				TargetEndTime: 0,
			},
		}

		candidateView := s.parentProtocolState.CurrentEpochSetup.FinalView - s.params.EpochCommitSafetyThreshold() + 1
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidateView, parentProtocolState)
		require.NoError(s.T(), err)

		mockConsumer(epochRecover)
		processed, err := stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("invalid-counter", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 2 // invalid counter
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		mockConsumer(epochRecover)
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("invalid-commit-counter", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		epochRecover.EpochCommit.Counter += 1 // invalid commit counter
		mockConsumer(epochRecover)
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("invalid-cluster-qcs", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		epochRecover.EpochCommit.ClusterQCs = epochRecover.EpochCommit.ClusterQCs[1:] // invalid cluster QCs
		mockConsumer(epochRecover)
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("invalid-DKG-group-key", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		epochRecover.EpochCommit.DKGGroupKey = nil // no DKG public group key
		mockConsumer(epochRecover)
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("invalid-dkg-participants", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		epochRecover.EpochCommit.DKGParticipantKeys = epochRecover.EpochCommit.DKGParticipantKeys[1:] // invalid DKG participants
		mockConsumer(epochRecover)
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("next-epoch-present", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})

		parentProtocolState := s.parentProtocolState.Copy()
		unittest.WithNextEpochProtocolState()(parentProtocolState)

		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, s.candidate.View, parentProtocolState)
		require.NoError(s.T(), err)

		mockConsumer(epochRecover)
		processed, err := stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
	s.Run("reached-CommitSafetyThreshold_without-next-epoch-committed", func() {
		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		thresholdView := s.parentProtocolState.CurrentEpochSetup.FinalView - s.params.EpochCommitSafetyThreshold()
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, thresholdView, s.parentProtocolState)
		require.NoError(s.T(), err)

		mockConsumer(epochRecover)
		processed, err := stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)
	})
}

// TestTransitionToNextEpoch tests a scenario where the FallbackStateMachine processes first block from next epoch.
// It has to discard the parent state and build a new state with data from next epoch.
func (s *EpochFallbackStateMachineSuite) TestTransitionToNextEpoch() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)

	candidate := unittest.BlockHeaderFixture(
		unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FinalView + 1))

	expectedState := &flow.EpochStateEntry{
		EpochMinStateEntry: &flow.EpochMinStateEntry{
			PreviousEpoch:          s.parentProtocolState.CurrentEpoch.Copy(),
			CurrentEpoch:           *s.parentProtocolState.NextEpoch.Copy(),
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		},
		PreviousEpochSetup:  s.parentProtocolState.CurrentEpochSetup,
		PreviousEpochCommit: s.parentProtocolState.CurrentEpochCommit,
		CurrentEpochSetup:   s.parentProtocolState.NextEpochSetup,
		CurrentEpochCommit:  s.parentProtocolState.NextEpochCommit,
		NextEpochSetup:      nil,
		NextEpochCommit:     nil,
	}

	// Irrespective of whether the parent state is in EFM, the FallbackStateMachine should always set
	// `EpochFallbackTriggered` to true and transition the next epoch, because the candidate block
	// belongs to the next epoch.
	var err error
	for _, parentAlreadyInEFM := range []bool{true, false} {
		parentProtocolState := s.parentProtocolState.Copy()
		parentProtocolState.EpochFallbackTriggered = parentAlreadyInEFM

		s.stateMachine, err = NewFallbackStateMachine(s.params, s.consumer, candidate.View, parentProtocolState.Copy())
		require.NoError(s.T(), err)
		err = s.stateMachine.TransitionToNextEpoch()
		require.NoError(s.T(), err)
		updatedState, stateID, hasChanges := s.stateMachine.Build()
		require.True(s.T(), hasChanges)
		require.NotEqual(s.T(), parentProtocolState.ID(), updatedState.ID())
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.Equal(s.T(), expectedState, updatedState, "FallbackStateMachine produced unexpected Protocol State")
	}
}

// TestTransitionToNextEpochNotAllowed tests different scenarios where transition to next epoch is not allowed.
func (s *EpochFallbackStateMachineSuite) TestTransitionToNextEpochNotAllowed() {
	s.Run("no next epoch protocol state", func() {
		protocolState := unittest.EpochStateFixture()
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if there is no next epoch protocol state")
	})
	s.Run("next epoch not committed", func() {
		protocolState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.EpochRichStateEntry) {
			entry.NextEpoch.CommitID = flow.ZeroID
			entry.NextEpochCommit = nil
			entry.NextEpochIdentityTable = nil
		})
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if it is not committed")
	})
	s.Run("candidate block is not from next epoch", func() {
		protocolState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView))
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if next block is not first block from next epoch")
	})
}

// TestNewEpochFallbackStateMachine tests that creating epoch fallback state machine sets
// `EpochFallbackTriggered` to true to record that we have entered epoch fallback mode[EFM].
// It tests scenarios where the EFM is entered in different phases of the epoch,
// and verifies protocol-compliant addition of epoch extensions, depending on the candidate view and epoch phase.
func (s *EpochFallbackStateMachineSuite) TestNewEpochFallbackStateMachine() {
	parentProtocolState := s.parentProtocolState.Copy()
	parentProtocolState.EpochFallbackTriggered = false

	thresholdView := parentProtocolState.CurrentEpochSetup.FinalView - s.params.EpochCommitSafetyThreshold()

	// The view we enter EFM is in the staking phase. The resulting epoch state should be unchanged to the
	// parent state _except_ that `EpochFallbackTriggered` is set to true.
	// We expect no epoch extension to be added since we have not reached the threshold view.
	s.Run("threshold-not-reached", func() {
		candidateView := thresholdView - 1
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidateView, parentProtocolState.Copy())
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), candidateView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.EpochMinStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
				EpochExtensions:  nil,
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.EpochMinStateEntry, "state should be equal to expected one")
	})

	// The view we enter EFM is in the staking phase. The resulting epoch state should set `EpochFallbackTriggered` to true.
	// We expect an epoch extension to be added since we have reached the threshold view.
	s.Run("staking-phase", func() {
		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, thresholdView, parentProtocolState.Copy())
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), thresholdView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.EpochMinStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
				EpochExtensions: []flow.EpochExtension{
					{
						FirstView:     parentProtocolState.CurrentEpochFinalView() + 1,
						FinalView:     parentProtocolState.CurrentEpochFinalView() + DefaultEpochExtensionViewCount,
						TargetEndTime: 0,
					},
				},
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.EpochMinStateEntry, "state should be equal to expected one")
	})

	// The view we enter EFM is in the epoch setup phase. This means that a SetupEvent for the next epoch is in the parent block's
	// protocol state. We expect an epoch extension to be added and the outdated information for the next epoch to be removed.
	s.Run("setup-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next epoch but without commit event
		unittest.WithNextEpochProtocolState()(parentProtocolState)
		parentProtocolState.NextEpoch.CommitID = flow.ZeroID
		parentProtocolState.NextEpochCommit = nil

		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, thresholdView, parentProtocolState)
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), thresholdView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Nil(s.T(), updatedState.NextEpoch, "outdated information for the next epoch should have been removed")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.EpochMinStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
				EpochExtensions: []flow.EpochExtension{
					{
						FirstView:     parentProtocolState.CurrentEpochFinalView() + 1,
						FinalView:     parentProtocolState.CurrentEpochFinalView() + DefaultEpochExtensionViewCount,
						TargetEndTime: 0,
					},
				},
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.EpochMinStateEntry, "state should be equal to expected one")
	})

	// If the next epoch has been committed, the extension shouldn't be added to the current epoch (verified below). Instead, the
	// extension should be added to the next epoch when **next** epoch reaches its safety threshold, which is covered in separate test.
	s.Run("commit-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next committed epoch
		unittest.WithNextEpochProtocolState()(parentProtocolState)

		// if the next epoch has been committed, the extension shouldn't be added to the current epoch
		// instead it will be added to the next epoch when **next** epoch reaches its safety threshold.

		stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, thresholdView, parentProtocolState)
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), thresholdView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.EpochMinStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
				EpochExtensions:  nil,
			},
			NextEpoch:              parentProtocolState.NextEpoch,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.EpochMinStateEntry, "state should be equal to expected one")
	})
}

// TestEpochFallbackStateMachineInjectsMultipleExtensions tests that the state machine injects multiple extensions
// as it reaches the safety threshold of the current epoch and the extensions themselves.
// In this test, we are simulating the scenario where the current epoch enters EFM at a view when the next epoch has not been committed yet.
// When the next epoch has been committed the extension should be added to the next epoch, this is covered in separate test.
func (s *EpochFallbackStateMachineSuite) TestEpochFallbackStateMachineInjectsMultipleExtensions() {
	parentStateInStakingPhase := s.parentProtocolState.Copy()
	parentStateInStakingPhase.EpochFallbackTriggered = false

	parentStateInSetupPhase := parentStateInStakingPhase.Copy()
	unittest.WithNextEpochProtocolState()(parentStateInSetupPhase)
	parentStateInSetupPhase.NextEpoch.CommitID = flow.ZeroID
	parentStateInSetupPhase.NextEpochCommit = nil

	for _, originalParentState := range []*flow.EpochRichStateEntry{parentStateInStakingPhase, parentStateInSetupPhase} {
		// In the previous test `TestNewEpochFallbackStateMachine`, we verified that the first extension is added correctly. Below we
		// test proper addition of the subsequent extension. A new extension should be added when we reach `firstExtensionViewThreshold`.
		// When reaching (equality) this threshold, the next extension should be added
		firstExtensionViewThreshold := originalParentState.CurrentEpochSetup.FinalView + DefaultEpochExtensionViewCount - s.params.EpochCommitSafetyThreshold()
		secondExtensionViewThreshold := originalParentState.CurrentEpochSetup.FinalView + 2*DefaultEpochExtensionViewCount - s.params.EpochCommitSafetyThreshold()
		// We progress through views that are strictly smaller than threshold. Up to this point, only the initial extension should exist

		// we will be asserting the validity of extensions after producing multiple extensions,
		// we expect 2 extensions to be added to the current epoch
		// 1 after we reach the commit threshold of the epoch and another one after reaching the threshold of the extension themselves
		firstExtension := flow.EpochExtension{
			FirstView:     originalParentState.CurrentEpochSetup.FinalView + 1,
			FinalView:     originalParentState.CurrentEpochSetup.FinalView + DefaultEpochExtensionViewCount,
			TargetEndTime: 0,
		}
		secondExtension := flow.EpochExtension{
			FirstView:     firstExtension.FinalView + 1,
			FinalView:     firstExtension.FinalView + DefaultEpochExtensionViewCount,
			TargetEndTime: 0,
		}

		parentProtocolState := originalParentState.Copy()
		candidateView := originalParentState.CurrentEpochSetup.FirstView + 1
		// an utility function to progress state to target view
		// updates variables that are defined in outer context
		evolveStateToView := func(targetView uint64) {
			for ; candidateView < targetView; candidateView++ {
				stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidateView, parentProtocolState.Copy())
				require.NoError(s.T(), err)
				updatedState, _, _ := stateMachine.Build()

				parentProtocolState, err = flow.NewEpochRichStateEntry(updatedState)
				require.NoError(s.T(), err)
			}
		}

		type TestData struct {
			TargetView         uint64
			ExpectedExtensions []flow.EpochExtension
		}

		for _, data := range []TestData{
			{
				TargetView:         firstExtensionViewThreshold,
				ExpectedExtensions: []flow.EpochExtension{firstExtension},
			},
			{
				TargetView:         secondExtensionViewThreshold,
				ExpectedExtensions: []flow.EpochExtension{firstExtension, secondExtension},
			},
		} {
			evolveStateToView(data.TargetView)

			expectedState := &flow.EpochMinStateEntry{
				PreviousEpoch: originalParentState.PreviousEpoch,
				CurrentEpoch: flow.EpochStateContainer{
					SetupID:          originalParentState.CurrentEpoch.SetupID,
					CommitID:         originalParentState.CurrentEpoch.CommitID,
					ActiveIdentities: originalParentState.CurrentEpoch.ActiveIdentities,
					EpochExtensions:  data.ExpectedExtensions,
				},
				NextEpoch:              nil,
				EpochFallbackTriggered: true,
			}
			require.Equal(s.T(), expectedState, parentProtocolState.EpochMinStateEntry)
			require.Greater(s.T(), parentProtocolState.CurrentEpochFinalView(), candidateView,
				"final view should be greater than final view of test")
		}
	}
}

// TestEpochFallbackStateMachineInjectsMultipleExtensions_NextEpochCommitted tests that the state machine injects multiple extensions
// as it reaches the safety threshold of the current epoch and the extensions themselves.
// In this test we are simulating the scenario where the current epoch enters fallback mode when the next epoch has been committed.
// It is expected that it will transition into the next epoch (since it was committed),
// then reach the safety threshold and add the extension to the next epoch, which at that point will be considered 'current'.
func (s *EpochFallbackStateMachineSuite) TestEpochFallbackStateMachineInjectsMultipleExtensions_NextEpochCommitted() {
	originalParentState := s.parentProtocolState.Copy()
	originalParentState.EpochFallbackTriggered = false
	unittest.WithNextEpochProtocolState()(originalParentState)

	// assert the validity of extensions after producing multiple extensions
	// we expect 3 extensions to be added to the current epoch
	// 1 after we reach the commit threshold of the epoch and two more after reaching the threshold of the extensions themselves
	firstExtension := flow.EpochExtension{
		FirstView:     originalParentState.NextEpochSetup.FinalView + 1,
		FinalView:     originalParentState.NextEpochSetup.FinalView + DefaultEpochExtensionViewCount,
		TargetEndTime: 0,
	}
	secondExtension := flow.EpochExtension{
		FirstView:     firstExtension.FinalView + 1,
		FinalView:     firstExtension.FinalView + DefaultEpochExtensionViewCount,
		TargetEndTime: 0,
	}
	thirdExtension := flow.EpochExtension{
		FirstView:     secondExtension.FinalView + 1,
		FinalView:     secondExtension.FinalView + DefaultEpochExtensionViewCount,
		TargetEndTime: 0,
	}

	// In the previous test `TestNewEpochFallbackStateMachine`, we verified that the first extension is added correctly. Below we
	// test proper addition of the subsequent extension. A new extension should be added when we reach `firstExtensionViewThreshold`.
	// When reaching (equality) this threshold, the next extension should be added
	firstExtensionViewThreshold := originalParentState.NextEpochSetup.FinalView + DefaultEpochExtensionViewCount - s.params.EpochCommitSafetyThreshold()
	secondExtensionViewThreshold := originalParentState.NextEpochSetup.FinalView + 2*DefaultEpochExtensionViewCount - s.params.EpochCommitSafetyThreshold()
	thirdExtensionViewThreshold := originalParentState.NextEpochSetup.FinalView + 3*DefaultEpochExtensionViewCount - s.params.EpochCommitSafetyThreshold()

	parentProtocolState := originalParentState.Copy()
	candidateView := originalParentState.CurrentEpochSetup.FirstView + 1
	// an utility function to progress state to target view
	// updates variables that are defined in outer context
	evolveStateToView := func(targetView uint64) {
		for ; candidateView < targetView; candidateView++ {
			stateMachine, err := NewFallbackStateMachine(s.params, s.consumer, candidateView, parentProtocolState.Copy())
			require.NoError(s.T(), err)

			if candidateView > parentProtocolState.CurrentEpochFinalView() {
				require.NoError(s.T(), stateMachine.TransitionToNextEpoch())
			}

			updatedState, _, _ := stateMachine.Build()
			parentProtocolState, err = flow.NewEpochRichStateEntry(updatedState)
			require.NoError(s.T(), err)
		}
	}

	type TestData struct {
		TargetView         uint64
		ExpectedExtensions []flow.EpochExtension
	}

	for _, data := range []TestData{
		{
			TargetView:         firstExtensionViewThreshold,
			ExpectedExtensions: []flow.EpochExtension{firstExtension},
		},
		{
			TargetView:         secondExtensionViewThreshold,
			ExpectedExtensions: []flow.EpochExtension{firstExtension, secondExtension},
		},
		{
			TargetView:         thirdExtensionViewThreshold,
			ExpectedExtensions: []flow.EpochExtension{firstExtension, secondExtension, thirdExtension},
		},
	} {
		evolveStateToView(data.TargetView)

		expectedState := &flow.EpochMinStateEntry{
			PreviousEpoch: originalParentState.CurrentEpoch.Copy(),
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          originalParentState.NextEpoch.SetupID,
				CommitID:         originalParentState.NextEpoch.CommitID,
				ActiveIdentities: originalParentState.NextEpoch.ActiveIdentities,
				EpochExtensions:  data.ExpectedExtensions,
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedState, parentProtocolState.EpochMinStateEntry)
		require.Greater(s.T(), parentProtocolState.CurrentEpochFinalView(), candidateView,
			"final view should be greater than final view of test")
	}
}

// TestEpochRecoverAndEjectionInSameBlock tests that processing an epoch recover event which re-admits an ejected identity results in an error.
// Such action should be considered illegal since smart contract emitted ejection before epoch recover and service events are delivered
// in an order-preserving manner.
func (s *EpochFallbackStateMachineSuite) TestEpochRecoverAndEjectionInSameBlock() {
	nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	ejectedIdentityID := nextEpochParticipants[0].NodeID
	epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
		setup.Participants = nextEpochParticipants.ToSkeleton()
		setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
		setup.FinalView = setup.FirstView + 10_000
	})
	err := s.stateMachine.EjectIdentity(ejectedIdentityID)
	require.NoError(s.T(), err)

	s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
	s.consumer.On("OnInvalidServiceEvent", epochRecover.ServiceEvent(),
		mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
	require.NoError(s.T(), err)
	require.False(s.T(), processed)
}

// TestProcessingMultipleEventsAtTheSameBlock tests that the state machine can process multiple events at the same block.
// EpochFallbackStateMachineSuite has to be able to process any combination of events in any order at the same block.
// This test generates a random amount of setup, commit and recover events and processes them in random order.
func (s *EpochFallbackStateMachineSuite) TestProcessingMultipleEventsAtTheSameBlock() {
	rapid.Check(s.T(), func(t *rapid.T) {
		s.SetupTest() // start each time with clean state
		var events []flow.ServiceEvent
		setupEvents := rapid.IntRange(0, 5).Draw(t, "number-of-setup-events")
		for i := 0; i < setupEvents; i++ {
			serviceEvent := unittest.EpochSetupFixture().ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnInvalidServiceEvent", serviceEvent,
				mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
			events = append(events, serviceEvent)
		}
		commitEvents := rapid.IntRange(0, 5).Draw(t, "number-of-commit-events")
		for i := 0; i < commitEvents; i++ {
			serviceEvent := unittest.EpochCommitFixture().ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnInvalidServiceEvent", serviceEvent,
				mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
			events = append(events, serviceEvent)
		}
		recoverEvents := rapid.IntRange(0, 5).Draw(t, "number-of-recover-events")
		for i := 0; i < recoverEvents; i++ {
			serviceEvent := unittest.EpochRecoverFixture().ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnInvalidServiceEvent", serviceEvent,
				mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
			events = append(events, serviceEvent)
		}
		includeValidRecover := rapid.Bool().Draw(t, "include-valid-recover-event")
		if includeValidRecover {
			serviceEvent := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
				nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
				setup.Participants = nextEpochParticipants.ToSkeleton()
				setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
				setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
				setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
				setup.FinalView = setup.FirstView + 10_000
			}).ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnServiceEventProcessed", serviceEvent).Once()
			events = append(events, serviceEvent)
		}
		includeEjection := rapid.Bool().Draw(t, "eject-node")
		ejectionGen := rapid.SampledFrom(s.parentProtocolState.CurrentEpoch.ActiveIdentities)
		var ejectedNodes flow.IdentifierList
		if includeEjection {
			identity := ejectionGen.Draw(t, "ejection-node")
			// TODO(EFM, #6020): this needs to be updated when a proper ejection event is implemented
			serviceEvent := flow.ServiceEvent{
				Type:  "ejection",
				Event: identity.NodeID,
			}
			events = append(events, serviceEvent)
			ejectedNodes = append(ejectedNodes, identity.NodeID)
		}
		events = rapid.Permutation(events).Draw(t, "events-permutation")
		for _, event := range events {
			var err error
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				_, err = s.stateMachine.ProcessEpochSetup(ev)
			case *flow.EpochCommit:
				_, err = s.stateMachine.ProcessEpochCommit(ev)
			case *flow.EpochRecover:
				_, err = s.stateMachine.ProcessEpochRecover(ev)
			case flow.Identifier:
				err = s.stateMachine.EjectIdentity(ev)
			}
			require.NoError(s.T(), err)
		}
		updatedState, _, hasChanges := s.stateMachine.Build()
		for _, nodeID := range ejectedNodes {
			ejectedIdentity, found := updatedState.CurrentEpoch.ActiveIdentities.ByNodeID(nodeID)
			require.True(s.T(), found)
			require.True(s.T(), ejectedIdentity.Ejected)
		}

		// TODO(EFM, #6019): next section needs to be updated when implementing related issue.
		// Extra context: logic in this test is based on the assumption that recover event readmits the same identity table.
		// This means if there is an ejection event in the same block and it precedes the recovery event then the recovery event needs to be rejected
		// since it tries to readmit an ejected identity.
		// Based on the setup of the test we will have changes to the state in next cases:
		// 1. includeValidRecover == true, includeEjection == false, in this case epoch is recovered and NextEpoch != nil.
		// 2. includeValidRecover == false, includeEjection == true, in this case we eject nodes and NextEpoch == nil.
		// 3. includeValidRecover == true, includeEjection == true, recover precedes ejection event, in this case epoch is recovered and NextEpoch != nil
		//    and nodes are ejected in both current and next epoch.
		// In all other cases there shouldn't be changes to the resulting state.
		require.Equal(t, includeValidRecover || includeEjection, hasChanges, "always should have changes since we eject nodes")
		if includeValidRecover {
			if includeEjection {
				//require.Nil(t, updatedState.NextEpoch, "recover event shouldn't be accepted since it readmits an ejected identity")
			} else {
				require.NotNil(t, updatedState.NextEpoch, "next epoch should be present")
			}
		}
	})
}
