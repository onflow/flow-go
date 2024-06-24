package epochs

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProtocolStateMachine(t *testing.T) {
	suite.Run(t, new(ProtocolStateMachineSuite))
}

// BaseStateMachineSuite is a base test suite that holds common functionality for testing protocol state machines.
// It reflects the portion of data which is present in baseStateMachine.
type BaseStateMachineSuite struct {
	suite.Suite

	parentProtocolState *flow.RichEpochProtocolStateEntry
	parentBlock         *flow.Header
	candidate           *flow.Header
	consumer            *protocol_statemock.StateMachineTelemetryConsumer
}

func (s *BaseStateMachineSuite) SetupTest() {
	s.parentProtocolState = unittest.EpochStateFixture(func(entry *flow.RichEpochProtocolStateEntry) {
		// have a fixed boundary for the current epoch
		entry.CurrentEpochSetup.FinalView = 5_000
		entry.CurrentEpoch.SetupID = entry.CurrentEpochSetup.ID()
	})
	s.parentBlock = unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FirstView + 1))
	s.candidate = unittest.BlockHeaderWithParentFixture(s.parentBlock)
	s.consumer = protocol_statemock.NewStateMachineTelemetryConsumer(s.T())
}

// ProtocolStateMachineSuite is a dedicated test suite for testing happy path state machine.
type ProtocolStateMachineSuite struct {
	BaseStateMachineSuite
	stateMachine *HappyPathStateMachine
}

func (s *ProtocolStateMachineSuite) SetupTest() {
	s.BaseStateMachineSuite.SetupTest()
	var err error
	s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
	require.NoError(s.T(), err)
}

// TestNewstateMachine tests if the constructor correctly setups invariants for HappyPathStateMachine.
func (s *ProtocolStateMachineSuite) TestNewstateMachine() {
	require.NotSame(s.T(), s.stateMachine.parentState, s.stateMachine.state, "except to take deep copy of parent state")
	require.Nil(s.T(), s.stateMachine.parentState.NextEpoch)
	require.Nil(s.T(), s.stateMachine.state.NextEpoch)
	require.Equal(s.T(), s.candidate.View, s.stateMachine.View())
	require.Equal(s.T(), s.parentProtocolState, s.stateMachine.ParentState())
}

// TestTransitionToNextEpoch tests a scenario where the HappyPathStateMachine processes first block from next epoch.
// It has to discard the parent state and build a new state with data from next epoch.
func (s *ProtocolStateMachineSuite) TestTransitionToNextEpoch() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)

	candidate := unittest.BlockHeaderFixture(
		unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FinalView + 1))
	var err error
	// since the candidate block is from next epoch, HappyPathStateMachine should transition to next epoch
	s.stateMachine, err = NewHappyPathStateMachine(s.consumer, candidate.View, s.parentProtocolState.Copy())
	require.NoError(s.T(), err)
	err = s.stateMachine.TransitionToNextEpoch()
	require.NoError(s.T(), err)
	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.True(s.T(), hasChanges)
	require.NotEqual(s.T(), s.parentProtocolState.ID(), updatedState.ID())
	require.Equal(s.T(), updatedState.ID(), stateID)
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID(), "should not modify parent protocol state")
	require.Equal(s.T(), updatedState.CurrentEpoch.ID(), s.parentProtocolState.NextEpoch.ID(), "should transition into next epoch")
	require.Nil(s.T(), updatedState.NextEpoch, "next epoch protocol state should be nil")
}

