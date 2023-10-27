package protocol_state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestUpdaterSuite(t *testing.T) {
	suite.Run(t, new(UpdaterSuite))
}

// UpdaterSuite is a dedicated test suite for testing updater. It holds a minimal state to initialize updater.
type UpdaterSuite struct {
	suite.Suite

	parentProtocolState *flow.RichProtocolStateEntry
	parentBlock         *flow.Header
	candidate           *flow.Header

	updater *Updater
}

func (s *UpdaterSuite) SetupTest() {
	s.parentProtocolState = unittest.ProtocolStateFixture()
	s.parentBlock = unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FirstView + 1))
	s.candidate = unittest.BlockHeaderWithParentFixture(s.parentBlock)

	s.updater = NewUpdater(s.candidate.View, s.parentProtocolState)
}

// TestNewUpdater tests if the constructor correctly setups invariants for updater.
func (s *UpdaterSuite) TestNewUpdater() {
	require.NotSame(s.T(), s.updater.parentState, s.updater.state, "except to take deep copy of parent state")
	require.Nil(s.T(), s.updater.parentState.NextEpoch)
	require.Nil(s.T(), s.updater.state.NextEpoch)
	require.Equal(s.T(), s.candidate.View, s.updater.View())
	require.Equal(s.T(), s.parentProtocolState, s.updater.ParentState())
}

// TestTransitionToNextEpoch tests a scenario where the updater processes first block from next epoch.
// It has to discard the parent state and build a new state with data from next epoch.
func (s *UpdaterSuite) TestTransitionToNextEpoch() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)

	candidate := unittest.BlockHeaderFixture(
		unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FinalView + 1))
	// since the candidate block is from next epoch, updater should transition to next epoch
	s.updater = NewUpdater(candidate.View, s.parentProtocolState)
	err := s.updater.TransitionToNextEpoch()
	require.NoError(s.T(), err)
	updatedState, _, _ := s.updater.Build()
	require.Equal(s.T(), updatedState.CurrentEpoch.ID(), s.parentProtocolState.NextEpoch.ID(), "should transition into next epoch")
	require.Nil(s.T(), updatedState.NextEpoch, "next epoch protocol state should be nil")
}

// TestTransitionToNextEpochNotAllowed tests different scenarios where transition to next epoch is not allowed.
func (s *UpdaterSuite) TestTransitionToNextEpochNotAllowed() {
	s.Run("no next epoch protocol state", func() {
		protocolState := unittest.ProtocolStateFixture()
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		updater := NewUpdater(candidate.View, protocolState)
		err := updater.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if there is no next epoch protocol state")
	})
	s.Run("next epoch not committed", func() {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichProtocolStateEntry) {
			entry.NextEpoch.CommitID = flow.ZeroID
			entry.NextEpochCommit = nil
		})
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		updater := NewUpdater(candidate.View, protocolState)
		err := updater.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if it is not committed")
	})
	s.Run("invalid state transition has been attempted", func() {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichProtocolStateEntry) {
			entry.InvalidStateTransitionAttempted = true
		})
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		updater := NewUpdater(candidate.View, protocolState)
		err := updater.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if next block is not first block from next epoch")
	})
	s.Run("candidate block is not from next epoch", func() {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView))
		updater := NewUpdater(candidate.View, protocolState)
		err := updater.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if next block is not first block from next epoch")
	})
}

// TestBuild tests if the updater returns correct protocol state.
func (s *UpdaterSuite) TestBuild() {
	updatedState, stateID, hasChanges := s.updater.Build()
	require.Equal(s.T(), stateID, s.parentProtocolState.ID(), "should return same protocol state")
	require.False(s.T(), hasChanges, "should not have changes")
	require.NotSame(s.T(), updatedState, s.updater.state, "should return a copy of protocol state")
	require.Equal(s.T(), updatedState.ID(), stateID, "should return correct ID")

	s.updater.SetInvalidStateTransitionAttempted()
	updatedState, stateID, hasChanges = s.updater.Build()
	require.NotEqual(s.T(), stateID, s.parentProtocolState.ID(), "should return same protocol state")
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), updatedState.ID(), stateID, "should return correct ID")
}

