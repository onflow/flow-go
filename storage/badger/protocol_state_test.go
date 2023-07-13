package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
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

	// invariant: Identities is a full identity table for the current epoch. Identities are sorted in canonical order. Never nil.
	allIdentities := append(state.PreviousEpochSetup.Participants, state.CurrentEpochSetup.Participants...).Sort(order.Canonical)
	assert.Equal(t, allIdentities, state.Identities, "identities should be a full identity table for the current epoch")

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