// TestTransitionToNextEpochNotAllowed tests different scenarios where transition to next epoch is not allowed.
func (s *ProtocolStateMachineSuite) TestTransitionToNextEpochNotAllowed() {
	s.Run("no next epoch protocol state", func() {
		protocolState := unittest.EpochStateFixture()
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		stateMachine, err := NewHappyPathStateMachine(s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if there is no next epoch protocol state")
	})
	s.Run("next epoch not committed", func() {
		protocolState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichEpochProtocolStateEntry) {
			entry.NextEpoch.CommitID = flow.ZeroID
			entry.NextEpochCommit = nil
		})
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		stateMachine, err := NewHappyPathStateMachine(s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if it is not committed")
	})
	s.Run("candidate block is not from next epoch", func() {
		protocolState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView))
		stateMachine, err := NewHappyPathStateMachine(s.consumer, candidate.View, protocolState)
		require.NoError(s.T(), err)
		err = stateMachine.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if next block is not first block from next epoch")
	})
}

// TestBuild tests if the HappyPathStateMachine returns correct protocol state.
func (s *ProtocolStateMachineSuite) TestBuild() {
	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.Equal(s.T(), stateID, s.parentProtocolState.ID(), "should return same protocol state")
	require.False(s.T(), hasChanges, "should not have changes")
	require.NotSame(s.T(), updatedState, s.stateMachine.state, "should return a copy of protocol state")
	require.Equal(s.T(), updatedState.ID(), stateID, "should return correct ID")
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID(), "should not modify parent protocol state")

	updatedDynamicIdentity := s.parentProtocolState.CurrentEpochIdentityTable[0].NodeID
	err := s.stateMachine.EjectIdentity(updatedDynamicIdentity)
	require.NoError(s.T(), err)
	updatedState, stateID, hasChanges = s.stateMachine.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.NotEqual(s.T(), stateID, s.parentProtocolState.ID(), "protocol state was modified but still has same ID")
	require.Equal(s.T(), updatedState.ID(), stateID, "should return correct ID")
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID(), "should not modify parent protocol state")
}

// TestCreateStateMachineAfterEFMTriggered tests if creating state machine after observing invalid state transition
// results in error .
func (s *ProtocolStateMachineSuite) TestCreateStateMachineAfterEFMTriggered() {
	s.parentProtocolState.EpochFallbackTriggered = true
	var err error
	// create new HappyPathStateMachine with next epoch information
	s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
	require.Error(s.T(), err)
}

