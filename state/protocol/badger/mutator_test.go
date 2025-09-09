package badger_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	mmetrics "github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/trace"
	st "github.com/onflow/flow-go/state"
	realprotocol "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/deferred"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(5, unittest.WithAllRoles())

func TestBootstrapValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithBootstrapState(t, rootSnapshot, func(db storage.DB, state *protocol.State) {
		var finalized uint64
		err := operation.RetrieveFinalizedHeight(db.Reader(), &finalized)
		require.NoError(t, err)

		var sealed uint64
		err = operation.RetrieveSealedHeight(db.Reader(), &sealed)
		require.NoError(t, err)

		var genesisID flow.Identifier
		err = operation.LookupBlockHeight(db.Reader(), 0, &genesisID)
		require.NoError(t, err)

		var header flow.Header
		err = operation.RetrieveHeader(db.Reader(), genesisID, &header)
		require.NoError(t, err)

		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), genesisID, &sealID)
		require.NoError(t, err)

		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)
		err = operation.RetrieveSeal(db.Reader(), sealID, seal)
		require.NoError(t, err)

		block, err := rootSnapshot.Head()
		require.NoError(t, err)
		require.Equal(t, block.Height, finalized)
		require.Equal(t, block.Height, sealed)
		require.Equal(t, block.ID(), genesisID)
		require.Equal(t, block.ID(), seal.BlockID)
		require.Equal(t, block, &header)
	})
}

// TestExtendValid tests the happy path of extending the state with a single block.
// * BlockFinalized is emitted when the block is finalized
// * BlockProcessable is emitted when a block's child is inserted
func TestExtendValid(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		db := pebbleimpl.ToDB(pdb)
		log := zerolog.Nop()
		all := store.InitAll(metrics, db)

		distributor := events.NewDistributor()
		consumer := mockprotocol.NewConsumer(t)
		distributor.AddConsumer(consumer)

		block, result, seal := unittest.BootstrapFixture(participants)
		qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(block.ID()))
		rootSnapshot, err := unittest.SnapshotFromBootstrapState(block, result, seal, qc)
		require.NoError(t, err)

		state, err := protocol.Bootstrap(
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
		require.NoError(t, err)

		fullState, err := protocol.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			util.MockBlockTimer(),
			util.MockReceiptValidator(),
			util.MockSealValidator(all.Seals),
		)
		require.NoError(t, err)

		// insert block1 on top of the root block
		block1 := unittest.BlockWithParentProtocolState(block)
		err = fullState.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)

		// we should not emit BlockProcessable for the root block
		consumer.AssertNotCalled(t, "BlockProcessable", block.ToHeader(), mock.Anything)

		t.Run("BlockFinalized event should be emitted when block1 is finalized", func(t *testing.T) {
			consumer.On("BlockFinalized", block1.ToHeader()).Once()
			err := fullState.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)
		})

		t.Run("BlockProcessable event should be emitted when any child of block1 is inserted", func(t *testing.T) {
			block2 := unittest.BlockWithParentProtocolState(block1)
			consumer.On("BlockProcessable", block1.ToHeader(), mock.Anything).Once()
			err := fullState.Extend(context.Background(), unittest.ProposalFromBlock(block2))
			require.NoError(t, err)

			// verify that block1's view is indexed as certified, because it has a child (block2)
			var indexedID flow.Identifier
			require.NoError(t, operation.LookupCertifiedBlockByView(db.Reader(), block1.View, &indexedID))
			require.Equal(t, block1.ID(), indexedID)

			// verify that block2's view is not indexed as certified, because it has no children
			err = operation.LookupCertifiedBlockByView(db.Reader(), block2.View, &indexedID)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

func TestSealedIndex(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		rootHeader, err := rootSnapshot.Head()
		require.NoError(t, err)

		// build a chain:
		// G <- B1 <- B2 (resultB1) <- B3 <- B4 (resultB2, resultB3) <- B5 (sealB1) <- B6 (sealB2, sealB3) <- B7
		// test that when B4 is finalized, can only find seal for G
		// 					 when B5 is finalized, can find seal for B1
		//					 when B7 is finalized, can find seals for B2, B3

		// block 1
		b1 := unittest.BlockWithParentAndPayload(
			rootHeader,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b1))
		require.NoError(t, err)

		// block 2(result B1)
		b1Receipt := unittest.ReceiptForBlockFixture(b1)
		b2 := unittest.BlockWithParentAndPayload(
			b1.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(b1Receipt),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b2))
		require.NoError(t, err)

		// block 3
		b3 := unittest.BlockWithParentProtocolState(b2)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b3))
		require.NoError(t, err)

		// block 4 (resultB2, resultB3)
		b2Receipt := unittest.ReceiptForBlockFixture(b2)
		b3Receipt := unittest.ReceiptForBlockFixture(b3)
		b4 := unittest.BlockWithParentAndPayload(
			b3.ToHeader(),
			flow.Payload{
				Receipts:        []*flow.ExecutionReceiptStub{b2Receipt.Stub(), b3Receipt.Stub()},
				Results:         []*flow.ExecutionResult{&b2Receipt.ExecutionResult, &b3Receipt.ExecutionResult},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b4))
		require.NoError(t, err)

		// block 5 (sealB1)
		b1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b1Receipt.ExecutionResult))
		b5 := unittest.BlockWithParentAndPayload(
			b4.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{b1Seal},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b5))
		require.NoError(t, err)

		// block 6 (sealB2, sealB3)
		b2Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b2Receipt.ExecutionResult))
		b3Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b3Receipt.ExecutionResult))
		b6 := unittest.BlockWithParentAndPayload(
			b5.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{b2Seal, b3Seal},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b6))
		require.NoError(t, err)

		// block 7
		b7 := unittest.BlockWithParentProtocolState(b6)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b7))
		require.NoError(t, err)

		// finalizing b1 - b4
		// when B4 is finalized, can only find seal for G
		err = state.Finalize(context.Background(), b1.ID())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), b2.ID())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), b3.ID())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), b4.ID())
		require.NoError(t, err)

		metrics := metrics.NewNoopCollector()
		seals := store.NewSeals(metrics, db)

		// can only find seal for G
		_, err = seals.FinalizedSealForBlock(rootHeader.ID())
		require.NoError(t, err)

		_, err = seals.FinalizedSealForBlock(b1.ID())
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// when B5 is finalized, can find seal for B1
		err = state.Finalize(context.Background(), b5.ID())
		require.NoError(t, err)

		s1, err := seals.FinalizedSealForBlock(b1.ID())
		require.NoError(t, err)
		require.Equal(t, b1Seal, s1)

		_, err = seals.FinalizedSealForBlock(b2.ID())
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// when B7 is finalized, can find seals for B2, B3
		err = state.Finalize(context.Background(), b6.ID())
		require.NoError(t, err)

		err = state.Finalize(context.Background(), b7.ID())
		require.NoError(t, err)

		s2, err := seals.FinalizedSealForBlock(b2.ID())
		require.NoError(t, err)
		require.Equal(t, b2Seal, s2)

		s3, err := seals.FinalizedSealForBlock(b3.ID())
		require.NoError(t, err)
		require.Equal(t, b3Seal, s3)
	})

}

func TestVersionBeaconIndex(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		rootHeader, err := rootSnapshot.Head()
		require.NoError(t, err)

		// build a chain:
		// G <- B1 <- B2 (resultB1(vb1)) <- B3 <- B4 (resultB2(vb2), resultB3(vb3)) <- B5 (sealB1) <- B6 (sealB2, sealB3)
		// up until and including finalization of B5 there should be no VBs indexed
		//    when B5 is finalized, index VB1
		//    when B6 is finalized, we can index VB2 and VB3, but (only) the last one should be indexed by seal height

		// block 1
		b1 := unittest.BlockWithParentAndPayload(
			rootHeader,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b1))
		require.NoError(t, err)

		vb1 := unittest.VersionBeaconFixture(
			unittest.WithBoundaries(
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height,
					Version:     "0.21.37",
				},
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height + 100,
					Version:     "0.21.38",
				},
			),
		)
		vb2 := unittest.VersionBeaconFixture(
			unittest.WithBoundaries(
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height,
					Version:     "0.21.37",
				},
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height + 101,
					Version:     "0.21.38",
				},
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height + 201,
					Version:     "0.21.39",
				},
			),
		)
		vb3 := unittest.VersionBeaconFixture(
			unittest.WithBoundaries(
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height,
					Version:     "0.21.37",
				},
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height + 99,
					Version:     "0.21.38",
				},
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height + 199,
					Version:     "0.21.39",
				},
				flow.VersionBoundary{
					BlockHeight: rootHeader.Height + 299,
					Version:     "0.21.40",
				},
			),
		)

		b1Receipt := unittest.ReceiptForBlockFixture(b1)
		b1Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{vb1.ServiceEvent()}
		b2 := unittest.BlockWithParentAndPayload(
			b1.ToHeader(),
			unittest.PayloadFixture(unittest.WithReceipts(b1Receipt), unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b2))
		require.NoError(t, err)

		// block 3
		b3 := unittest.BlockWithParentProtocolState(b2)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b3))
		require.NoError(t, err)

		// block 4 (resultB2, resultB3)
		b2Receipt := unittest.ReceiptForBlockFixture(b2)
		b2Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{vb2.ServiceEvent()}

		b3Receipt := unittest.ReceiptForBlockFixture(b3)
		b3Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{vb3.ServiceEvent()}

		b4 := unittest.BlockWithParentAndPayload(
			b3.ToHeader(),
			flow.Payload{
				Receipts:        []*flow.ExecutionReceiptStub{b2Receipt.Stub(), b3Receipt.Stub()},
				Results:         []*flow.ExecutionResult{&b2Receipt.ExecutionResult, &b3Receipt.ExecutionResult},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b4))
		require.NoError(t, err)

		// block 5 (sealB1)
		b1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b1Receipt.ExecutionResult))
		b5 := unittest.BlockWithParentAndPayload(
			b4.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{b1Seal},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b5))
		require.NoError(t, err)

		// block 6 (sealB2, sealB3)
		b2Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b2Receipt.ExecutionResult))
		b3Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b3Receipt.ExecutionResult))
		b6 := unittest.BlockWithParentAndPayload(
			b5.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{b2Seal, b3Seal},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(b6))
		require.NoError(t, err)

		versionBeacons := store.NewVersionBeacons(db)

		// No VB can be found before finalizing anything
		vb, err := versionBeacons.Highest(b6.Height)
		require.NoError(t, err)
		require.Nil(t, vb)

		// finalizing b1 - b5
		err = state.Finalize(context.Background(), b1.ID())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), b2.ID())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), b3.ID())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), b4.ID())
		require.NoError(t, err)

		// No VB can be found after finalizing B4
		vb, err = versionBeacons.Highest(b6.Height)
		require.NoError(t, err)
		require.Nil(t, vb)

		// once B5 is finalized, B1 and VB1 are sealed, hence index should now find it
		err = state.Finalize(context.Background(), b5.ID())
		require.NoError(t, err)

		versionBeacon, err := versionBeacons.Highest(b6.Height)
		require.NoError(t, err)
		require.Equal(t,
			&flow.SealedVersionBeacon{
				VersionBeacon: vb1,
				SealHeight:    b5.Height,
			},
			versionBeacon,
		)

		// finalizing B6 should index events sealed by B6, so VB2 and VB3
		// while we don't expect multiple VBs in one block, we index newest, so last one emitted - VB3
		err = state.Finalize(context.Background(), b6.ID())
		require.NoError(t, err)

		versionBeacon, err = versionBeacons.Highest(b6.Height)
		require.NoError(t, err)
		require.Equal(t,
			&flow.SealedVersionBeacon{
				VersionBeacon: vb3,
				SealHeight:    b6.Height,
			},
			versionBeacon,
		)
	})
}

func TestExtendSealedBoundary(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)
		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "original commit should be root commit")

		// Create a first block on top of the snapshot
		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)

		// Add a second block containing a receipt committing to the first block
		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block2 := unittest.BlockWithParentAndPayload(
			block1.ToHeader(),
			flow.Payload{
				Receipts:        []*flow.ExecutionReceiptStub{block1Receipt.Stub()},
				Results:         []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)

		// Add a third block containing a seal for the first block
		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block3 := unittest.BlockWithParentAndPayload(
			block2.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{block1Seal},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "commit should not change before finalizing")

		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "commit should not change after finalizing non-sealing block")

		err = state.Finalize(context.Background(), block2.ID())
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "commit should not change after finalizing non-sealing block")

		err = state.Finalize(context.Background(), block3.ID())
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, block1Seal.FinalState, finalCommit, "commit should change after finalizing sealing block")
	})
}

// TestExtendMissingParent tests the behaviour when attempting to extend the protocol state by a block
// whose parent is unknown. Per convention, the protocol state requires that the candidate's
// parent has already been ingested. Otherwise, an exception is returned.
func TestExtendMissingParent(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		extend := unittest.BlockFixture(
			unittest.Block.WithHeight(2),
			unittest.Block.WithView(2),
			unittest.Block.WithParentView(1),
		)

		err := state.Extend(context.Background(), unittest.ProposalFromBlock(extend))
		require.Error(t, err)
		require.False(t, st.IsInvalidExtensionError(err), err)
		require.False(t, st.IsOutdatedExtensionError(err), err)

		// verify seal that was contained in candidate block is not indexed
		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), extend.ID(), &sealID)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestExtendHeightTooSmall tests the behaviour when attempting to extend the protocol state by a block