// TestSetInvalidStateTransitionAttempted tests if setting `InvalidStateTransitionAttempted` flag is
// reflected in updating the protocol state.
func (s *UpdaterSuite) TestSetInvalidStateTransitionAttempted() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)
	// create new updater with next epoch information
	s.updater = NewUpdater(s.candidate.View, s.parentProtocolState)

	s.updater.SetInvalidStateTransitionAttempted()
	updatedState, _, hasChanges := s.updater.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.True(s.T(), updatedState.InvalidStateTransitionAttempted, "should set invalid state transition attempted")
}

// TestProcessEpochCommit tests if processing epoch commit event correctly updates internal state of updater and
// correctly behaves when invariants are violated.
func (s *UpdaterSuite) TestProcessEpochCommit() {
	s.Run("invalid counter", func() {
		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 10 // set invalid counter for next epoch
		})
		err := s.updater.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("no next epoch protocol state", func() {
		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		err := s.updater.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("invalid state transition attempted", func() {
		updater := NewUpdater(s.candidate.View, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			setup.FirstView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1
			setup.FinalView = s.parentProtocolState.CurrentEpochSetup.FinalView + 1000
		})
		// processing setup event results in creating next epoch protocol state
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		updater.SetInvalidStateTransitionAttempted()

		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})

		// processing epoch commit should be no-op since we have observed an invalid state transition
		err = updater.ProcessEpochCommit(commit)
		require.NoError(s.T(), err)

		newState, _, _ := updater.Build()
		require.Equal(s.T(), flow.ZeroID, newState.NextEpoch.CommitID, "operation must be no-op")
	})
	s.Run("happy path processing", func() {
		updater := NewUpdater(s.candidate.View, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(
			unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
			unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
			unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		)
		// processing setup event results in creating next epoch protocol state
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		updatedState, _, _ := updater.Build()

		parentState, err := flow.NewRichProtocolStateEntry(updatedState,
			s.parentProtocolState.PreviousEpochSetup,
			s.parentProtocolState.PreviousEpochCommit,
			s.parentProtocolState.CurrentEpochSetup,
			s.parentProtocolState.CurrentEpochCommit,
			setup,
			nil,
		)
		require.NoError(s.T(), err)
		updater = NewUpdater(s.candidate.View+1, parentState)
		commit := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(setup.Counter),
			unittest.WithDKGFromParticipants(setup.Participants),
		)

		err = updater.ProcessEpochCommit(commit)
		require.NoError(s.T(), err)

		// processing another epoch commit has to be an error since we have already processed one
		err = updater.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))

		newState, _, _ := updater.Build()
		require.Equal(s.T(), commit.ID(), newState.NextEpoch.CommitID, "next epoch must be committed")
	})
}

// TestUpdateIdentityUnknownIdentity tests if updating the identity of unknown node results in an error.
func (s *UpdaterSuite) TestUpdateIdentityUnknownIdentity() {
	identity := &flow.DynamicIdentityEntry{
		NodeID:  unittest.IdentifierFixture(),
		Dynamic: flow.DynamicIdentity{},
	}
	err := s.updater.UpdateIdentity(identity)
	require.Error(s.T(), err, "should not be able to update data of unknown identity")

	_, _, hasChanges := s.updater.Build()
	require.False(s.T(), hasChanges, "should not have changes")
}