// TestProcessEpochCommit tests if processing epoch commit event correctly updates internal state of HappyPathStateMachine and
// correctly behaves when invariants are violated.
func (s *ProtocolStateMachineSuite) TestProcessEpochCommit() {
	mockConsumer := func(commit *flow.EpochCommit) {
		s.consumer.On("OnServiceEventReceived", commit.ServiceEvent()).Once()
		s.consumer.On("OnInvalidServiceEvent", commit.ServiceEvent(),
			mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	}
	var err error
	s.Run("invalid counter", func() {
		s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 10 // set invalid counter for next epoch
		})
		mockConsumer(commit)
		_, err := s.stateMachine.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("no next epoch protocol state", func() {
		s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		mockConsumer(commit)
		_, err := s.stateMachine.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("conflicting epoch commit", func() {
		s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		setup := unittest.EpochSetupFixture(
			unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
			unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
			unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		)
		// processing setup event results in creating next epoch protocol state
		s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", setup.ServiceEvent()).Once()
		_, err := s.stateMachine.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		updatedState, _, _ := s.stateMachine.Build()

		parentState, err := flow.NewRichEpochProtocolStateEntry(updatedState,
			s.parentProtocolState.PreviousEpochSetup,
			s.parentProtocolState.PreviousEpochCommit,
			s.parentProtocolState.CurrentEpochSetup,
			s.parentProtocolState.CurrentEpochCommit,
			setup,
			nil,
		)
		require.NoError(s.T(), err)

		s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View+1, parentState)
		require.NoError(s.T(), err)
		commit := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(setup.Counter),
			unittest.WithDKGFromParticipants(setup.Participants),
		)

		s.consumer.On("OnServiceEventReceived", commit.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", commit.ServiceEvent()).Once()
		_, err = s.stateMachine.ProcessEpochCommit(commit)
		require.NoError(s.T(), err)

		// processing another epoch commit has to be an error since we have already processed one
		mockConsumer(commit)
		_, err = s.stateMachine.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))

		newState, _, _ := s.stateMachine.Build()
		require.Equal(s.T(), commit.ID(), newState.NextEpoch.CommitID, "next epoch should be committed since we have observed, a valid event")
	})
	s.Run("happy path processing", func() {
		s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		setup := unittest.EpochSetupFixture(
			unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
			unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
			unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		)
		// processing setup event results in creating next epoch protocol state
		s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", setup.ServiceEvent()).Once()
		_, err := s.stateMachine.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		updatedState, stateID, hasChanges := s.stateMachine.Build()
		require.True(s.T(), hasChanges)
		require.NotEqual(s.T(), s.parentProtocolState.ID(), updatedState.ID())
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID(), "should not modify parent protocol state")

		parentState, err := flow.NewRichEpochProtocolStateEntry(updatedState,
			s.parentProtocolState.PreviousEpochSetup,
			s.parentProtocolState.PreviousEpochCommit,
			s.parentProtocolState.CurrentEpochSetup,
			s.parentProtocolState.CurrentEpochCommit,
			setup,
			nil,
		)
		require.NoError(s.T(), err)
		s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View+1, parentState.Copy())
		require.NoError(s.T(), err)
		commit := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(setup.Counter),
			unittest.WithDKGFromParticipants(setup.Participants),
		)
		s.consumer.On("OnServiceEventReceived", commit.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", commit.ServiceEvent()).Once()
		_, err = s.stateMachine.ProcessEpochCommit(commit)
		require.NoError(s.T(), err)

		newState, newStateID, newStateHasChanges := s.stateMachine.Build()
		require.True(s.T(), newStateHasChanges)
		require.Equal(s.T(), commit.ID(), newState.NextEpoch.CommitID, "next epoch should be committed")
		require.Equal(s.T(), newState.ID(), newStateID)
		require.NotEqual(s.T(), s.parentProtocolState.ID(), newState.ID())
		require.NotEqual(s.T(), updatedState.ID(), newState.ID())
		require.Equal(s.T(), parentState.ID(), s.stateMachine.ParentState().ID(),
			"should not modify parent protocol state")
	})
}

// TestUpdateIdentityUnknownIdentity tests if updating the identity of unknown node results in an error.
func (s *ProtocolStateMachineSuite) TestUpdateIdentityUnknownIdentity() {
	err := s.stateMachine.EjectIdentity(unittest.IdentifierFixture())
	require.Error(s.T(), err, "should not be able to update data of unknown identity")
	require.True(s.T(), protocol.IsInvalidServiceEventError(err))

	updatedState, updatedStateID, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges, "should not have changes")
	require.Equal(s.T(), updatedState.ID(), s.parentProtocolState.ID())
	require.Equal(s.T(), updatedState.ID(), updatedStateID)
}

// TestUpdateIdentityHappyPath tests if identity updates are correctly processed and reflected in the resulting protocol state.
func (s *ProtocolStateMachineSuite) TestUpdateIdentityHappyPath() {
	// update protocol state to have next epoch protocol state
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)
	var err error
	s.stateMachine, err = NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
	require.NoError(s.T(), err)

	currentEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	ejectedChanges, err := currentEpochParticipants.Sample(2)
	require.NoError(s.T(), err)

	for _, update := range ejectedChanges {
		err := s.stateMachine.EjectIdentity(update.NodeID)
		require.NoError(s.T(), err)
	}
	updatedState, updatedStateID, hasChanges := s.stateMachine.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), updatedState.ID(), updatedStateID)
	require.NotEqual(s.T(), s.parentProtocolState.ID(), updatedState.ID())
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID(),
		"should not modify parent protocol state")

	// assert that all changes made in the previous epoch are preserved
	currentEpochLookup := updatedState.CurrentEpoch.ActiveIdentities.Lookup()
	nextEpochLookup := updatedState.NextEpoch.ActiveIdentities.Lookup()

	for _, updated := range ejectedChanges {
		currentEpochIdentity, foundInCurrentEpoch := currentEpochLookup[updated.NodeID]
		if foundInCurrentEpoch {
			require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
			require.True(s.T(), currentEpochIdentity.Ejected)
		}

		nextEpochIdentity, foundInNextEpoch := nextEpochLookup[updated.NodeID]
		if foundInNextEpoch {
			require.Equal(s.T(), updated.NodeID, nextEpochIdentity.NodeID)
			require.True(s.T(), nextEpochIdentity.Ejected)
		}
		require.True(s.T(), foundInCurrentEpoch || foundInNextEpoch, "identity should be found in either current or next epoch")
	}
}