// whose height is not larger than its parent's height. The protocol mandates that the candidate's
// height is exactly one larger than its parent's height. Otherwise, an exception should be returned.
func TestExtendHeightTooSmall(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)

	// we create the following to descendants of head:
	//   head <- blockB <- blockC
	// where blockB and blockC have exactly the same height
	emptyPayload := unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID))
	blockB := unittest.BlockFixture( // creates child with increased height and view (protocol compliant)
		unittest.Block.WithParent(head.ID(), head.View, head.Height),
		unittest.Block.WithHeight(head.Height+1),
		unittest.Block.WithView(head.View+1),
		unittest.Block.WithPayload(emptyPayload))

	blockC := unittest.BlockFixture( // creates child with height identical to parent (protocol violation) but increased view (protocol compliant)
		unittest.Block.WithParent(blockB.ID(), blockB.View, blockB.Height),
		unittest.Block.WithHeight(blockB.Height),
		unittest.Block.WithView(blockB.View+1),
		unittest.Block.WithPayload(emptyPayload))

	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, chainState *protocol.ParticipantState) {
		require.NoError(t, chainState.Extend(context.Background(), unittest.ProposalFromBlock(blockB)))

		err = chainState.Extend(context.Background(), unittest.ProposalFromBlock(blockC))
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err))

		// Whenever the state ingests a block, it indexes the latest seal as of this block.
		// Therefore, we can use this as a check to confirm that blockB was successfully ingested,
		// but the information from blockC was not.
		var sealID flow.Identifier
		// latest seal for blockB should be found, as blockB was successfully ingested:
		require.NoError(t, operation.LookupLatestSealAtBlock(db.Reader(), blockB.ID(), &sealID))
		// latest seal for blockC should NOT be found, because extending the state with blockC errored:
		require.ErrorIs(t,
			operation.LookupLatestSealAtBlock(db.Reader(), blockC.ID(), &sealID),
			storage.ErrNotFound)
	})
}

func TestExtendHeightTooLarge(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentAndPayload(
			head,
			*flow.NewEmptyPayload(),
		)
		// set an invalid height
		block.Height = head.Height + 2

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
	})
}

// TestExtendInconsistentParentView tests if mutableState rejects block with invalid ParentView. ParentView must be consistent
// with view of block referred by ParentID.
func TestExtendInconsistentParentView(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentAndPayload(
			head,
			*flow.NewEmptyPayload(),
		)
		// set an invalid parent view
		block.ParentView++
		block.View++

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err))
	})
}

func TestExtendBlockNotConnected(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// add 2 blocks, the second finalizing/sealing the state of the first
		extend := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(extend))
		require.NoError(t, err)

		err = state.Finalize(context.Background(), extend.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		extend.Timestamp += 1000 // shift time stamp forward by 1 second = 1000ms
		extend.ParentID = head.ID()

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(extend))
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), extend.ID(), &sealID)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestExtendInvalidChainID(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentAndPayload(
			head,
			*flow.NewEmptyPayload(),
		)
		// use an invalid chain ID
		block.ChainID = head.ChainID + "-invalid"

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

// TestExtendReceiptsNotSorted tests the case where receipts are included in a block payload
// not sorted by height. Previously, this constraint was required (unordered receipts resulted
// in an error). Now, any ordering of receipts should be accepted by the EvolvingState.
func TestExtendReceiptsNotSorted(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		// create block2 and block3
		block2 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err := state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)

		block3 := unittest.BlockWithParentAndPayload(
			block2.ToHeader(),
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.NoError(t, err)

		receiptA := unittest.ReceiptForBlockFixture(block3)
		receiptB := unittest.ReceiptForBlockFixture(block2)

		// insert a block with payload receipts not sorted by block height.
		block4 := unittest.BlockWithParentAndPayload(
			block3.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithProtocolStateID(rootProtocolStateID),
				unittest.WithReceipts(receiptA, receiptB),
			),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
		require.NoError(t, err)
	})
}

func TestExtendReceiptsInvalid(t *testing.T) {
	validator := mockmodule.NewReceiptValidator(t)
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolStateAndValidator(t, rootSnapshot, validator, func(db storage.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// create block2 and block3 as descendants of head
		block2 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		receipt := unittest.ReceiptForBlockFixture(block2) // receipt for block 2
		block3 := unittest.BlockWithParentAndPayload(
			block2.ToHeader(),
			flow.Payload{
				Receipts:        []*flow.ExecutionReceiptStub{receipt.Stub()},
				Results:         []*flow.ExecutionResult{&receipt.ExecutionResult},
				ProtocolStateID: rootProtocolStateID,
			},
		)

		// validator accepts block 2
		validator.On("ValidatePayload", block2).Return(nil).Once()
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)

		// but receipt for block 2 is invalid, which the ParticipantState should reject with an InvalidExtensionError
		validator.On("ValidatePayload", block3).Return(engine.NewInvalidInputErrorf("")).Once()
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

// TestOnReceiptValidatorExceptions tests that ParticipantState escalates unexpected errors and exceptions
// returned by the ReceiptValidator. We expect that such errors are *not* interpreted as the block being invalid.
func TestOnReceiptValidatorExceptions(t *testing.T) {
	validator := mockmodule.NewReceiptValidator(t)

	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolStateAndValidator(t, rootSnapshot, validator, func(db storage.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		block := unittest.BlockWithParentFixture(head)

		// Check that _unexpected_ failure causes the error to be escalated and is *not* interpreted as an invalid block.
		validator.On("ValidatePayload", block).Return(fmt.Errorf("")).Once()
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
		require.False(t, st.IsInvalidExtensionError(err), err)

		// Check that an `UnknownBlockError` causes the error to be escalated and is *not* interpreted as an invalid receipt.
		// Reasoning: per convention, the ParticipantState requires that the candidate's parent has already been ingested.
		// Otherwise, an exception is returned. The `ReceiptValidator.ValidatePayload(..)` returning an `UnknownBlockError`
		// indicates exactly this situation, where the parent block is unknown.
		validator.On("ValidatePayload", block).Return(module.NewUnknownBlockError("")).Once()
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
		require.False(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		block2 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)

		block3 := unittest.BlockWithParentProtocolState(block2)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.NoError(t, err)

		block4 := unittest.BlockWithParentProtocolState(block3)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
		require.NoError(t, err)

		receipt3a := unittest.ReceiptForBlockFixture(block3)
		receipt3b := unittest.ReceiptForBlockFixture(block3)
		receipt3c := unittest.ReceiptForBlockFixture(block4)

		block5 := unittest.BlockWithParentAndPayload(
			block4.ToHeader(),
			flow.Payload{
				Receipts: []*flow.ExecutionReceiptStub{
					receipt3a.Stub(),
					receipt3b.Stub(),
					receipt3c.Stub(),
				},
				Results: []*flow.ExecutionResult{
					&receipt3a.ExecutionResult,
					&receipt3b.ExecutionResult,
					&receipt3c.ExecutionResult,
				},
				ProtocolStateID: rootProtocolStateID,
			},
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block5))
		require.NoError(t, err)
	})
}

// Tests the full flow of transitioning between epochs by finalizing a setup
// event, then a commit event, then finalizing the first block of the next epoch.
// Also tests that appropriate epoch transition events are fired.
//
// Epoch information becomes available in the protocol state in the block containing the seal
// for the block whose execution emitted the service event.
//
// ROOT <- B1 <- B2(R1) <- B3(S1) <- B4 <- B5(R2) <- B6(S2) <- B7 <-|- B8
//
// B3 seals B1, in which EpochSetup is emitted.
//   - we can query the EpochSetup beginning with B3
//   - EpochSetupPhaseStarted triggered when B3 is finalized
//
// B6 seals B2, in which EpochCommitted is emitted.
//   - we can query the EpochCommit beginning with B6
//   - EpochCommittedPhaseStarted triggered when B6 is finalized
//
// B7 is the final block of the epoch.
// B8 is the first block of the NEXT epoch.
func TestExtendEpochTransitionValid(t *testing.T) {
	// create an event consumer to test epoch transition events
	consumer := mockprotocol.NewConsumer(t)
	consumer.On("BlockFinalized", mock.Anything)
	consumer.On("BlockProcessable", mock.Anything, mock.Anything)
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)

		// set up state and mock ComplianceMetrics object
		metrics := mockmodule.NewComplianceMetrics(t)
		metrics.On("BlockSealed", mock.Anything)
		metrics.On("SealedHeight", mock.Anything)
		metrics.On("FinalizedHeight", mock.Anything)
		metrics.On("BlockFinalized", mock.Anything)
		metrics.On("ProtocolStateVersion", mock.Anything)

		// expect epoch metric calls on bootstrap
		initialCurrentEpoch, err := rootSnapshot.Epochs().Current()
		require.NoError(t, err)
		counter := initialCurrentEpoch.Counter()
		finalView := initialCurrentEpoch.FinalView()
		initialPhase, err := rootSnapshot.EpochPhase()
		require.NoError(t, err)

		metrics.On("CurrentEpochCounter", counter).Once()
		metrics.On("CurrentEpochPhase", initialPhase).Once()
		metrics.On("CurrentEpochFinalView", finalView).Once()

		metrics.On("CurrentDKGPhaseViews",
			initialCurrentEpoch.DKGPhase1FinalView(),
			initialCurrentEpoch.DKGPhase2FinalView(),
			initialCurrentEpoch.DKGPhase3FinalView()).Once()

		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := store.InitAll(mmetrics.NewNoopCollector(), db)
		protoState, err := protocol.Bootstrap(
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
		require.NoError(t, err)
		receiptValidator := util.MockReceiptValidator()
		sealValidator := util.MockSealValidator(all.Seals)
		state, err := protocol.NewFullConsensusState(
			log,
			tracer,
			consumer,
			protoState,
			all.Index,
			all.Payloads,
			util.MockBlockTimer(),
			receiptValidator,
			sealValidator,
		)
		require.NoError(t, err)

		mutableState := protocol_state.NewMutableProtocolState(
			log,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			state.Params(),
			all.Headers,
			all.Results,
			all.EpochSetups,
			all.EpochCommits,
		)
		expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)
		_, err = state.AtBlockID(head.ID()).Epochs().Current()
		require.NoError(t, err)

		// we should begin the epoch in the staking phase
		phase, err := state.AtBlockID(head.ID()).EpochPhase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseStaking, phase)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
			unittest.WithFirstView(epoch1FinalView+1),
		)
		// create a receipt for block 1 containing the EpochSetup event
		receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1, epoch2Setup.ServiceEvent())

		// add a second block with the receipt for block 1
		block2 := unittest.BlockWithParentAndPayload(
			block1.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(block1.Payload.ProtocolStateID),
			),
		)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block2.ID())
		require.NoError(t, err)

		// block 3 contains the seal for block 1
		seals := []*flow.Seal{seal1}
		block3View := block2.View + 1
		block3 := unittest.BlockFixture(
			unittest.Block.WithParent(block2.ID(), block2.View, block2.Height),
			unittest.Block.WithView(block3View),
			unittest.Block.WithPayload(
				flow.Payload{
					Seals:           seals,
					ProtocolStateID: expectedStateIdCalculator(block2.ID(), block3View, seals),
				}),
		)
		// insert the block sealing the EpochSetup event
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.NoError(t, err)

		// now that the setup event has been emitted, we should be in the setup phase
		phase, err = state.AtBlockID(block3.ID()).EpochPhase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseSetup, phase)

		// we should NOT be able to query epoch 2 wrt blocks before 3
		for _, blockID := range []flow.Identifier{block1.ID(), block2.ID()} {
			_, err = state.AtBlockID(blockID).Epochs().NextUnsafe()
			require.Error(t, err)
		}

		// we should be able to query epoch 2 as a TentativeEpoch wrt block 3
		_, err = state.AtBlockID(block3.ID()).Epochs().NextUnsafe()
		assert.NoError(t, err)

		// only setup event is finalized, not commit, so shouldn't be able to read a CommittedEpoch
		_, err = state.AtBlockID(block3.ID()).Epochs().NextCommitted()
		require.Error(t, err)

		// insert B4
		block4 := unittest.BlockWithParentProtocolState(block3)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
		require.NoError(t, err)

		consumer.On("EpochSetupPhaseStarted", epoch2Setup.Counter-1, block3.ToHeader()).Once()
		metrics.On("CurrentEpochPhase", flow.EpochPhaseSetup).Once()
		// finalize block 3, so we can finalize subsequent blocks
		// ensure an epoch phase transition when we finalize block 3
		err = state.Finalize(context.Background(), block3.ID())
		require.NoError(t, err)
		consumer.AssertCalled(t, "EpochSetupPhaseStarted", epoch2Setup.Counter-1, block3.ToHeader())
		metrics.AssertCalled(t, "CurrentEpochPhase", flow.EpochPhaseSetup)

		// now that the setup event has been emitted, we should be in the setup phase
		phase, err = state.AtBlockID(block3.ID()).EpochPhase()
		require.NoError(t, err)
		require.Equal(t, flow.EpochPhaseSetup, phase)

		// finalize block 4
		err = state.Finalize(context.Background(), block4.ID())
		require.NoError(t, err)

		epoch2Commit := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(epoch2Setup.Counter),
			unittest.WithClusterQCsFromAssignments(epoch2Setup.Assignments),
			unittest.WithDKGFromParticipants(epoch2Participants.ToSkeleton()),
		)
		// create receipt and seal for block 2
		// the receipt for block 2 contains the EpochCommit event
		receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2, epoch2Commit.ServiceEvent())

		// block 5 contains the receipt for block 2
		block5 := unittest.BlockWithParentAndPayload(
			block4.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(receipt2),
				unittest.WithProtocolStateID(block4.Payload.ProtocolStateID),
			),
		)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block5))
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block5.ID())
		require.NoError(t, err)

		// block 6 contains the seal for block 2
		seals = []*flow.Seal{seal2}
		block6View := block5.View + 1
		block6 := unittest.BlockFixture(
			unittest.Block.WithParent(block5.ID(), block5.View, block5.Height),
			unittest.Block.WithView(block6View),
			unittest.Block.WithPayload(
				flow.Payload{
					Seals:           seals,
					ProtocolStateID: expectedStateIdCalculator(block5.ID(), block6View, seals),
				}),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block6))
		require.NoError(t, err)

		// we should NOT be able to query epoch 2 commit info wrt blocks before 6
		for _, blockID := range []flow.Identifier{block4.ID(), block5.ID()} {
			_, err = state.AtBlockID(blockID).Epochs().NextCommitted()
			require.Error(t, err)
		}

		// now epoch 2 is committed, we can query anything we want about it wrt block 6 (or later)
		_, err = state.AtBlockID(block6.ID()).Epochs().NextCommitted()
		require.NoError(t, err)

		// now that the commit event has been emitted, we should be in the committed phase
		phase, err = state.AtBlockID(block6.ID()).EpochPhase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseCommitted, phase)

		// block 7 has the final view of the epoch, insert it, finalized after finalizing block 6
		block7 := unittest.BlockWithParentProtocolState(block6)
		block7.View = epoch1FinalView
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block7))
		require.NoError(t, err)

		// expect epoch phase transition once we finalize block 6
		consumer.On("EpochCommittedPhaseStarted", epoch2Setup.Counter-1, block6.ToHeader()).Once()
		metrics.On("CurrentEpochPhase", flow.EpochPhaseCommitted).Once()

		err = state.Finalize(context.Background(), block6.ID())
		require.NoError(t, err)

		consumer.AssertCalled(t, "EpochCommittedPhaseStarted", epoch2Setup.Counter-1, block6.ToHeader())
		metrics.AssertCalled(t, "CurrentEpochPhase", flow.EpochPhaseCommitted)

		// we should still be in epoch 1
		block4epoch, err := state.AtBlockID(block4.ID()).Epochs().Current()
		require.NoError(t, err)
		require.Equal(t, epoch1Setup.Counter, block4epoch.Counter())

		err = state.Finalize(context.Background(), block7.ID())
		require.NoError(t, err)

		// we should still be in epoch 1, since epochs are inclusive of final view
		block7epoch, err := state.AtBlockID(block7.ID()).Epochs().Current()
		require.NoError(t, err)
		require.Equal(t, epoch1Setup.Counter, block7epoch.Counter())

		// block 8 has a view > final view of epoch 1, it will be considered the first block of epoch 2
		// we should handle views that aren't exactly the first valid view of the epoch
		block8View := epoch1FinalView + uint64(1+rand.Intn(10))
		block8 := unittest.BlockFixture(
			unittest.Block.WithParent(block7.ID(), block7.View, block7.Height),
			unittest.Block.WithView(block8View),
			unittest.Block.WithPayload(
				flow.Payload{
					ProtocolStateID: expectedStateIdCalculator(block7.ID(), block8View, nil),
				}),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block8))
		require.NoError(t, err)

		// now, at long last, we are in epoch 2
		block8epoch, err := state.AtBlockID(block8.ID()).Epochs().Current()
		require.NoError(t, err)
		require.Equal(t, epoch2Setup.Counter, block8epoch.Counter())

		// we should begin epoch 2 in staking phase
		// now that we have entered view range of epoch 2, should be in staking phase
		phase, err = state.AtBlockID(block8.ID()).EpochPhase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseStaking, phase)

		// expect epoch transition once we finalize block 8
		consumer.On("EpochTransition", epoch2Setup.Counter, block8.ToHeader()).Once()
		metrics.On("EpochTransitionHeight", block8.Height).Once()
		metrics.On("CurrentEpochCounter", epoch2Setup.Counter).Once()
		metrics.On("CurrentEpochPhase", flow.EpochPhaseStaking).Once()
		metrics.On("CurrentEpochFinalView", epoch2Setup.FinalView).Once()
		metrics.On("CurrentDKGPhaseViews", epoch2Setup.DKGPhase1FinalView, epoch2Setup.DKGPhase2FinalView, epoch2Setup.DKGPhase3FinalView).Once()

		// before block 9 is finalized, the epoch 1-2 boundary is unknown
		_, err = block8epoch.FinalHeight()
		assert.ErrorIs(t, err, realprotocol.ErrUnknownEpochBoundary)
		_, err = block8epoch.FirstHeight()
		assert.ErrorIs(t, err, realprotocol.ErrUnknownEpochBoundary)

		err = state.Finalize(context.Background(), block8.ID())
		require.NoError(t, err)

		// once block 8 is finalized, epoch 2 has unambiguously begun - the epoch 1-2 boundary is known
		block8previous, err := state.AtBlockID(block8.ID()).Epochs().Previous()
		require.NoError(t, err)
		epoch1FinalHeight, err := block8previous.FinalHeight()
		require.NoError(t, err)
		assert.Equal(t, block7.Height, epoch1FinalHeight)
		block8epoch, err = state.AtBlockID(block8.ID()).Epochs().Current()
		require.NoError(t, err)
		epoch2FirstHeight, err := block8epoch.FirstHeight()
		require.NoError(t, err)
		assert.Equal(t, block8.Height, epoch2FirstHeight)
	})
}