// TestUpdateIdentityHappyPath tests if identity updates are correctly processed and reflected in the resulting protocol state.
func (s *UpdaterSuite) TestUpdateIdentityHappyPath() {
	// update protocol state to have next epoch protocol state
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)
	s.updater = NewUpdater(s.candidate.View, s.parentProtocolState)

	currentEpochParticipants := s.parentProtocolState.CurrentEpochIdentityTable.Copy()
	weightChanges, err := currentEpochParticipants.Sample(3)
	require.NoError(s.T(), err)
	ejectedChanges, err := currentEpochParticipants.Sample(2)
	require.NoError(s.T(), err)
	require.Greater(s.T(), len(weightChanges), len(ejectedChanges),
		"due to sampling and test setup we want to have more weight changes than ejected changes")
	for i, identity := range weightChanges {
		identity.DynamicIdentity.Weight = uint64(100 * i)
	}
	for _, identity := range ejectedChanges {
		identity.Ejected = true
	}

	allUpdates := append(weightChanges, ejectedChanges...)
	for _, update := range allUpdates {
		err := s.updater.UpdateIdentity(&flow.DynamicIdentityEntry{
			NodeID:  update.NodeID,
			Dynamic: update.DynamicIdentity,
		})
		require.NoError(s.T(), err)
	}
	updatedState, _, hasChanges := s.updater.Build()
	require.True(s.T(), hasChanges, "should have changes")

	// assert that all changes made in the previous epoch are preserved
	currentEpochLookup := updatedState.CurrentEpoch.ActiveIdentities.Lookup()
	nextEpochLookup := updatedState.NextEpoch.ActiveIdentities.Lookup()

	for _, updated := range allUpdates {
		currentEpochIdentity, foundInCurrentEpoch := currentEpochLookup[updated.NodeID]
		if foundInCurrentEpoch {
			require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
			require.Equal(s.T(), updated.DynamicIdentity, currentEpochIdentity.Dynamic)
		}

		nextEpochIdentity, foundInNextEpoch := nextEpochLookup[updated.NodeID]
		if foundInNextEpoch {
			require.Equal(s.T(), updated.NodeID, nextEpochIdentity.NodeID)
			require.Equal(s.T(), updated.DynamicIdentity, nextEpochIdentity.Dynamic)
		}
		require.True(s.T(), foundInCurrentEpoch || foundInNextEpoch, "identity should be found in either current or next epoch")
	}
}

