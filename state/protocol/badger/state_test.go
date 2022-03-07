package badger_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mock "github.com/onflow/flow-go/module/mock"
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

		// expect the final view metric to be set to current epoch's final view
		epoch := rootSnapshot.Epochs().Current()
		finalView, err := epoch.FinalView()
		require.NoError(t, err)
		counter, err := epoch.Counter()
		require.NoError(t, err)
		phase, err := rootSnapshot.Phase()
		require.NoError(t, err)

		complianceMetrics := new(mock.ComplianceMetrics)
		complianceMetrics.On("CommittedEpochFinalView", finalView).Once()
		complianceMetrics.On("CurrentEpochCounter", counter).Once()
		complianceMetrics.On("CurrentEpochPhase", phase).Once()
		complianceMetrics.On("CurrentEpochFinalView", finalView).Once()

		dkgPhase1FinalView, dkgPhase2FinalView, dkgPhase3FinalView, err := protocol.DKGPhaseViews(epoch)
		require.NoError(t, err)
		complianceMetrics.On("CurrentDKGPhase1FinalView", dkgPhase1FinalView).Once()
		complianceMetrics.On("CurrentDKGPhase2FinalView", dkgPhase2FinalView).Once()
		complianceMetrics.On("CurrentDKGPhase3FinalView", dkgPhase3FinalView).Once()

		noopMetrics := new(metrics.NoopCollector)
		all := storagebadger.InitAll(noopMetrics, db)
		// protocol state has been bootstrapped, now open a protocol state with the database
		state, err := bprotocol.OpenState(complianceMetrics, db, all.Headers, all.Seals, all.Results, all.Blocks, all.Setups, all.EpochCommits, all.Statuses)
		require.NoError(t, err)

		complianceMetrics.AssertExpectations(t)

		unittest.AssertSnapshotsEqual(t, rootSnapshot, state.Final())
	})
}

// TestBootstrapAndOpen_EpochCommitted verifies after bootstrapping with a
// root snapshot from EpochCommitted phase  we should be able to open it and
// got the same state.
func TestBootstrapAndOpen_EpochCommitted(t *testing.T) {

	// create a state root and bootstrap the protocol state with it
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants, func(block *flow.Block) {
		block.Header.ParentID = unittest.IdentifierFixture()
	})
	rootBlock, err := rootSnapshot.Head()
	require.NoError(t, err)

	// build an epoch on the root state and return a snapshot from the committed phase
	committedPhaseSnapshot := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
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

	protoutil.RunWithBootstrapState(t, committedPhaseSnapshot, func(db *badger.DB, _ *bprotocol.State) {

		complianceMetrics := new(mock.ComplianceMetrics)

		// expect the final view metric to be set to next epoch's final view
		finalView, err := committedPhaseSnapshot.Epochs().Next().FinalView()
		require.NoError(t, err)
		complianceMetrics.On("CommittedEpochFinalView", finalView).Once()

		// expect counter to be set to current epochs counter
		counter, err := committedPhaseSnapshot.Epochs().Current().Counter()
		require.NoError(t, err)
		complianceMetrics.On("CurrentEpochCounter", counter).Once()

		// expect epoch phase to be set to current phase
		phase, err := committedPhaseSnapshot.Phase()
		require.NoError(t, err)
		complianceMetrics.On("CurrentEpochPhase", phase).Once()

		currentEpochFinalView, err := committedPhaseSnapshot.Epochs().Current().FinalView()
		require.NoError(t, err)
		complianceMetrics.On("CurrentEpochFinalView", currentEpochFinalView).Once()

		dkgPhase1FinalView, dkgPhase2FinalView, dkgPhase3FinalView, err := protocol.DKGPhaseViews(committedPhaseSnapshot.Epochs().Current())
		require.NoError(t, err)
		complianceMetrics.On("CurrentDKGPhase1FinalView", dkgPhase1FinalView).Once()
		complianceMetrics.On("CurrentDKGPhase2FinalView", dkgPhase2FinalView).Once()
		complianceMetrics.On("CurrentDKGPhase3FinalView", dkgPhase3FinalView).Once()

		noopMetrics := new(metrics.NoopCollector)
		all := storagebadger.InitAll(noopMetrics, db)
		state, err := bprotocol.OpenState(complianceMetrics, db, all.Headers, all.Seals, all.Results, all.Blocks, all.Setups, all.EpochCommits, all.Statuses)
		require.NoError(t, err)

		// assert update final view was called
		complianceMetrics.AssertExpectations(t)

		unittest.AssertSnapshotsEqual(t, committedPhaseSnapshot, state.Final())
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

	// should be able to bootstrap from snapshot after sealing a non-root block
	// ROOT <- B1 <- B2(S1) <- CHILD
	t.Run("with sealed block", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState) protocol.Snapshot {
			block1 := unittest.BlockWithParentFixture(rootBlock)
			buildBlock(t, state, block1)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal1), unittest.WithReceipts(receipt1)))
			buildBlock(t, state, block2)

			child := unittest.BlockWithParentFixture(block2.Header)
			buildBlock(t, state, child)

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