// we should be able to have conflicting forks with two different instances of
// the same service event for the same epoch
//
//	         /--B1<--B3(R1)<--B5(S1)<--B7
//	ROOT <--+
//	         \--B2<--B4(R2)<--B6(S2)<--B8
func TestExtendConflictingEpochEvents(t *testing.T) {
	// add more collectors so that we can have multiple distinct cluster assignments
	extraCollectors := unittest.IdentityListFixture(2, func(identity *flow.Identity) {
		identity.Role = flow.RoleCollection
	})
	rootSnapshot := unittest.RootSnapshotFixture(append(participants, extraCollectors...))
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// In this test, we create two conflicting forks. To prevent accidentally creating byzantine scenarios, where
		// multiple blocks have the same view, we keep track of used views and ensure that each new block has a unique view.
		usedViews := make(map[uint64]struct{})
		usedViews[head.View] = struct{}{}

		// add two conflicting blocks for each service event to reference
		block1 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)

		block2 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)

		rootSetup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// create two conflicting epoch setup events for the next epoch (clustering differs)
		nextEpochSetup1 := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+1000),
			unittest.WithFirstView(rootSetup.FinalView+1),
		)
		nextEpochSetup1.Assignments = unittest.ClusterAssignment(1, rootSetup.Participants)
		nextEpochSetup2 := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+1000),
			unittest.WithFirstView(rootSetup.FinalView+1),
		)
		nextEpochSetup2.Assignments = unittest.ClusterAssignment(2, rootSetup.Participants)
		assert.NotEqual(t, nextEpochSetup1.Assignments, nextEpochSetup2.Assignments)

		// add blocks containing receipts for block1 and block2 (necessary for sealing)
		// block 1 receipt contains nextEpochSetup1
		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block1Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup1.ServiceEvent()}

		// add block 1 receipt to block 3 payload
		block3 := unittest.BlockWithParentAndPayloadAndUniqueView(
			block1.ToHeader(),
			flow.Payload{
				Receipts:        []*flow.ExecutionReceiptStub{block1Receipt.Stub()},
				Results:         []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
				ProtocolStateID: block1.Payload.ProtocolStateID,
			},
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.NoError(t, err)

		// block 2 receipt contains nextEpochSetup2
		block2Receipt := unittest.ReceiptForBlockFixture(block2)
		block2Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup2.ServiceEvent()}

		// add block 2 receipt to block 4 payload
		block4 := unittest.BlockWithParentAndPayloadAndUniqueView(
			block2.ToHeader(),
			flow.Payload{
				Receipts:        []*flow.ExecutionReceiptStub{block2Receipt.Stub()},
				Results:         []*flow.ExecutionResult{&block2Receipt.ExecutionResult},
				ProtocolStateID: block2.Payload.ProtocolStateID,
			},
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
		require.NoError(t, err)

		// seal for block 1
		seals1 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))}

		// seal for block 2
		seals2 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))}

		// block 5 builds on block 3, contains seal for block 1
		block5View := nextUnusedViewSince(block3.View, usedViews)
		block5 := unittest.BlockFixture(
			unittest.Block.WithParent(block3.ID(), block3.View, block3.Height),
			unittest.Block.WithView(block5View),
			unittest.Block.WithPayload(
				flow.Payload{
					Seals:           seals1,
					ProtocolStateID: expectedStateIdCalculator(block3.ID(), block5View, seals1),
				}),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block5))
		require.NoError(t, err)

		// block 6 builds on block 4, contains seal for block 2
		block6View := nextUnusedViewSince(block4.View, usedViews)
		block6 := unittest.BlockFixture(
			unittest.Block.WithParent(block4.ID(), block4.View, block4.Height),
			unittest.Block.WithView(block6View),
			unittest.Block.WithPayload(
				flow.Payload{
					Seals:           seals2,
					ProtocolStateID: expectedStateIdCalculator(block4.ID(), block6View, seals2),
				}),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block6))
		require.NoError(t, err)

		// block 7 builds on block 5, contains QC for block 5
		block7 := unittest.BlockWithParentProtocolStateAndUniqueView(block5, usedViews)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block7))
		require.NoError(t, err)

		// block 8 builds on block 6, contains QC for block 6
		block8 := unittest.BlockWithParentProtocolStateAndUniqueView(block6, usedViews)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block8))
		require.NoError(t, err)

		// should be able to query each epoch from the appropriate reference block
		nextEpoch1, err := state.AtBlockID(block7.ID()).Epochs().NextUnsafe()
		require.NoError(t, err)
		setup1clustering, err := nextEpoch1.Clustering()
		assert.NoError(t, err)
		require.Equal(t, nextEpochSetup1.Assignments, setup1clustering.Assignments())

		phase, err := state.AtBlockID(block8.ID()).EpochPhase()
		assert.NoError(t, err)
		require.Equal(t, phase, flow.EpochPhaseSetup)
		nextEpoch2, err := state.AtBlockID(block8.ID()).Epochs().NextUnsafe()
		require.NoError(t, err)
		setup2clustering, err := nextEpoch2.Clustering()
		assert.NoError(t, err)
		require.Equal(t, nextEpochSetup2.Assignments, setup2clustering.Assignments())

	})
}

// we should be able to have conflicting forks with two DUPLICATE instances of
// the same service event for the same epoch
//
//	        /--B1<--B3(R1)<--B5(S1)<--B7
//	ROOT <--+
//	        \--B2<--B4(R2)<--B6(S2)<--B8
func TestExtendDuplicateEpochEvents(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// In this test, we create two conflicting forks. To prevent accidentally creating byzantine scenarios, where
		// multiple blocks have the same view, we keep track of used views and ensure that each new block has a unique view.
		usedViews := make(map[uint64]struct{})
		usedViews[head.View] = struct{}{}

		// add two conflicting blocks for each service event to reference
		block1 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)

		block2 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)

		rootSetup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// create an epoch setup event to insert to BOTH forks
		nextEpochSetup := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+1000),
			unittest.WithFirstView(rootSetup.FinalView+1),
		)

		// add blocks containing receipts for block1 and block2 (necessary for sealing)
		// block 1 receipt contains nextEpochSetup1
		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block1Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup.ServiceEvent()}

		// add block 1 receipt to block 3 payload
		block3 := unittest.BlockWithParentAndPayloadAndUniqueView(
			block1.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(block1Receipt),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.NoError(t, err)

		// block 2 receipt contains nextEpochSetup2
		block2Receipt := unittest.ReceiptForBlockFixture(block2)
		block2Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup.ServiceEvent()}

		// add block 2 receipt to block 4 payload
		block4 := unittest.BlockWithParentAndPayloadAndUniqueView(
			block2.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(block2Receipt),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
		require.NoError(t, err)

		// seal for block 1
		seals1 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))}

		// seal for block 2
		seals2 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))}

		// block 5 builds on block 3, contains seal for block 1
		block5View := nextUnusedViewSince(block3.View, usedViews)
		block5 := unittest.BlockFixture(
			unittest.Block.WithParent(block3.ID(), block3.View, block3.Height),
			unittest.Block.WithView(block5View),
			unittest.Block.WithPayload(
				flow.Payload{
					Seals:           seals1,
					ProtocolStateID: expectedStateIdCalculator(block3.ID(), block5View, seals1),
				}),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block5))
		require.NoError(t, err)

		// block 6 builds on block 4, contains seal for block 2
		block6View := nextUnusedViewSince(block4.View, usedViews)
		block6 := unittest.BlockFixture(
			unittest.Block.WithParent(block4.ID(), block4.View, block4.Height),
			unittest.Block.WithView(block6View),
			unittest.Block.WithPayload(
				flow.Payload{
					Seals:           seals2,
					ProtocolStateID: expectedStateIdCalculator(block4.ID(), block6View, seals2),
				}),
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block6))
		require.NoError(t, err)

		// block 7 builds on block 5, contains QC for block 5
		block7 := unittest.BlockWithParentProtocolStateAndUniqueView(block5, usedViews)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block7))
		require.NoError(t, err)

		// block 8 builds on block 6, contains QC for block 6
		// at this point we are inserting the duplicate EpochSetup, should not error
		block8 := unittest.BlockWithParentProtocolStateAndUniqueView(block6, usedViews)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block8))
		require.NoError(t, err)

		// should be able to query each epoch from the appropriate reference block
		block7next, err := state.AtBlockID(block7.ID()).Epochs().NextUnsafe()
		require.NoError(t, err)
		require.Equal(t, nextEpochSetup.Participants, block7next.InitialIdentities())

		block8next, err := state.AtBlockID(block8.ID()).Epochs().NextUnsafe()
		require.NoError(t, err)
		require.Equal(t, nextEpochSetup.Participants, block8next.InitialIdentities())
	})
}

