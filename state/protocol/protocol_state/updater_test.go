package protocol_state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/mapfunc"
	"github.com/onflow/flow-go/model/flow/order"
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

	s.updater = NewUpdater(s.candidate, s.parentProtocolState)
}

// TestNewUpdater tests if the constructor correctly setups invariants for updater.
func (s *UpdaterSuite) TestNewUpdater() {
	require.NotSame(s.T(), s.updater.parentState, s.updater.state, "except to take deep copy of parent state")
	require.Nil(s.T(), s.updater.parentState.NextEpoch)
	require.Nil(s.T(), s.updater.state.NextEpoch)
	require.Equal(s.T(), s.candidate, s.updater.Block())
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
	s.updater = NewUpdater(candidate, s.parentProtocolState)
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
		updater := NewUpdater(candidate, protocolState)
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
		updater := NewUpdater(candidate, protocolState)
		err := updater.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if it is not committed")
	})
	s.Run("invalid state transition has been attempted", func() {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichProtocolStateEntry) {
			entry.InvalidStateTransitionAttempted = true
		})
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView + 1))
		updater := NewUpdater(candidate, protocolState)
		err := updater.TransitionToNextEpoch()
		require.Error(s.T(), err, "should not allow transition to next epoch if next block is not first block from next epoch")
	})
	s.Run("candidate block is not from next epoch", func() {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		candidate := unittest.BlockHeaderFixture(
			unittest.HeaderWithView(protocolState.CurrentEpochSetup.FinalView))
		updater := NewUpdater(candidate, protocolState)
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
	s.updater = NewUpdater(s.candidate, s.parentProtocolState)

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
	})
	s.Run("no next epoch protocol state", func() {
		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		err := s.updater.ProcessEpochCommit(commit)
		require.Error(s.T(), err)
	})
	s.Run("invalid state transition attempted", func() {
		updater := NewUpdater(s.candidate, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
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
		updater := NewUpdater(s.candidate, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		// processing setup event results in creating next epoch protocol state
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		commit := unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})

		err = updater.ProcessEpochCommit(commit)
		require.NoError(s.T(), err)

		// processing another epoch commit has to be an error since we have already processed one
		err = updater.ProcessEpochCommit(commit)
		require.Error(s.T(), err)

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
	s.updater = NewUpdater(s.candidate, s.parentProtocolState)

	currentEpochParticipants := s.parentProtocolState.CurrentEpochSetup.Participants.Copy()
	weightChanges, err := currentEpochParticipants.Sample(2)
	require.NoError(s.T(), err)
	ejectedChanges, err := currentEpochParticipants.Sample(2)
	require.NoError(s.T(), err)
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

	requireUpdatesApplied := func(identityLookup map[flow.Identifier]*flow.DynamicIdentityEntry) {
		for _, identity := range allUpdates {
			updatedIdentity := identityLookup[identity.NodeID]
			require.Equal(s.T(), identity.NodeID, updatedIdentity.NodeID)
			require.Equal(s.T(), identity.DynamicIdentity, updatedIdentity.Dynamic, "identity should be updated")
		}
	}

	// check if changes are reflected in current and next epochs
	requireUpdatesApplied(updatedState.CurrentEpoch.Identities.Lookup())
	requireUpdatesApplied(updatedState.NextEpoch.Identities.Lookup())
}

// TestProcessEpochSetupInvariants tests if processing epoch setup when invariants are violated doesn't update internal structures.
func (s *UpdaterSuite) TestProcessEpochSetupInvariants() {
	s.Run("invalid counter", func() {
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 10 // set invalid counter for next epoch
		})
		err := s.updater.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
	})
	s.Run("invalid state transition attempted", func() {
		updater := NewUpdater(s.candidate, s.parentProtocolState)
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
		updater := NewUpdater(s.candidate, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		err = updater.ProcessEpochSetup(setup)
		require.Error(s.T(), err)
	})
}

