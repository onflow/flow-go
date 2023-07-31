package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
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

	s.updater = newUpdater(s.candidate, s.parentProtocolState)
}

// TestNewUpdater tests if the constructor correctly setups invariants for updater.
func (s *UpdaterSuite) TestNewUpdater() {
	require.NotSame(s.T(), s.updater.parentState, s.updater.state, "except to take deep copy of parent state")
	require.Nil(s.T(), s.updater.parentState.NextEpochProtocolState)
	require.Nil(s.T(), s.updater.state.NextEpochProtocolState)
}

// TestTransitionToNextEpoch tests a scenario where the updater processes first block from next epoch.
// It has to discard the parent state and build a new state with data from next epoch.
func (s *UpdaterSuite) TestTransitionToNextEpoch() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)

	candidate := unittest.BlockHeaderFixture(
		unittest.HeaderWithView(s.parentProtocolState.CurrentEpochSetup.FinalView + 1))
	// since candidate block is from next epoch, updater should transition to next epoch
	s.updater = newUpdater(candidate, s.parentProtocolState)
	updatedState, _, _ := s.updater.Build()
	require.Equal(s.T(), updatedState.ID(), s.parentProtocolState.NextEpochProtocolState.ID(), "should transition into next epoch")
	require.Nil(s.T(), updatedState.NextEpochProtocolState, "next epoch protocol state should be nil")
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

// TestSetInvalidStateTransitionAttempted tests if setting invalid state transition attempted flag is reflected in built
// protocol state. It should be set for both current and next epoch protocol state.
func (s *UpdaterSuite) TestSetInvalidStateTransitionAttempted() {
	// update protocol state with next epoch information
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)
	// create new updater with next epoch information
	s.updater = newUpdater(s.candidate, s.parentProtocolState)

	s.updater.SetInvalidStateTransitionAttempted()
	updatedState, _, hasChanges := s.updater.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.True(s.T(), updatedState.InvalidStateTransitionAttempted, "should set invalid state transition attempted")
	require.True(s.T(), updatedState.NextEpochProtocolState.InvalidStateTransitionAttempted,
		"should set invalid state transition attempted for next epoch as well")
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
		updater := newUpdater(s.candidate, s.parentProtocolState)
		setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
		})
		updater.SetInvalidStateTransitionAttempted()
		err := updater.ProcessEpochSetup(setup)
		require.NoError(s.T(), err)

		updatedState, _, _ := updater.Build()
		require.Nil(s.T(), updatedState.NextEpochProtocolState, "should not process epoch setup if invalid state transition attempted")
	})
	s.Run("processing second epoch setup", func() {
		updater := newUpdater(s.candidate, s.parentProtocolState)
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
	for _, participant := range setup.Participants {
		expectedCurrentEpochIdentityTable = append(expectedCurrentEpochIdentityTable, &flow.DynamicIdentityEntry{
			NodeID: participant.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  0,
				Ejected: participant.Ejected,
			},
		})

		expectedNextEpochIdentityTable = append(expectedNextEpochIdentityTable, &flow.DynamicIdentityEntry{
			NodeID: participant.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  participant.InitialWeight,
				Ejected: participant.Ejected,
			},
		})
	}

	// process actual event
	err := s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)

	updatedState, _, hasChanges := s.updater.Build()
	require.True(s.T(), hasChanges, "should have changes")
	require.Equal(s.T(), expectedCurrentEpochIdentityTable, updatedState.Identities)
	nextEpochProtocolState := updatedState.NextEpochProtocolState
	require.NotNil(s.T(), nextEpochProtocolState, "should have next epoch protocol state")
	require.Equal(s.T(), nextEpochProtocolState.CurrentEpochEventIDs.SetupID, setup.ID(),
		"should have correct setup ID for next protocol state")
	require.False(s.T(), nextEpochProtocolState.InvalidStateTransitionAttempted, "should not set invalid state transition attempted")
	require.Nil(s.T(), nextEpochProtocolState.NextEpochProtocolState, "should not have next epoch protocol state")
	require.Equal(s.T(), expectedNextEpochIdentityTable, nextEpochProtocolState.Identities)
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
		updater := newUpdater(s.candidate, s.parentProtocolState)
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
		require.Equal(s.T(), flow.ZeroID, newState.NextEpochProtocolState.CurrentEpochEventIDs.CommitID,
			"operation must be no-op")
	})
	s.Run("happy path processing", func() {
		updater := newUpdater(s.candidate, s.parentProtocolState)
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
		require.Equal(s.T(), commit.ID(), newState.NextEpochProtocolState.CurrentEpochEventIDs.CommitID, "next epoch must be committed")
	})
}

// TestUpdateIdentityUnknownIdentity tests if updating identity of unknown node results in an error.
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

// TestUpdateIdentityHappyPath tests if identity updates are correctly processed and reflected in resulting protocol state.
func (s *UpdaterSuite) TestUpdateIdentityHappyPath() {
	// update protocol state to have next epoch protocol state
	unittest.WithNextEpochProtocolState()(s.parentProtocolState)
	s.updater = newUpdater(s.candidate, s.parentProtocolState)

	weightChanges, err := s.parentProtocolState.CurrentEpochSetup.Participants.Sample(5)
	require.NoError(s.T(), err)
	ejectedChanges, err := s.parentProtocolState.CurrentEpochSetup.Participants.Sample(2)
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
	requireUpdatesApplied(updatedState.Identities.Lookup())
	requireUpdatesApplied(updatedState.NextEpochProtocolState.Identities.Lookup())
}

// TestEpochSetupAfterIdentityChange tests that after processing epoch setup event all previously made changes to the identity table
// are preserved and reflected in the resulting protocol state.
func (s *UpdaterSuite) TestEpochSetupAfterIdentityChange() {
	weightChanges, err := s.parentProtocolState.CurrentEpochSetup.Participants.Sample(5)
	require.NoError(s.T(), err)
	ejectedChanges, err := s.parentProtocolState.CurrentEpochSetup.Participants.Sample(2)
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
	updatedState, _, _ := s.updater.Build()

	// Construct a valid flow.RichProtocolStateEntry for next block
	// We do this by copying the parent protocol state and updating the identities manually
	updatedRichProtocolState := &flow.RichProtocolStateEntry{
		ProtocolStateEntry:     *updatedState,
		CurrentEpochSetup:      s.parentProtocolState.CurrentEpochSetup,
		CurrentEpochCommit:     s.parentProtocolState.CurrentEpochCommit,
		PreviousEpochSetup:     s.parentProtocolState.PreviousEpochSetup,
		PreviousEpochCommit:    s.parentProtocolState.PreviousEpochCommit,
		Identities:             s.parentProtocolState.Identities.Copy(),
		NextEpochProtocolState: nil,
	}
	// Update enriched data with the changes made to the low-level identity table
	for _, identity := range allUpdates {
		toBeUpdated, _ := updatedRichProtocolState.Identities.ByNodeID(identity.NodeID)
		toBeUpdated.DynamicIdentity = identity.DynamicIdentity
	}

	// now we can use it to construct updater for next block, which will process epoch setup event.
	nextBlock := unittest.BlockHeaderWithParentFixture(s.candidate)
	s.updater = newUpdater(nextBlock, updatedRichProtocolState)

	setup := unittest.EpochSetupFixture(func(setup *flow.EpochSetup) {
		setup.Counter = s.parentProtocolState.CurrentEpochSetup.Counter + 1
	})

	err = s.updater.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)
}