// TestExtendEpochSetupInvalid tests that incorporating an invalid EpochSetup
// service event should trigger epoch fallback when the fork is finalized.
func TestExtendEpochSetupInvalid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)

	// setupState initializes the protocol state for a test case
	// * creates and finalizes a new block for the first seal to reference
	// * creates a factory method for test cases to generated valid EpochSetup events
	setupState := func(t *testing.T, _ storage.DB, state *protocol.ParticipantState) (
		*flow.Block,
		func(...func(*flow.EpochSetup)) (*flow.EpochSetup, *flow.ExecutionReceipt, *flow.Seal),
	) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		unittest.InsertAndFinalize(t, state, block1)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

		// this function will return a VALID setup event and seal, we will modify
		// in different ways in each test case
		createSetupEvent := func(opts ...func(*flow.EpochSetup)) (*flow.EpochSetup, *flow.ExecutionReceipt, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			for _, apply := range opts {
				apply(setup)
			}
			receipt, seal := unittest.ReceiptAndSealForBlock(block1, setup.ServiceEvent())
			return setup, receipt, seal
		}

		return block1, createSetupEvent
	}

	// expect a setup event with wrong counter to trigger EFM without error
	t.Run("wrong counter [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				setup.Counter = rand.Uint64()
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})

	// expect a setup event with wrong final view to trigger EFM without error
	t.Run("invalid final view [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				setup.FinalView = block1.View
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})

	// expect a setup event with empty seed to trigger EFM without error
	t.Run("empty seed [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				setup.RandomSource = nil
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})

	t.Run("participants not ordered [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				var err error
				setup.Participants, err = setup.Participants.Shuffle()
				require.NoError(t, err)
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})
}

// TestExtendEpochCommitInvalid tests that incorporating an invalid EpochCommit
// service event should trigger epoch fallback when the fork is finalized.
func TestExtendEpochCommitInvalid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)

	// setupState initializes the protocol state for a test case
	// * creates and finalizes a new block for the first seal to reference
	// * creates a factory method for test cases to generated valid EpochSetup events
	// * creates a factory method for test cases to generated valid EpochCommit events
	setupState := func(t *testing.T, state *protocol.ParticipantState) (
		*flow.Block,
		func(*flow.Block) (*flow.EpochSetup, *flow.ExecutionReceipt, *flow.Seal),
		func(*flow.Block, ...func(*flow.EpochCommit)) (*flow.EpochCommit, *flow.ExecutionReceipt, *flow.Seal),
	) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		unittest.InsertAndFinalize(t, state, block1)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// swap consensus node for a new one for epoch 2
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		epoch2Participants := append(
			participants.Filter(filter.Not(filter.HasRole[flow.Identity](flow.RoleConsensus))),
			epoch2NewParticipant,
		).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

		// factory method to create a valid EpochSetup method w.r.t. the generated state
		createSetup := func(block *flow.Block) (*flow.EpochSetup, *flow.ExecutionReceipt, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)

			receipt, seal := unittest.ReceiptAndSealForBlock(block, setup.ServiceEvent())
			return setup, receipt, seal
		}

		// factory method to create a valid EpochCommit method w.r.t. the generated state
		createCommit := func(block *flow.Block, opts ...func(*flow.EpochCommit)) (*flow.EpochCommit, *flow.ExecutionReceipt, *flow.Seal) {
			commit := unittest.EpochCommitFixture(
				unittest.CommitWithCounter(epoch1Setup.Counter+1),
				unittest.WithDKGFromParticipants(epoch2Participants),
			)
			for _, apply := range opts {
				apply(commit)
			}
			receipt, seal := unittest.ReceiptAndSealForBlock(block, commit.ServiceEvent())
			return commit, receipt, seal
		}

		return block1, createSetup, createCommit
	}

	t.Run("without setup [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, _, createCommit := setupState(t, state)

			_, receipt, seal := createCommit(block1)

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})

	// expect a commit event with wrong counter to trigger EFM without error
	t.Run("inconsistent counter [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup, createCommit := setupState(t, state)

			// seal block 1, in which EpochSetup was emitted
			epoch2Setup, setupReceipt, setupSeal := createSetup(block1)
			epochSetupReceiptBlock, epochSetupSealingBlock := unittest.SealBlock(t, state, mutableState, block1, setupReceipt, setupSeal)
			err := state.Finalize(context.Background(), epochSetupReceiptBlock.ID())
			require.NoError(t, err)
			err = state.Finalize(context.Background(), epochSetupSealingBlock.ID())
			require.NoError(t, err)

			// insert a block with a QC for block 2
			block3 := unittest.BlockWithParentProtocolState(epochSetupSealingBlock)
			unittest.InsertAndFinalize(t, state, block3)

			_, receipt, seal := createCommit(block3, func(commit *flow.EpochCommit) {
				commit.Counter = epoch2Setup.Counter + 1
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			err = state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})

	// expect a commit event with wrong cluster QCs to trigger EFM without error
	t.Run("inconsistent cluster QCs [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup, createCommit := setupState(t, state)

			// seal block 1, in which EpochSetup was emitted
			_, setupReceipt, setupSeal := createSetup(block1)
			epochSetupReceiptBlock, epochSetupSealingBlock := unittest.SealBlock(t, state, mutableState, block1, setupReceipt, setupSeal)
			err := state.Finalize(context.Background(), epochSetupReceiptBlock.ID())
			require.NoError(t, err)
			err = state.Finalize(context.Background(), epochSetupSealingBlock.ID())
			require.NoError(t, err)

			// insert a block with a QC for block 2
			block3 := unittest.BlockWithParentProtocolState(epochSetupSealingBlock)
			unittest.InsertAndFinalize(t, state, block3)

			_, receipt, seal := createCommit(block3, func(commit *flow.EpochCommit) {
				commit.ClusterQCs = append(commit.ClusterQCs, flow.ClusterQCVoteDataFromQC(unittest.QuorumCertificateWithSignerIDsFixture()))
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			err = state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})

	// expect a commit event with wrong dkg participants to trigger EFM without error
	t.Run("inconsistent DKG participants [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup, createCommit := setupState(t, state)

			// seal block 1, in which EpochSetup was emitted
			_, setupReceipt, setupSeal := createSetup(block1)
			epochSetupReceiptBlock, epochSetupSealingBlock := unittest.SealBlock(t, state, mutableState, block1, setupReceipt, setupSeal)
			err := state.Finalize(context.Background(), epochSetupReceiptBlock.ID())
			require.NoError(t, err)
			err = state.Finalize(context.Background(), epochSetupSealingBlock.ID())
			require.NoError(t, err)

			// insert a block with a QC for block 2
			block3 := unittest.BlockWithParentProtocolState(epochSetupSealingBlock)
			unittest.InsertAndFinalize(t, state, block3)

			_, receipt, seal := createCommit(block3, func(commit *flow.EpochCommit) {
				// add an extra Random Beacon key
				commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, unittest.KeyFixture(crypto.BLSBLS12381).PublicKey())
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			err = state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochFallbackTriggered(t, state.Final(), false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochFallbackTriggered(t, state.Final(), true)
		})
	})
}

// TestEpochFallbackMode tests that epoch fallback mode is triggered
// when an epoch fails to be committed before the epoch commitment deadline,
// or when an invalid service event (indicating service account smart contract bug)
// is sealed.
func TestEpochFallbackMode(t *testing.T) {

	// if we finalize the first block past the epoch commitment deadline while
	// in the EpochStaking phase, EFM should be triggered
	//
	//       Epoch Commitment Deadline
	//       |     Epoch Boundary
	//       |     |
	//       v     v
	// ROOT <- B1 <- B2
	t.Run("passed epoch commitment deadline in EpochStaking phase - should trigger EFM", func(t *testing.T) {

		rootSnapshot := unittest.RootSnapshotFixture(participants)
		metricsMock := mockmodule.NewComplianceMetrics(t)
		mockMetricsForRootSnapshot(metricsMock, rootSnapshot)
		protoEventsMock := mockprotocol.NewConsumer(t)
		protoEventsMock.On("BlockFinalized", mock.Anything)
		protoEventsMock.On("BlockProcessable", mock.Anything, mock.Anything)

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			result, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)
			rootProtocolState, err := rootSnapshot.ProtocolState()
			require.NoError(t, err)
			epochExtensionViewCount := rootProtocolState.GetEpochExtensionViewCount()
			safetyThreshold := rootProtocolState.GetFinalizationSafetyThreshold()
			require.GreaterOrEqual(t, epochExtensionViewCount, safetyThreshold, "epoch extension view count must be at least as large as safety threshold")

			expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

			epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch1FinalView := epoch1Setup.FinalView
			epoch1CommitmentDeadline := epoch1FinalView - safetyThreshold

			// we begin the epoch in the EpochStaking phase and
			// block 1 will be the first block on or past the epoch commitment deadline
			block1View := epoch1CommitmentDeadline + rand.Uint64()%2
			block1 := unittest.BlockFixture(
				unittest.Block.WithParent(head.ID(), head.View, head.Height),
				unittest.Block.WithView(block1View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(head.ID(), block1View, nil),
					}),
			)
			// finalizing block 1 should trigger EFM
			metricsMock.On("EpochFallbackModeTriggered").Once()
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseFallback).Once()
			metricsMock.On("CurrentEpochFinalView", epoch1FinalView+epochExtensionViewCount)
			protoEventsMock.On("EpochFallbackModeTriggered", epoch1Setup.Counter, block1.ToHeader()).Once()
			protoEventsMock.On("EpochExtended", epoch1Setup.Counter, block1.ToHeader(), unittest.MatchEpochExtension(epoch1FinalView, epochExtensionViewCount)).Once()

			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // not triggered before finalization
			err = state.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true)     // triggered after finalization
			assertInPhase(t, state.Final(), flow.EpochPhaseFallback) // immediately enter fallback phase

			// block 2 will be the first block past the first epoch boundary
			block2 := unittest.BlockWithParentProtocolState(block1)
			block2.View = epoch1FinalView + 1
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			// since EFM has been triggered, epoch transition metrics should not be updated
			metricsMock.AssertNotCalled(t, "EpochTransition", mock.Anything, mock.Anything)
			metricsMock.AssertNotCalled(t, "CurrentEpochCounter", epoch1Setup.Counter+1)
		})
	})

	// if we finalize the first block past the epoch commitment deadline while
	// in the EpochSetup phase, EFM should be triggered
	//
	//                       Epoch Commitment Deadline
	//                       |         Epoch Boundary
	//                       |         |
	//                       v         v
	// ROOT <- B1 <- B2(R1) <- B3(S1) <- B4
	t.Run("passed epoch commitment deadline in EpochSetup phase - should trigger EFM", func(t *testing.T) {

		rootSnapshot := unittest.RootSnapshotFixture(participants)
		rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
		metricsMock := mockmodule.NewComplianceMetrics(t)
		mockMetricsForRootSnapshot(metricsMock, rootSnapshot)
		protoEventsMock := mockprotocol.NewConsumer(t)
		protoEventsMock.On("BlockFinalized", mock.Anything)
		protoEventsMock.On("BlockProcessable", mock.Anything, mock.Anything)

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			result, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)
			rootProtocolState, err := rootSnapshot.ProtocolState()
			require.NoError(t, err)
			epochExtensionViewCount := rootProtocolState.GetEpochExtensionViewCount()
			safetyThreshold := rootProtocolState.GetFinalizationSafetyThreshold()
			require.GreaterOrEqual(t, epochExtensionViewCount, safetyThreshold, "epoch extension view count must be at least as large as safety threshold")

			// add a block for the first seal to reference
			block1 := unittest.BlockWithParentAndPayload(
				head,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)

			epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch1FinalView := epoch1Setup.FinalView
			epoch1CommitmentDeadline := epoch1FinalView - safetyThreshold

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

			// create the epoch setup event for the second epoch
			epoch2Setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1FinalView+1000),
				unittest.WithFirstView(epoch1FinalView+1),
			)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1, epoch2Setup.ServiceEvent())

			// add a block containing a receipt for block 1
			block2 := unittest.BlockWithParentAndPayload(
				block1.ToHeader(),
				unittest.PayloadFixture(
					unittest.WithReceipts(receipt1),
					unittest.WithProtocolStateID(rootProtocolStateID),
				),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			// block 3 seals block 1 and will be the first block on or past the epoch commitment deadline
			block3View := epoch1CommitmentDeadline + rand.Uint64()%2
			seals := []*flow.Seal{seal1}
			block3 := unittest.BlockFixture(
				unittest.Block.WithParent(block2.ID(), block2.View, block2.Height),
				unittest.Block.WithView(block3View),
				unittest.Block.WithPayload(
					flow.Payload{
						Seals:           seals,
						ProtocolStateID: calculateExpectedStateId(t, mutableState)(block2.ID(), block3View, seals),
					}),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
			require.NoError(t, err)

			// finalizing block 3 should trigger EFM
			metricsMock.On("EpochFallbackModeTriggered").Once()
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseFallback).Once()
			metricsMock.On("CurrentEpochFinalView", epoch1FinalView+epochExtensionViewCount)
			protoEventsMock.On("EpochFallbackModeTriggered", epoch1Setup.Counter, block3.ToHeader()).Once()
			protoEventsMock.On("EpochExtended", epoch1Setup.Counter, block3.ToHeader(), unittest.MatchEpochExtension(epoch1FinalView, epochExtensionViewCount)).Once()

			assertEpochFallbackTriggered(t, state.Final(), false) // not triggered before finalization
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true) // triggered after finalization
			assertInPhase(t, state.Final(), flow.EpochPhaseFallback)

			// block 4 will be the first block past the first epoch boundary
			block4 := unittest.BlockWithParentProtocolState(block3)
			block4.View = epoch1FinalView + 1
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block4.ID())
			require.NoError(t, err)

			// since EFM has been triggered, epoch transition metrics should not be updated
			metricsMock.AssertNotCalled(t, "EpochTransition", epoch2Setup.Counter, mock.Anything)
			metricsMock.AssertNotCalled(t, "CurrentEpochCounter", epoch2Setup.Counter)
		})
	})

	// if an invalid epoch service event is incorporated, we should:
	//   - not apply the phase transition corresponding to the invalid service event
	//   - immediately trigger EFM
	//
	//                            Epoch Boundary
	//                                 |
	//                                 v
	// ROOT <- B1 <- B2(R1) <- B3(S1) <- B4
	t.Run("epoch transition with invalid service event - should trigger EFM", func(t *testing.T) {

		rootSnapshot := unittest.RootSnapshotFixture(participants)
		rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
		metricsMock := mockmodule.NewComplianceMetrics(t)
		mockMetricsForRootSnapshot(metricsMock, rootSnapshot)
		protoEventsMock := mockprotocol.NewConsumer(t)
		protoEventsMock.On("BlockFinalized", mock.Anything)
		protoEventsMock.On("BlockProcessable", mock.Anything, mock.Anything)

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			result, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)
			rootProtocolState, err := rootSnapshot.ProtocolState()
			require.NoError(t, err)
			epochExtensionViewCount := rootProtocolState.GetEpochExtensionViewCount()

			// add a block for the first seal to reference
			block1 := unittest.BlockWithParentAndPayload(
				head,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)

			epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch1FinalView := epoch1Setup.FinalView

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

			// create the epoch setup event for the second epoch
			// this event is invalid because it used a non-contiguous first view
			epoch2Setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1FinalView+1000),
				unittest.WithFirstView(epoch1FinalView+10), // invalid first view
			)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1, epoch2Setup.ServiceEvent())

			// add a block containing a receipt for block 1
			block2 := unittest.BlockWithParentAndPayload(
				block1.ToHeader(),
				unittest.PayloadFixture(
					unittest.WithReceipts(receipt1),
					unittest.WithProtocolStateID(rootProtocolStateID),
				),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			// block 3 is where the service event state change comes into effect
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
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block3))
			require.NoError(t, err)

			// incorporating the service event should trigger EFM
			metricsMock.On("EpochFallbackModeTriggered").Once()
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseFallback).Once()
			protoEventsMock.On("EpochFallbackModeTriggered", epoch1Setup.Counter, block3.ToHeader()).Once()

			assertEpochFallbackTriggered(t, state.Final(), false) // not triggered before finalization
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true)     // triggered after finalization
			assertInPhase(t, state.Final(), flow.EpochPhaseFallback) // immediately enters fallback phase

			// block 4 is the first block past the current epoch boundary
			block4View := epoch1Setup.FinalView + 1
			block4 := unittest.BlockFixture(
				unittest.Block.WithParent(block3.ID(), block3.View, block3.Height),
				unittest.Block.WithView(block4View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: calculateExpectedStateId(t, mutableState)(block3.ID(), block4View, nil),
					}),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block4))
			require.NoError(t, err)

			// we add the epoch extension after the epoch transition
			metricsMock.On("CurrentEpochFinalView", epoch1FinalView+epochExtensionViewCount).Once()
			protoEventsMock.On("EpochExtended", epoch1Setup.Counter, block4.ToHeader(), unittest.MatchEpochExtension(epoch1FinalView, epochExtensionViewCount)).Once()

			err = state.Finalize(context.Background(), block4.ID())
			require.NoError(t, err)

			// since EFM has been triggered, epoch transition metrics should not be updated
			metricsMock.AssertNotCalled(t, "EpochTransition", epoch2Setup.Counter, mock.Anything)
			metricsMock.AssertNotCalled(t, "CurrentEpochCounter", epoch2Setup.Counter)
		})
	})
}

