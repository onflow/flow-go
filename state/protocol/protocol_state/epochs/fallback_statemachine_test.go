package epochs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	mockstate "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// extensionViewCount is the number of views for which the epoch is extended. This value is returned from KV store.
const extensionViewCount = uint64(10_000)

func TestEpochFallbackStateMachine(t *testing.T) {
	suite.Run(t, new(EpochFallbackStateMachineSuite))
}

// ProtocolStateMachineSuite is a dedicated test suite for testing happy path state machine.
type EpochFallbackStateMachineSuite struct {
	BaseStateMachineSuite
	kvstore *mockstate.KVStoreReader

	stateMachine *FallbackStateMachine
}

func (s *EpochFallbackStateMachineSuite) SetupTest() {
	s.BaseStateMachineSuite.SetupTest()
	s.parentProtocolState.EpochFallbackTriggered = true

	s.kvstore = mockstate.NewKVStoreReader(s.T())
	s.kvstore.On("GetEpochExtensionViewCount").Return(extensionViewCount).Maybe()
	s.kvstore.On("GetFinalizationSafetyThreshold").Return(uint64(200))

	var err error
	s.stateMachine, err = NewFallbackStateMachine(s.kvstore, s.consumer, s.candidate.View, s.parentProtocolState.Copy())
	require.NoError(s.T(), err)
}

// ProcessEpochSetupIsNoop ensures that processing epoch setup event is noop.
func (s *EpochFallbackStateMachineSuite) TestProcessEpochSetupIsNoop() {
	setup := unittest.EpochSetupFixture()
	s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
	s.consumer.On("OnInvalidServiceEvent", setup.ServiceEvent(),
		unittest.MatchInvalidServiceEventError).Once()
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
		unittest.MatchInvalidServiceEventError).Once()
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

	expectedState := &flow.MinEpochStateEntry{
		PreviousEpoch: s.parentProtocolState.PreviousEpoch.Copy(),
		CurrentEpoch:  s.parentProtocolState.CurrentEpoch,
		NextEpoch: &flow.EpochStateContainer{
			SetupID:          epochRecover.EpochSetup.ID(),
			CommitID:         epochRecover.EpochCommit.ID(),
			ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(nextEpochParticipants),
		},
		EpochFallbackTriggered: false,
	}
	require.Equal(s.T(), expectedState, updatedState.MinEpochStateEntry, "updatedState should be equal to expected one")
}