// TestProcessEpochSetupInvariants tests if processing epoch setup when invariants are violated doesn't update internal structures.
func (s *ProtocolStateMachineSuite) TestProcessEpochSetupInvariants() {
	mockConsumer := func(setup *flow.EpochSetup) {
		s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
		s.consumer.On("OnInvalidServiceEvent", setup.ServiceEvent(),
			mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	}
	s.Run("invalid counter", func() {
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 10 // set invalid counter for next epoch
		})
		mockConsumer(setup)
		_, err := s.stateMachine.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("processing second epoch setup", func() {
		stateMachine, err := NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		setup := unittest.EpochSetupFixture(
			unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
			unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
			unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		)
		s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
		s.consumer.On("OnServiceEventProcessed", setup.ServiceEvent()).Once()
		_, err = stateMachine.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		mockConsumer(setup)
		_, err = stateMachine.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("participants not sorted", func() {
		stateMachine, err := NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			var err error
			setup.Participants, err = setup.Participants.Shuffle()
			require.NoError(s.T(), err)
		})
		mockConsumer(setup)
		_, err = stateMachine.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("epoch setup state conflicts with protocol state", func() {
		conflictingIdentity := s.parentProtocolState.EpochProtocolStateEntry.CurrentEpoch.ActiveIdentities[0]
		conflictingIdentity.Ejected = true

		stateMachine, err := NewHappyPathStateMachine(s.consumer, s.candidate.View, s.parentProtocolState.Copy())
		require.NoError(s.T(), err)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			// using same identities as in previous epoch should result in an error since
			// we have ejected conflicting identity but it was added back in epoch setup
			// such epoch setup event is invalid.
			setup.Participants = s.parentProtocolState.CurrentEpochSetup.Participants
		})

		mockConsumer(setup)
		_, err = stateMachine.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestProcessEpochSetupHappyPath tests if processing epoch setup when invariants are not violated updates internal structures.
// We test correct construction of the *active identities* for the current and next epoch. Specifically, observing an EpochSetup
// event should leave `PreviousEpoch` and `CurrentEpoch`'s EpochStateContainer unchanged.
// The next epoch's EpochStateContainer should reference the EpochSetup event and hold the respective ActiveIdentities.
func (s *ProtocolStateMachineSuite) TestProcessEpochSetupHappyPath() {
	setupParticipants := unittest.IdentityListFixture(5, unittest.WithAllRoles()).Sort(flow.Canonical[flow.Identity])
	setupParticipants[0].InitialWeight = 13
	setup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
		unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
		unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		unittest.WithParticipants(setupParticipants.ToSkeleton()),
	)

	// for next epoch we will have all the identities from setup event
	expectedNextEpochActiveIdentities := flow.DynamicIdentityEntryListFromIdentities(setupParticipants)

	// process actual event
	s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
	s.consumer.On("OnServiceEventProcessed", setup.ServiceEvent()).Once()
	_, err := s.stateMachine.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, hasChanges := s.stateMachine.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), s.parentProtocolState.PreviousEpoch, updatedState.PreviousEpoch, "previous epoch's EpochStateContainer should not change")
	require.Equal(s.T(), s.parentProtocolState.CurrentEpoch, updatedState.CurrentEpoch, "current epoch's EpochStateContainer should not change")
	nextEpoch := updatedState.NextEpoch
	require.NotNil(s.T(), nextEpoch, "should have next epoch protocol state")
	require.Equal(s.T(), nextEpoch.SetupID, setup.ID(),
		"should have correct setup ID for next protocol state")
	require.Equal(s.T(), nextEpoch.CommitID, flow.ZeroID, "ID for EpochCommit event should still be nil")
	require.Equal(s.T(), expectedNextEpochActiveIdentities, nextEpoch.ActiveIdentities,
		"should have filled active identities for next epoch")
}

// TestProcessEpochSetupWithSameParticipants tests that processing epoch setup with overlapping participants results in correctly
// built updated protocol state. It should build a union of participants from current and next epoch for current and
// next epoch protocol states respectively.
func (s *ProtocolStateMachineSuite) TestProcessEpochSetupWithSameParticipants() {
	participantsFromCurrentEpochSetup, err := flow.ComposeFullIdentities(
		s.parentProtocolState.CurrentEpochSetup.Participants,
		s.parentProtocolState.CurrentEpoch.ActiveIdentities,
		flow.EpochParticipationStatusActive,
	)
	require.NoError(s.T(), err)
	// Function `ComposeFullIdentities` verified that `Participants` and `ActiveIdentities` have identical ordering w.r.t nodeID.
	// By construction, `participantsFromCurrentEpochSetup` lists the full Identities in the same ordering as `Participants` and
	// `ActiveIdentities`. By confirming that `participantsFromCurrentEpochSetup` follows canonical ordering, we can conclude that
	// also `Participants` and `ActiveIdentities` are canonically ordered.
	require.True(s.T(), participantsFromCurrentEpochSetup.Sorted(flow.Canonical[flow.Identity]), "participants in current epoch's setup event are not in canonical order")

	overlappingNodes, err := participantsFromCurrentEpochSetup.Sample(2)
	require.NoError(s.T(), err)
	setupParticipants := append(unittest.IdentityListFixture(len(s.parentProtocolState.CurrentEpochIdentityTable), unittest.WithAllRoles()),
		overlappingNodes...).Sort(flow.Canonical[flow.Identity])
	setup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
		unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
		unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		unittest.WithParticipants(setupParticipants.ToSkeleton()),
	)
	s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
	s.consumer.On("OnServiceEventProcessed", setup.ServiceEvent()).Once()
	_, err = s.stateMachine.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)
	updatedState, _, _ := s.stateMachine.Build()

	require.Equal(s.T(), s.parentProtocolState.CurrentEpoch.ActiveIdentities,
		updatedState.CurrentEpoch.ActiveIdentities,
		"should not change active identities for current epoch")

	expectedNextEpochActiveIdentities := flow.DynamicIdentityEntryListFromIdentities(setupParticipants)
	require.Equal(s.T(), expectedNextEpochActiveIdentities, updatedState.NextEpoch.ActiveIdentities,
		"should have filled active identities for next epoch")
}