func TestBootstrap_InvalidIdentities(t *testing.T) {
	t.Run("duplicate node ID", func(t *testing.T) {
		participants := unittest.CompleteIdentitySet()
		dupeIDIdentity := unittest.IdentityFixture(unittest.WithNodeID(participants[0].NodeID))
		participants = append(participants, dupeIDIdentity)

		root := unittest.RootSnapshotFixture(participants)
		bootstrap(t, root, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("zero weight", func(t *testing.T) {
		zeroWeightIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification), unittest.WithWeight(0))
		participants := unittest.CompleteIdentitySet(zeroWeightIdentity)
		root := unittest.RootSnapshotFixture(participants)
		bootstrap(t, root, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("missing role", func(t *testing.T) {
		requiredRoles := []flow.Role{
			flow.RoleConsensus,
			flow.RoleCollection,
			flow.RoleExecution,
			flow.RoleVerification,
		}

		for _, role := range requiredRoles {
			t.Run(fmt.Sprintf("no %s nodes", role), func(t *testing.T) {
				participants := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(role))
				root := unittest.RootSnapshotFixture(participants)
				bootstrap(t, root, func(state *bprotocol.State, err error) {
					assert.Error(t, err)
				})
			})
		}
	})

	t.Run("duplicate address", func(t *testing.T) {
		participants := unittest.CompleteIdentitySet()
		dupeAddressIdentity := unittest.IdentityFixture(unittest.WithAddress(participants[0].Address))
		participants = append(participants, dupeAddressIdentity)

		root := unittest.RootSnapshotFixture(participants)
		bootstrap(t, root, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("non-canonical ordering", func(t *testing.T) {
		participants := unittest.IdentityListFixture(20, unittest.WithAllRoles())

		root := unittest.RootSnapshotFixture(participants)
		// randomly shuffle the identities so they are not canonically ordered
		encodable := root.Encodable()
		encodable.Identities = participants.DeterministicShuffle(time.Now().UnixNano())
		root = inmem.SnapshotFromEncodable(encodable)
		bootstrap(t, root, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})
}

func TestBootstrap_DisconnectedSealingSegment(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	// convert to encodable to easily modify snapshot
	encodable := rootSnapshot.Encodable()
	// add an un-connected tail block to the sealing segment
	tail := unittest.BlockFixture()
	encodable.SealingSegment.Blocks = append([]*flow.Block{&tail}, encodable.SealingSegment.Blocks...)
	rootSnapshot = inmem.SnapshotFromEncodable(encodable)

	bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrap_SealingSegmentMissingSeal(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	// convert to encodable to easily modify snapshot
	encodable := rootSnapshot.Encodable()
	// we are missing the required first seal
	encodable.SealingSegment.FirstSeal = nil
	rootSnapshot = inmem.SnapshotFromEncodable(encodable)

	bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrap_SealingSegmentMissingResult(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	// convert to encodable to easily modify snapshot
	encodable := rootSnapshot.Encodable()
	// we are missing the result referenced by the root seal
	encodable.SealingSegment.ExecutionResults = nil
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
	err := state.Extend(context.Background(), block)
	require.NoError(t, err)
	err = state.MarkValid(block.ID())
	require.NoError(t, err)
}

// assertSealingSegmentBlocksQueryable bootstraps the state with the given
// snapshot, then verifies that all sealing segment blocks are queryable.
func assertSealingSegmentBlocksQueryableAfterBootstrap(t *testing.T, snapshot protocol.Snapshot) {
	bootstrap(t, snapshot, func(state *bprotocol.State, err error) {
		require.NoError(t, err)

		segment, err := state.Final().SealingSegment()
		assert.NoError(t, err)

		// for each block in the sealing segment we should be able to query:
		// * Head
		// * SealedResult
		// * Commit
		for _, block := range segment.Blocks {
			blockID := block.ID()
			snap := state.AtBlockID(blockID)
			header, err := snap.Head()
			assert.NoError(t, err)
			assert.Equal(t, blockID, header.ID())
			_, seal, err := snap.SealedResult()
			assert.NoError(t, err)
			assert.Equal(t, segment.LatestSeals[blockID], seal.ID())
			commit, err := snap.Commit()
			assert.NoError(t, err)
			assert.Equal(t, seal.FinalState, commit)
		}
		// for all blocks but the head, we should be unable to query SealingSegment:
		for _, block := range segment.Blocks[:len(segment.Blocks)-1] {
			snap := state.AtBlockID(block.ID())
			_, err := snap.SealingSegment()
			assert.ErrorIs(t, err, protocol.ErrSealingSegmentBelowRootBlock)
		}
	})
}
