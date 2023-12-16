// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/trace"
	st "github.com/onflow/flow-go/state"
	realprotocol "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	stoerr "github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	storeutil "github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(5, unittest.WithAllRoles())

func TestBootstrapValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *protocol.State) {
		var finalized uint64
		err := db.View(operation.RetrieveFinalizedHeight(&finalized))
		require.NoError(t, err)

		var sealed uint64
		err = db.View(operation.RetrieveSealedHeight(&sealed))
		require.NoError(t, err)

		var genesisID flow.Identifier
		err = db.View(operation.LookupBlockHeight(0, &genesisID))
		require.NoError(t, err)

		var header flow.Header
		err = db.View(operation.RetrieveHeader(genesisID, &header))
		require.NoError(t, err)

		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(genesisID, &sealID))
		require.NoError(t, err)

		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)
		err = db.View(operation.RetrieveSeal(sealID, seal))
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
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := storeutil.StorageLayer(t, db)

		distributor := events.NewDistributor()
		consumer := mockprotocol.NewConsumer(t)
		distributor.AddConsumer(consumer)

		block, result, seal := unittest.BootstrapFixture(participants)
		qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(block.ID()))
		rootSnapshot, err := inmem.SnapshotFromBootstrapState(block, result, seal, qc)
		require.NoError(t, err)

		state, err := protocol.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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
		err = fullState.Extend(context.Background(), block1)
		require.NoError(t, err)

		// we should not emit BlockProcessable for the root block
		consumer.AssertNotCalled(t, "BlockProcessable", block.Header)

		t.Run("BlockFinalized event should be emitted when block1 is finalized", func(t *testing.T) {
			consumer.On("BlockFinalized", block1.Header).Once()
			err := fullState.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)
		})

		t.Run("BlockProcessable event should be emitted when any child of block1 is inserted", func(t *testing.T) {
			block2 := unittest.BlockWithParentProtocolState(block1)
			consumer.On("BlockProcessable", block1.Header, mock.Anything).Once()
			err := fullState.Extend(context.Background(), block2)
			require.NoError(t, err)
		})
	})
}

func TestSealedIndex(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		rootHeader, err := rootSnapshot.Head()
		require.NoError(t, err)

		// build a chain:
		// G <- B1 <- B2 (resultB1) <- B3 <- B4 (resultB2, resultB3) <- B5 (sealB1) <- B6 (sealB2, sealB3) <- B7
		// test that when B4 is finalized, can only find seal for G
		// 					 when B5 is finalized, can find seal for B1
		//					 when B7 is finalized, can find seals for B2, B3

		// block 1
		b1 := unittest.BlockWithParentFixture(rootHeader)
		b1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), b1)
		require.NoError(t, err)

		// block 2(result B1)
		b1Receipt := unittest.ReceiptForBlockFixture(b1)
		b2 := unittest.BlockWithParentFixture(b1.Header)
		b2.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(b1Receipt),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		err = state.Extend(context.Background(), b2)
		require.NoError(t, err)

		// block 3
		b3 := unittest.BlockWithParentProtocolState(b2)
		err = state.Extend(context.Background(), b3)
		require.NoError(t, err)

		// block 4 (resultB2, resultB3)
		b2Receipt := unittest.ReceiptForBlockFixture(b2)
		b3Receipt := unittest.ReceiptForBlockFixture(b3)
		b4 := unittest.BlockWithParentFixture(b3.Header)
		b4.SetPayload(flow.Payload{
			Receipts:        []*flow.ExecutionReceiptMeta{b2Receipt.Meta(), b3Receipt.Meta()},
			Results:         []*flow.ExecutionResult{&b2Receipt.ExecutionResult, &b3Receipt.ExecutionResult},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), b4)
		require.NoError(t, err)

		// block 5 (sealB1)
		b1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b1Receipt.ExecutionResult))
		b5 := unittest.BlockWithParentFixture(b4.Header)
		b5.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{b1Seal},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), b5)
		require.NoError(t, err)

		// block 6 (sealB2, sealB3)
		b2Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b2Receipt.ExecutionResult))
		b3Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b3Receipt.ExecutionResult))
		b6 := unittest.BlockWithParentFixture(b5.Header)
		b6.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{b2Seal, b3Seal},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), b6)
		require.NoError(t, err)

		// block 7
		b7 := unittest.BlockWithParentProtocolState(b6)
		err = state.Extend(context.Background(), b7)
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
		seals := bstorage.NewSeals(metrics, db)

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
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		rootHeader, err := rootSnapshot.Head()
		require.NoError(t, err)

		// build a chain:
		// G <- B1 <- B2 (resultB1(vb1)) <- B3 <- B4 (resultB2(vb2), resultB3(vb3)) <- B5 (sealB1) <- B6 (sealB2, sealB3)
		// up until and including finalization of B5 there should be no VBs indexed
		//    when B5 is finalized, index VB1
		//    when B6 is finalized, we can index VB2 and VB3, but (only) the last one should be indexed by seal height

		// block 1
		b1 := unittest.BlockWithParentFixture(rootHeader)
		b1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), b1)
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
		b2 := unittest.BlockWithParentFixture(b1.Header)
		b2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(b1Receipt),
			unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), b2)
		require.NoError(t, err)

		// block 3
		b3 := unittest.BlockWithParentProtocolState(b2)
		err = state.Extend(context.Background(), b3)
		require.NoError(t, err)

		// block 4 (resultB2, resultB3)
		b2Receipt := unittest.ReceiptForBlockFixture(b2)
		b2Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{vb2.ServiceEvent()}

		b3Receipt := unittest.ReceiptForBlockFixture(b3)
		b3Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{vb3.ServiceEvent()}

		b4 := unittest.BlockWithParentFixture(b3.Header)
		b4.SetPayload(flow.Payload{
			Receipts:        []*flow.ExecutionReceiptMeta{b2Receipt.Meta(), b3Receipt.Meta()},
			Results:         []*flow.ExecutionResult{&b2Receipt.ExecutionResult, &b3Receipt.ExecutionResult},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), b4)
		require.NoError(t, err)

		// block 5 (sealB1)
		b1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b1Receipt.ExecutionResult))
		b5 := unittest.BlockWithParentFixture(b4.Header)
		b5.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{b1Seal},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), b5)
		require.NoError(t, err)

		// block 6 (sealB2, sealB3)
		b2Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b2Receipt.ExecutionResult))
		b3Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&b3Receipt.ExecutionResult))
		b6 := unittest.BlockWithParentFixture(b5.Header)
		b6.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{b2Seal, b3Seal},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), b6)
		require.NoError(t, err)

		versionBeacons := bstorage.NewVersionBeacons(db)

		// No VB can be found before finalizing anything
		vb, err := versionBeacons.Highest(b6.Header.Height)
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
		vb, err = versionBeacons.Highest(b6.Header.Height)
		require.NoError(t, err)
		require.Nil(t, vb)

		// once B5 is finalized, B1 and VB1 are sealed, hence index should now find it
		err = state.Finalize(context.Background(), b5.ID())
		require.NoError(t, err)

		versionBeacon, err := versionBeacons.Highest(b6.Header.Height)
		require.NoError(t, err)
		require.Equal(t,
			&flow.SealedVersionBeacon{
				VersionBeacon: vb1,
				SealHeight:    b5.Header.Height,
			},
			versionBeacon,
		)

		// finalizing B6 should index events sealed by B6, so VB2 and VB3
		// while we don't expect multiple VBs in one block, we index newest, so last one emitted - VB3
		err = state.Finalize(context.Background(), b6.ID())
		require.NoError(t, err)

		versionBeacon, err = versionBeacons.Highest(b6.Header.Height)
		require.NoError(t, err)
		require.Equal(t,
			&flow.SealedVersionBeacon{
				VersionBeacon: vb3,
				SealHeight:    b6.Header.Height,
			},
			versionBeacon,
		)
	})
}