// TestProcessInvalidEpochRecover tests that processing epoch recover event which is invalid or is not compatible with current
// protocol state returns a sentinel error.
func (s *EpochFallbackStateMachineSuite) TestProcessInvalidEpochRecover() {
	nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	mockConsumer := func(epochRecover *flow.EpochRecover) {
		s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
		s.consumer.On("OnInvalidServiceEvent", epochRecover.ServiceEvent(),
			unittest.MatchInvalidServiceEventError).Once()
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
				FirstView: s.parentProtocolState.CurrentEpochSetup.FinalView + 2, // invalid view for extension
				FinalView: s.parentProtocolState.CurrentEpochSetup.FinalView + 1 + 10_000,
			},
		}

		candidateView := s.parentProtocolState.CurrentEpochSetup.FinalView - s.kvstore.GetFinalizationSafetyThreshold() + 1
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidateView, parentProtocolState)
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

		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, s.candidate.View, parentProtocolState)
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
		thresholdView := s.parentProtocolState.CurrentEpochSetup.FinalView - s.kvstore.GetFinalizationSafetyThreshold()
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, thresholdView, s.parentProtocolState)
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
		MinEpochStateEntry: &flow.MinEpochStateEntry{
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

		s.stateMachine, err = NewFallbackStateMachine(s.kvstore, s.consumer, candidate.View, parentProtocolState.Copy())
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
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if there is no next epoch protocol state")
	})
	s.Run("next epoch not committed", func() {
		protocolState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichEpochStateEntry) {
			entry.NextEpoch.CommitID = flow.ZeroID
			entry.NextEpochCommit = nil
			entry.NextEpochIdentityTable = nil
		})
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if it is not committed")
	})
	s.Run("candidate block is not from next epoch", func() {
		protocolState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView))
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidate.View, protocolState)
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

	thresholdView := parentProtocolState.CurrentEpochSetup.FinalView - s.kvstore.GetFinalizationSafetyThreshold()

	// The view we enter EFM is in the staking phase. The resulting epoch state should be unchanged to the
	// parent state _except_ that `EpochFallbackTriggered` is set to true.
	// We expect no epoch extension to be added since we have not reached the threshold view.
	s.Run("threshold-not-reached", func() {
		candidateView := thresholdView - 1
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidateView, parentProtocolState.Copy())
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), candidateView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.MinEpochStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.MinEpochStateEntry, "state should be equal to expected one")
	})

	// The view we enter EFM is in the staking phase. The resulting epoch state should set `EpochFallbackTriggered` to true.
	// We expect an epoch extension to be added since we have reached the threshold view.
	s.Run("staking-phase", func() {
		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, thresholdView, parentProtocolState.Copy())
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), thresholdView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.MinEpochStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
				EpochExtensions: []flow.EpochExtension{
					{
						FirstView: parentProtocolState.CurrentEpochFinalView() + 1,
						FinalView: parentProtocolState.CurrentEpochFinalView() + extensionViewCount,
					},
				},
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.MinEpochStateEntry, "state should be equal to expected one")
	})

	// The view we enter EFM is in the epoch setup phase. This means that a SetupEvent for the next epoch is in the parent block's
	// protocol state. We expect an epoch extension to be added and the outdated information for the next epoch to be removed.
	s.Run("setup-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next epoch but without commit event
		unittest.WithNextEpochProtocolState()(parentProtocolState)
		parentProtocolState.NextEpoch.CommitID = flow.ZeroID
		parentProtocolState.NextEpochCommit = nil

		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, thresholdView, parentProtocolState)
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), thresholdView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Nil(s.T(), updatedState.NextEpoch, "outdated information for the next epoch should have been removed")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.MinEpochStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
				EpochExtensions: []flow.EpochExtension{
					{
						FirstView: parentProtocolState.CurrentEpochFinalView() + 1,
						FinalView: parentProtocolState.CurrentEpochFinalView() + extensionViewCount,
					},
				},
			},
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.MinEpochStateEntry, "state should be equal to expected one")
	})

	// If the next epoch has been committed, the extension shouldn't be added to the current epoch (verified below). Instead, the
	// extension should be added to the next epoch when **next** epoch reaches its safety threshold, which is covered in separate test.
	s.Run("commit-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next committed epoch
		unittest.WithNextEpochProtocolState()(parentProtocolState)

		// if the next epoch has been committed, the extension shouldn't be added to the current epoch
		// instead it will be added to the next epoch when **next** epoch reaches its safety threshold.

		stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, thresholdView, parentProtocolState)
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), thresholdView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "EpochFallbackTriggered has to be updated")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		expectedProtocolState := &flow.MinEpochStateEntry{
			PreviousEpoch: parentProtocolState.PreviousEpoch,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          parentProtocolState.CurrentEpoch.SetupID,
				CommitID:         parentProtocolState.CurrentEpoch.CommitID,
				ActiveIdentities: parentProtocolState.CurrentEpoch.ActiveIdentities,
			},
			NextEpoch:              parentProtocolState.NextEpoch,
			EpochFallbackTriggered: true,
		}
		require.Equal(s.T(), expectedProtocolState, updatedState.MinEpochStateEntry, "state should be equal to expected one")
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

	for _, originalParentState := range []*flow.RichEpochStateEntry{parentStateInStakingPhase, parentStateInSetupPhase} {
		// In the previous test `TestNewEpochFallbackStateMachine`, we verified that the first extension is added correctly. Below we
		// test proper addition of the subsequent extension. A new extension should be added when we reach `firstExtensionViewThreshold`.
		// When reaching (equality) this threshold, the next extension should be added
		firstExtensionViewThreshold := originalParentState.CurrentEpochSetup.FinalView + extensionViewCount - s.kvstore.GetFinalizationSafetyThreshold()
		secondExtensionViewThreshold := originalParentState.CurrentEpochSetup.FinalView + 2*extensionViewCount - s.kvstore.GetFinalizationSafetyThreshold()
		// We progress through views that are strictly smaller than threshold. Up to this point, only the initial extension should exist

		// we will be asserting the validity of extensions after producing multiple extensions,
		// we expect 2 extensions to be added to the current epoch
		// 1 after we reach the commit threshold of the epoch and another one after reaching the threshold of the extension themselves
		firstExtension := flow.EpochExtension{
			FirstView: originalParentState.CurrentEpochSetup.FinalView + 1,
			FinalView: originalParentState.CurrentEpochSetup.FinalView + extensionViewCount,
		}
		secondExtension := flow.EpochExtension{
			FirstView: firstExtension.FinalView + 1,
			FinalView: firstExtension.FinalView + extensionViewCount,
		}

		parentProtocolState := originalParentState.Copy()
		candidateView := originalParentState.CurrentEpochSetup.FirstView + 1
		// an utility function to progress state to target view
		// updates variables that are defined in outer context
		evolveStateToView := func(targetView uint64) {
			for ; candidateView < targetView; candidateView++ {
				stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidateView, parentProtocolState.Copy())
				require.NoError(s.T(), err)
				updatedState, _, _ := stateMachine.Build()

				parentProtocolState, err = flow.NewRichEpochStateEntry(updatedState)
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

			expectedState := &flow.MinEpochStateEntry{
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
			require.Equal(s.T(), expectedState, parentProtocolState.MinEpochStateEntry)
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
		FirstView: originalParentState.NextEpochSetup.FinalView + 1,
		FinalView: originalParentState.NextEpochSetup.FinalView + extensionViewCount,
	}
	secondExtension := flow.EpochExtension{
		FirstView: firstExtension.FinalView + 1,
		FinalView: firstExtension.FinalView + extensionViewCount,
	}
	thirdExtension := flow.EpochExtension{
		FirstView: secondExtension.FinalView + 1,
		FinalView: secondExtension.FinalView + extensionViewCount,
	}

	// In the previous test `TestNewEpochFallbackStateMachine`, we verified that the first extension is added correctly. Below we
	// test proper addition of the subsequent extension. A new extension should be added when we reach `firstExtensionViewThreshold`.
	// When reaching (equality) this threshold, the next extension should be added
	firstExtensionViewThreshold := originalParentState.NextEpochSetup.FinalView + extensionViewCount - s.kvstore.GetFinalizationSafetyThreshold()
	secondExtensionViewThreshold := originalParentState.NextEpochSetup.FinalView + 2*extensionViewCount - s.kvstore.GetFinalizationSafetyThreshold()
	thirdExtensionViewThreshold := originalParentState.NextEpochSetup.FinalView + 3*extensionViewCount - s.kvstore.GetFinalizationSafetyThreshold()

	parentProtocolState := originalParentState.Copy()
	candidateView := originalParentState.CurrentEpochSetup.FirstView + 1
	// an utility function to progress state to target view
	// updates variables that are defined in outer context
	evolveStateToView := func(targetView uint64) {
		for ; candidateView < targetView; candidateView++ {
			stateMachine, err := NewFallbackStateMachine(s.kvstore, s.consumer, candidateView, parentProtocolState.Copy())
			require.NoError(s.T(), err)

			if candidateView > parentProtocolState.CurrentEpochFinalView() {
				require.NoError(s.T(), stateMachine.TransitionToNextEpoch())
			}

			updatedState, _, _ := stateMachine.Build()
			parentProtocolState, err = flow.NewRichEpochStateEntry(updatedState)
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

		expectedState := &flow.MinEpochStateEntry{
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
		require.Equal(s.T(), expectedState, parentProtocolState.MinEpochStateEntry)
		require.Greater(s.T(), parentProtocolState.CurrentEpochFinalView(), candidateView,
			"final view should be greater than final view of test")
	}
}

// TestEpochRecoverAndEjectionInSameBlock tests that state machine correctly handles ejection events and a subsequent
// epoch recover in the same block. Specifically we test two cases:
//  1. Happy Path: The Epoch Recover event excludes the previously ejected node.
//  2. Invalid Epoch Recover: an epoch recover event which re-admits an ejected identity is ignored. Such action should
//     be considered illegal since smart contract emitted ejection before epoch recover and service events are delivered
//     in an order-preserving manner. However, since the FallbackStateMachine is intended to keep the protocol alive even
//     in the presence of (largely) arbitrary Epoch Smart Contract bugs, it should also handle this case gracefully.
//     In this case, the EpochRecover service event should be discarded and the internal state should remain unchanged.
func (s *EpochFallbackStateMachineSuite) TestEpochRecoverAndEjectionInSameBlock() {
	nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	ejectedIdentityID := nextEpochParticipants.Filter(filter.HasRole[flow.Identity](flow.RoleAccess))[0].NodeID
	ejectionEvent := &flow.EjectNode{NodeID: ejectedIdentityID}

	s.Run("happy path", func() {
		s.consumer.On("OnServiceEventReceived", ejectionEvent.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", ejectionEvent.ServiceEvent()).Once()
		wasEjected := s.stateMachine.EjectIdentity(ejectionEvent)
		require.True(s.T(), wasEjected)

		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton().Filter(
				filter.Not(filter.HasNodeID[flow.IdentitySkeleton](ejectedIdentityID)))
			setup.Assignments = unittest.ClusterAssignment(1, setup.Participants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", epochRecover.ServiceEvent()).Once()
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.True(s.T(), processed)

		updatedState, _, _ := s.stateMachine.Build()
		require.False(s.T(), updatedState.EpochFallbackTriggered, "should exit EFM")
		require.NotNil(s.T(), updatedState.NextEpoch, "should setup & commit next epoch")
	})
	s.Run("invalid epoch recover event", func() {
		s.kvstore = mockstate.NewKVStoreReader(s.T())
		s.kvstore.On("GetEpochExtensionViewCount").Return(extensionViewCount).Maybe()
		s.kvstore.On("GetFinalizationSafetyThreshold").Return(uint64(200))

		var err error
		s.stateMachine, err = NewFallbackStateMachine(s.kvstore, s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)

		s.consumer.On("OnServiceEventReceived", ejectionEvent.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", ejectionEvent.ServiceEvent()).Once()
		wasEjected := s.stateMachine.EjectIdentity(ejectionEvent)
		require.True(s.T(), wasEjected)

		epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
			setup.Participants = nextEpochParticipants.ToSkeleton()
			setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 10_000
		})
		s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
		s.consumer.On("OnInvalidServiceEvent", epochRecover.ServiceEvent(),
			unittest.MatchInvalidServiceEventError).Once()
		processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
		require.NoError(s.T(), err)
		require.False(s.T(), processed)

		updatedState, _, _ := s.stateMachine.Build()
		require.True(s.T(), updatedState.EpochFallbackTriggered, "should remain in EFM")
		require.Nil(s.T(), updatedState.NextEpoch, "next epoch should be nil as recover event is invalid")
	})
}

// TestProcessingMultipleEventsInTheSameBlock tests that the state machine can process multiple events at the same block.
// EpochFallbackStateMachineSuite has to be able to process any combination of events in any order at the same block.
// This test generates a random amount of setup, commit and recover events and processes them in random order.
// A special rule is used to inject an ejection event, depending on the random draw we will inject an ejection event before or after the recover event.
// Depending on the ordering of events, the recovering event needs to be structured differently.
func (s *EpochFallbackStateMachineSuite) TestProcessingMultipleEventsInTheSameBlock() {
	rapid.Check(s.T(), func(t *rapid.T) {
		s.SetupTest() // start each time with clean state
		var events []flow.ServiceEvent
		// ATTENTION: explicitly grouped draw events because drawing an int range can raise a panic.
		// It's not an issue on its own, but we are using telemetry to check correct invocations of state machine
		// and when a panic occurs, it will still try to assert expectations on the consumer leading to a test failure.
		// Specifically, for that reason, we are drawing the values before setting up the expectations on mocked objects.
		setupEvents := rapid.IntRange(0, 5).Draw(t, "number-of-setup-events")
		commitEvents := rapid.IntRange(0, 5).Draw(t, "number-of-commit-events")
		recoverEvents := rapid.IntRange(0, 5).Draw(t, "number-of-recover-events")
		includeEjection := rapid.Bool().Draw(t, "eject-node")
		includeValidRecover := rapid.Bool().Draw(t, "include-valid-recover-event")
		ejectionBeforeRecover := rapid.Bool().Draw(t, "ejection-before-recover")

		for i := 0; i < setupEvents; i++ {
			serviceEvent := unittest.EpochSetupFixture().ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnInvalidServiceEvent", serviceEvent,
				unittest.MatchInvalidServiceEventError).Once()
			events = append(events, serviceEvent)
		}

		for i := 0; i < commitEvents; i++ {
			serviceEvent := unittest.EpochCommitFixture().ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnInvalidServiceEvent", serviceEvent,
				unittest.MatchInvalidServiceEventError).Once()
			events = append(events, serviceEvent)
		}

		for i := 0; i < recoverEvents; i++ {
			serviceEvent := unittest.EpochRecoverFixture().ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnInvalidServiceEvent", serviceEvent,
				unittest.MatchInvalidServiceEventError).Once()
			events = append(events, serviceEvent)
		}

		var ejectedNodes flow.IdentifierList
		var ejectionEvents flow.ServiceEventList

		if includeEjection {
			accessNodes := s.parentProtocolState.CurrentEpochSetup.Participants.Filter(filter.HasRole[flow.IdentitySkeleton](flow.RoleAccess))
			identity := rapid.SampledFrom(accessNodes).Draw(t, "ejection-node")
			serviceEvent := (&flow.EjectNode{NodeID: identity.NodeID}).ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnServiceEventProcessed", serviceEvent).Once()
			ejectionEvents = append(ejectionEvents, serviceEvent)
			ejectedNodes = append(ejectedNodes, identity.NodeID)
		}

		if includeValidRecover {
			serviceEvent := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
				nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
				if ejectionBeforeRecover {
					// a valid recovery event cannot readmit a node ejected previously
					setup.Participants = nextEpochParticipants.ToSkeleton().Filter(
						filter.Not(filter.HasNodeID[flow.IdentitySkeleton](ejectedNodes...)))
				} else {
					setup.Participants = nextEpochParticipants.ToSkeleton()
				}
				setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
				setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
				setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
				setup.FinalView = setup.FirstView + 10_000
			}).ServiceEvent()
			s.consumer.On("OnServiceEventReceived", serviceEvent).Once()
			s.consumer.On("OnServiceEventProcessed", serviceEvent).Once()
			events = append(events, serviceEvent)
		}

		events = rapid.Permutation(events).Draw(t, "events-permutation")
		if ejectionBeforeRecover {
			events = append(ejectionEvents, events...)
		} else {
			events = append(events, ejectionEvents...)
		}

		for _, event := range events {
			var err error
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				_, err = s.stateMachine.ProcessEpochSetup(ev)
			case *flow.EpochCommit:
				_, err = s.stateMachine.ProcessEpochCommit(ev)
			case *flow.EpochRecover:
				_, err = s.stateMachine.ProcessEpochRecover(ev)
			case *flow.EjectNode:
				_ = s.stateMachine.EjectIdentity(ev)
			}
			require.NoError(s.T(), err)
		}
		updatedState, _, hasChanges := s.stateMachine.Build()
		for _, nodeID := range ejectedNodes {
			ejectedIdentity, found := updatedState.CurrentEpoch.ActiveIdentities.ByNodeID(nodeID)
			require.True(s.T(), found)
			require.True(s.T(), ejectedIdentity.Ejected)
		}

		require.Equal(t, includeValidRecover || includeEjection, hasChanges,
			"changes are expected if we include valid recovery or eject nodes")
		if includeValidRecover {
			require.NotNil(t, updatedState.NextEpoch, "next epoch should be present")
			for _, nodeID := range ejectedNodes {
				ejectedIdentity, found := updatedState.NextEpoch.ActiveIdentities.ByNodeID(nodeID)
				if ejectionBeforeRecover {
					// if ejection is before recover, the ejected node should not be present in the next epoch
					require.False(s.T(), found)
				} else {
					// in-case of ejection after recover, the ejected node should be present in the next epoch, but it has to be 'ejected'.
					require.True(s.T(), found)
					require.True(s.T(), ejectedIdentity.Ejected)
				}
			}
		}
	})
}
