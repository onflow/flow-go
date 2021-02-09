package badger_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	protoutil "github.com/onflow/flow-go/state/protocol/util"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	storutil "github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBootstrapAndOpen verifies after bootstrapping with a root snapshot
// we should be able to open it and got the same state.
func TestBootstrapAndOpen(t *testing.T) {

	// create a state root and bootstrap the protocol state with it
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants, func(block *flow.Block) {
		block.Header.ParentID = unittest.IdentifierFixture()
	})

	protoutil.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {
		// protocol state has been bootstrapped, now open a protocol state with the database
		metrics := new(metrics.NoopCollector)
		all := storagebadger.InitAll(metrics, db)
		state, err := bprotocol.OpenState(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.Setups,
			all.EpochCommits,
			all.Statuses)
		require.NoError(t, err)

		unittest.AssertSnapshotsEqual(t, rootSnapshot, state.Final())
	})
}

// TestBootstrapNonRoot tests bootstrapping the protocol state
// from arbitrary states.
func TestBootstrapNonRoot(t *testing.T) {

	// start with a regular post-spork root snapshot
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootBlock, err := rootSnapshot.Head()
	require.NoError(t, err)

	t.Run("with one block built", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) {
			block1 := unittest.BlockWithParentFixture(rootBlock)
			err = state.Extend(&block1)
			require.NoError(t, err)
		})
		bootstrap(t, after, func(state *bprotocol.State, err error) {
			assert.NoError(t, err)
		})
	})
}

func TestBootstrapDuplicateID(t *testing.T) {
	participants := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4}, // dupe
	}
	root := unittest.RootSnapshotFixture(participants)
	bootstrap(t, root, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrapZeroStake(t *testing.T) {
	participants := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 0},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
	}
	root := unittest.RootSnapshotFixture(participants)
	bootstrap(t, root, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrapMissingRole(t *testing.T) {
	requiredRoles := []flow.Role{
		flow.RoleConsensus,
		flow.RoleCollection,
		flow.RoleExecution,
		flow.RoleVerification,
	}

	for _, role := range requiredRoles {
		t.Run(fmt.Sprintf("no %s nodes", role.String()), func(t *testing.T) {
			participants := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(role))
			root := unittest.RootSnapshotFixture(participants)
			bootstrap(t, root, func(state *bprotocol.State, err error) {
				assert.Error(t, err)
			})
		})
	}
}

func TestBootstrapExistingAddress(t *testing.T) {
	participants := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a1", Role: flow.RoleConsensus, Stake: 2}, // dupe address
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
	}

	root := unittest.RootSnapshotFixture(participants)
	bootstrap(t, root, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

// bootstraps protocol state with the given snapshot and invokes the callback
// with the result of the constructor
func bootstrap(t *testing.T, rootSnapshot protocol.Snapshot, f func(*bprotocol.State, error)) {
	metrics := metrics.NewNoopCollector()
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)
	db := unittest.BadgerDB(t, dir)
	defer db.Close()
	headers, _, seals, _, _, blocks, setups, commits, statuses, results := storutil.StorageLayer(t, db)
	state, err := bprotocol.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
	f(state, err)
}

// snapshotAfter returns an in-memory state snapshot of the FINALIZED state after
// bootstrapping the protocol state from the root snapshot, applying the state
// mutating function f, and clearing the on-disk protocol state.
//
// This is used for generating valid snapshots to use when testing bootstrapping
// from non-root states.
func snapshotAfter(t *testing.T, rootSnapshot protocol.Snapshot, f func(*bprotocol.FollowerState)) protocol.Snapshot {
	var after protocol.Snapshot
	protoutil.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
		f(state)
		final := state.Final()
		var err error
		after, err = inmem.FromSnapshot(final)
		require.NoError(t, err)
	})
	return after
}