// TestRecoveryFromEpochFallbackMode tests a few scenarios where the protocol first enters EFM in different phases
// and then recovers from it by incorporating and finalizing a valid EpochRecover service event.
// We expect different behavior depending on the phase in which the protocol enters EFM, specifically for the committed phase,
// as the protocol cannot be immediately recovered from it. First, we need to enter the next epoch before we can accept an EpochRecover event.
// Specifically, for this case we make progress till the epoch extension event to make sure that we cover the most complex scenario.
func TestRecoveryFromEpochFallbackMode(t *testing.T) {

	// assertCorrectRecovery checks that the recovery epoch is correctly setup.
	// We expect the next epoch will use setup and commit events from EpochRecover service event.
	// According to the specification, the current epoch after processing an EpochRecover event must be in committed phase,
	// since it contains EpochSetup and EpochCommit events.
	assertCorrectRecovery := func(state *protocol.ParticipantState, epochRecover *flow.EpochRecover) {
		finalSnap := state.Final()
		epochState, err := finalSnap.EpochProtocolState()
		require.NoError(t, err)
		epochPhase := epochState.EpochPhase()
		require.Equal(t, flow.EpochPhaseCommitted, epochPhase, "next epoch has to be committed")
		require.Equal(t, &epochRecover.EpochSetup, epochState.Entry().NextEpochSetup, "next epoch has to be setup according to EpochRecover")
		require.Equal(t, &epochRecover.EpochCommit, epochState.Entry().NextEpochCommit, "next epoch has to be committed according to EpochRecover")
	}

	// if we enter EFM in the EpochStaking phase, we should be able to recover by incorporating a valid EpochRecover event
	// since the epoch commitment deadline has not been reached.
	// ROOT <- B1 <- B2(ER(B1, InvalidEpochSetup)) <- B3(S(ER(B1))) <- B4(ER(B2, EpochRecover)) <- B5(S(ER(B2)))
	t.Run("entered-EFM-in-staking-phase", func(t *testing.T) {

		rootSnapshot := unittest.RootSnapshotFixture(participants)
		metricsMock := mockmodule.NewComplianceMetrics(t)
		mockMetricsForRootSnapshot(metricsMock, rootSnapshot)
		protoEventsMock := mockprotocol.NewConsumer(t)
		protoEventsMock.On("BlockFinalized", mock.Anything)
		protoEventsMock.On("BlockProcessable", mock.Anything, mock.Anything)

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			rootResult, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)

			expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

			// add a block for the first seal to reference
			block1View := head.View + 1
			block1 := unittest.BlockFixture(
				unittest.Block.WithParent(head.ID(), head.View, head.Height),
				unittest.Block.WithView(block1View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(head.ID(), block1View, nil),
					}),
			)
			unittest.InsertAndFinalize(t, state, block1)

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

			// build an invalid setup event which will trigger EFM
			epoch1Setup := rootResult.ServiceEvents[0].Event.(*flow.EpochSetup)
			invalidSetup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+10), // invalid counter
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			receipt, seal := unittest.ReceiptAndSealForBlock(block1, invalidSetup.ServiceEvent())

			// ingesting block 2 and 3, block 3 seals the invalid setup event
			block2, block3 := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			assertEpochFallbackTriggered(t, state.AtBlockID(block2.ID()), false) // EFM shouldn't be triggered since block 2 only incorporates the event, sealing happens in block 3
			assertEpochFallbackTriggered(t, state.AtBlockID(block3.ID()), true)  // EFM has to be triggered at block 3, since it seals the invalid setup event
			assertEpochFallbackTriggered(t, state.Final(), false)                // EFM should still not be triggered for finalized state since the invalid service event does not have a finalized seal

			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // EFM should still not be triggered after finalizing block 2

			// Since we enter EFM before the commitment deadline, no epoch extension is added
			metricsMock.On("EpochFallbackModeTriggered").Once()
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseFallback).Once()
			protoEventsMock.On("EpochFallbackModeTriggered", epoch1Setup.Counter, block3.ToHeader()).Once()
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true) // finalizing block 3 should have triggered EFM since it seals invalid setup event
			assertInPhase(t, state.Final(), flow.EpochPhaseFallback)

			// Block 4 incorporates Execution Result [ER] for block2, where the ER also includes EpochRecover event.
			// Only when ingesting block 5, which _seals_ the EpochRecover event, the state should switch back to
			// `EpochFallbackTriggered` being false.
			epochRecover := unittest.EpochRecoverFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			receipt, seal = unittest.ReceiptAndSealForBlock(block2, epochRecover.ServiceEvent())

			// ingesting block 4 and 5, block 5 seals the EpochRecover event
			block4, block5 := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			assertEpochFallbackTriggered(t, state.AtBlockID(block4.ID()), true)
			assertEpochFallbackTriggered(t, state.AtBlockID(block5.ID()), false)
			assertEpochFallbackTriggered(t, state.Final(), true) // the latest finalized state should still be in EFM as `epochRecover` event does not have a finalized seal

			err = state.Finalize(context.Background(), block4.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true) // should still be in EFM as `epochRecover` is not yet finalized

			// Epoch recovery results in entering Committed phase
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseCommitted).Once()
			metricsMock.On("EpochFallbackModeExited").Once()
			protoEventsMock.On("EpochFallbackModeExited", epoch1Setup.Counter, block5.ToHeader()).Once()
			protoEventsMock.On("EpochCommittedPhaseStarted", mock.Anything, mock.Anything).Once()
			// finalize the block sealing the EpochRecover event
			err = state.Finalize(context.Background(), block5.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false)     // should be unset after finalizing block 5 which contains a seal for EpochRecover.
			assertInPhase(t, state.Final(), flow.EpochPhaseCommitted) // enter committed phase after recovery
			assertCorrectRecovery(state, epochRecover)
		})
	})

	// if we enter EFM in the EpochSetup phase, we should be able to recover by incorporating a valid EpochRecover event
	// since the epoch commitment deadline has not been reached.
	// ROOT <- B1 <- B2(ER(B1, EpochSetup)) <- B3(S(ER(B1))) <- B4(ER(B2, InvalidEpochCommit)) <- B5(S(ER(B2))) <- B6(ER(B3, EpochRecover)) <- B7(S(ER(B3)))
	t.Run("entered-EFM-in-setup-phase", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		metricsMock := mockmodule.NewComplianceMetrics(t)
		mockMetricsForRootSnapshot(metricsMock, rootSnapshot)
		protoEventsMock := mockprotocol.NewConsumer(t)
		protoEventsMock.On("BlockFinalized", mock.Anything)
		protoEventsMock.On("BlockProcessable", mock.Anything, mock.Anything)

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			rootResult, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)

			expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

			// add a block for the first seal to reference
			block1View := head.View + 1
			block1 := unittest.BlockFixture(
				unittest.Block.WithParent(head.ID(), head.View, head.Height),
				unittest.Block.WithView(block1View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(head.ID(), block1View, nil),
					}),
			)
			unittest.InsertAndFinalize(t, state, block1)

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

			// Block 2 incorporates Execution Result [ER] for block1, where the ER also includes `EpochSetup` event.
			// Only when ingesting block 3, which _seals_ the `EpochSetup` event, the epoch moves to setup phase.
			epoch1Setup := rootResult.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch2Setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			receipt, seal := unittest.ReceiptAndSealForBlock(block1, epoch2Setup.ServiceEvent())

			// ingesting block 2 and 3, block 3 seals the EpochSetup event
			block2, block3 := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseSetup).Once()
			protoEventsMock.On("EpochSetupPhaseStarted", epoch2Setup.Counter-1, mock.Anything)
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // EFM is not expected

			// Block 4 incorporates Execution Result [ER] for block2, where the ER also includes invalid service event.
			// Only when ingesting block 5, which _seals_ the invalid service event, the state should switch to
			// `EpochFallbackTriggered` being true.
			invalidEpochCommit := unittest.EpochCommitFixture() // a random epoch commit event will be invalid
			receipt, seal = unittest.ReceiptAndSealForBlock(block2, invalidEpochCommit.ServiceEvent())

			// ingesting block 4 and 5, block 5 seals the invalid commit event
			block4, block5 := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			assertEpochFallbackTriggered(t, state.AtBlockID(block4.ID()), false) // EFM shouldn't be triggered since block 4 only incorporates the event, sealing happens in block 5
			assertEpochFallbackTriggered(t, state.AtBlockID(block5.ID()), true)  // EFM has to be triggered at block 5, since it seals the invalid commit event
			assertEpochFallbackTriggered(t, state.Final(), false)                // EFM should still not be triggered for finalized state since the invalid service event does not have a finalized seal

			err = state.Finalize(context.Background(), block4.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // EFM should still not be triggered after finalizing block 4

			metricsMock.On("EpochFallbackModeTriggered").Once()
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseFallback).Once()
			protoEventsMock.On("EpochFallbackModeTriggered", epoch1Setup.Counter, block5.ToHeader()).Once()
			err = state.Finalize(context.Background(), block5.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true)     // finalizing block 5 should have triggered EFM
			assertInPhase(t, state.Final(), flow.EpochPhaseFallback) // immediately enter fallback phase

			// Block 6 incorporates Execution Result [ER] for block3, where the ER also includes EpochRecover event.
			// Only when ingesting block 7, which _seals_ the EpochRecover event, the state should switch back to
			// `EpochFallbackTriggered` being false.
			epochRecover := unittest.EpochRecoverFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			receipt, seal = unittest.ReceiptAndSealForBlock(block3, epochRecover.ServiceEvent())

			// ingesting block 6 and 7, block 7 seals the `epochRecover` event
			block6, block7 := unittest.SealBlock(t, state, mutableState, block5, receipt, seal)
			assertEpochFallbackTriggered(t, state.AtBlockID(block6.ID()), true)
			assertEpochFallbackTriggered(t, state.AtBlockID(block7.ID()), false)
			assertEpochFallbackTriggered(t, state.Final(), true) // the latest finalized state should still be in EFM as `epochRecover` event does not have a finalized seal

			err = state.Finalize(context.Background(), block6.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true) // should still be in EFM as `epochRecover` is not yet finalized

			// Epoch recovery results in entering Committed phase
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseCommitted).Once()
			metricsMock.On("EpochFallbackModeExited").Once()
			protoEventsMock.On("EpochFallbackModeExited", epoch1Setup.Counter, block7.ToHeader()).Once()
			protoEventsMock.On("EpochCommittedPhaseStarted", mock.Anything, mock.Anything).Once()
			// finalize the block sealing the EpochRecover event
			err = state.Finalize(context.Background(), block7.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false)     // should be unset after finalization
			assertInPhase(t, state.Final(), flow.EpochPhaseCommitted) // enter committed phase after recovery
			assertCorrectRecovery(state, epochRecover)
		})
	})

	// Entering EFM in the commit phase is the most complex case since we can't revert an already committed epoch. In this case,
	// we proceed as follows:
	// - We build valid EpochSetup and EpochCommit events for the next epoch, effectively moving the protocol to the EpochCommit phase.
	// - Next, we incorporate an invalid EpochCommit event, which will trigger EFM.
	// - At this point, we are in EFM but the next epoch has been committed, so we can't create EpochRecover event yet.
	// - Instead, we progress to the next epoch. Note that it's possible to build an EpochRecover event at this point,
	//   but we want to test that epoch extension can be added.
	// - We build a block with a view reaching the epoch commitment deadline, which should trigger the creation of an epoch extension.
	// - Next, we build a valid EpochRecover event, incorporate and seal it, effectively recovering from EFM.
	// - To check that the state waits for recovering from EFM until we enter the next epoch (recovery epoch),
	//   we build a block with a view that is past the epoch extension but not in the recovery epoch.
	// - Finally, we build a block with a view which is in the recovery epoch to make sure that the state successfully enters it.
	// ROOT <- B1 <- B2(ER(B1, EpochSetup)) <- B3(S(ER(B1))) <- B4(ER(B2, EpochCommit)) <- B5(S(ER(B2))) <- B6(ER(B3, InvalidEpochCommit)) <-
	// <- B7(S(ER(B3))) <- B8 <- B9 <- B10 <- B11(ER(B4, EpochRecover)) <- B12(S(ER(B4))) <- B13 <- B14
	//                  ^ Epoch 1 Final View                           Last View of epoch extension  ^
	//				  			^ Epoch 2 Commitment Deadline                                         ^ Epoch 3(recovery) First View
	//								  ^ Epoch 2 Final View
	//								   ^ First View of epoch extension
	// 						^ Epoch 2 Setup Counter
	t.Run("entered-EFM-in-commit-phase", func(t *testing.T) {

		rootSnapshot := unittest.RootSnapshotFixture(participants)
		metricsMock := mockmodule.NewComplianceMetrics(t)
		mockMetricsForRootSnapshot(metricsMock, rootSnapshot)
		protoEventsMock := mockprotocol.NewConsumer(t)
		protoEventsMock.On("BlockFinalized", mock.Anything)
		protoEventsMock.On("BlockProcessable", mock.Anything, mock.Anything)

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			rootResult, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)
			rootProtocolState, err := rootSnapshot.ProtocolState()
			require.NoError(t, err)
			epochExtensionViewCount := rootProtocolState.GetEpochExtensionViewCount()
			safetyThreshold := rootProtocolState.GetFinalizationSafetyThreshold()
			require.GreaterOrEqual(t, epochExtensionViewCount, safetyThreshold, "epoch extension view count must be at least as large as safety threshold")

			expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

			// Constructing blocks
			//   ... <- B1 <- B2(ER(B1, EpochSetup)) <- B3(S(ER(B1))) <- B4(ER(B2, EpochCommit)) <- B5(S(ER(B2))) <- ...
			// B1 will be the first block that we will use as reference block for first seal. Block B2 incorporates the Execution Result [ER]
			// for block 1 and the EpochSetup service event. Block B3 seals the EpochSetup event.
			// Block B4 incorporates the Execution Result [ER] for block 2 and the EpochCommit service event. Block B5 seals the EpochCommit event.
			// We expect that the Protocol state at B5 enters `epoch committed` phase.

			// add a block for the first seal to reference
			block1View := head.View + 1
			block1 := unittest.BlockFixture(
				unittest.Block.WithParent(head.ID(), head.View, head.Height),
				unittest.Block.WithView(block1View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(head.ID(), block1View, nil),
					}),
			)
			unittest.InsertAndFinalize(t, state, block1)

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

			// Block 2 incorporates Execution Result [ER] for block1, where the ER also includes `EpochSetup` event.
			// Only when ingesting block 3, which _seals_ the `EpochSetup` event, epoch moves to the setup phase.
			epoch1Setup := rootResult.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch2Setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			receipt, seal := unittest.ReceiptAndSealForBlock(block1, epoch2Setup.ServiceEvent())

			// ingesting block 2 and 3, block 3 seals the `epochSetup` for the next epoch
			block2, block3 := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseSetup).Once()
			protoEventsMock.On("EpochSetupPhaseStarted", epoch2Setup.Counter-1, mock.Anything).Once()
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // EFM is not expected

			// Block 4 incorporates Execution Result [ER] for block2, where the ER also includes `EpochCommit` event.
			// Only when ingesting block 5, which _seals_ the `EpochCommit` event, the epoch moves to committed phase.
			epoch2Commit := unittest.EpochCommitFixture(
				unittest.CommitWithCounter(epoch2Setup.Counter),
				unittest.WithClusterQCsFromAssignments(epoch2Setup.Assignments),
				unittest.WithDKGFromParticipants(epoch2Participants.ToSkeleton()),
			)
			receipt, seal = unittest.ReceiptAndSealForBlock(block2, epoch2Commit.ServiceEvent())

			// ingesting block 4 and 5, block 5 seals the `epochCommit` for the next epoch
			block4, block5 := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			err = state.Finalize(context.Background(), block4.ID())
			require.NoError(t, err)

			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseCommitted).Once()
			protoEventsMock.On("EpochCommittedPhaseStarted", epoch2Setup.Counter-1, mock.Anything).Once()
			err = state.Finalize(context.Background(), block5.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // EFM is not expected

			// Constructing blocks
			//   ... <- B6(ER(B3, InvalidEpochCommit)) <- B7(S(ER(B3))) <- B8 <- B9 <- ...
			// Block B6 incorporates the Execution Result [ER] for block 3 and the invalid service event.
			// Block B7 seals the invalid service event.
			// We expect that the Protocol state at B7 switches `EpochFallbackTriggered` to true.
			// B8 will be the first block past the epoch boundary, which will trigger epoch transition to the next epoch.
			// B9 will be the first block past the epoch commitment deadline, which will trigger construction of an epoch extension.

			// Block 6 incorporates Execution Result [ER] for block3, where the ER also includes invalid service event.
			// Only when ingesting block 7, which _seals_ the invalid service event, the state should switch to
			// `EpochFallbackTriggered` being true.
			invalidCommit := unittest.EpochCommitFixture()
			receipt, seal = unittest.ReceiptAndSealForBlock(block3, invalidCommit.ServiceEvent())

			// seal B3 by building two blocks on top of B5 that contain ER and seal respectively
			block6, block7 := unittest.SealBlock(t, state, mutableState, block5, receipt, seal)
			assertEpochFallbackTriggered(t, state.AtBlockID(block6.ID()), false) // EFM shouldn't be triggered since block 6 only incorporates the event, sealing happens in block 7
			assertEpochFallbackTriggered(t, state.AtBlockID(block7.ID()), true)  // EFM has to be triggered at block 7, since it seals the invalid commit event
			assertEpochFallbackTriggered(t, state.Final(), false)                // EFM should still not be triggered for finalized state since the invalid service event does not have a finalized seal

			err = state.Finalize(context.Background(), block6.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false) // EFM should still not be triggered after finalizing block 4

			metricsMock.On("EpochFallbackModeTriggered").Once()
			protoEventsMock.On("EpochFallbackModeTriggered", epoch1Setup.Counter, block7.ToHeader()).Once()
			err = state.Finalize(context.Background(), block7.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true)      // finalizing block 7 should have triggered EFM
			assertInPhase(t, state.Final(), flow.EpochPhaseCommitted) // remain in committed phase until next transition

			// TODO: try submitting EpochRecover. We don't do this in current implementation since there is no way
			//  to actually check that event was ignored. We will use pub/sub mechanism to notify about invalid service event.
			//  After we have notification mechanism in place, we can extend this test.

			// B8 will trigger epoch transition to already committed epoch
			block8View := epoch1Setup.FinalView + 1 // first block past the epoch boundary
			block8 := unittest.BlockFixture(
				unittest.Block.WithParent(block7.ID(), block7.View, block7.Height),
				unittest.Block.WithView(block8View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(block7.ID(), block8View, nil),
					}),
			)

			metricsMock.On("CurrentEpochCounter", epoch2Setup.Counter).Once()
			metricsMock.On("EpochTransitionHeight", block8.Height).Once()
			metricsMock.On("CurrentEpochFinalView", epoch2Setup.FinalView).Once()
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseFallback).Once()
			protoEventsMock.On("EpochTransition", epoch2Setup.Counter, block8.ToHeader()).Once()

			// epoch transition happens at this point
			unittest.InsertAndFinalize(t, state, block8)
			assertInPhase(t, state.Final(), flow.EpochPhaseFallback) // enter fallback phase immediately after transition

			metricsMock.AssertCalled(t, "CurrentEpochCounter", epoch2Setup.Counter)
			metricsMock.AssertCalled(t, "EpochTransitionHeight", block8.Height)
			metricsMock.AssertCalled(t, "CurrentEpochFinalView", epoch2Setup.FinalView)
			protoEventsMock.AssertCalled(t, "EpochTransition", epoch2Setup.Counter, block8.ToHeader())

			// B9 doesn't have any seals, but it reaches the safety threshold for the current epoch, meaning we will create an EpochExtension
			block9View := epoch2Setup.FinalView - safetyThreshold
			block9 := unittest.BlockFixture(
				unittest.Block.WithParent(block8.ID(), block8.View, block8.Height),
				unittest.Block.WithView(block9View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(block8.ID(), block9View, nil),
					}),
			)
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block9))
			require.NoError(t, err)

			epochProtocolState, err := state.AtBlockID(block9.ID()).EpochProtocolState()
			require.NoError(t, err)
			epochExtensions := epochProtocolState.Entry().CurrentEpoch.EpochExtensions
			require.Len(t, epochExtensions, 1)
			require.Equal(t, epochExtensions[0].FirstView, epoch2Setup.FinalView+1)

			protoEventsMock.On("EpochExtended", epoch2Setup.Counter, block9.ToHeader(), unittest.MatchEpochExtension(epoch2Setup.FinalView, epochExtensionViewCount)).Once()
			metricsMock.On("CurrentEpochFinalView", epoch2Setup.FinalView+epochExtensionViewCount)
			err = state.Finalize(context.Background(), block9.ID())
			require.NoError(t, err)

			// After epoch extension, FinalView must be updated accordingly
			epochAfterExtension, err := state.Final().Epochs().Current()
			require.NoError(t, err)
			finalView := epochAfterExtension.FinalView()
			assert.Equal(t, epochExtensions[0].FinalView, finalView)

			// Constructing blocks
			//   ... <- B10 <- B11(ER(B4, EpochRecover)) <- B12(S(ER(B4))) <- ...
			// B10 will be the first block past the epoch extension. Block B11 incorporates the Execution Result [ER]
			// for block 10 and the EpochRecover service event. Block B12 seals the EpochRecover event.
			// We expect that the Protocol state at B12 switches `EpochFallbackTriggered` back to false.

			// B10 will be the first block past the epoch extension
			block10 := unittest.BlockWithParentProtocolState(block9)
			block10.View = epochExtensions[0].FirstView
			unittest.InsertAndFinalize(t, state, block10)

			// Block 11 incorporates Execution Result [ER] for block4, where the ER also includes EpochRecover event.
			// Only when ingesting block 12, which _seals_ the EpochRecover event, the state should switch back to
			// `EpochFallbackTriggered` being false.
			epochRecover := unittest.EpochRecoverFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch2Setup.Counter+1),
				unittest.WithFinalView(epochExtensions[0].FinalView+1000),
				unittest.WithFirstView(epochExtensions[0].FinalView+1),
			)
			receipt, seal = unittest.ReceiptAndSealForBlock(block4, epochRecover.ServiceEvent())

			// ingesting block 11 and 12, block 12 seals the `epochRecover` event
			block11, block12 := unittest.SealBlock(t, state, mutableState, block10, receipt, seal)
			assertEpochFallbackTriggered(t, state.AtBlockID(block11.ID()), true)
			assertEpochFallbackTriggered(t, state.AtBlockID(block12.ID()), false)
			assertEpochFallbackTriggered(t, state.Final(), true) // the latest finalized state should still be in EFM as `epochRecover` event does not have a finalized seal

			err = state.Finalize(context.Background(), block11.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), true) // should still be in EFM as `epochRecover` is not yet finalized

			// Epoch recovery causes us to enter the Committed phase
			metricsMock.On("CurrentEpochPhase", flow.EpochPhaseCommitted).Once()
			metricsMock.On("EpochFallbackModeExited").Once()
			protoEventsMock.On("EpochFallbackModeExited", epochRecover.EpochSetup.Counter-1, block12.ToHeader()).Once()
			protoEventsMock.On("EpochCommittedPhaseStarted", epochRecover.EpochSetup.Counter-1, mock.Anything).Once()
			// finalize the block sealing the EpochRecover event
			err = state.Finalize(context.Background(), block12.ID())
			require.NoError(t, err)
			assertEpochFallbackTriggered(t, state.Final(), false)     // should be unset after finalization
			assertInPhase(t, state.Final(), flow.EpochPhaseCommitted) // enter committed phase after recovery
			assertCorrectRecovery(state, epochRecover)

			// Constructing blocks
			//   ... <- B13 <- B14
			// B13 will be a child block of B12 to ensure that we don't transition into recovered epoch immediately the next block
			// but actually finish the epoch extension. B14 will be the first block past the epoch extension,
			// which will trigger epoch transition to the recovered epoch.
			// We expect that Protocol state will be at first view of recovered epoch and in epoch staking phase after B14 is incorporated.

			block13 := unittest.BlockWithParentProtocolState(block12)
			unittest.InsertAndFinalize(t, state, block13)

			// ensure we are still in the current epoch and transition only when we reach the final view of the extension
			epochProtocolState, err = state.Final().EpochProtocolState()
			require.NoError(t, err)
			require.Equal(t, epoch2Setup.Counter, epochProtocolState.Epoch(), "expect to be in the previously setup epoch")

			// B14 will be the first block past the epoch extension, meaning it will enter the next epoch which
			// had been set up by EpochRecover event
			block14View := epochExtensions[0].FinalView + 1
			block14 := unittest.BlockFixture(
				unittest.Block.WithParent(block13.ID(), block13.View, block13.Height),
				unittest.Block.WithView(block14View),
				unittest.Block.WithPayload(
					flow.Payload{
						ProtocolStateID: expectedStateIdCalculator(block13.ID(), block14View, nil),
					}),
			)

			metricsMock.On("CurrentEpochCounter", epochRecover.EpochSetup.Counter).Once()
			metricsMock.On("EpochTransitionHeight", block14.Height).Once()
			metricsMock.On("CurrentEpochFinalView", epochRecover.EpochSetup.FinalView).Once()
			protoEventsMock.On("EpochTransition", epochRecover.EpochSetup.Counter, block14.ToHeader()).Once()

			unittest.InsertAndFinalize(t, state, block14)

			epochProtocolState, err = state.Final().EpochProtocolState()
			require.NoError(t, err)
			require.Equal(t, epochRecover.EpochSetup.Counter, epochProtocolState.Epoch(), "expect to be in recovered epoch")
			require.Equal(t, flow.EpochPhaseStaking, epochProtocolState.EpochPhase(), "expect to be in staking phase")
		})
	})
}

