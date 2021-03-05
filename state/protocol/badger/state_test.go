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

	protoutil.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, _ *bprotocol.State) {
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

// TestBootstrapNonRoot tests bootstrapping the protocol state from arbitrary states.
//
// NOTE: for all these cases, we build a final child block (CHILD). This is
// needed otherwise the parent block would not have a valid QC, since the QC
// is stored in the child.
func TestBootstrapNonRoot(t *testing.T) {
	t.Parallel()
	// start with a regular post-spork root snapshot
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootBlock, err := rootSnapshot.Head()
	require.NoError(t, err)

	// should be able to bootstrap from snapshot after building one block
	// ROOT <- B1 <- CHILD
	t.Run("with one block built", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
			block1 := unittest.BlockWithParentFixture(rootBlock)
			buildBlock(t, state, &block1)
			child := unittest.BlockWithParentFixture(block1.Header)
			buildBlock(t, state, &child)

			return state.AtBlockID(block1.ID())
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			unittest.AssertSnapshotsEqual(t, after, state.Final())
		})
	})

	// should be able to bootstrap from snapshot after sealing a non-root block
	// ROOT <- B1 <- B2(S1) <- CHILD
	t.Run("with sealed block", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
			block1 := unittest.BlockWithParentFixture(rootBlock)
			buildBlock(t, state, &block1)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(&block1)
			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal1), unittest.WithReceipts(receipt1)))
			buildBlock(t, state, &block2)

			child := unittest.BlockWithParentFixture(block2.Header)
			buildBlock(t, state, &child)

			return state.AtBlockID(block2.ID())
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			unittest.AssertSnapshotsEqual(t, after, state.Final())
		})
	})

	t.Run("with setup next epoch", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
			unittest.NewEpochBuilder(t, state).BuildEpoch()

			// find the point where we transition to the epoch setup phase
			for height := rootBlock.Height + 1; ; height++ {
				phase, err := state.AtHeight(height).Phase()
				require.NoError(t, err)
				if phase == flow.EpochPhaseSetup {
					return state.AtHeight(height)
				}
			}
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			unittest.AssertSnapshotsEqual(t, after, state.Final())
		})
	})

	t.Run("with committed next epoch", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
			unittest.NewEpochBuilder(t, state).BuildEpoch().CompleteEpoch()

			// find the point where we transition to the epoch committed phase
			for height := rootBlock.Height + 1; ; height++ {
				phase, err := state.AtHeight(height).Phase()
				require.NoError(t, err)
				if phase == flow.EpochPhaseCommitted {
					return state.AtHeight(height)
				}
			}
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			unittest.AssertSnapshotsEqual(t, after, state.Final())
		})
	})

	t.Run("with previous and next epoch", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
			unittest.NewEpochBuilder(t, state).
				BuildEpoch().CompleteEpoch(). // build epoch 2
				BuildEpoch()                  // build epoch 3

			// find a snapshot from epoch setup phase in epoch 2
			epoch1Counter, err := rootSnapshot.Epochs().Current().Counter()
			require.NoError(t, err)
			for height := rootBlock.Height + 1; ; height++ {
				snap := state.AtHeight(height)
				counter, err := snap.Epochs().Current().Counter()
				require.NoError(t, err)
				phase, err := snap.Phase()
				require.NoError(t, err)
				if phase == flow.EpochPhaseSetup && counter == epoch1Counter+1 {
					return snap
				}
			}
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			unittest.AssertSnapshotsEqual(t, after, state.Final())
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

func TestBootstrap_DisconnectedSealingSegment(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	// convert to encodable to easily modify snapshot
	encodable := rootSnapshot.Encodable()
	// add an un-connected tail block to the sealing segment
	tail := unittest.BlockFixture()
	encodable.SealingSegment = append([]*flow.Block{&tail}, encodable.SealingSegment...)
	rootSnapshot = inmem.SnapshotFromEncodable(encodable)

	bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrap_InvalidQuorumCertificate(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	// convert to encodable to easily modify snapshot
	encodable := rootSnapshot.Encodable()
	encodable.QuorumCertificate.BlockID = unittest.IdentifierFixture()
	rootSnapshot = inmem.SnapshotFromEncodable(encodable)

	bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrap_SealMismatch(t *testing.T) {
	t.Run("seal doesn't match tail block", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
		// convert to encodable to easily modify snapshot
		encodable := rootSnapshot.Encodable()
		encodable.LatestSeal.BlockID = unittest.IdentifierFixture()

		bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("result doesn't match tail block", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
		// convert to encodable to easily modify snapshot
		encodable := rootSnapshot.Encodable()
		encodable.LatestResult.BlockID = unittest.IdentifierFixture()

		bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("seal doesn't match result", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
		// convert to encodable to easily modify snapshot
		encodable := rootSnapshot.Encodable()
		encodable.LatestSeal.ResultID = unittest.IdentifierFixture()

		bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
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

// snapshotAfter bootstraps the protocol state from the root snapshot, applies
// the state-changing function f, clears the on-disk state, and returns a
// memory-backed snapshot corresponding to that returned by f.
//
// This is used for generating valid snapshots to use when testing bootstrapping
// from non-root states.
func snapshotAfter(t *testing.T, rootSnapshot protocol.Snapshot, f func(*bprotocol.FollowerState) protocol.Snapshot) protocol.Snapshot {
	var after protocol.Snapshot
	protoutil.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
		snap := f(state)
		var err error
		after, err = inmem.FromSnapshot(snap)
		require.NoError(t, err)
	})
	return after
}

// buildBlock builds and marks valid the given block
func buildBlock(t *testing.T, state protocol.MutableState, block *flow.Block) {
	err := state.Extend(block)
	require.NoError(t, err)
	err = state.MarkValid(block.ID())
	require.NoError(t, err)
}