func TestExtendSealedBoundary(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)
		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "original commit should be root commit")

		// Create a first block on top of the snapshot
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block1)
		require.NoError(t, err)

		// Add a second block containing a receipt committing to the first block
		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts:        []*flow.ExecutionReceiptMeta{block1Receipt.Meta()},
			Results:         []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), block2)
		require.NoError(t, err)

		// Add a third block containing a seal for the first block
		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{block1Seal},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), block3)
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

func TestExtendMissingParent(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 2
		extend.Header.View = 2
		extend.Header.ParentID = unittest.BlockFixture().ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err := state.Extend(context.Background(), &extend)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(extend.ID(), &sealID))
		require.Error(t, err)
		require.ErrorIs(t, err, stoerr.ErrNotFound)
	})
}

func TestExtendHeightTooSmall(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = head.ID()
		extend.Header.ParentView = head.View

		err = state.Extend(context.Background(), &extend)
		require.NoError(t, err)

		// create another block with the same height and view, that is coming after
		extend.Header.ParentID = extend.Header.ID()
		extend.Header.Height = 1
		extend.Header.View = 2

		err = state.Extend(context.Background(), &extend)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(extend.ID(), &sealID))
		require.Error(t, err)
		require.ErrorIs(t, err, stoerr.ErrNotFound)
	})
}

func TestExtendHeightTooLarge(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())
		// set an invalid height
		block.Header.Height = head.Height + 2

		err = state.Extend(context.Background(), block)
		require.Error(t, err)
	})
}

// TestExtendInconsistentParentView tests if mutableState rejects block with invalid ParentView. ParentView must be consistent
// with view of block referred by ParentID.
func TestExtendInconsistentParentView(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())
		// set an invalid parent view
		block.Header.ParentView++

		err = state.Extend(context.Background(), block)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err))
	})
}

func TestExtendBlockNotConnected(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// add 2 blocks, the second finalizing/sealing the state of the first
		extend := unittest.BlockWithParentFixture(head)
		extend.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))

		err = state.Extend(context.Background(), extend)
		require.NoError(t, err)

		err = state.Finalize(context.Background(), extend.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		extend.Header.Timestamp = extend.Header.Timestamp.Add(time.Second)
		extend.Header.ParentID = head.ID()

		err = state.Extend(context.Background(), extend)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(extend.ID(), &sealID))
		require.Error(t, err)
		require.ErrorIs(t, err, stoerr.ErrNotFound)
	})
}

