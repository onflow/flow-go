package badger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestUnknownSnapshotReference verifies that AtBlockID returns a snapshot that
// returns state.ErrUnknownSnapshotReference for all methods when given an unknown block ID.
func TestUnknownSnapshotReference(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		// Setup
		genesis, err := unittest.ClusterBlock.Genesis()
		require.NoError(t, err)

		root := unittest.RootSnapshotFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))
		epochCounter := root.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.EpochCounter()

		clusterStateRoot, err := NewStateRoot(genesis, unittest.QuorumCertificateFixture(), epochCounter)
		require.NoError(t, err)
		clusterState, err := Bootstrap(db, lockManager, clusterStateRoot)
		require.NoError(t, err)

		// Test
		unknownBlockID := unittest.IdentifierFixture()
		snapshot := clusterState.AtBlockID(unknownBlockID)

		// Verify that Collection() returns state.ErrUnknownSnapshotReference
		_, err = snapshot.Collection()
		assert.Error(t, err)
		assert.ErrorIs(t, err, state.ErrUnknownSnapshotReference)

		// Verify that Head() returns state.ErrUnknownSnapshotReference
		_, err = snapshot.Head()
		assert.Error(t, err)
		assert.ErrorIs(t, err, state.ErrUnknownSnapshotReference)

		// Verify that Pending() returns state.ErrUnknownSnapshotReference
		_, err = snapshot.Pending()
		assert.Error(t, err)
		assert.ErrorIs(t, err, state.ErrUnknownSnapshotReference)
	})
}

// TestValidSnapshotReference verifies that AtBlockID returns a working snapshot
// when given a valid block ID (the genesis block).
func TestValidSnapshotReference(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		// Setup
		genesis, err := unittest.ClusterBlock.Genesis()
		require.NoError(t, err)

		root := unittest.RootSnapshotFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))
		epochCounter := root.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.EpochCounter()

		clusterStateRoot, err := NewStateRoot(genesis, unittest.QuorumCertificateFixture(), epochCounter)
		require.NoError(t, err)
		clusterState, err := Bootstrap(db, lockManager, clusterStateRoot)
		require.NoError(t, err)

		// Test with valid block ID (genesis block)
		snapshot := clusterState.AtBlockID(genesis.ID())

		// Verify that Collection() works correctly
		collection, err := snapshot.Collection()
		assert.NoError(t, err)
		assert.Equal(t, &genesis.Payload.Collection, collection)

		// Verify that Head() works correctly
		head, err := snapshot.Head()
		assert.NoError(t, err)
		assert.Equal(t, genesis.ToHeader().ID(), head.ID())

		// Verify that Pending() works correctly (should return empty list for genesis)
		pending, err := snapshot.Pending()
		assert.NoError(t, err)
		assert.Empty(t, pending)
	})
}