// TestProcessEpochSetupHappyPath tests if processing epoch setup when invariants are not violated updates internal structures.
// It must produce valid identity table for current and next epochs.
// For current epoch we should have identity table with all the nodes from the current epoch + nodes from the next epoch with 0 weight.
// For next epoch we should have identity table with all the nodes from the next epoch + nodes from the current epoch with 0 weight.
func (s *UpdaterSuite) TestProcessEpochSetupHappyPath() {
	setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		setup.Participants[0].InitialWeight = 13
	})

	// prepare expected identity tables for current and next epochs
	expectedCurrentEpochIdentityTable := make(flow.DynamicIdentityEntryList, 0,
		len(s.parentProtocolState.CurrentEpochSetup.Participants)+len(setup.Participants))
	expectedNextEpochIdentityTable := make(flow.DynamicIdentityEntryList, 0,
		len(expectedCurrentEpochIdentityTable))

	// for current epoch we will have all the nodes from the current epoch + nodes from the next epoch with 0 weight
	// for next epoch we will have all the nodes from the next epoch + nodes from the current epoch with 0 weight
	for _, participant := range s.parentProtocolState.CurrentEpochSetup.Participants {
		expectedCurrentEpochIdentityTable = append(expectedCurrentEpochIdentityTable, &flow.DynamicIdentityEntry{
			NodeID: participant.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  participant.Weight,
				Ejected: participant.Ejected,
			},
		})

		expectedNextEpochIdentityTable = append(expectedNextEpochIdentityTable, &flow.DynamicIdentityEntry{
			NodeID: participant.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  0,
				Ejected: participant.Ejected,
			},
		})
	}

	// in this loop we perform a few extra lookups to avoid duplicates
	for _, participant := range setup.Participants {
		if _, found := expectedCurrentEpochIdentityTable.ByNodeID(participant.NodeID); !found {
			expectedCurrentEpochIdentityTable = append(expectedCurrentEpochIdentityTable, &flow.DynamicIdentityEntry{
				NodeID: participant.NodeID,
				Dynamic: flow.DynamicIdentity{
					Weight:  0,
					Ejected: participant.Ejected,
				},
			})
		}

		entry := &flow.DynamicIdentityEntry{
			NodeID: participant.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  participant.InitialWeight,
				Ejected: participant.Ejected,
			},
		}
		existing, found := expectedNextEpochIdentityTable.ByNodeID(participant.NodeID)
		if found {
			existing.Dynamic = entry.Dynamic
		} else {
			expectedNextEpochIdentityTable = append(expectedNextEpochIdentityTable, entry)
		}
	}
	// finally, sort in canonical order
	expectedCurrentEpochIdentityTable = expectedCurrentEpochIdentityTable.Sort(order.IdentifierCanonical)
	expectedNextEpochIdentityTable = expectedNextEpochIdentityTable.Sort(order.IdentifierCanonical)

	// process actual event
	err := s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, hasChanges := s.updater.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), expectedCurrentEpochIdentityTable, updatedState.CurrentEpoch.Identities)
	nextEpoch := updatedState.NextEpoch
	require.NotNil(s.T(), nextEpoch, "should have next epoch protocol state")
	require.Equal(s.T(), nextEpoch.SetupID, setup.ID(),
		"should have correct setup ID for next protocol state")
	require.Equal(s.T(), expectedNextEpochIdentityTable, nextEpoch.Identities)
}

// TestProcessEpochSetupWithSameParticipants tests that processing epoch setup with overlapping participants results in correctly
// built updated protocol state. It should build a union of participants from current and next epoch for current and
// next epoch protocol states respectively.
func (s *UpdaterSuite) TestProcessEpochSetupWithSameParticipants() {
	overlappingNodes, err := s.parentProtocolState.CurrentEpochSetup.Participants.Sample(2)
	require.NoError(s.T(), err)
	setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		setup.Participants = append(setup.Participants, overlappingNodes...)
	})
	err = s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)
	updatedState, _, _ := s.updater.Build()
	expectedParticipants := flow.DynamicIdentityEntryListFromIdentities(
		s.parentProtocolState.CurrentEpochSetup.Participants.Union(setup.Participants.Map(mapfunc.WithWeight(0))),
	)
	require.Equal(s.T(), updatedState.CurrentEpoch.Identities,
		expectedParticipants,
		"should have all participants from current epoch and next epoch, but without duplicates")

	nextEpochParticipants := flow.DynamicIdentityEntryListFromIdentities(
		setup.Participants.Union(s.parentProtocolState.CurrentEpochSetup.Participants.Map(mapfunc.WithWeight(0))),
	)
	require.Equal(s.T(), updatedState.NextEpoch.Identities,
		nextEpochParticipants,
		"should have all participants from previous epoch and current epoch, but without duplicates")
}

