package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/mapfunc"
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
			err = setups.StoreTx(expected.NextEpochSetup)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.PreviousEpochCommit)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.CurrentEpochCommit)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.NextEpochCommit)(tx)
			require.NoError(t, err)

			err = store.StoreTx(protocolStateID, expected.ProtocolStateEntry)(tx)
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
		invalid.CurrentEpoch.ActiveIdentities[0], invalid.CurrentEpoch.ActiveIdentities[1] = invalid.CurrentEpoch.ActiveIdentities[1], invalid.CurrentEpoch.ActiveIdentities[0]

		err := transaction.Update(db, store.StoreTx(invalid.ID(), invalid))
		require.Error(t, err)

		invalid = unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState()).ProtocolStateEntry
		// swap first and second elements to break canonical order
		invalid.NextEpoch.ActiveIdentities[0], invalid.NextEpoch.ActiveIdentities[1] = invalid.NextEpoch.ActiveIdentities[1], invalid.NextEpoch.ActiveIdentities[0]

		err = transaction.Update(db, store.StoreTx(invalid.ID(), invalid))
		require.Error(t, err)
	})
}

// TestProtocolStateMergeParticipants tests that merging participants between epochs works correctly. We always take participants
// from current epoch and additionally add participants from previous epoch if they are not present in current epoch.
// If the same participant is in the previous and current epochs, we should see it only once in the merged list and the dynamic portion has to be from current epoch.
func TestProtocolStateMergeParticipants(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		setups := NewEpochSetups(metrics, db)
		commits := NewEpochCommits(metrics, db)
		store := NewProtocolState(metrics, setups, commits, db, DefaultCacheSize)

		stateEntry := unittest.ProtocolStateFixture()
		// change address of participant in current epoch, so we can distinguish it from the one in previous epoch
		// when performing assertion.
		newAddress := "123"
		nodeID := stateEntry.CurrentEpochSetup.Participants[1].NodeID
		stateEntry.CurrentEpochSetup.Participants[1].Address = newAddress
		stateEntry.CurrentEpoch.SetupID = stateEntry.CurrentEpochSetup.ID()
		identity, _ := stateEntry.CurrentEpochIdentityTable.ByNodeID(nodeID)
		identity.Address = newAddress
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

			return store.StoreTx(protocolStateID, stateEntry.ProtocolStateEntry)(tx)
		})
		require.NoError(t, err)

		// fetch protocol state
		actual, err := store.ByID(protocolStateID)
		require.NoError(t, err)
		require.Equal(t, stateEntry, actual)

		assertRichProtocolStateValidity(t, actual)
		identity, ok := actual.CurrentEpochIdentityTable.ByNodeID(nodeID)
		require.True(t, ok)
		require.Equal(t, newAddress, identity.Address)
	})
}

