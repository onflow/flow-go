package badger_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	testmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/util"
	protoutil "github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBootstrapAndOpen verifies after bootstrapping with a root snapshot
// we should be able to open it and got the same state.
func TestBootstrapAndOpen(t *testing.T) {

	// create a state root and bootstrap the protocol state with it
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants, func(block *flow.Block) {
		block.ParentID = unittest.IdentifierFixture()
	})

	protoutil.RunWithBootstrapState(t, rootSnapshot, func(db storage.DB, _ *bprotocol.State) {
		lockManager := storage.NewTestingLockManager()
		// expect the final view metric to be set to current epoch's final view
		epoch, err := rootSnapshot.Epochs().Current()
		require.NoError(t, err)
		counter := epoch.Counter()
		phase, err := rootSnapshot.EpochPhase()
		require.NoError(t, err)

		complianceMetrics := new(mock.ComplianceMetrics)
		complianceMetrics.On("CurrentEpochCounter", counter).Once()
		complianceMetrics.On("CurrentEpochPhase", phase).Once()
		complianceMetrics.On("CurrentEpochFinalView", epoch.FinalView()).Once()
		complianceMetrics.On("BlockFinalized", testmock.Anything).Once()
		complianceMetrics.On("FinalizedHeight", testmock.Anything).Once()
		complianceMetrics.On("BlockSealed", testmock.Anything).Once()
		complianceMetrics.On("SealedHeight", testmock.Anything).Once()

		complianceMetrics.On("CurrentDKGPhaseViews",
			epoch.DKGPhase1FinalView(), epoch.DKGPhase2FinalView(), epoch.DKGPhase3FinalView()).Once()

		noopMetrics := new(metrics.NoopCollector)
		all := store.InitAll(noopMetrics, db)
		// protocol state has been bootstrapped, now open a protocol state with the database
		state, err := bprotocol.OpenState(
			complianceMetrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
		)
		require.NoError(t, err)
		complianceMetrics.AssertExpectations(t)

		finalSnap := state.Final()
		unittest.AssertSnapshotsEqual(t, rootSnapshot, finalSnap)

		vb, err := finalSnap.VersionBeacon()
		require.NoError(t, err)
		require.Nil(t, vb)
	})
}

// TestBootstrapAndOpen_EpochCommitted verifies after bootstrapping with a
// root snapshot from EpochCommitted phase  we should be able to open it and
// got the same state.
func TestBootstrapAndOpen_EpochCommitted(t *testing.T) {
	// create a state root and bootstrap the protocol state with it
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants, func(block *flow.Block) {
		block.ParentID = unittest.IdentifierFixture()
	})
	rootBlock, err := rootSnapshot.Head()
	require.NoError(t, err)

	// build an epoch on the root state and return a snapshot from the committed phase
	committedPhaseSnapshot := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
		unittest.NewEpochBuilder(t, mutableState, state).BuildEpoch().CompleteEpoch()

		// find the point where we transition to the epoch committed phase
		for height := rootBlock.Height + 1; ; height++ {
			phase, err := state.AtHeight(height).EpochPhase()
			require.NoError(t, err)
			if phase == flow.EpochPhaseCommitted {
				return state.AtHeight(height)
			}
		}
	})

	protoutil.RunWithBootstrapState(t, committedPhaseSnapshot, func(db storage.DB, _ *bprotocol.State) {
		lockManager := storage.NewTestingLockManager()

		complianceMetrics := new(mock.ComplianceMetrics)

		currentEpoch, err := committedPhaseSnapshot.Epochs().Current()
		require.NoError(t, err)
		// expect counter to be set to current epochs counter
		counter := currentEpoch.Counter()
		complianceMetrics.On("CurrentEpochCounter", counter).Once()

		// expect epoch phase to be set to current phase
		phase, err := committedPhaseSnapshot.EpochPhase()
		require.NoError(t, err)
		complianceMetrics.On("CurrentEpochPhase", phase).Once()
		complianceMetrics.On("CurrentEpochFinalView", currentEpoch.FinalView()).Once()
		complianceMetrics.On("CurrentDKGPhaseViews", currentEpoch.DKGPhase1FinalView(), currentEpoch.DKGPhase2FinalView(), currentEpoch.DKGPhase3FinalView()).Once()

		// expect finalized and sealed to be set to the latest block
		complianceMetrics.On("FinalizedHeight", testmock.Anything).Once()
		complianceMetrics.On("BlockFinalized", testmock.Anything).Once()
		complianceMetrics.On("SealedHeight", testmock.Anything).Once()
		complianceMetrics.On("BlockSealed", testmock.Anything).Once()

		noopMetrics := new(metrics.NoopCollector)
		all := store.InitAll(noopMetrics, db)
		state, err := bprotocol.OpenState(
			complianceMetrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
		)
		require.NoError(t, err)

		// assert update final view was called
		complianceMetrics.AssertExpectations(t)

		unittest.AssertSnapshotsEqual(t, committedPhaseSnapshot, state.Final())
	})
}