func TestExtendInvalidChainID(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())
		// use an invalid chain ID
		block.Header.ChainID = head.ChainID + "-invalid"

		err = state.Extend(context.Background(), block)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsNotSorted(t *testing.T) {
	// TODO: this test needs to be updated:
	// We don't require the receipts to be sorted by height anymore
	// We could require an "parent first" ordering, which is less strict than
	// a full ordering by height
	unittest.SkipUnless(t, unittest.TEST_TODO, "needs update")

	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		// create block2 and block3
		block2 := unittest.BlockWithParentFixture(head)
		block2.Payload.Guarantees = nil
		block2.Header.PayloadHash = block2.Payload.Hash()
		err := state.Extend(context.Background(), block2)
		require.NoError(t, err)

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.Payload.Guarantees = nil
		block3.Header.PayloadHash = block3.Payload.Hash()
		err = state.Extend(context.Background(), block3)
		require.NoError(t, err)

		receiptA := unittest.ReceiptForBlockFixture(block3)
		receiptB := unittest.ReceiptForBlockFixture(block2)

		// insert a block with payload receipts not sorted by block height.
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.Payload = &flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta(), receiptB.Meta()},
			Results:  []*flow.ExecutionResult{&receiptA.ExecutionResult, &receiptB.ExecutionResult},
		}
		block4.Header.PayloadHash = block4.Payload.Hash()
		err = state.Extend(context.Background(), block4)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsInvalid(t *testing.T) {
	validator := mockmodule.NewReceiptValidator(t)
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolStateAndValidator(t, rootSnapshot, validator, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		validator.On("ValidatePayload", mock.Anything).Return(nil).Once()

		// create block2 and block3
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block2)
		require.NoError(t, err)

		// Add a receipt for block 2
		receipt := unittest.ExecutionReceiptFixture()

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts:        []*flow.ExecutionReceiptMeta{receipt.Meta()},
			Results:         []*flow.ExecutionResult{&receipt.ExecutionResult},
			ProtocolStateID: rootProtocolStateID,
		})

		// force the receipt validator to refuse this payload
		validator.On("ValidatePayload", block3).Return(engine.NewInvalidInputError("")).Once()

		err = state.Extend(context.Background(), block3)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block2)
		require.NoError(t, err)

		block3 := unittest.BlockWithParentProtocolState(block2)
		err = state.Extend(context.Background(), block3)
		require.NoError(t, err)

		block4 := unittest.BlockWithParentProtocolState(block3)
		err = state.Extend(context.Background(), block4)
		require.NoError(t, err)

		receipt3a := unittest.ReceiptForBlockFixture(block3)
		receipt3b := unittest.ReceiptForBlockFixture(block3)
		receipt3c := unittest.ReceiptForBlockFixture(block4)

		block5 := unittest.BlockWithParentFixture(block4.Header)
		block5.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceiptMeta{
				receipt3a.Meta(),
				receipt3b.Meta(),
				receipt3c.Meta(),
			},
			Results: []*flow.ExecutionResult{
				&receipt3a.ExecutionResult,
				&receipt3b.ExecutionResult,
				&receipt3c.ExecutionResult,
			},
			ProtocolStateID: rootProtocolStateID,
		})
		err = state.Extend(context.Background(), block5)
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
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// set up state and mock ComplianceMetrics object
		metrics := mockmodule.NewComplianceMetrics(t)
		metrics.On("BlockSealed", mock.Anything)
		metrics.On("SealedHeight", mock.Anything)
		metrics.On("FinalizedHeight", mock.Anything)
		metrics.On("BlockFinalized", mock.Anything)

		// expect epoch metric calls on bootstrap
		initialCurrentEpoch := rootSnapshot.Epochs().Current()
		counter, err := initialCurrentEpoch.Counter()
		require.NoError(t, err)
		finalView, err := initialCurrentEpoch.FinalView()
		require.NoError(t, err)
		initialPhase, err := rootSnapshot.Phase()
		require.NoError(t, err)
		metrics.On("CurrentEpochCounter", counter).Once()
		metrics.On("CurrentEpochPhase", initialPhase).Once()

		metrics.On("CurrentEpochFinalView", finalView).Once()

		dkgPhase1FinalView, dkgPhase2FinalView, dkgPhase3FinalView, err := realprotocol.DKGPhaseViews(initialCurrentEpoch)
		require.NoError(t, err)
		metrics.On("CurrentDKGPhase1FinalView", dkgPhase1FinalView).Once()
		metrics.On("CurrentDKGPhase2FinalView", dkgPhase2FinalView).Once()
		metrics.On("CurrentDKGPhase3FinalView", dkgPhase3FinalView).Once()

		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := storeutil.StorageLayer(t, db)
		protoState, err := protocol.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

		mutableProtocolState := protocol_state.NewMutableProtocolState(
			all.ProtocolState,
			state.Params(),
			all.Headers,
			all.Results,
			all.Setups,
			all.EpochCommits,
		)
		calculateExpectedStateId := calculateExpectedStateId(t, mutableProtocolState)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// we should begin the epoch in the staking phase
		phase, err := state.AtBlockID(head.ID()).Phase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseStaking, phase)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block1)
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Sort(order.Canonical[flow.Identity]).ToSkeleton()

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
			unittest.WithFirstView(epoch1FinalView+1),
		)

		// create a receipt for block 1 containing the EpochSetup event
		receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
		receipt1.ExecutionResult.ServiceEvents = []flow.ServiceEvent{epoch2Setup.ServiceEvent()}
		seal1.ResultID = receipt1.ExecutionResult.ID()

		// add a second block with the receipt for block 1
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1), unittest.WithProtocolStateID(block1.Payload.ProtocolStateID)))

		err = state.Extend(context.Background(), block2)
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block2.ID())
		require.NoError(t, err)

		// block 3 contains the seal for block 1
		seals := []*flow.Seal{seal1}
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals:           seals,
			ProtocolStateID: calculateExpectedStateId(block3.Header, seals),
		})

		// insert the block sealing the EpochSetup event
		err = state.Extend(context.Background(), block3)
		require.NoError(t, err)

		// now that the setup event has been emitted, we should be in the setup phase
		phase, err = state.AtBlockID(block3.ID()).Phase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseSetup, phase)

		// we should NOT be able to query epoch 2 wrt blocks before 3
		for _, blockID := range []flow.Identifier{block1.ID(), block2.ID()} {
			_, err = state.AtBlockID(blockID).Epochs().Next().InitialIdentities()
			require.Error(t, err)
			_, err = state.AtBlockID(blockID).Epochs().Next().Clustering()
			require.Error(t, err)
		}

		// we should be able to query epoch 2 wrt block 3
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().InitialIdentities()
		assert.NoError(t, err)
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().Clustering()
		assert.NoError(t, err)

		// only setup event is finalized, not commit, so shouldn't be able to get certain info
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().DKG()
		require.Error(t, err)

		// insert B4
		block4 := unittest.BlockWithParentProtocolState(block3)
		err = state.Extend(context.Background(), block4)
		require.NoError(t, err)

		consumer.On("EpochSetupPhaseStarted", epoch2Setup.Counter-1, block3.Header).Once()
		metrics.On("CurrentEpochPhase", flow.EpochPhaseSetup).Once()
		// finalize block 3, so we can finalize subsequent blocks
		// ensure an epoch phase transition when we finalize block 3
		err = state.Finalize(context.Background(), block3.ID())
		require.NoError(t, err)
		consumer.AssertCalled(t, "EpochSetupPhaseStarted", epoch2Setup.Counter-1, block3.Header)
		metrics.AssertCalled(t, "CurrentEpochPhase", flow.EpochPhaseSetup)

		// now that the setup event has been emitted, we should be in the setup phase
		phase, err = state.AtBlockID(block3.ID()).Phase()
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
		receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
		receipt2.ExecutionResult.ServiceEvents = []flow.ServiceEvent{epoch2Commit.ServiceEvent()}
		seal2.ResultID = receipt2.ExecutionResult.ID()

		// block 5 contains the receipt for block 2
		block5 := unittest.BlockWithParentFixture(block4.Header)
		block5.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt2),
			unittest.WithProtocolStateID(block4.Payload.ProtocolStateID)))

		err = state.Extend(context.Background(), block5)
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block5.ID())
		require.NoError(t, err)

		// block 6 contains the seal for block 2
		seals = []*flow.Seal{seal2}
		block6 := unittest.BlockWithParentFixture(block5.Header)
		block6.SetPayload(flow.Payload{
			Seals:           seals,
			ProtocolStateID: calculateExpectedStateId(block6.Header, seals),
		})

		err = state.Extend(context.Background(), block6)
		require.NoError(t, err)

		// we should NOT be able to query epoch 2 commit info wrt blocks before 6
		for _, blockID := range []flow.Identifier{block4.ID(), block5.ID()} {
			_, err = state.AtBlockID(blockID).Epochs().Next().DKG()
			require.Error(t, err)
		}

		// now epoch 2 is fully ready, we can query anything we want about it wrt block 6 (or later)
		_, err = state.AtBlockID(block6.ID()).Epochs().Next().InitialIdentities()
		require.NoError(t, err)
		_, err = state.AtBlockID(block6.ID()).Epochs().Next().Clustering()
		require.NoError(t, err)
		_, err = state.AtBlockID(block6.ID()).Epochs().Next().DKG()
		assert.NoError(t, err)

		// now that the commit event has been emitted, we should be in the committed phase
		phase, err = state.AtBlockID(block6.ID()).Phase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseCommitted, phase)

		// block 7 has the final view of the epoch, insert it, finalized after finalizing block 6
		block7 := unittest.BlockWithParentProtocolState(block6)
		block7.Header.View = epoch1FinalView
		err = state.Extend(context.Background(), block7)
		require.NoError(t, err)

		// expect epoch phase transition once we finalize block 6
		consumer.On("EpochCommittedPhaseStarted", epoch2Setup.Counter-1, block6.Header).Once()
		metrics.On("CurrentEpochPhase", flow.EpochPhaseCommitted).Once()

		err = state.Finalize(context.Background(), block6.ID())
		require.NoError(t, err)

		consumer.AssertCalled(t, "EpochCommittedPhaseStarted", epoch2Setup.Counter-1, block6.Header)
		metrics.AssertCalled(t, "CurrentEpochPhase", flow.EpochPhaseCommitted)

		// we should still be in epoch 1
		epochCounter, err := state.AtBlockID(block4.ID()).Epochs().Current().Counter()
		require.NoError(t, err)
		require.Equal(t, epoch1Setup.Counter, epochCounter)

		err = state.Finalize(context.Background(), block7.ID())
		require.NoError(t, err)

		// we should still be in epoch 1, since epochs are inclusive of final view
		epochCounter, err = state.AtBlockID(block7.ID()).Epochs().Current().Counter()
		require.NoError(t, err)
		require.Equal(t, epoch1Setup.Counter, epochCounter)

		// block 8 has a view > final view of epoch 1, it will be considered the first block of epoch 2
		block8 := unittest.BlockWithParentFixture(block7.Header)
		// we should handle views that aren't exactly the first valid view of the epoch
		block8.Header.View = epoch1FinalView + uint64(1+rand.Intn(10))
		// need to update root protocol state since we enter new epoch
		block8.SetPayload(
			unittest.PayloadFixture(
				unittest.WithProtocolStateID(calculateExpectedStateId(block8.Header, nil))))

		err = state.Extend(context.Background(), block8)
		require.NoError(t, err)

		// now, at long last, we are in epoch 2
		epochCounter, err = state.AtBlockID(block8.ID()).Epochs().Current().Counter()
		require.NoError(t, err)
		require.Equal(t, epoch2Setup.Counter, epochCounter)

		// we should begin epoch 2 in staking phase
		// how that the commit event has been emitted, we should be in the committed phase
		phase, err = state.AtBlockID(block8.ID()).Phase()
		assert.NoError(t, err)
		require.Equal(t, flow.EpochPhaseStaking, phase)

		// expect epoch transition once we finalize block 9
		consumer.On("EpochTransition", epoch2Setup.Counter, block8.Header).Once()
		metrics.On("EpochTransitionHeight", block8.Header.Height).Once()
		metrics.On("CurrentEpochCounter", epoch2Setup.Counter).Once()
		metrics.On("CurrentEpochPhase", flow.EpochPhaseStaking).Once()
		metrics.On("CurrentEpochFinalView", epoch2Setup.FinalView).Once()
		metrics.On("CurrentDKGPhase1FinalView", epoch2Setup.DKGPhase1FinalView).Once()
		metrics.On("CurrentDKGPhase2FinalView", epoch2Setup.DKGPhase2FinalView).Once()
		metrics.On("CurrentDKGPhase3FinalView", epoch2Setup.DKGPhase3FinalView).Once()

		// before block 9 is finalized, the epoch 1-2 boundary is unknown
		_, err = state.AtBlockID(block8.ID()).Epochs().Current().FinalHeight()
		assert.ErrorIs(t, err, realprotocol.ErrEpochTransitionNotFinalized)
		_, err = state.AtBlockID(block8.ID()).Epochs().Current().FirstHeight()
		assert.ErrorIs(t, err, realprotocol.ErrEpochTransitionNotFinalized)

		err = state.Finalize(context.Background(), block8.ID())
		require.NoError(t, err)

		// once block 8 is finalized, epoch 2 has unambiguously begun - the epoch 1-2 boundary is known
		epoch1FinalHeight, err := state.AtBlockID(block8.ID()).Epochs().Previous().FinalHeight()
		require.NoError(t, err)
		assert.Equal(t, block7.Header.Height, epoch1FinalHeight)
		epoch2FirstHeight, err := state.AtBlockID(block8.ID()).Epochs().Current().FirstHeight()
		require.NoError(t, err)
		assert.Equal(t, block8.Header.Height, epoch2FirstHeight)
	})
}