// TestEpochSetupAfterIdentityChange tests that after processing epoch an setup event, all previously made changes to the identity table
// are preserved and reflected in the resulting protocol state.
func (s *UpdaterSuite) TestEpochSetupAfterIdentityChange() {
	currentEpochParticipants := s.parentProtocolState.CurrentEpochSetup.Participants.Copy() // DEEP copy of Identity List
	weightChanges, err := currentEpochParticipants.Sample(2)
	require.NoError(s.T(), err)
	ejectedChanges, err := currentEpochParticipants.Sample(2)
	require.NoError(s.T(), err)
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
		ProtocolStateEntry:  updatedState,
		PreviousEpochSetup:  s.parentProtocolState.PreviousEpochSetup,
		PreviousEpochCommit: s.parentProtocolState.PreviousEpochCommit,
		CurrentEpochSetup:   s.parentProtocolState.CurrentEpochSetup,
		CurrentEpochCommit:  s.parentProtocolState.CurrentEpochCommit,
		NextEpochSetup:      nil,
		NextEpochCommit:     nil,
		Identities:          s.parentProtocolState.Identities.Copy(),
		NextIdentities:      flow.IdentityList{},
	}
	// Update enriched data with the changes made to the low-level updated table
	for _, identity := range allUpdates {
		toBeUpdated, _ := updatedRichProtocolState.Identities.ByNodeID(identity.NodeID)
		toBeUpdated.DynamicIdentity = identity.DynamicIdentity
	}

	// now we can use it to construct updater for next block, which will process epoch setup event.
	nextBlock := unittest.BlockHeaderWithParentFixture(s.candidate)
	s.updater = NewUpdater(nextBlock, updatedRichProtocolState)

	setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		setup.Participants = append(setup.Participants, allUpdates...) // add those nodes that were changed in previous epoch
	})

	err = s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, _ = s.updater.Build()

	// assert that all changes made in previous epoch are preserved
	currentEpochLookup := updatedState.CurrentEpoch.Identities.Lookup()
	nextEpochLookup := updatedState.NextEpoch.Identities.Lookup()

	for _, updated := range ejectedChanges {
		currentEpochIdentity := currentEpochLookup[updated.NodeID]
		nextEpochIdentity := nextEpochLookup[updated.NodeID]
		require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
		require.Equal(s.T(), updated.NodeID, nextEpochIdentity.NodeID)

		require.Equal(s.T(), updated.Ejected, currentEpochIdentity.Dynamic.Ejected)
		require.Equal(s.T(), updated.Ejected, nextEpochIdentity.Dynamic.Ejected)
	}

	for _, updated := range weightChanges {
		currentEpochIdentity := currentEpochLookup[updated.NodeID]
		nextEpochIdentity := nextEpochLookup[updated.NodeID]
		require.Equal(s.T(), updated.NodeID, currentEpochIdentity.NodeID)
		require.Equal(s.T(), updated.NodeID, nextEpochIdentity.NodeID)

		require.Equal(s.T(), updated.DynamicIdentity.Weight, currentEpochIdentity.Dynamic.Weight)
		require.NotEqual(s.T(), updated.InitialWeight, currentEpochIdentity.Dynamic.Weight,
			"since we have updated weight it should not be equal to initial weight")
		require.Equal(s.T(), updated.InitialWeight, nextEpochIdentity.Dynamic.Weight,
			"we take information about weight from next epoc setup event")
	}
}