// TestEpochTargetEndTime ensurers that the target end time of an epoch is correctly calculated depending on if the epoch is extended or not.
// When we haven't added an extension yet, then the TargetEndTime is simply the value from the epoch setup event.
// Otherwise, the TargetEndTime is calculated by adding the duration of the extension to the TargetEndTime of epoch setup.
// We assume we keep the same view duration for all views in the epoch and all extensions.
func TestEpochTargetEndTime(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		rootResult, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		epoch1Setup := rootResult.ServiceEvents[0].Event.(*flow.EpochSetup)
		currentEpoch, err := rootSnapshot.Epochs().Current()
		require.NoError(t, err)
		rootTargetEndTime := currentEpoch.TargetEndTime()
		require.Equal(t, epoch1Setup.TargetEndTime, rootTargetEndTime)

		expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

		// add a block that will trigger EFM and add an epoch extension since the view of the epoch exceeds the safety threshold
		block1View := epoch1Setup.FinalView
		block1 := unittest.BlockFixture(
			unittest.Block.WithParent(head.ID(), head.View, head.Height),
			unittest.Block.WithView(block1View),
			unittest.Block.WithPayload(
				flow.Payload{
					ProtocolStateID: expectedStateIdCalculator(head.ID(), block1View, nil),
				}),
		)
		unittest.InsertAndFinalize(t, state, block1)

		block1snap := state.Final()
		assertEpochFallbackTriggered(t, block1snap, true)
		assertInPhase(t, block1snap, flow.EpochPhaseFallback)

		epochState, err := block1snap.EpochProtocolState()
		require.NoError(t, err)
		firstExtension := epochState.EpochExtensions()[0]
		targetViewDuration := float64(epoch1Setup.TargetDuration) / float64(epoch1Setup.FinalView-epoch1Setup.FirstView+1)
		expectedTargetEndTime := rootTargetEndTime + uint64(float64(firstExtension.FinalView-epoch1Setup.FinalView)*targetViewDuration)
		afterFirstExtensionEpoch, err := block1snap.Epochs().Current()
		require.NoError(t, err)
		afterFirstExtensionTargetEndTime := afterFirstExtensionEpoch.TargetEndTime()
		require.Equal(t, expectedTargetEndTime, afterFirstExtensionTargetEndTime)

		// add a second block that exceeds the safety threshold and triggers another epoch extension
		block2View := firstExtension.FinalView
		block2 := unittest.BlockFixture(
			unittest.Block.WithParent(block1.ID(), block1.View, block1.Height),
			unittest.Block.WithView(block2View),
			unittest.Block.WithPayload(
				flow.Payload{
					ProtocolStateID: expectedStateIdCalculator(block1.ID(), block2View, nil),
				}),
		)
		unittest.InsertAndFinalize(t, state, block2)

		block2snap := state.Final()
		epochState, err = block2snap.EpochProtocolState()
		require.NoError(t, err)
		secondExtension := epochState.EpochExtensions()[1]
		expectedTargetEndTime = rootTargetEndTime + uint64(float64(secondExtension.FinalView-epoch1Setup.FinalView)*targetViewDuration)
		afterSecondExtensionEpoch, err := block2snap.Epochs().Current()
		require.NoError(t, err)
		afterSecondExtensionTargetEndTime := afterSecondExtensionEpoch.TargetEndTime()
		require.Equal(t, expectedTargetEndTime, afterSecondExtensionTargetEndTime)
	})
}