// TestBootstrap_EpochHeightBoundaries tests that epoch height indexes are indexed
// when they are available in the input snapshot.
//
// DIAGRAM LEGEND:
//
//	< = low endpoint of a sealing segment
//	> = high endpoint of a sealing segment
//	x = root sealing segment
//	| = epoch boundary
func TestBootstrap_EpochHeightBoundaries(t *testing.T) {
	t.Parallel()
	// start with a regular post-spork root snapshot
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	epoch1FirstHeight := rootSnapshot.Encodable().Head().Height

	// For the spork root snapshot, only the first height of the root epoch should be indexed.
	// [x]
	t.Run("spork root snapshot", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *bprotocol.FollowerState) {
			currentEpoch, err := state.Final().Epochs().Current()
			require.NoError(t, err)
			// first height of started current epoch should be known
			firstHeight, err := currentEpoch.FirstHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch1FirstHeight, firstHeight)
			// final height of not completed current epoch should be unknown
			_, err = currentEpoch.FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
		})
	})

	// In this test we construct a snapshot where the sealing segment is entirely
	// within a particular epoch (does not cross any boundary). In this case,
	// no boundaries should be queriable in the API.
	// [---<--->--]
	t.Run("snapshot excludes start boundary", func(t *testing.T) {
		var epochHeights *unittest.EpochHeights
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			builder := unittest.NewEpochBuilder(t, mutableState, state)
			builder.BuildEpoch().
				AddBlocksWithSeals(flow.DefaultTransactionExpiry, 1). // ensure sealing segment excludes start boundary
				CompleteEpoch()                                       // building epoch 1 (prepare epoch 2)
			var ok bool
			epochHeights, ok = builder.EpochHeights(1)
			require.True(t, ok)
			// return a snapshot with reference block in the Committed phase of Epoch 1
			return state.AtHeight(epochHeights.CommittedFinal)
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			currentEpoch, err := finalSnap.Epochs().Current()
			require.NoError(t, err)
			nextEpoch, err := finalSnap.Epochs().NextCommitted()
			require.NoError(t, err)
			// first height of started current epoch should be unknown
			_, err = currentEpoch.FirstHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			// final height of not completed current epoch should be unknown
			_, err = currentEpoch.FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			// first and final height of not started next epoch should be unknown
			_, err = nextEpoch.FirstHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			_, err = nextEpoch.FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			// nonexistent previous epoch should be unknown
			_, err = finalSnap.Epochs().Previous()
			assert.ErrorIs(t, err, protocol.ErrNoPreviousEpoch)
		})
	})

	// In this test we construct a root snapshot such that the Previous epoch w.r.t
	// the snapshot reference block has only the end boundary included in the
	// sealing segment. Therefore, only FinalBlock should be queriable in the API.
	// [---<---|--->---]
	t.Run("root snapshot includes previous epoch end boundary only", func(t *testing.T) {
		var epoch2Heights *unittest.EpochHeights
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			builder := unittest.NewEpochBuilder(t, mutableState, state)
			builder.
				BuildEpoch().
				AddBlocksWithSeals(flow.DefaultTransactionExpiry, 1). // ensure sealing segment excludes start boundary
				CompleteEpoch().                                      // building epoch 1 (prepare epoch 2)
				BuildEpoch()                                          // building epoch 2 (prepare epoch 3)
			var ok bool
			epoch2Heights, ok = builder.EpochHeights(2)
			require.True(t, ok)

			// return snapshot from Committed phase of epoch 2
			return state.AtHeight(epoch2Heights.Committed)
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			currentEpoch, err := finalSnap.Epochs().Current()
			require.NoError(t, err)
			// first height of started current epoch should be known
			firstHeight, err := currentEpoch.FirstHeight()
			assert.Equal(t, epoch2Heights.FirstHeight(), firstHeight)
			require.NoError(t, err)
			// final height of not completed current epoch should be unknown
			_, err = currentEpoch.FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			previousEpoch, err := finalSnap.Epochs().Previous()
			require.NoError(t, err)
			// first height of previous epoch should be unknown
			_, err = previousEpoch.FirstHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			// final height of previous epoch should be known
			finalHeight, err := previousEpoch.FinalHeight()
			require.NoError(t, err)
			assert.Equal(t, finalHeight, epoch2Heights.FirstHeight()-1)
		})
	})

	// In this test we construct a root snapshot such that the Previous epoch w.r.t
	// the snapshot reference block has both start and end boundaries included in the
	// sealing segment. Therefore, both boundaries should be queryable in the API.
	// [---<---|---|--->---]
	t.Run("root snapshot includes previous epoch start and end boundary", func(t *testing.T) {
		var epoch3Heights *unittest.EpochHeights
		var epoch2Heights *unittest.EpochHeights
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			builder := unittest.NewEpochBuilder(t, mutableState, state)
			builder.
				BuildEpoch().CompleteEpoch(). // building epoch 1 (prepare epoch 2)
				BuildEpoch().CompleteEpoch(). // building epoch 2 (prepare epoch 3)
				BuildEpoch()                  // building epoch 3 (prepare epoch 4)
			var ok bool
			epoch3Heights, ok = builder.EpochHeights(3)
			require.True(t, ok)
			epoch2Heights, ok = builder.EpochHeights(2)
			require.True(t, ok)

			// return snapshot from Committed phase of epoch 3
			return state.AtHeight(epoch3Heights.Committed)
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			currentEpoch, err := finalSnap.Epochs().Current()
			require.NoError(t, err)
			// first height of started current epoch should be known
			firstHeight, err := currentEpoch.FirstHeight()
			assert.Equal(t, epoch3Heights.FirstHeight(), firstHeight)
			require.NoError(t, err)
			// final height of not completed current epoch should be unknown
			_, err = currentEpoch.FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrUnknownEpochBoundary)
			previousEpoch, err := finalSnap.Epochs().Previous()
			require.NoError(t, err)
			// first height of previous epoch should be known
			firstHeight, err = previousEpoch.FirstHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch2Heights.FirstHeight(), firstHeight)
			// final height of completed previous epoch should be known
			finalHeight, err := previousEpoch.FinalHeight()
			require.NoError(t, err)
			assert.Equal(t, finalHeight, epoch2Heights.FinalHeight())
		})
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
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	rootBlock, err := rootSnapshot.Head()
	require.NoError(t, err)

	// should be able to bootstrap from snapshot after sealing a non-root block
	// ROOT <- B1 <- B2(R1) <- B3(S1) <- CHILD
	t.Run("with sealed block", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			block1 := unittest.BlockWithParentAndPayload(
				rootBlock,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			buildFinalizedBlock(t, state, block1)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2 := unittest.BlockWithParentAndPayload(
				block1.ToHeader(),
				unittest.PayloadFixture(
					unittest.WithReceipts(receipt1),
					unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block2)

			seals := []*flow.Seal{seal1}
			block3View := block2.View + 1
			block3 := unittest.BlockFixture(
				unittest.Block.WithParent(block2.ID(), block2.View, block2.Height),
				unittest.Block.WithView(block3View),
				unittest.Block.WithPayload(
					flow.Payload{
						Seals:           seals,
						ProtocolStateID: calculateExpectedStateId(t, mutableState)(block2.ID(), block3View, seals),
					}),
			)
			buildFinalizedBlock(t, state, block3)

			child := unittest.BlockWithParentProtocolState(block3)
			buildBlock(t, state, child)

			return state.AtBlockID(block3.ID())
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			unittest.AssertSnapshotsEqual(t, after, finalSnap)
			segment, err := finalSnap.SealingSegment()
			require.NoError(t, err)
			for _, proposal := range segment.Blocks {
				snapshot := state.AtBlockID(proposal.Block.ID())
				// should be able to read all QCs
				_, err := snapshot.QuorumCertificate()
				require.NoError(t, err)
				_, err = snapshot.RandomSource()
				require.NoError(t, err)
			}
		})
	})

	// should be able to bootstrap from snapshot when the sealing segment contains
	// a block which references a result included outside the sealing segment.
	// In this case, B2 contains the result for B1, but is omitted from the segment.
	// B3 contains only the receipt for B1 and is included in the segment.
	//
	//                                      Extra Blocks             Sealing Segment
	//                                     [-----------------------][--------------------------------------]
	// ROOT <- B1 <- B2(Receipt1a,Result1) <- B3(Receipt1b) <- ... <- G1 <- G2(R[G1]) <- G3(Seal[G1])
	t.Run("with detached execution result reference in sealing segment", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			block1 := unittest.BlockWithParentAndPayload(rootBlock,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)

			receipt1a, seal1 := unittest.ReceiptAndSealForBlock(block1)
			receipt1b := unittest.ExecutionReceiptFixture(unittest.WithResult(&receipt1a.ExecutionResult))

			block2 := unittest.BlockWithParentAndPayload(block1.ToHeader(), unittest.PayloadFixture(
				unittest.WithReceipts(receipt1a),
				unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block2)

			block3 := unittest.BlockWithParentAndPayload(block2.ToHeader(), unittest.PayloadFixture(
				unittest.WithReceiptsAndNoResults(receipt1b),
				unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block3)

			receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
			receipt3, seal3 := unittest.ReceiptAndSealForBlock(block3)

			receipts := []*flow.ExecutionReceipt{receipt2, receipt3}
			seals := []*flow.Seal{seal1, seal2, seal3}

			parent := block3
			for i := 0; i < flow.DefaultTransactionExpiry-1; i++ {
				next := unittest.BlockWithParentAndPayload(parent.ToHeader(), unittest.PayloadFixture(
					unittest.WithReceipts(receipts[0]),
					unittest.WithProtocolStateID(calculateExpectedStateId(t, mutableState)(parent.ID(), parent.View+1, []*flow.Seal{seals[0]})),
					unittest.WithSeals(seals[0])))
				seals, receipts = seals[1:], receipts[1:]

				nextReceipt, nextSeal := unittest.ReceiptAndSealForBlock(next)
				receipts = append(receipts, nextReceipt)
				seals = append(seals, nextSeal)
				buildFinalizedBlock(t, state, next)
				parent = next
			}

			// G1 adds all receipts from all blocks before G1
			blockG1 := unittest.BlockWithParentAndPayload(parent.ToHeader(), unittest.PayloadFixture(
				unittest.WithReceipts(receipts...),
				unittest.WithProtocolStateID(parent.Payload.ProtocolStateID)))
			buildFinalizedBlock(t, state, blockG1)

			receiptS1, sealS1 := unittest.ReceiptAndSealForBlock(blockG1)

			// G2 adds all seals from all blocks before G1
			blockG2 := unittest.BlockWithParentAndPayload(blockG1.ToHeader(), unittest.PayloadFixture(
				unittest.WithSeals(seals...),
				unittest.WithProtocolStateID(calculateExpectedStateId(t, mutableState)(blockG1.ID(), blockG1.View+1, seals)),
				unittest.WithReceipts(receiptS1)))
			buildFinalizedBlock(t, state, blockG2)

			// G3 seals G1, creating a sealing segment
			blockG3 := unittest.BlockWithParentAndPayload(blockG2.ToHeader(), unittest.PayloadFixture(
				unittest.WithSeals(sealS1),
				unittest.WithProtocolStateID(calculateExpectedStateId(t, mutableState)(blockG2.ID(), blockG2.View+1, []*flow.Seal{sealS1}))))
			buildFinalizedBlock(t, state, blockG3)

			child := unittest.BlockWithParentAndPayload(blockG3.ToHeader(), unittest.PayloadFixture(unittest.WithProtocolStateID(blockG3.Payload.ProtocolStateID)))
			buildFinalizedBlock(t, state, child)

			return state.AtBlockID(blockG3.ID())
		})

		segment, err := after.SealingSegment()
		require.NoError(t, err)
		// To accurately test the desired edge case we require that the lowest block in ExtraBlocks is B3
		assert.Equal(t, uint64(3), segment.ExtraBlocks[0].Block.Height)

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
		})
	})

	// should be able to bootstrap from snapshot after entering EFM because of sealing invalid service event
	// ROOT <- B1 <- B2(R1) <- B3(S1) <- CHILD
	t.Run("in EFM", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			block1 := unittest.BlockWithParentAndPayload(
				rootBlock,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			buildFinalizedBlock(t, state, block1)

			invalidEpochSetup := unittest.EpochSetupFixture()
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1, invalidEpochSetup.ServiceEvent())
			block2 := unittest.BlockWithParentAndPayload(
				block1.ToHeader(),
				unittest.PayloadFixture(
					unittest.WithReceipts(receipt1),
					unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			buildFinalizedBlock(t, state, block2)

			seals := []*flow.Seal{seal1}
			block3View := block2.View + 1
			block3 := unittest.BlockFixture(
				unittest.Block.WithParent(block2.ID(), block2.View, block2.Height),
				unittest.Block.WithView(block3View),
				unittest.Block.WithPayload(
					flow.Payload{
						Seals:           seals,
						ProtocolStateID: calculateExpectedStateId(t, mutableState)(block2.ID(), block3View, seals),
					}),
			)
			buildFinalizedBlock(t, state, block3)

			child := unittest.BlockWithParentProtocolState(block3)
			buildBlock(t, state, child)

			// ensure we have entered EFM
			snapshot := state.AtBlockID(block3.ID())
			epochState, err := snapshot.EpochProtocolState()
			require.NoError(t, err)
			require.Equal(t, flow.EpochPhaseFallback, epochState.EpochPhase())

			return snapshot
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			unittest.AssertSnapshotsEqual(t, after, finalSnap)
			segment, err := finalSnap.SealingSegment()
			require.NoError(t, err)
			for _, proposal := range segment.Blocks {
				snapshot := state.AtBlockID(proposal.Block.ID())
				// should be able to read all QCs
				_, err := snapshot.QuorumCertificate()
				require.NoError(t, err)
				_, err = snapshot.RandomSource()
				require.NoError(t, err)
			}

			epochState, err := finalSnap.EpochProtocolState()
			require.NoError(t, err)
			require.True(t, epochState.EpochFallbackTriggered())
			require.Equal(t, flow.EpochPhaseFallback, epochState.EpochPhase())
		})
	})

	t.Run("with setup next epoch", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			unittest.NewEpochBuilder(t, mutableState, state).BuildEpoch()

			// find the point where we transition to the epoch setup phase
			for height := rootBlock.Height + 1; ; height++ {
				phase, err := state.AtHeight(height).EpochPhase()
				require.NoError(t, err)
				if phase == flow.EpochPhaseSetup {
					return state.AtHeight(height)
				}
			}
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			unittest.AssertSnapshotsEqual(t, after, finalSnap)

			segment, err := finalSnap.SealingSegment()
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(segment.ProtocolStateEntries), 2, "should have >2 distinct protocol state entries")
			for _, proposal := range segment.Blocks {
				snapshot := state.AtBlockID(proposal.Block.ID())
				// should be able to read all protocol state entries
				protocolStateEntry, err := snapshot.ProtocolState()
				require.NoError(t, err)
				assert.Equal(t, proposal.Block.Payload.ProtocolStateID, protocolStateEntry.ID())
			}
		})
	})

	t.Run("with committed next epoch", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			unittest.NewEpochBuilder(t, mutableState, state).BuildEpoch().CompleteEpoch()

			// find the point where we transition to the epoch committed phase
			for height := rootBlock.Height + 1; ; height++ {
				phase, err := state.AtHeight(height).EpochPhase()
				require.NoError(t, err)
				if phase == flow.EpochPhaseCommitted {
					return state.AtHeight(height)
				}
			}
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			unittest.AssertSnapshotsEqual(t, after, finalSnap)

			segment, err := finalSnap.SealingSegment()
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(segment.ProtocolStateEntries), 2, "should have >2 distinct protocol state entries")
			for _, proposal := range segment.Blocks {
				snapshot := state.AtBlockID(proposal.Block.ID())
				// should be able to read all protocol state entries
				protocolStateEntry, err := snapshot.ProtocolState()
				require.NoError(t, err)
				assert.Equal(t, proposal.Block.Payload.ProtocolStateID, protocolStateEntry.ID())
			}
		})
	})

	t.Run("with previous and next epoch", func(t *testing.T) {
		after := snapshotAfter(t, rootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			unittest.NewEpochBuilder(t, mutableState, state).
				BuildEpoch().CompleteEpoch(). // build epoch 2
				BuildEpoch()                  // build epoch 3

			// find a snapshot from epoch setup phase in epoch 2
			epoch1, err := rootSnapshot.Epochs().Current()
			require.NoError(t, err)
			epoch1Counter := epoch1.Counter()
			for height := rootBlock.Height + 1; ; height++ {
				snap := state.AtHeight(height)
				epoch, err := snap.Epochs().Current()
				require.NoError(t, err)
				phase, err := snap.EpochPhase()
				require.NoError(t, err)
				if phase == flow.EpochPhaseSetup && epoch.Counter() == epoch1Counter+1 {
					return snap
				}
			}
		})

		bootstrap(t, after, func(state *bprotocol.State, err error) {
			require.NoError(t, err)
			finalSnap := state.Final()
			unittest.AssertSnapshotsEqual(t, after, finalSnap)

			segment, err := finalSnap.SealingSegment()
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(segment.ProtocolStateEntries), 2, "should have >2 distinct protocol state entries")
			for _, proposal := range segment.Blocks {
				snapshot := state.AtBlockID(proposal.Block.ID())
				// should be able to read all protocol state entries
				protocolStateEntry, err := snapshot.ProtocolState()
				require.NoError(t, err)
				assert.Equal(t, proposal.Block.Payload.ProtocolStateID, protocolStateEntry.ID())
			}
		})
	})
}