// we should be able to have conflicting forks with two different instances of
// the same service event for the same epoch
//
//	         /--B1<--B3(R1)<--B5(S1)<--B7
//	ROOT <--+
//	         \--B2<--B4(R2)<--B6(S2)<--B8
func TestExtendConflictingEpochEvents(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		calculateExpectedStateId := calculateExpectedStateId(t, mutableState)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add two conflicting blocks for each service event to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block1)
		require.NoError(t, err)

		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block2)
		require.NoError(t, err)

		rootSetup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// create two conflicting epoch setup events for the next epoch (final view differs)
		nextEpochSetup1 := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+1000),
			unittest.WithFirstView(rootSetup.FinalView+1),
		)
		nextEpochSetup2 := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+2000), // final view differs
			unittest.WithFirstView(rootSetup.FinalView+1),
		)

		// add blocks containing receipts for block1 and block2 (necessary for sealing)
		// block 1 receipt contains nextEpochSetup1
		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block1Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup1.ServiceEvent()}

		// add block 1 receipt to block 3 payload
		block3 := unittest.BlockWithParentFixture(block1.Header)
		block3.SetPayload(flow.Payload{
			Receipts:        []*flow.ExecutionReceiptMeta{block1Receipt.Meta()},
			Results:         []*flow.ExecutionResult{&block1Receipt.ExecutionResult},
			ProtocolStateID: block1.Payload.ProtocolStateID,
		})
		err = state.Extend(context.Background(), block3)
		require.NoError(t, err)

		// block 2 receipt contains nextEpochSetup2
		block2Receipt := unittest.ReceiptForBlockFixture(block2)
		block2Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup2.ServiceEvent()}

		// add block 2 receipt to block 4 payload
		block4 := unittest.BlockWithParentFixture(block2.Header)
		block4.SetPayload(flow.Payload{
			Receipts:        []*flow.ExecutionReceiptMeta{block2Receipt.Meta()},
			Results:         []*flow.ExecutionResult{&block2Receipt.ExecutionResult},
			ProtocolStateID: block1.Payload.ProtocolStateID,
		})
		err = state.Extend(context.Background(), block4)
		require.NoError(t, err)

		// seal for block 1
		seals1 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))}

		// seal for block 2
		seals2 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))}

		// block 5 builds on block 3, contains seal for block 1
		block5 := unittest.BlockWithParentFixture(block3.Header)
		block5.SetPayload(flow.Payload{
			Seals:           seals1,
			ProtocolStateID: calculateExpectedStateId(block5.Header, seals1),
		})
		err = state.Extend(context.Background(), block5)
		require.NoError(t, err)

		// block 6 builds on block 4, contains seal for block 2
		block6 := unittest.BlockWithParentFixture(block4.Header)
		block6.SetPayload(flow.Payload{
			Seals:           seals2,
			ProtocolStateID: calculateExpectedStateId(block6.Header, seals2),
		})
		err = state.Extend(context.Background(), block6)
		require.NoError(t, err)

		// block 7 builds on block 5, contains QC for block 7
		block7 := unittest.BlockWithParentProtocolState(block5)
		err = state.Extend(context.Background(), block7)
		require.NoError(t, err)

		// block 8 builds on block 6, contains QC for block 6
		block8 := unittest.BlockWithParentProtocolState(block6)
		err = state.Extend(context.Background(), block8)
		require.NoError(t, err)

		// should be able query each epoch from the appropriate reference block
		setup1FinalView, err := state.AtBlockID(block7.ID()).Epochs().Next().FinalView()
		assert.NoError(t, err)
		require.Equal(t, nextEpochSetup1.FinalView, setup1FinalView)

		setup2FinalView, err := state.AtBlockID(block8.ID()).Epochs().Next().FinalView()
		assert.NoError(t, err)
		require.Equal(t, nextEpochSetup2.FinalView, setup2FinalView)
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
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		calculateExpectedStateId := calculateExpectedStateId(t, mutableState)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add two conflicting blocks for each service event to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block1)
		require.NoError(t, err)

		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block2)
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
		block3 := unittest.BlockWithParentFixture(block1.Header)
		block3.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(block1Receipt),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		err = state.Extend(context.Background(), block3)
		require.NoError(t, err)

		// block 2 receipt contains nextEpochSetup2
		block2Receipt := unittest.ReceiptForBlockFixture(block2)
		block2Receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{nextEpochSetup.ServiceEvent()}

		// add block 2 receipt to block 4 payload
		block4 := unittest.BlockWithParentFixture(block2.Header)
		block4.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(block2Receipt),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		err = state.Extend(context.Background(), block4)
		require.NoError(t, err)

		// seal for block 1
		seals1 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))}

		// seal for block 2
		seals2 := []*flow.Seal{unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))}

		// block 5 builds on block 3, contains seal for block 1
		block5 := unittest.BlockWithParentFixture(block3.Header)
		block5.SetPayload(flow.Payload{
			Seals:           seals1,
			ProtocolStateID: calculateExpectedStateId(block5.Header, seals1),
		})
		err = state.Extend(context.Background(), block5)
		require.NoError(t, err)

		// block 6 builds on block 4, contains seal for block 2
		block6 := unittest.BlockWithParentFixture(block4.Header)
		block6.SetPayload(flow.Payload{
			Seals:           seals2,
			ProtocolStateID: calculateExpectedStateId(block6.Header, seals2),
		})
		err = state.Extend(context.Background(), block6)
		require.NoError(t, err)

		// block 7 builds on block 5, contains QC for block 7
		block7 := unittest.BlockWithParentProtocolState(block5)
		err = state.Extend(context.Background(), block7)
		require.NoError(t, err)

		// block 8 builds on block 6, contains QC for block 6
		// at this point we are inserting the duplicate EpochSetup, should not error
		block8 := unittest.BlockWithParentProtocolState(block6)
		err = state.Extend(context.Background(), block8)
		require.NoError(t, err)

		// should be able query each epoch from the appropriate reference block
		finalView, err := state.AtBlockID(block7.ID()).Epochs().Next().FinalView()
		assert.NoError(t, err)
		require.Equal(t, nextEpochSetup.FinalView, finalView)

		finalView, err = state.AtBlockID(block8.ID()).Epochs().Next().FinalView()
		assert.NoError(t, err)
		require.Equal(t, nextEpochSetup.FinalView, finalView)
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
	setupState := func(t *testing.T, db *badger.DB, state *protocol.ParticipantState) (
		*flow.Block,
		func(...func(*flow.EpochSetup)) (*flow.EpochSetup, *flow.ExecutionReceipt, *flow.Seal),
	) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		unittest.InsertAndFinalize(t, state, block1)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Sort(order.Canonical[flow.Identity]).ToSkeleton()

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
			receipt, seal := unittest.ReceiptAndSealForBlock(block1)
			receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{setup.ServiceEvent()}
			seal.ResultID = receipt.ExecutionResult.ID()
			return setup, receipt, seal
		}

		return block1, createSetupEvent
	}

	// expect a setup event with wrong counter to trigger EFM without error
	t.Run("wrong counter [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				setup.Counter = rand.Uint64()
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})

	// expect a setup event with wrong final view to trigger EFM without error
	t.Run("invalid final view [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				setup.FinalView = block1.Header.View
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})

	// expect a setup event with empty seed to trigger EFM without error
	t.Run("empty seed [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, createSetup := setupState(t, db, state)

			_, receipt, seal := createSetup(func(setup *flow.EpochSetup) {
				setup.RandomSource = nil
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})

	t.Run("participants not ordered [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
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
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
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
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		unittest.InsertAndFinalize(t, state, block1)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)

		// swap consensus node for a new one for epoch 2
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		epoch2Participants := append(
			participants.Filter(filter.Not[flow.Identity](filter.HasRole[flow.Identity](flow.RoleConsensus))),
			epoch2NewParticipant,
		).Sort(order.Canonical[flow.Identity]).ToSkeleton()

		// factory method to create a valid EpochSetup method w.r.t. the generated state
		createSetup := func(block *flow.Block) (*flow.EpochSetup, *flow.ExecutionReceipt, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)

			receipt, seal := unittest.ReceiptAndSealForBlock(block)
			receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{setup.ServiceEvent()}
			seal.ResultID = receipt.ExecutionResult.ID()
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
			receipt, seal := unittest.ReceiptAndSealForBlock(block)
			receipt.ExecutionResult.ServiceEvents = []flow.ServiceEvent{commit.ServiceEvent()}
			seal.ResultID = receipt.ExecutionResult.ID()
			return commit, receipt, seal
		}

		return block1, createSetup, createCommit
	}

	t.Run("without setup [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			block1, _, createCommit := setupState(t, state)

			_, receipt, seal := createCommit(block1)

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block1, receipt, seal)
			err := state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})

	// expect a commit event with wrong counter to trigger EFM without error
	t.Run("inconsistent counter [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
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
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})

	// expect a commit event with wrong cluster QCs to trigger EFM without error
	t.Run("inconsistent cluster QCs [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
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
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})

	// expect a commit event with wrong dkg participants to trigger EFM without error
	t.Run("inconsistent DKG participants [EFM]", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
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
				// add an extra dkg key
				commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, unittest.KeyFixture(crypto.BLSBLS12381).PublicKey())
			})

			receiptBlock, sealingBlock := unittest.SealBlock(t, state, mutableState, block3, receipt, seal)
			err = state.Finalize(context.Background(), receiptBlock.ID())
			require.NoError(t, err)
			// epoch fallback not triggered before finalization
			assertEpochEmergencyFallbackTriggered(t, state, false)
			err = state.Finalize(context.Background(), sealingBlock.ID())
			require.NoError(t, err)
			// epoch fallback triggered after finalization
			assertEpochEmergencyFallbackTriggered(t, state, true)
		})
	})
}