// TestProcessEpochSetupInvariants tests if processing epoch setup when invariants are violated doesn't update internal structures.
func (s *UpdaterSuite) TestProcessEpochSetupInvariants() {
	s.Run("invalid counter", func() {
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 10 // set invalid counter for next epoch
		})
		err := s.updater.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("invalid state transition attempted", func() {
		updater := NewUpdater(s.candidate.View, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		updater.SetInvalidStateTransitionAttempted()
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		updatedState, _, _ := updater.Build()
		require.Nil(s.T(), updatedState.NextEpoch, "should not process epoch setup if invalid state transition attempted")
	})
	s.Run("processing second epoch setup", func() {
		updater := NewUpdater(s.candidate.View, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(
			unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
			unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
			unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		)
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		err = updater.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("participants not sorted", func() {
		updater := NewUpdater(s.candidate.View, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			var err error
			setup.Participants, err = setup.Participants.Shuffle()
			require.NoError(s.T(), err)
		})
		err := updater.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
	s.Run("epoch setup state conflicts with protocol state", func() {
		conflictingIdentity := s.parentProtocolState.ProtocolStateEntry.CurrentEpoch.ActiveIdentities[0]
		conflictingIdentity.Dynamic.Ejected = true

		updater := NewUpdater(s.candidate.View, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
			// using same identities as in previous epoch should result in an error since
			// we have ejected conflicting identity but it was added back in epoch setup
			// such epoch setup event is invalid.
			setup.Participants = s.parentProtocolState.CurrentEpochSetup.Participants
		})

		err := updater.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
		require.True(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestProcessEpochSetupHappyPath tests if processing epoch setup when invariants are not violated updates internal structures.
// It must produce valid identity table for current and next epochs.
// For current epoch we should have identity table with all the nodes from the current epoch + nodes from the next epoch with 0 weight.
// For next epoch we should have identity table with all the nodes from the next epoch + nodes from the current epoch with 0 weight.
func (s *UpdaterSuite) TestProcessEpochSetupHappyPath() {
	setupParticipants := unittest.IdentityListFixture(5, unittest.WithAllRoles()).Sort(order.Canonical[flow.Identity])
	setupParticipants[0].InitialWeight = 13
	setupParticipants[0].Weight = setupParticipants[0].InitialWeight
	setup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
		unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
		unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		unittest.WithParticipants(setupParticipants.ToSkeleton()),
	)

	// for next epoch we will have all the identities from setup event
	expectedNextEpochActiveIdentities := flow.DynamicIdentityEntryListFromIdentities(setupParticipants)

	// process actual event
	err := s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, hasChanges := s.updater.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), s.parentProtocolState.PreviousEpoch.ActiveIdentities, updatedState.PreviousEpoch.ActiveIdentities,
		"should not change active identities for previous epoch")
	require.Equal(s.T(), s.parentProtocolState.CurrentEpoch.ActiveIdentities, updatedState.CurrentEpoch.ActiveIdentities,
		"should not change active identities for current epoch")
	nextEpoch := updatedState.NextEpoch
	require.NotNil(s.T(), nextEpoch, "should have next epoch protocol state")
	require.Equal(s.T(), nextEpoch.SetupID, setup.ID(),
		"should have correct setup ID for next protocol state")
	require.Equal(s.T(), expectedNextEpochActiveIdentities, nextEpoch.ActiveIdentities,
		"should have filled active identities for next epoch")
}

// TestProcessEpochSetupWithSameParticipants tests that processing epoch setup with overlapping participants results in correctly
// built updated protocol state. It should build a union of participants from current and next epoch for current and
// next epoch protocol states respectively.
func (s *UpdaterSuite) TestProcessEpochSetupWithSameParticipants() {
	participantsFromCurrentEpochSetup, err := flow.ComposeFullIdentities(s.parentProtocolState.CurrentEpochSetup.Participants,
		s.parentProtocolState.CurrentEpoch.ActiveIdentities)
	require.NoError(s.T(), err)
	// Function `ComposeFullIdentities` verified that `Participants` and `ActiveIdentities` have identical ordering w.r.t nodeID.
	// By construction, `participantsFromCurrentEpochSetup` lists the full Identities in the same ordering as `Participants` and
	// `ActiveIdentities`. By confirming that `participantsFromCurrentEpochSetup` follows canonical ordering, we can conclude that
	// also `Participants` and `ActiveIdentities` are canonically ordered.
	require.True(s.T(), participantsFromCurrentEpochSetup.Sorted(order.Canonical[flow.Identity]), "participants in current epoch's setup event are not in canonical order")

	overlappingNodes, err := participantsFromCurrentEpochSetup.Sample(2)
	require.NoError(s.T(), err)
	setupParticipants := append(unittest.IdentityListFixture(len(s.parentProtocolState.CurrentEpochIdentityTable), unittest.WithAllRoles()),
		overlappingNodes...).Sort(order.Canonical[flow.Identity])
	setup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
		unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
		unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		unittest.WithParticipants(setupParticipants.ToSkeleton()),
	)
	err = s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)
	updatedState, _, _ := s.updater.Build()

	require.Equal(s.T(), s.parentProtocolState.CurrentEpoch.ActiveIdentities,
		updatedState.CurrentEpoch.ActiveIdentities,
		"should not change active identities for current epoch")

	expectedNextEpochActiveIdentities := flow.DynamicIdentityEntryListFromIdentities(setupParticipants)
	require.Equal(s.T(), expectedNextEpochActiveIdentities, updatedState.NextEpoch.ActiveIdentities,
		"should have filled active identities for next epoch")
}

// TestEpochSetupAfterIdentityChange tests that after processing epoch an setup event, all previously made changes to the identity table
// are preserved and reflected in the resulting protocol state.
func (s *UpdaterSuite) TestEpochSetupAfterIdentityChange() {
	participantsFromCurrentEpochSetup := s.parentProtocolState.CurrentEpochIdentityTable.Filter(func(i *flow.Identity) bool {
		_, exists := s.parentProtocolState.CurrentEpochSetup.Participants.ByNodeID(i.NodeID)
		return exists
	}).Sort(order.Canonical[flow.Identity])
	weightChanges, err := participantsFromCurrentEpochSetup.Sample(3)
	require.NoError(s.T(), err)
	ejectedChanges, err := participantsFromCurrentEpochSetup.Sample(2)
	require.NoError(s.T(), err)
	require.Greater(s.T(), len(weightChanges), len(ejectedChanges),
		"due to sampling and test setup we want to have more weight changes than ejected changes")
	for i, identity := range weightChanges {
		identity.DynamicIdentity.Weight = uint64(100 * (i + 1))
	}
	for _, identity := range ejectedChanges {
		identity.Ejected = true
	}
	allUpdates := append(weightChanges, ejectedChanges...)
	for _, update := range allUpdates {
		err := s.updater.UpdateIdentity(&flow.DynamicIdentityEntry{
			NodeID:  update.NodeID,
			Dynamic: update.DynamicIdentity,
		})
		require.NoError(s.T(), err)
	}
	updatedState, _, _ := s.updater.Build()

	// Construct a valid flow.RichProtocolStateEntry for next block
	// We do this by copying the parent protocol state and updating the identities manually
	updatedRichProtocolState := &flow.RichProtocolStateEntry{
		ProtocolStateEntry:        updatedState,
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
	for _, identity := range allUpdates {
		toBeUpdated, _ := updatedRichProtocolState.CurrentEpochIdentityTable.ByNodeID(identity.NodeID)
		toBeUpdated.DynamicIdentity = identity.DynamicIdentity
	}

	// now we can use it to construct updater for next block, which will process epoch setup event.
	nextBlock := unittest.BlockHeaderWithParentFixture(s.candidate)
	s.updater = NewUpdater(nextBlock.View, updatedRichProtocolState)

	setup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(s.parentProtocolState.CurrentEpochSetup.Counter+1),
		unittest.WithFirstView(s.parentProtocolState.CurrentEpochSetup.FinalView+1),
		unittest.WithFinalView(s.parentProtocolState.CurrentEpochSetup.FinalView+1000),
		func(setup *flow.EpochSetup) {
			// add those nodes that were changed in the previous epoch, but not those that were ejected
			// it's important to exclude ejected nodes, since we expect that service smart contract has emitted ejection operation
			// and service events are delivered (asynchronously) in an *order-preserving* manner meaning if ejection has happened before
			// epoch setup then there is no possible way that it will include ejected node unless there is a severe bug in the service contract.
			setup.Participants = append(setup.Participants, weightChanges.ToSkeleton()...).Filter(
				filter.Not(filter.In(ejectedChanges.ToSkeleton()))).Sort(order.Canonical[flow.IdentitySkeleton])
		},
	)

	err = s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, _ = s.updater.Build()

	// assert that all changes made in previous epoch are preserved
	currentEpochLookup := updatedState.CurrentEpoch.ActiveIdentities.Lookup()
	nextEpochLookup := updatedState.NextEpoch.ActiveIdentities.Lookup()

	for _, updated := range ejectedChanges {
		currentEpochIdentity := currentEpochLookup[updated.NodeID]
		require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
		require.Equal(s.T(), updated.Ejected, currentEpochIdentity.Dynamic.Ejected)

		_, foundInNextEpoch := nextEpochLookup[updated.NodeID]
		require.False(s.T(), foundInNextEpoch)
	}

	for _, updated := range weightChanges {
		currentEpochIdentity := currentEpochLookup[updated.NodeID]
		require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
		require.Equal(s.T(), updated.DynamicIdentity.Weight, currentEpochIdentity.Dynamic.Weight)
		require.NotEqual(s.T(), updated.InitialWeight, currentEpochIdentity.Dynamic.Weight,
			"since we have updated weight it should not be equal to initial weight")

		// it's possible that we have sampled weight and ejected changes for the same node so we need to check if it was ejected
		if nextEpochIdentity, found := nextEpochLookup[updated.NodeID]; found {
			require.Equal(s.T(), updated.NodeID, nextEpochIdentity.NodeID)
			require.Equal(s.T(), updated.InitialWeight, nextEpochIdentity.Dynamic.Weight,
				"we take information about weight from next epoc setup event")
		} else {
			_, wasEjected := ejectedChanges.ByNodeID(updated.NodeID)
			require.True(s.T(), wasEjected, "only if node is ejected it could be missing from next epoch lookup")
		}
	}
}