func TestBootstrap_InvalidIdentities(t *testing.T) {
	t.Run("duplicate node ID", func(t *testing.T) {
		participants := unittest.CompleteIdentitySet()
		// Make sure the duplicate node ID is not a consensus node, otherwise this will form an invalid DKGIDMapping
		// See [flow.EpochCommit] for details.
		dupeIDIdentity := unittest.IdentityFixture(unittest.WithNodeID(participants[0].NodeID), unittest.WithRole(flow.RoleVerification))
		participants = append(participants, dupeIDIdentity)

		root := unittest.RootSnapshotFixture(participants)
		bootstrap(t, root, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("zero weight", func(t *testing.T) {
		zeroWeightIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification), unittest.WithInitialWeight(0))
		participants := unittest.CompleteIdentitySet(zeroWeightIdentity)
		root := unittest.RootSnapshotFixture(participants)
		bootstrap(t, root, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("missing role", func(t *testing.T) {
		requiredRoles := []flow.Role{
			flow.RoleConsensus,
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
		// randomly shuffle the identities so they are not canonically ordered
		unorderedParticipants, err := participants.ToSkeleton().Shuffle()
		require.NoError(t, err)

		root := unittest.RootSnapshotFixture(participants)
		encodable := root.Encodable()

		// modify EpochSetup participants, making them unordered
		latestProtocolStateEntry := encodable.SealingSegment.LatestProtocolStateEntry()
		currentEpochSetup := latestProtocolStateEntry.EpochEntry.CurrentEpochSetup
		currentEpochSetup.Participants = unorderedParticipants
		currentEpochSetup.Participants = unorderedParticipants
		latestProtocolStateEntry.EpochEntry.CurrentEpoch.SetupID = currentEpochSetup.ID()

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
	tail := unittest.ProposalFixture()
	encodable.SealingSegment.Blocks = append([]*flow.Proposal{tail}, encodable.SealingSegment.Blocks...)
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
		latestSeal, err := encodable.LatestSeal()
		require.NoError(t, err)
		latestSeal.BlockID = unittest.IdentifierFixture()

		bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("result doesn't match tail block", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
		// convert to encodable to easily modify snapshot
		encodable := rootSnapshot.Encodable()
		latestSealedResult, err := encodable.LatestSealedResult()
		require.NoError(t, err)
		latestSealedResult.BlockID = unittest.IdentifierFixture()

		bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})

	t.Run("seal doesn't match result", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
		// convert to encodable to easily modify snapshot
		encodable := rootSnapshot.Encodable()
		latestSeal, err := encodable.LatestSeal()
		require.NoError(t, err)
		latestSeal.ResultID = unittest.IdentifierFixture()

		bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
			assert.Error(t, err)
		})
	})
}

// TestBootstrap_InvalidSporkBlockView verifies that bootstrapStatePointers
// returns an error when the SporkRootBlockView is set to a value higher or equal
// than the LivenessData.CurrentView.
func TestBootstrap_InvalidSporkBlockView(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	// convert to encodable to easily modify snapshot
	encodable := rootSnapshot.Encodable()

	segment, err := rootSnapshot.SealingSegment()
	require.NoError(t, err)

	// invalid configuration, where the latest finalized block's view is lower than the spork root block's view:
	encodable.Params.SporkRootBlockView = segment.Finalized().View + 1

	rootSnapshot = inmem.SnapshotFromEncodable(encodable)

	bootstrap(t, rootSnapshot, func(state *bprotocol.State, err error) {
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("sealing segment is invalid, because the latest finalized block's view %d is lower than the spork root block's view %d", segment.Finalized().View, rootSnapshot.Params().SporkRootBlockView()))
	})
}

// bootstraps protocol state with the given snapshot and invokes the callback
// with the result of the constructor
func bootstrap(t *testing.T, rootSnapshot protocol.Snapshot, f func(*bprotocol.State, error)) {
	metrics := metrics.NewNoopCollector()
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)
	pdb := unittest.PebbleDB(t, dir)
	db := pebbleimpl.ToDB(pdb)
	lockManager := storage.NewTestingLockManager()
	defer db.Close()
	all := store.InitAll(metrics, db)
	state, err := bprotocol.Bootstrap(
		metrics,
		db,
		lockManager,
		all.Headers,
		all.Seals,
		all.Results,
		all.Blocks,
		all.QuorumCertificates,
		all.EpochSetups,
		all.EpochCommits,
		all.EpochProtocolStateEntries,
		all.ProtocolKVStore,
		all.VersionBeacons,
		rootSnapshot,
	)
	f(state, err)
}

// snapshotAfter bootstraps the protocol state from the root snapshot, applies
// the state-changing function f, clears the on-disk state, and returns a
// memory-backed snapshot corresponding to that returned by f.
//
// This is used for generating valid snapshots to use when testing bootstrapping
// from non-root states.
func snapshotAfter(t *testing.T, rootSnapshot protocol.Snapshot, f func(*bprotocol.FollowerState, protocol.MutableProtocolState) protocol.Snapshot) protocol.Snapshot {
	var after protocol.Snapshot
	protoutil.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(_ storage.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
		snap := f(state.FollowerState, mutableState)
		var err error
		after, err = inmem.FromSnapshot(snap)
		require.NoError(t, err)
	})
	return after
}