// if we reach the first block of the next epoch before both setup and commit
// service events are finalized, the chain should halt
//
// ROOT <- B1 <- B2(R1) <- B3(S1) <- B4
func TestExtendEpochTransitionWithoutCommit(t *testing.T) {

	// skipping because this case will now result in emergency epoch continuation kicking in
	unittest.SkipUnless(t, unittest.TEST_TODO, "disabled as the current implementation uses a temporary fallback measure in this case (triggers EFM), rather than returning an error")

	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		result, _, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(context.Background(), block1)
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Sort(order.Canonical[flow.Identity]).ToSkeleton()

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
			unittest.WithFirstView(epoch1FinalView+1),
		)

		receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
		receipt1.ExecutionResult.ServiceEvents = []flow.ServiceEvent{epoch2Setup.ServiceEvent()}

		// add a block containing a receipt for block 1
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1)))
		err = state.Extend(context.Background(), block2)
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block2.ID())
		require.NoError(t, err)

		// block 3 seals block 1
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal1},
		})
		err = state.Extend(context.Background(), block3)
		require.NoError(t, err)

		// block 4 will be the first block for epoch 2
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.Header.View = epoch1Setup.FinalView + 1

		err = state.Extend(context.Background(), block4)
		require.Error(t, err)
	})
}