// TestProtocolStateRootSnapshot tests that storing and retrieving root protocol state (in case of bootstrap) works as expected.
// Specifically, this means that no prior epoch exists (situation after a spork) from the perspective of the freshly-sporked network.
func TestProtocolStateRootSnapshot(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		setups := NewEpochSetups(metrics, db)
		commits := NewEpochCommits(metrics, db)
		store := NewProtocolState(metrics, setups, commits, db, DefaultCacheSize)
		expected := unittest.RootProtocolStateFixture()

		protocolStateID := expected.ID()
		blockID := unittest.IdentifierFixture()

		// store protocol state and auxiliary info
		err := transaction.Update(db, func(tx *transaction.Tx) error {
			// store epoch events to be able to retrieve them later
			err := setups.StoreTx(expected.CurrentEpochSetup)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.CurrentEpochCommit)(tx)
			require.NoError(t, err)

			err = store.StoreTx(protocolStateID, expected.ProtocolStateEntry)(tx)
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
	// invariants:
	//  - CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
	//  - CurrentEpochSetup and CurrentEpochCommit IDs match respective commitments in the `ProtocolStateEntry`.
	assert.Equal(t, state.CurrentEpochSetup.Counter, state.CurrentEpochCommit.Counter, "current epoch setup and commit should be for the same epoch")
	assert.Equal(t, state.CurrentEpochSetup.ID(), state.ProtocolStateEntry.CurrentEpoch.SetupID, "epoch setup should be for correct event ID")
	assert.Equal(t, state.CurrentEpochCommit.ID(), state.ProtocolStateEntry.CurrentEpoch.CommitID, "epoch commit should be for correct event ID")

	var (
		previousEpochParticipants flow.IdentityList
		err                       error
	)
	// invariant: PreviousEpochSetup and PreviousEpochCommit should be present if respective ID is not zero.
	if state.PreviousEpoch != nil {
		// invariant: PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Never nil.
		assert.Equal(t, state.PreviousEpochSetup.Counter+1, state.CurrentEpochSetup.Counter, "current epoch (%d) should be following right after previous epoch (%d)", state.CurrentEpochSetup.Counter, state.PreviousEpochSetup.Counter)
		assert.Equal(t, state.PreviousEpochSetup.Counter, state.PreviousEpochCommit.Counter, "previous epoch setup and commit should be for the same epoch")

		// invariant: PreviousEpochSetup and PreviousEpochCommit IDs are the equal to the ID of the protocol state entry. Never nil.
		assert.Equal(t, state.PreviousEpochSetup.ID(), state.ProtocolStateEntry.PreviousEpoch.SetupID, "epoch setup should be for correct event ID")
		assert.Equal(t, state.PreviousEpochCommit.ID(), state.ProtocolStateEntry.PreviousEpoch.CommitID, "epoch commit should be for correct event ID")

		// invariant: ComposeFullIdentities ensures that we can build full identities of previous epoch's active participants. This step also confirms that the
		// previous epoch's `Participants` [IdentitySkeletons] and `ActiveIdentities` [DynamicIdentity properties] list the same nodes in canonical ordering.
		previousEpochParticipants, err = flow.ComposeFullIdentities(
			state.PreviousEpochSetup.Participants,
			state.PreviousEpoch.ActiveIdentities,
			flow.EpochParticipationStatusActive,
		)
		assert.NoError(t, err, "should be able to reconstruct previous epoch active participants")
		// Function `ComposeFullIdentities` verified that `Participants` and `ActiveIdentities` have identical ordering w.r.t nodeID.
		// By construction, `participantsFromCurrentEpochSetup` lists the full Identities in the same ordering as `Participants` and
		// `ActiveIdentities`. By confirming that `participantsFromCurrentEpochSetup` follows canonical ordering, we can conclude that
		// also `Participants` and `ActiveIdentities` are canonically ordered.
		require.True(t, previousEpochParticipants.Sorted(order.Canonical[flow.Identity]), "participants in previous epoch's setup event are not in canonical order")
	}

	// invariant: ComposeFullIdentities ensures that we can build full identities of current epoch's *active* participants. This step also confirms that the
	// current epoch's `Participants` [IdentitySkeletons] and `ActiveIdentities` [DynamicIdentity properties] list the same nodes in canonical ordering.
	participantsFromCurrentEpochSetup, err := flow.ComposeFullIdentities(
		state.CurrentEpochSetup.Participants,
		state.CurrentEpoch.ActiveIdentities,
		flow.EpochParticipationStatusActive,
	)
	assert.NoError(t, err, "should be able to reconstruct current epoch active participants")
	require.True(t, participantsFromCurrentEpochSetup.Sorted(order.Canonical[flow.Identity]), "participants in current epoch's setup event are not in canonical order")

	// invariants for `CurrentEpochIdentityTable`:
	//  - full identity table containing *active* nodes for the current epoch + weight-zero identities of adjacent epoch
	//  - Identities are sorted in canonical order. Without duplicates. Never nil.
	var allIdentities, participantsFromNextEpochSetup flow.IdentityList
	if state.NextEpoch != nil {
		// setup/commit phase
		// invariant: ComposeFullIdentities ensures that we can build full identities of next epoch's *active* participants. This step also confirms that the
		// next epoch's `Participants` [IdentitySkeletons] and `ActiveIdentities` [DynamicIdentity properties] list the same nodes in canonical ordering.
		participantsFromNextEpochSetup, err = flow.ComposeFullIdentities(
			state.NextEpochSetup.Participants,
			state.NextEpoch.ActiveIdentities,
			flow.EpochParticipationStatusActive,
		)
		assert.NoError(t, err, "should be able to reconstruct next epoch active participants")
		allIdentities = participantsFromCurrentEpochSetup.Union(participantsFromNextEpochSetup.Copy().Map(mapfunc.WithEpochParticipationStatus(flow.EpochParticipationStatusJoining)))
	} else {
		// staking phase
		allIdentities = participantsFromCurrentEpochSetup.Union(previousEpochParticipants.Copy().Map(mapfunc.WithEpochParticipationStatus(flow.EpochParticipationStatusLeaving)))
	}
	assert.Equal(t, allIdentities, state.CurrentEpochIdentityTable, "identities should be a full identity table for the current epoch, without duplicates")
	require.True(t, allIdentities.Sorted(order.Canonical[flow.Identity]), "current epoch's identity table is not in canonical order")

	// check next epoch; only applicable during setup/commit phase
	if state.NextEpoch == nil { // during staking phase, next epoch is not yet specified; hence there is nothing else to check
		return
	}

	// invariants:
	//  - NextEpochSetup and NextEpochCommit are for the same epoch. Never nil.
	//  - NextEpochSetup and NextEpochCommit IDs match respective commitments in the `ProtocolStateEntry`.
	assert.Equal(t, state.CurrentEpochSetup.Counter+1, state.NextEpochSetup.Counter, "next epoch (%d) should be following right after current epoch (%d)", state.NextEpochSetup.Counter, state.CurrentEpochSetup.Counter)
	assert.Equal(t, state.NextEpochSetup.Counter, state.NextEpochCommit.Counter, "next epoch setup and commit should be for the same epoch")
	assert.Equal(t, state.NextEpochSetup.ID(), state.NextEpoch.SetupID, "epoch setup should be for correct event ID")
	assert.Equal(t, state.NextEpochCommit.ID(), state.NextEpoch.CommitID, "epoch commit should be for correct event ID")

	// invariants for `NextEpochIdentityTable`:
	//  - full identity table containing *active* nodes for next epoch + weight-zero identities of current epoch
	//  - Identities are sorted in canonical order. Without duplicates. Never nil.
	allIdentities = participantsFromNextEpochSetup.Union(participantsFromCurrentEpochSetup.Copy().Map(mapfunc.WithEpochParticipationStatus(flow.EpochParticipationStatusLeaving)))
	assert.Equal(t, allIdentities, state.NextEpochIdentityTable, "identities should be a full identity table for the next epoch, without duplicates")
}
