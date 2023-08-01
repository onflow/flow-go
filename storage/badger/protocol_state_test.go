package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProtocolStateStorage tests if the protocol state is stored, retrieved and indexed correctly
func TestProtocolStateStorage(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		setups := NewEpochSetups(metrics, db)
		commits := NewEpochCommits(metrics, db)
		store := NewProtocolState(metrics, setups, commits, db, DefaultCacheSize)

		expected := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		protocolStateID := expected.ID()
		blockID := unittest.IdentifierFixture()

		// store protocol state and auxiliary info
		err := transaction.Update(db, func(tx *transaction.Tx) error {
			// store epoch events to be able to retrieve them later
			err := setups.StoreTx(expected.PreviousEpochSetup)(tx)
			require.NoError(t, err)
			err = setups.StoreTx(expected.CurrentEpochSetup)(tx)
			require.NoError(t, err)
			err = setups.StoreTx(expected.NextEpochProtocolState.CurrentEpochSetup)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.PreviousEpochCommit)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.CurrentEpochCommit)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.NextEpochProtocolState.CurrentEpochCommit)(tx)
			require.NoError(t, err)

			err = store.StoreTx(protocolStateID, &expected.ProtocolStateEntry)(tx)
			require.NoError(t, err)
			return store.Index(blockID, protocolStateID)(tx)
		})
		require.NoError(t, err)

		// fetch protocol state
		actual, err := store.ByID(protocolStateID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)

		assertRichProtocolStateValidity(t, actual)

		// fetch protocol state by block ID
		actualByBlockID, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, actualByBlockID)

		assertRichProtocolStateValidity(t, actualByBlockID)
	})
}

// TestProtocolStateStoreInvalidProtocolState tests that storing protocol state which has unsorted identities fails for
// current and next epoch protocol states.
func TestProtocolStateStoreInvalidProtocolState(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		setups := NewEpochSetups(metrics, db)
		commits := NewEpochCommits(metrics, db)
		store := NewProtocolState(metrics, setups, commits, db, DefaultCacheSize)
		invalid := unittest.ProtocolStateFixture().ProtocolStateEntry
		// swap first and second elements to break canonical order
		invalid.Identities[0], invalid.Identities[1] = invalid.Identities[1], invalid.Identities[0]

		err := transaction.Update(db, store.StoreTx(invalid.ID(), &invalid))
		require.Error(t, err)

		invalid = unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState()).ProtocolStateEntry
		// swap first and second elements to break canonical order
		invalid.NextEpochProtocolState.Identities[0], invalid.NextEpochProtocolState.Identities[1] = invalid.NextEpochProtocolState.Identities[1], invalid.NextEpochProtocolState.Identities[0]

		err = transaction.Update(db, store.StoreTx(invalid.ID(), &invalid))
		require.Error(t, err)
	})
}

// TestProtocolStateMergeParticipants tests that merging participants between epochs works correctly. We always take participants
// from current epoch and additionally add participants from previous epoch if they are not present in current epoch.
// If there is participant in previous and current epochs we should see it only once in the merged list and the entity has to be from current epoch.
func TestProtocolStateMergeParticipants(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		setups := NewEpochSetups(metrics, db)
		commits := NewEpochCommits(metrics, db)
		store := NewProtocolState(metrics, setups, commits, db, DefaultCacheSize)

		stateEntry := unittest.ProtocolStateFixture()
		require.Equal(t, stateEntry.CurrentEpochSetup.Participants[1], stateEntry.PreviousEpochSetup.Participants[1])
		// change address of participant in current epoch, so we can distinguish it from the one in previous epoch
		// when performing assertion.
		newAddress := "123"
		nodeID := stateEntry.CurrentEpochSetup.Participants[1].NodeID
		stateEntry.CurrentEpochSetup.Participants[1].Address = newAddress
		stateEntry.CurrentEpochEventIDs.SetupID = stateEntry.CurrentEpochSetup.ID()
		protocolStateID := stateEntry.ID()

		// store protocol state and auxiliary info
		err := transaction.Update(db, func(tx *transaction.Tx) error {
			// store epoch events to be able to retrieve them later
			err := setups.StoreTx(stateEntry.PreviousEpochSetup)(tx)
			require.NoError(t, err)
			err = setups.StoreTx(stateEntry.CurrentEpochSetup)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(stateEntry.PreviousEpochCommit)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(stateEntry.CurrentEpochCommit)(tx)
			require.NoError(t, err)

			return store.StoreTx(protocolStateID, &stateEntry.ProtocolStateEntry)(tx)
		})
		require.NoError(t, err)

		// fetch protocol state
		actual, err := store.ByID(protocolStateID)
		require.NoError(t, err)
		require.Equal(t, stateEntry, actual)

		assertRichProtocolStateValidity(t, actual)
		identity, ok := actual.Identities.ByNodeID(nodeID)
		require.True(t, ok)
		require.Equal(t, newAddress, identity.Address)
	})
}

// assertRichProtocolStateValidity checks if RichProtocolState holds its invariant and is correctly populated by storage layer.
func assertRichProtocolStateValidity(t *testing.T, state *flow.RichProtocolStateEntry) {
	// invariant: CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
	assert.Equal(t, state.CurrentEpochSetup.Counter, state.CurrentEpochCommit.Counter, "current epoch setup and commit should be for the same epoch")
	assert.Equal(t, state.CurrentEpochSetup.Counter, state.PreviousEpochSetup.Counter+1, "current epoch setup should be next after previous epoch")

	// invariant: CurrentEpochSetup and CurrentEpochCommit IDs are the equal to the ID of the protocol state entry. Never nil.
	assert.Equal(t, state.CurrentEpochSetup.ID(), state.ProtocolStateEntry.CurrentEpochEventIDs.SetupID, "epoch setup should be for correct event ID")
	assert.Equal(t, state.CurrentEpochCommit.ID(), state.ProtocolStateEntry.CurrentEpochEventIDs.CommitID, "epoch commit should be for correct event ID")

	// invariant: PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Never nil.
	assert.Equal(t, state.PreviousEpochSetup.Counter, state.PreviousEpochCommit.Counter, "previous epoch setup and commit should be for the same epoch")

	// invariant: PreviousEpochSetup and PreviousEpochCommit IDs are the equal to the ID of the protocol state entry. Never nil.
	assert.Equal(t, state.PreviousEpochSetup.ID(), state.ProtocolStateEntry.PreviousEpochEventIDs.SetupID, "epoch setup should be for correct event ID")
	assert.Equal(t, state.PreviousEpochCommit.ID(), state.ProtocolStateEntry.PreviousEpochEventIDs.CommitID, "epoch commit should be for correct event ID")

	// invariant: Identities is a full identity table for the current epoch. Identities are sorted in canonical order. Without duplicates. Never nil.
	allIdentities := state.CurrentEpochSetup.Participants.Union(state.PreviousEpochSetup.Participants)
	assert.Equal(t, allIdentities, state.Identities, "identities should be a full identity table for the current epoch, without duplicates")

	for i, identity := range state.ProtocolStateEntry.Identities {
		assert.Equal(t, identity.NodeID, allIdentities[i].NodeID, "identity node ID should match")
	}

	nextEpochState := state.NextEpochProtocolState
	if nextEpochState == nil {
		return
	}
	// invariant: NextEpochProtocolState is a protocol state for the next epoch. Can be nil.
	assertRichProtocolStateValidity(t, nextEpochState)
}