// TestEmergencyEpochFallback tests that epoch emergency fallback is triggered
// when an epoch fails to be committed before the epoch commitment deadline,
// or when an invalid service event (indicating service account smart contract bug)
// is sealed.
func TestEmergencyEpochFallback(t *testing.T) {

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

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db *badger.DB, state *protocol.ParticipantState, mutableProtocolState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			result, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)
			safetyThreshold := rootSnapshot.Params().EpochCommitSafetyThreshold()
			require.NoError(t, err)

			calculateExpectedStateId := calculateExpectedStateId(t, mutableProtocolState)

			epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch1FinalView := epoch1Setup.FinalView
			epoch1CommitmentDeadline := epoch1FinalView - safetyThreshold

			// finalizing block 1 should trigger EFM
			metricsMock.On("EpochEmergencyFallbackTriggered").Once()
			protoEventsMock.On("EpochEmergencyFallbackTriggered").Once()

			// we begin the epoch in the EpochStaking phase and
			// block 1 will be the first block on or past the epoch commitment deadline
			block1 := unittest.BlockWithParentFixture(head)
			block1.Header.View = epoch1CommitmentDeadline + rand.Uint64()%2
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(calculateExpectedStateId(block1.Header, nil))))
			err = state.Extend(context.Background(), block1)
			require.NoError(t, err)
			assertEpochEmergencyFallbackTriggered(t, state, false) // not triggered before finalization
			err = state.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)
			assertEpochEmergencyFallbackTriggered(t, state, true) // triggered after finalization

			// block 2 will be the first block past the first epoch boundary
			block2 := unittest.BlockWithParentProtocolState(block1)
			block2.Header.View = epoch1FinalView + 1
			err = state.Extend(context.Background(), block2)
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

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			result, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)
			safetyThreshold := rootSnapshot.Params().EpochCommitSafetyThreshold()
			require.NoError(t, err)

			// add a block for the first seal to reference
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			err = state.Extend(context.Background(), block1)
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)

			epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch1FinalView := epoch1Setup.FinalView
			epoch1CommitmentDeadline := epoch1FinalView - safetyThreshold

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(order.Canonical[flow.Identity]).ToSkeleton()

			// create the epoch setup event for the second epoch
			epoch2Setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1FinalView+1000),
				unittest.WithFirstView(epoch1FinalView+1),
			)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			receipt1.ExecutionResult.ServiceEvents = []flow.ServiceEvent{epoch2Setup.ServiceEvent()}
			seal1.ResultID = receipt1.ExecutionResult.ID()

			// add a block containing a receipt for block 1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			err = state.Extend(context.Background(), block2)
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			// block 3 seals block 1 and will be the first block on or past the epoch commitment deadline
			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.Header.View = epoch1CommitmentDeadline + rand.Uint64()%2
			seals := []*flow.Seal{seal1}
			block3.SetPayload(flow.Payload{
				Seals:           seals,
				ProtocolStateID: calculateExpectedStateId(t, mutableState)(block3.Header, seals),
			})
			err = state.Extend(context.Background(), block3)
			require.NoError(t, err)

			// finalizing block 3 should trigger EFM
			metricsMock.On("EpochEmergencyFallbackTriggered").Once()
			protoEventsMock.On("EpochEmergencyFallbackTriggered").Once()

			assertEpochEmergencyFallbackTriggered(t, state, false) // not triggered before finalization
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochEmergencyFallbackTriggered(t, state, true) // triggered after finalization

			// block 4 will be the first block past the first epoch boundary
			block4 := unittest.BlockWithParentProtocolState(block3)
			block4.Header.View = epoch1FinalView + 1
			err = state.Extend(context.Background(), block4)
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

		util.RunWithFullProtocolStateAndMetricsAndConsumer(t, rootSnapshot, metricsMock, protoEventsMock, func(db *badger.DB, state *protocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)
			result, _, err := rootSnapshot.SealedResult()
			require.NoError(t, err)

			// add a block for the first seal to reference
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			err = state.Extend(context.Background(), block1)
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block1.ID())
			require.NoError(t, err)

			epoch1Setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
			epoch1FinalView := epoch1Setup.FinalView

			// add a participant for the next epoch
			epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
			epoch2Participants := append(participants, epoch2NewParticipant).Sort(order.Canonical[flow.Identity]).ToSkeleton()

			// create the epoch setup event for the second epoch
			// this event is invalid because it used a non-contiguous first view
			epoch2Setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1FinalView+1000),
				unittest.WithFirstView(epoch1FinalView+10), // invalid first view
			)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			receipt1.ExecutionResult.ServiceEvents = []flow.ServiceEvent{epoch2Setup.ServiceEvent()}
			seal1.ResultID = receipt1.ExecutionResult.ID()

			// add a block containing a receipt for block 1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			err = state.Extend(context.Background(), block2)
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block2.ID())
			require.NoError(t, err)

			// block 3 is where the service event state change comes into effect
			block3 := unittest.BlockWithParentFixture(block2.Header)
			seals := []*flow.Seal{seal1}
			block3.SetPayload(flow.Payload{
				Seals:           seals,
				ProtocolStateID: calculateExpectedStateId(t, mutableState)(block3.Header, seals),
			})
			err = state.Extend(context.Background(), block3)
			require.NoError(t, err)

			// incorporating the service event should trigger EFM
			metricsMock.On("EpochEmergencyFallbackTriggered").Once()
			protoEventsMock.On("EpochEmergencyFallbackTriggered").Once()

			assertEpochEmergencyFallbackTriggered(t, state, false) // not triggered before finalization
			err = state.Finalize(context.Background(), block3.ID())
			require.NoError(t, err)
			assertEpochEmergencyFallbackTriggered(t, state, true) // triggered after finalization

			// block 5 is the first block past the current epoch boundary
			block4 := unittest.BlockWithParentProtocolState(block3)
			block4.Header.View = epoch1Setup.FinalView + 1
			err = state.Extend(context.Background(), block4)
			require.NoError(t, err)
			err = state.Finalize(context.Background(), block4.ID())
			require.NoError(t, err)

			// since EFM has been triggered, epoch transition metrics should not be updated
			metricsMock.AssertNotCalled(t, "EpochTransition", epoch2Setup.Counter, mock.Anything)
			metricsMock.AssertNotCalled(t, "CurrentEpochCounter", epoch2Setup.Counter)
		})
	})
}

func TestExtendInvalidSealsInBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := storeutil.StorageLayer(t, db)

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
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))

		block1Receipt := unittest.ReceiptForBlockFixture(block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(block1Receipt),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))

		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{block1Seal},
			ProtocolStateID: rootProtocolStateID,
		})

		sealValidator := mockmodule.NewSealValidator(t)
		sealValidator.On("Validate", mock.Anything).
			Return(func(candidate *flow.Block) *flow.Seal {
				if candidate.ID() == block3.ID() {
					return nil
				}
				seal, _ := all.Seals.HighestInFork(candidate.Header.ParentID)
				return seal
			}, func(candidate *flow.Block) error {
				if candidate.ID() == block3.ID() {
					return engine.NewInvalidInputError("")
				}
				_, err := all.Seals.HighestInFork(candidate.Header.ParentID)
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

		err = fullState.Extend(context.Background(), block1)
		require.NoError(t, err)
		err = fullState.Extend(context.Background(), block2)
		require.NoError(t, err)
		err = fullState.Extend(context.Background(), block3)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err))
	})
}

func TestHeaderExtendValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		extend := unittest.BlockWithParentFixture(head)
		extend.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))

		err = state.ExtendCertified(context.Background(), extend, unittest.CertifyBlock(extend.Header))
		require.NoError(t, err)

		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit)
	})
}

func TestHeaderExtendMissingParent(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 2
		extend.Header.View = 2
		extend.Header.ParentID = unittest.BlockFixture().ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err := state.ExtendCertified(context.Background(), &extend, unittest.CertifyBlock(extend.Header))
		require.Error(t, err)
		require.False(t, st.IsInvalidExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(extend.ID(), &sealID))
		require.Error(t, err)
		require.ErrorIs(t, err, stoerr.ErrNotFound)
	})
}

func TestHeaderExtendHeightTooSmall(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))

		// create another block that points to the previous block `extend` as parent
		// but has _same_ height as parent. This violates the condition that a child's
		// height must increment the parent's height by one, i.e. it should be rejected
		// by the follower right away
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.Header.Height = block1.Header.Height

		err = state.ExtendCertified(context.Background(), block1, block2.Header.QuorumCertificate())
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), block2, unittest.CertifyBlock(block2.Header))
		require.False(t, st.IsInvalidExtensionError(err))

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(block2.ID(), &sealID))
		require.ErrorIs(t, err, stoerr.ErrNotFound)
	})
}

func TestHeaderExtendHeightTooLarge(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())
		// set an invalid height
		block.Header.Height = head.Height + 2

		err = state.ExtendCertified(context.Background(), block, unittest.CertifyBlock(block.Header))
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
	util.RunWithFullProtocolStateAndConsumer(t, rootSnapshot, consumer, func(db *badger.DB, state *protocol.ParticipantState) {
		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		child := unittest.BlockWithParentProtocolState(block)
		grandChild := unittest.BlockWithParentProtocolState(child)

		// extend block using certifying QC, expect that BlockProcessable will be emitted once
		consumer.On("BlockProcessable", block.Header, child.Header.QuorumCertificate()).Once()
		err := state.ExtendCertified(context.Background(), block, child.Header.QuorumCertificate())
		require.NoError(t, err)

		// extend block without certifying QC, expect that BlockProcessable won't be called
		err = state.Extend(context.Background(), child)
		require.NoError(t, err)
		consumer.AssertNumberOfCalls(t, "BlockProcessable", 1)

		// extend block using certifying QC, expect that BlockProcessable will be emitted twice.
		// One for parent block and second for current block.
		grandChildCertifyingQC := unittest.CertifyBlock(grandChild.Header)
		consumer.On("BlockProcessable", child.Header, grandChild.Header.QuorumCertificate()).Once()
		consumer.On("BlockProcessable", grandChild.Header, grandChildCertifyingQC).Once()
		err = state.ExtendCertified(context.Background(), grandChild, grandChildCertifyingQC)
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
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.ExtendCertified(context.Background(), block1, unittest.CertifyBlock(block1.Header))
		require.NoError(t, err)

		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.ExtendCertified(context.Background(), block2, unittest.CertifyBlock(block2.Header))
		require.NoError(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(block2.ID(), &sealID))
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
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block1)
		require.NoError(t, err)

		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		err = state.Extend(context.Background(), block2)
		require.True(t, st.IsOutdatedExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupLatestSealAtBlock(block2.ID(), &sealID))
		require.ErrorIs(t, err, stoerr.ErrNotFound)
	})
}

func TestHeaderExtendHighestSeal(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		// create block2 and block3
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))

		block3 := unittest.BlockWithParentProtocolState(block2)

		err := state.ExtendCertified(context.Background(), block2, block3.Header.QuorumCertificate())
		require.NoError(t, err)

		// create receipts and seals for block2 and block3
		receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
		receipt3, seal3 := unittest.ReceiptAndSealForBlock(block3)

		// include the seals in block4
		block4 := unittest.BlockWithParentFixture(block3.Header)
		// include receipts and results
		block4.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(receipt3, receipt2),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))

		// include the seals in block4
		block5 := unittest.BlockWithParentFixture(block4.Header)
		// placing seals in the reversed order to test
		// Extend will pick the highest sealed block
		block5.SetPayload(unittest.PayloadFixture(
			unittest.WithSeals(seal3, seal2),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))

		err = state.ExtendCertified(context.Background(), block3, block4.Header.QuorumCertificate())
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), block4, block5.Header.QuorumCertificate())
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), block5, unittest.CertifyBlock(block5.Header))
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
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		// create child block
		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())

		t.Run("qc-invalid-view", func(t *testing.T) {
			certifyingQC := unittest.CertifyBlock(block.Header)
			certifyingQC.View++ // invalidate block view
			err = state.ExtendCertified(context.Background(), block, certifyingQC)
			require.Error(t, err)
			require.False(t, st.IsOutdatedExtensionError(err))
		})
		t.Run("qc-invalid-block-id", func(t *testing.T) {
			certifyingQC := unittest.CertifyBlock(block.Header)
			certifyingQC.BlockID = unittest.IdentifierFixture() // invalidate blockID
			err = state.ExtendCertified(context.Background(), block, certifyingQC)
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
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		// create a valid block
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		cluster, err := unittest.SnapshotClusterByIndex(rootSnapshot, 0)
		require.NoError(t, err)

		// prepare for a valid guarantor signer indices to be used in the valid block
		all := cluster.Members().NodeIDs()
		validSignerIndices, err := signature.EncodeSignersToIndices(all, all)
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		payload := flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					ChainID:          cluster.ChainID(),
					ReferenceBlockID: head.ID(),
					SignerIndices:    validSignerIndices,
				},
			},
			ProtocolStateID: rootProtocolStateID,
		}

		// now the valid block has a guarantee in the payload with valid signer indices.
		block.SetPayload(payload)

		// check Extend should accept this valid block
		err = state.Extend(context.Background(), block)
		require.NoError(t, err)

		// now the guarantee has invalid signer indices: the checksum should have 4 bytes, but it only has 1
		payload.Guarantees[0].SignerIndices = []byte{byte(1)}

		// create new block that has invalid collection guarantee
		block = unittest.BlockWithParentFixture(head)
		block.SetPayload(payload)

		err = state.Extend(context.Background(), block)
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
		err = state.Extend(context.Background(), block)
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrInvalidChecksum)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// let's test even if the checksum is correct, but signer indices is still wrong because the tailing are not 0,
		// then the block should still be rejected.
		wrongTailing := make([]byte, len(validSignerIndices))
		copy(wrongTailing, validSignerIndices)
		wrongTailing[len(wrongTailing)-1] = byte(255)

		payload.Guarantees[0].SignerIndices = wrongTailing
		err = state.Extend(context.Background(), block)
		require.Error(t, err)
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// test imcompatible bit vector length
		wrongbitVectorLength := validSignerIndices[0 : len(validSignerIndices)-1]
		payload.Guarantees[0].SignerIndices = wrongbitVectorLength
		err = state.Extend(context.Background(), block)
		require.True(t, signature.IsInvalidSignerIndicesError(err), err)
		require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// revert back to good value
		payload.Guarantees[0].SignerIndices = validSignerIndices

		// test the ReferenceBlockID is not found
		payload.Guarantees[0].ReferenceBlockID = flow.ZeroID
		err = state.Extend(context.Background(), block)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// revert back to good value
		payload.Guarantees[0].ReferenceBlockID = head.ID()

		// TODO: test the guarantee has bad reference block ID that would return protocol.ErrNextEpochNotCommitted
		// this case is not easy to create, since the test case has no such block yet.
		// we need to refactor the ParticipantState to add a guaranteeValidator, so that we can mock it and
		// return the protocol.ErrNextEpochNotCommitted for testing

		// test the guarantee has wrong chain ID, and should return ErrClusterNotFound
		payload.Guarantees[0].ChainID = flow.ChainID("some_bad_chain_ID")
		err = state.Extend(context.Background(), block)
		require.Error(t, err)
		require.ErrorIs(t, err, realprotocol.ErrClusterNotFound)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