// TestEpochSetupAfterIdentityChange tests that after processing epoch an setup event, all previously made changes to the identity table
// are preserved and reflected in the resulting protocol state.
func (s *ProtocolStateMachineSuite) TestEpochSetupAfterIdentityChange() {
	participantsFromCurrentEpochSetup := s.parentProtocolState.CurrentEpochIdentityTable.Filter(func(i *flow.Identity) bool {
		_, exists := s.parentProtocolState.CurrentEpochSetup.Participants.ByNodeID(i.NodeID)
		return exists
	}).Sort(flow.Canonical[flow.Identity])
	ejectedChanges, err := participantsFromCurrentEpochSetup.Sample(2)
	require.NoError(s.T(), err)
	for _, update := range ejectedChanges {
		err := s.stateMachine.EjectIdentity(update.NodeID)
		require.NoError(s.T(), err)
	}
	updatedState, _, _ := s.stateMachine.Build()

	// Construct a valid flow.RichEpochProtocolStateEntry for next block
	// We do this by copying the parent protocol state and updating the identities manually
	updatedRichProtocolState := &flow.RichEpochProtocolStateEntry{
		EpochProtocolStateEntry:   updatedState,
		PreviousEpochSetup:        s.parentProtocolState.PreviousEpochSetup,
		PreviousEpochCommit:       s.parentProtocolState.PreviousEpochCommit,
		CurrentEpochSetup:         s.parentProtocolState.CurrentEpochSetup,
		CurrentEpochCommit:        s.parentProtocolState.CurrentEpochCommit,
		NextEpochSetup:            nil,
		NextEpochCommit:           nil,
		CurrentEpochIdentityTable: s.parentProtocolState.CurrentEpochIdentityTable.Copy(),
		NextEpochIdentityTable:    flow.IdentityList{},
	}
	// Update enriched data with the changes made to the low-level updated table
	for _, identity := range ejectedChanges {
		toBeUpdated, _ := updatedRichProtocolState.CurrentEpochIdentityTable.ByNodeID(identity.NodeID)
		toBeUpdated.EpochParticipationStatus = flow.EpochParticipationStatusEjected
	}

	// now we can use it to construct HappyPathStateMachine for next block, which will process epoch setup event.
	nextBlock := unittest.BlockHeaderWithParentFixture(s.candidate)
	s.stateMachine, err = NewHappyPathStateMachine(s.consumer, nextBlock.View, updatedRichProtocolState)
	require.NoError(s.T(), err)

	setup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
		unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
		unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		func(setup *flow.EpochSetup) {
			// add those nodes that were changed in the previous epoch, but not those that were ejected
			// it's important to exclude ejected nodes, since we expect that service smart contract has emitted ejection operation
			// and service events are delivered (asynchronously) in an *order-preserving* manner meaning if ejection has happened before
			// epoch setup then there is no possible way that it will include ejected node unless there is a severe bug in the service contract.
			setup.Participants = setup.Participants.Filter(
				filter.Not(filter.In(ejectedChanges.ToSkeleton()))).Sort(flow.Canonical[flow.IdentitySkeleton])
		},
	)

	s.consumer.On("OnServiceEventReceived", setup.ServiceEvent()).Once()
	s.consumer.On("OnServiceEventProcessed", setup.ServiceEvent()).Once()
	_, err = s.stateMachine.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, _ = s.stateMachine.Build()

	// assert that all changes made in previous epoch are preserved
	currentEpochLookup := updatedState.CurrentEpoch.ActiveIdentities.Lookup()
	nextEpochLookup := updatedState.NextEpoch.ActiveIdentities.Lookup()

	for _, updated := range ejectedChanges {
		currentEpochIdentity := currentEpochLookup[updated.NodeID]
		require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
		require.True(s.T(), currentEpochIdentity.Ejected)

		_, foundInNextEpoch := nextEpochLookup[updated.NodeID]
		require.False(s.T(), foundInNextEpoch)
	}
}

// TestProcessEpochRecover ensures that HappyPathStateMachine returns a sentinel error when processing an EpochRecover event.
func (s *ProtocolStateMachineSuite) TestProcessEpochRecover() {
	nextEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	epochRecover := unittest.EpochRecoverFixture(func(setup *flow.EpochSetup) {
		setup.Participants = nextEpochParticipants.ToSkeleton()
		setup.Assignments = unittest.ClusterAssignment(1, nextEpochParticipants.ToSkeleton())
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
		setup.FinalView = setup.FirstView + 10_000
	})
	s.consumer.On("OnServiceEventReceived", epochRecover.ServiceEvent()).Once()
	s.consumer.On("OnInvalidServiceEvent", epochRecover.ServiceEvent(),
		mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })).Once()
	processed, err := s.stateMachine.ProcessEpochRecover(epochRecover)
	require.Error(s.T(), err)
	require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	require.False(s.T(), processed)
}