// buildBlock extends the protocol state by the given block
func buildBlock(t *testing.T, state protocol.FollowerState, block *flow.Block) {
	require.NoError(t, state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block)))
}

// buildFinalizedBlock extends the protocol state by the given block and marks the block as finalized
func buildFinalizedBlock(t *testing.T, state protocol.FollowerState, block *flow.Block) {
	require.NoError(t, state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block)))
	require.NoError(t, state.Finalize(context.Background(), block.ID()))
}

// assertSealingSegmentBlocksQueryable bootstraps the state with the given
// snapshot, then verifies that all sealing segment blocks are queryable.
func assertSealingSegmentBlocksQueryableAfterBootstrap(t *testing.T, snapshot protocol.Snapshot) {
	bootstrap(t, snapshot, func(state *bprotocol.State, err error) {
		require.NoError(t, err)

		segment, err := state.Final().SealingSegment()
		require.NoError(t, err)

		rootBlock := state.Params().FinalizedRoot()

		// root block should be the highest block from the sealing segment
		assert.Equal(t, segment.Highest().ToHeader(), rootBlock)

		// for each block in the sealing segment we should be able to query:
		// * Head
		// * SealedResult
		// * Commit
		for _, proposal := range segment.Blocks {
			blockID := proposal.Block.ID()
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
		for _, proposal := range segment.Blocks[:len(segment.Blocks)-1] {
			snap := state.AtBlockID(proposal.Block.ID())
			_, err := snap.SealingSegment()
			assert.ErrorIs(t, err, protocol.ErrSealingSegmentBelowRootBlock)
		}
	})
}

// BenchmarkFinal benchmarks retrieving the latest finalized block from storage.
func BenchmarkFinal(b *testing.B) {
	util.RunWithBootstrapState(b, unittest.RootSnapshotFixture(unittest.CompleteIdentitySet()), func(db storage.DB, state *bprotocol.State) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			header, err := state.Final().Head()
			assert.NoError(b, err)
			assert.NotNil(b, header)
		}
	})
}

// BenchmarkFinal benchmarks retrieving the block by height from storage.
func BenchmarkByHeight(b *testing.B) {
	util.RunWithBootstrapState(b, unittest.RootSnapshotFixture(unittest.CompleteIdentitySet()), func(db storage.DB, state *bprotocol.State) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			header, err := state.AtHeight(0).Head()
			assert.NoError(b, err)
			assert.NotNil(b, header)
		}
	})
}