// TestEpochTargetDuration ensurers that the target duration of an epoch is correctly calculated depending on if the epoch is extended or not.
// When we haven't added an extension yet, then the TargetDuration is simply the value from the epoch setup event.
// Otherwise, the TargetDuration is calculated by adding the duration of the extension to the TargetDuration of epoch setup.
// We assume we keep the same view duration for all views in the epoch and all extensions.
func TestEpochTargetDuration(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		rootResult, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		epoch1Setup := rootResult.ServiceEvents[0].Event.(*flow.EpochSetup)
		currentEpoch, err := rootSnapshot.Epochs().Current()
		require.NoError(t, err)
		rootTargetDuration := currentEpoch.TargetDuration()
		require.Equal(t, epoch1Setup.TargetDuration, rootTargetDuration)

		expectedStateIdCalculator := calculateExpectedStateId(t, mutableState)

		// add a block that will trigger EFM and add an epoch extension since the view of the epoch exceeds the safety threshold
		block1View := epoch1Setup.FinalView
		block1 := unittest.BlockFixture(
			unittest.Block.WithParent(head.ID(), head.View, head.Height),
			unittest.Block.WithView(block1View),
			unittest.Block.WithPayload(
				flow.Payload{
					ProtocolStateID: expectedStateIdCalculator(head.ID(), block1View, nil),
				}),
		)
		unittest.InsertAndFinalize(t, state, block1)

		assertEpochFallbackTriggered(t, state.Final(), true)
		assertInPhase(t, state.Final(), flow.EpochPhaseFallback)

		epochState, err := state.Final().EpochProtocolState()
		require.NoError(t, err)
		firstExtension := epochState.EpochExtensions()[0]
		targetViewDuration := float64(epoch1Setup.TargetDuration) / float64(epoch1Setup.FinalView-epoch1Setup.FirstView+1)
		afterFirstExtensionEpoch, err := state.Final().Epochs().Current()
		require.NoError(t, err)
		afterFirstExtensionTargetDuration := afterFirstExtensionEpoch.TargetDuration()
		expectedTargetDuration := rootTargetDuration + uint64(float64(firstExtension.FinalView-firstExtension.FirstView+1)*targetViewDuration)
		require.Equal(t, expectedTargetDuration, afterFirstExtensionTargetDuration)

		// add a second block that exceeds the safety threshold and triggers another epoch extension
		block2View := firstExtension.FinalView
		block2 := unittest.BlockFixture(
			unittest.Block.WithParent(block1.ID(), block1.View, block1.Height),
			unittest.Block.WithView(block2View),
			unittest.Block.WithPayload(
				flow.Payload{
					ProtocolStateID: expectedStateIdCalculator(block1.ID(), block2View, nil),
				}),
		)
		unittest.InsertAndFinalize(t, state, block2)

		epochState, err = state.Final().EpochProtocolState()
		require.NoError(t, err)
		secondExtension := epochState.EpochExtensions()[1]
		afterSecondExtensionEpoch, err := state.Final().Epochs().Current()
		require.NoError(t, err)
		afterSecondExtensionTargetDuration := afterSecondExtensionEpoch.TargetDuration()
		expectedTargetDuration = rootTargetDuration + uint64(float64(secondExtension.FinalView-epoch1Setup.FinalView)*targetViewDuration)
		require.Equal(t, expectedTargetDuration, afterSecondExtensionTargetDuration)
	})
}

func TestExtendInvalidSealsInBlock(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		db := pebbleimpl.ToDB(pdb)
		all := store.InitAll(metrics, db)

		// create a event consumer to test epoch transition events
		distributor := events.NewDistributor()
		consumer := mockprotocol.NewConsumer(t)
		distributor.AddConsumer(consumer)
		consumer.On("BlockProcessable", mock.Anything, mock.Anything)

		rootSnapshot := unittest.RootSnapshotFixture(participants)
		rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)

		state, err := protocol.Bootstrap(
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
		require.NoError(t, err)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)

		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block2 := unittest.BlockWithParentAndPayload(
			block1.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(block1Receipt),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
		)

		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block3 := unittest.BlockWithParentAndPayload(
			block2.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{block1Seal},
				ProtocolStateID: rootProtocolStateID,
			},
		)

		sealValidator := mockmodule.NewSealValidator(t)
		sealValidator.On("Validate", mock.Anything).
			Return(func(candidate *flow.Block) *flow.Seal {
				if candidate.ID() == block3.ID() {
					return nil
				}
				seal, _ := all.Seals.HighestInFork(candidate.ParentID)
				return seal
			}, func(candidate *flow.Block) error {
				if candidate.ID() == block3.ID() {
					return engine.NewInvalidInputErrorf("")
				}
				_, err := all.Seals.HighestInFork(candidate.ParentID)
				return err
			}).
			Times(3)

		fullState, err := protocol.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			util.MockBlockTimer(),
			util.MockReceiptValidator(),
			sealValidator,
		)
		require.NoError(t, err)

		err = fullState.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)
		err = fullState.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.NoError(t, err)
		err = fullState.Extend(context.Background(), unittest.ProposalFromBlock(block3))
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err))
	})
}

func TestHeaderExtendValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		extend := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)

		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(extend))
		require.NoError(t, err)

		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit)
	})
}

func TestHeaderExtendMissingParent(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
		extend := unittest.BlockFixture(
			unittest.Block.WithHeight(2),
			unittest.Block.WithParentView(1),
			unittest.Block.WithView(2),
		)

		err := state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(extend))
		require.Error(t, err)
		require.False(t, st.IsInvalidExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), extend.ID(), &sealID)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestHeaderExtendHeightTooSmall(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)

		// create another block that points to the previous block `extend` as parent
		// but has _same_ height as parent. This violates the condition that a child's
		// height must increment the parent's height by one, i.e. it should be rejected
		// by the follower right away
		block2 := unittest.BlockWithParentFixture(block1.ToHeader())
		block2.Height = block1.Height

		err = state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block1, block2))
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block2))
		require.False(t, st.IsInvalidExtensionError(err))

		// verify seal not indexed
		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), block2.ID(), &sealID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestHeaderExtendHeightTooLarge(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentAndPayload(
			head,
			*flow.NewEmptyPayload(),
		)
		// set an invalid height
		block.Height = head.Height + 2

		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
		require.False(t, st.IsInvalidExtensionError(err))
	})
}

// TestExtendBlockProcessable tests that BlockProcessable is called correctly and doesn't produce duplicates of same notifications
// when extending blocks with and without certifying QCs.
func TestExtendBlockProcessable(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	consumer := mockprotocol.NewConsumer(t)
	util.RunWithFullProtocolStateAndConsumer(t, rootSnapshot, consumer, func(db storage.DB, state *protocol.ParticipantState) {
		block := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		child := unittest.BlockWithParentProtocolState(block)
		grandChild := unittest.BlockWithParentProtocolState(child)

		// extend block using certifying QC, expect that BlockProcessable will be emitted once
		consumer.On("BlockProcessable", block.ToHeader(), child.ParentQC()).Once()
		err := state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block, child))
		require.NoError(t, err)

		// extend block without certifying QC, expect that BlockProcessable won't be called
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(child))
		require.NoError(t, err)
		consumer.AssertNumberOfCalls(t, "BlockProcessable", 1)

		// extend block using certifying QC, expect that BlockProcessable will be emitted twice.
		// One for parent block and second for current block.
		certifiedGrandchild := unittest.NewCertifiedBlock(grandChild)
		consumer.On("BlockProcessable", child.ToHeader(), grandChild.ParentQC()).Once()
		consumer.On("BlockProcessable", grandChild.ToHeader(), certifiedGrandchild.CertifyingQC).Once()
		err = state.ExtendCertified(context.Background(), certifiedGrandchild)
		require.NoError(t, err)
	})
}

// TestFollowerHeaderExtendBlockNotConnected tests adding an orphaned block to the follower state.
// Specifically, we add 2 blocks, where:
// first block is added and then finalized;
// second block is a sibling to the finalized block
// The Follower should accept this block since tracking of orphan blocks is implemented by another component.
func TestFollowerHeaderExtendBlockNotConnected(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// In this test, we create two conflicting forks. To prevent accidentally creating byzantine scenarios, where
		// multiple blocks have the same view, we keep track of used views and ensure that each new block has a unique view.
		usedViews := make(map[uint64]struct{})
		usedViews[head.View] = struct{}{}

		block1 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block1))
		require.NoError(t, err)

		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		block2 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block2))
		require.NoError(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), block2.ID(), &sealID)
		require.NoError(t, err)
	})
}