// If block B is finalized and contains a seal for block A, then A is the last sealed block
func TestSealed(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// block 1 will be sealed
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

		// block 2 contains receipt for block 1
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(receipt1),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))

		err = state.ExtendCertified(context.Background(), block1, block2.Header.QuorumCertificate())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block1.ID())
		require.NoError(t, err)

		// block 3 contains seal for block 1
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals:           []*flow.Seal{seal1},
			ProtocolStateID: rootProtocolStateID,
		})

		err = state.ExtendCertified(context.Background(), block2, block3.Header.QuorumCertificate())
		require.NoError(t, err)
		err = state.Finalize(context.Background(), block2.ID())
		require.NoError(t, err)

		err = state.ExtendCertified(context.Background(), block3, unittest.CertifyBlock(block3.Header))
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
		func(db *badger.DB, state *protocol.FollowerState, headers storage.Headers, index storage.Index) {
			head, err := rootSnapshot.Head()
			require.NoError(t, err)

			block := unittest.BlockWithParentFixture(head)
			block.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			blockID := block.ID()

			// check 100 times to see if either 1) or 2) satisfies
			var wg sync.WaitGroup
			wg.Add(1)
			go func(blockID flow.Identifier) {
				for i := 0; i < 100; i++ {
					_, err := headers.ByBlockID(blockID)
					if errors.Is(err, stoerr.ErrNotFound) {
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
			err = state.ExtendCertified(context.Background(), block, unittest.CertifyBlock(block.Header))
			require.NoError(t, err)
			wg.Wait()
		})
}

// TestHeaderInvalidTimestamp tests that extending header with invalid timestamp results in sentinel error
func TestHeaderInvalidTimestamp(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := storeutil.StorageLayer(t, db)

		// create a event consumer to test epoch transition events
		distributor := events.NewDistributor()
		consumer := mockprotocol.NewConsumer(t)
		distributor.AddConsumer(consumer)

		block, result, seal := unittest.BootstrapFixture(participants)
		qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(block.ID()))
		rootSnapshot, err := inmem.SnapshotFromBootstrapState(block, result, seal, qc)
		require.NoError(t, err)

		state, err := protocol.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

		extend := unittest.BlockWithParentFixture(block.Header)
		extend.Payload.Guarantees = nil
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = fullState.Extend(context.Background(), extend)
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
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
			block := unittest.BlockWithParentFixture(head)
			block.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			err := state.ExtendCertified(context.Background(), block, unittest.CertifyBlock(block.Header))
			require.NoError(t, err)

			// same operation should be no-op
			err = state.ExtendCertified(context.Background(), block, unittest.CertifyBlock(block.Header))
			require.NoError(t, err)
		})
	})
	t.Run("participant", func(t *testing.T) {
		util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
			block := unittest.BlockWithParentFixture(head)
			block.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			err := state.Extend(context.Background(), block)
			require.NoError(t, err)

			// same operation should be no-op
			err = state.Extend(context.Background(), block)
			require.NoError(t, err)

			err = state.ExtendCertified(context.Background(), block, unittest.CertifyBlock(block.Header))
			require.NoError(t, err)
		})
	})
}

func assertEpochEmergencyFallbackTriggered(t *testing.T, state realprotocol.State, expected bool) {
	triggered, err := state.Params().EpochFallbackTriggered()
	require.NoError(t, err)
	assert.Equal(t, expected, triggered)
}

// mockMetricsForRootSnapshot mocks the given metrics mock object to expect all
// metrics which are set during bootstrapping and building blocks.
func mockMetricsForRootSnapshot(metricsMock *mockmodule.ComplianceMetrics, rootSnapshot *inmem.Snapshot) {
	metricsMock.On("CurrentEpochCounter", rootSnapshot.Encodable().Epochs.Current.Counter)
	phase, _ := rootSnapshot.Phase()
	metricsMock.On("CurrentEpochPhase", phase)
	metricsMock.On("CurrentEpochFinalView", rootSnapshot.Encodable().Epochs.Current.FinalView)
	metricsMock.On("CurrentDKGPhase1FinalView", rootSnapshot.Encodable().Epochs.Current.DKGPhase1FinalView)
	metricsMock.On("CurrentDKGPhase2FinalView", rootSnapshot.Encodable().Epochs.Current.DKGPhase2FinalView)
	metricsMock.On("CurrentDKGPhase3FinalView", rootSnapshot.Encodable().Epochs.Current.DKGPhase3FinalView)
	metricsMock.On("BlockSealed", mock.Anything)
	metricsMock.On("BlockFinalized", mock.Anything)
	metricsMock.On("FinalizedHeight", mock.Anything)
	metricsMock.On("SealedHeight", mock.Anything)
}

func getRootProtocolStateID(t *testing.T, rootSnapshot *inmem.Snapshot) flow.Identifier {
	rootProtocolState, err := rootSnapshot.ProtocolState()
	require.NoError(t, err)
	return rootProtocolState.Entry().ID()
}

// calculateExpectedStateId is a utility function which makes easier to get expected protocol state ID after applying service events contained in seals.
func calculateExpectedStateId(t *testing.T, mutableProtocolState realprotocol.MutableProtocolState) func(header *flow.Header, seals []*flow.Seal) flow.Identifier {
	return func(header *flow.Header, seals []*flow.Seal) flow.Identifier {
		stateMutator, err := mutableProtocolState.Mutator(header.View, header.ParentID)
		require.NoError(t, err)

		err = stateMutator.ApplyServiceEventsFromValidatedSeals(seals)
		require.NoError(t, err)

		_, _, expectedStateID, _ := stateMutator.Build()
		require.NoError(t, err)
		return expectedStateID
	}
}