// TestParticipantHeaderExtendBlockNotConnected tests adding an orphaned block to the consensus participant state.
// Specifically, we add 2 blocks, where:
// first block is added and then finalized;
// second block is a sibling to the finalized block
// The Participant should reject this block as an outdated chain extension
func TestParticipantHeaderExtendBlockNotConnected(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// In this test, we create two conflicting forks. To prevent accidentally creating byzantine scenarios, where
		// multiple blocks have the same view, we keep track of used views and ensure that each new block has a unique view.
		usedViews := make(map[uint64]struct{})
		usedViews[head.View] = struct{}{}

		block1 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block1))
		require.NoError(t, err)

		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		block2 := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			usedViews,
		)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block2))
		require.True(t, st.IsOutdatedExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = operation.LookupLatestSealAtBlock(db.Reader(), block2.ID(), &sealID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestHeaderExtendHighestSeal(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {

		// create block2 and block3
		block2 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)

		block3 := unittest.BlockWithParentProtocolState(block2)

		err := state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block2, block3))
		require.NoError(t, err)

		// create receipts and seals for block2 and block3
		receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
		receipt3, seal3 := unittest.ReceiptAndSealForBlock(block3)

		// include the seals in block4
		block4 := unittest.BlockWithParentAndPayload(
			block3.ToHeader(),
			// include receipts and results
			unittest.PayloadFixture(
				unittest.WithReceipts(receipt3, receipt2),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
		)

		// include the seals in block4
		block5 := unittest.BlockWithParentAndPayload(
			block4.ToHeader(),
			// placing seals in the reversed order to test
			// Extend will pick the highest sealed block
			unittest.PayloadFixture(
				unittest.WithSeals(seal3, seal2),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
		)

		err = state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block3, block4))
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block4, block5))
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block5))
		require.NoError(t, err)

		finalCommit, err := state.AtBlockID(block5.ID()).Commit()
		require.NoError(t, err)
		require.Equal(t, seal3.FinalState, finalCommit)
	})
}

// TestExtendCertifiedInvalidQC checks if ExtendCertified performs a sanity check of certifying QC.
func TestExtendCertifiedInvalidQC(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		// create child block
		block := unittest.BlockWithParentAndPayload(
			head,
			*flow.NewEmptyPayload(),
		)

		t.Run("qc-invalid-view", func(t *testing.T) {
			certified := unittest.NewCertifiedBlock(block)
			certified.CertifyingQC.View++ // invalidate block view
			err = state.ExtendCertified(context.Background(), certified)
			require.Error(t, err)
			require.False(t, st.IsOutdatedExtensionError(err))
		})
		t.Run("qc-invalid-block-id", func(t *testing.T) {
			certified := unittest.NewCertifiedBlock(block)
			certified.CertifyingQC.BlockID = unittest.IdentifierFixture() // invalidate blockID
			err = state.ExtendCertified(context.Background(), certified)
			require.Error(t, err)
			require.False(t, st.IsOutdatedExtensionError(err))
		})
	})
}

// TestExtendInvalidGuarantee checks if Extend method will reject invalid blocks that contain
// guarantees with invalid guarantors
func TestExtendInvalidGuarantee(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		// create a valid block
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		cluster, err := unittest.SnapshotClusterByIndex(rootSnapshot, 0)
		require.NoError(t, err)

		// prepare for a valid guarantor signer indices to be used in the valid block
		all := cluster.Members().NodeIDs()
		validSignerIndices, err := signature.EncodeSignersToIndices(all, all)
		require.NoError(t, err)

		// In this test, we create two conflicting forks. To prevent accidentally creating byzantine scenarios, where
		// multiple blocks have the same view, we keep track of used views and ensure that each new block has a unique view.
		usedViews := make(map[uint64]struct{})
		usedViews[head.View] = struct{}{}

		payload := flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					ClusterChainID:   cluster.ChainID(),
					ReferenceBlockID: head.ID(),
					SignerIndices:    validSignerIndices,
				},
			},
			ProtocolStateID: rootProtocolStateID,
		}

		// now the valid block has a guarantee in the payload with valid signer indices.
		block := unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			payload,
			usedViews,
		)

		// check Extend should accept this valid block
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.NoError(t, err)

		// now the guarantee has invalid signer indices: the checksum should have 4 bytes, but it only has 1
		payload.Guarantees[0].SignerIndices = []byte{byte(1)}

		// create new block that has invalid collection guarantee
		block = unittest.BlockWithParentAndPayloadAndUniqueView(
			head,
			payload,
			usedViews,
		)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrInvalidChecksum)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// now the guarantee has invalid signer indices: the checksum should have 4 bytes, but it only has 1
		checksumMismatch := make([]byte, len(validSignerIndices))
		copy(checksumMismatch, validSignerIndices)
		checksumMismatch[0] = byte(1)
		if checksumMismatch[0] == validSignerIndices[0] {
			checksumMismatch[0] = byte(2)
		}
		payload.Guarantees[0].SignerIndices = checksumMismatch
		block, err = flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: block.HeaderBody,
				Payload:    payload,
			},
		)
		require.NoError(t, err)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrInvalidChecksum)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// let's test even if the checksum is correct, but signer indices is still wrong because the tailing are not 0,
		// then the block should still be rejected.
		wrongTailing := make([]byte, len(validSignerIndices))
		copy(wrongTailing, validSignerIndices)
		wrongTailing[len(wrongTailing)-1] = byte(255)

		payload.Guarantees[0].SignerIndices = wrongTailing
		block, err = flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: block.HeaderBody,
				Payload:    payload,
			},
		)
		require.NoError(t, err)
		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// test imcompatible bit vector length
		wrongbitVectorLength := validSignerIndices[0 : len(validSignerIndices)-1]
		payload.Guarantees[0].SignerIndices = wrongbitVectorLength
		block, err = flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: block.HeaderBody,
				Payload:    payload,
			},
		)
		require.NoError(t, err)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// revert back to good value
		payload.Guarantees[0].SignerIndices = validSignerIndices

		// test the ReferenceBlockID is not found
		payload.Guarantees[0].ReferenceBlockID = flow.ZeroID
		block, err = flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: block.HeaderBody,
				Payload:    payload,
			},
		)
		require.NoError(t, err)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// revert back to good value
		payload.Guarantees[0].ReferenceBlockID = head.ID()

		// TODO: test the guarantee has bad reference block ID that would return protocol.ErrNextEpochNotCommitted
		// this case is not easy to create, since the test case has no such block yet.
		// we need to refactor the ParticipantState to add a guaranteeValidator, so that we can mock it and
		// return the protocol.ErrNextEpochNotCommitted for testing

		// test the guarantee has wrong chain ID, and should return ErrClusterNotFound
		payload.Guarantees[0].ClusterChainID = flow.ChainID("some_bad_chain_ID")
		block, err = flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: block.HeaderBody,
				Payload:    payload,
			},
		)
		require.NoError(t, err)

		err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
		require.Error(t, err)
		require.ErrorIs(t, err, realprotocol.ErrClusterNotFound)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

// If block B is finalized and contains a seal for block A, then A is the last sealed block
func TestSealed(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// block 1 will be sealed
		block1 := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

		// block 2 contains receipt for block 1
		block2 := unittest.BlockWithParentAndPayload(
			block1.ToHeader(),
			unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			),
		)

		err = state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block1, block2))
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		// block 3 contains seal for block 1
		block3 := unittest.BlockWithParentAndPayload(
			block2.ToHeader(),
			flow.Payload{
				Seals:           []*flow.Seal{seal1},
				ProtocolStateID: rootProtocolStateID,
			},
		)

		err = state.ExtendCertified(context.Background(), unittest.CertifiedByChild(block2, block3))
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block2.ID())
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block3))
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block3.ID())
		require.NoError(t, err)

		sealed, err := state.Sealed().Head()
		require.NoError(t, err)
		require.Equal(t, block1.ID(), sealed.ID())
	})
}

// Test that when adding a block to database, there are only two cases at any point of time:
// 1) neither the block header, nor the payload index exist in database
// 2) both the block header and the payload index can be found in database
// A non atomic bug would be: header is found in DB, but payload index is not found
func TestCacheAtomicity(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolStateAndHeaders(t, rootSnapshot,
		func(db storage.DB, state *protocol.FollowerState, headers storage.Headers, index storage.Index) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)

			block := unittest.BlockWithParentAndPayload(
				head,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			blockID := block.ID()

			// check 100 times to see if either 1) or 2) satisfies
			var wg sync.WaitGroup
			wg.Add(1)
			go func(blockID flow.Identifier) {
				for range 100 {
					_, err := headers.ByBlockID(blockID)
					if errors.Is(err, storage.ErrNotFound) {
						continue
					}
					require.NoError(t, err)

					_, err = index.ByBlockID(blockID)
					require.NoError(t, err, "found block ID, but index is missing, DB updates is non-atomic")
				}
				wg.Done()
			}(blockID)

			// storing the block to database, which supposed to be atomic updates to headers and index,
			// both to badger database and the cache.
			err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
			require.NoError(t, err)
			wg.Wait()
		})
}

// TestHeaderInvalidTimestamp tests that extending header with invalid timestamp results in sentinel error
func TestHeaderInvalidTimestamp(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		db := pebbleimpl.ToDB(pdb)
		all := store.InitAll(metrics, db)

		// create a event consumer to test epoch transition events
		distributor := events.NewDistributor()
		consumer := mockprotocol.NewConsumer(t)
		distributor.AddConsumer(consumer)

		block, result, seal := unittest.BootstrapFixture(participants)
		qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(block.ID()))
		rootSnapshot, err := unittest.SnapshotFromBootstrapState(block, result, seal, qc)
		require.NoError(t, err)

		state, err := protocol.Bootstrap(
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
		require.NoError(t, err)

		blockTimer := &mockprotocol.BlockTimer{}
		blockTimer.On("Validate", mock.Anything, mock.Anything).Return(realprotocol.NewInvalidBlockTimestamp(""))

		fullState, err := protocol.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			blockTimer,
			util.MockReceiptValidator(),
			util.MockSealValidator(all.Seals),
		)
		require.NoError(t, err)

		extend := unittest.BlockWithParentFixture(block.ToHeader())
		extend.Payload.Guarantees = nil

		err = fullState.Extend(context.Background(), unittest.ProposalFromBlock(extend))
		assert.Error(t, err, "a proposal with invalid timestamp has to be rejected")
		assert.True(t, st.IsInvalidExtensionError(err), "if timestamp is invalid it should return invalid block error")
	})
}

// TestProtocolStateIdempotent tests that both participant and follower states correctly process adding same block twice
// where second extend doesn't result in an error and effectively is no-op.
func TestProtocolStateIdempotent(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	t.Run("follower", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.FollowerState) {
			block := unittest.BlockWithParentAndPayload(
				head,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			err := state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
			require.NoError(t, err)

			// same operation should be no-op
			err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
			require.NoError(t, err)
		})
	})
	t.Run("participant", func(t *testing.T) {
		util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
			block := unittest.BlockWithParentAndPayload(
				head,
				unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
			)
			err := state.Extend(context.Background(), unittest.ProposalFromBlock(block))
			require.NoError(t, err)

			// same operation should be no-op
			err = state.Extend(context.Background(), unittest.ProposalFromBlock(block))
			require.NoError(t, err)

			err = state.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
			require.NoError(t, err)
		})
	})
}

// assertEpochFallbackTriggered tests that the given `stateSnapshot` has the `EpochFallbackTriggered` flag to the `expected` value.
func assertEpochFallbackTriggered(t *testing.T, stateSnapshot realprotocol.Snapshot, expected bool) {
	epochState, err := stateSnapshot.EpochProtocolState()
	require.NoError(t, err)
	assert.Equal(t, expected, epochState.EpochFallbackTriggered())
}

// assertInPhase tests that the input snapshot is in the expected epoch phase.
func assertInPhase(t *testing.T, snap realprotocol.Snapshot, expectedPhase flow.EpochPhase) {
	phase, err := snap.EpochPhase()
	require.NoError(t, err)
	assert.Equal(t, expectedPhase, phase)
}

// mockMetricsForRootSnapshot mocks the given metrics mock object to expect all
// metrics which are set during bootstrapping and building blocks.
func mockMetricsForRootSnapshot(metricsMock *mockmodule.ComplianceMetrics, rootSnapshot *inmem.Snapshot) {
	epochProtocolState := rootSnapshot.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry
	epochSetup := epochProtocolState.CurrentEpochSetup
	metricsMock.On("CurrentEpochCounter", epochSetup.Counter)
	metricsMock.On("CurrentEpochPhase", epochProtocolState.EpochPhase())
	metricsMock.On("CurrentEpochFinalView", epochSetup.FinalView)
	metricsMock.On("CurrentDKGPhaseViews", epochSetup.DKGPhase1FinalView, epochSetup.DKGPhase2FinalView, epochSetup.DKGPhase3FinalView)
	metricsMock.On("BlockSealed", mock.Anything)
	metricsMock.On("BlockFinalized", mock.Anything)
	metricsMock.On("ProtocolStateVersion", mock.Anything)
	metricsMock.On("FinalizedHeight", mock.Anything)
	metricsMock.On("SealedHeight", mock.Anything)
}

func getRootProtocolStateID(t *testing.T, rootSnapshot *inmem.Snapshot) flow.Identifier {
	rootProtocolState, err := rootSnapshot.ProtocolState()
	require.NoError(t, err)
	return rootProtocolState.ID()
}

// calculateExpectedStateId is a utility function which makes easier to get expected protocol state ID after applying service events contained in seals.
func calculateExpectedStateId(t *testing.T, mutableProtocolState realprotocol.MutableProtocolState) func(parentBlockID flow.Identifier, candidateView uint64, candidateSeals []*flow.Seal) flow.Identifier {
	return func(parentBlockID flow.Identifier, candidateView uint64, candidateSeals []*flow.Seal) flow.Identifier {
		expectedStateID, err := mutableProtocolState.EvolveState(deferred.NewDeferredBlockPersist(), parentBlockID, candidateView, candidateSeals)
		require.NoError(t, err)
		return expectedStateID
	}
}

// nextUnusedViewSince is a utility function which:
//   - returns the smallest view number which is greater than the given `view`
//     and NOT contained in the `forbiddenViews` set.
//   - the next unused view number is added to `forbiddenViews` to prevent it from being used again.
func nextUnusedViewSince(view uint64, forbiddenViews map[uint64]struct{}) uint64 {
	next := view + 1
	for {
		if _, exists := forbiddenViews[next]; !exists {
			forbiddenViews[next] = struct{}{}
			return next
		}
		next++
	}
}
