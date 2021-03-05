// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
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
	"github.com/onflow/flow-go/module/trace"
	st "github.com/onflow/flow-go/state"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/util"
	stoerr "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	storeutil "github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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
		err = db.View(operation.LookupBlockSeal(genesisID, &sealID))
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

func TestExtendValid(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := storeutil.StorageLayer(t, db)

		// create a event consumer to test epoch transition events
		distributor := events.NewDistributor()
		consumer := new(mockprotocol.Consumer)
		distributor.AddConsumer(consumer)

		block, result, seal := unittest.BootstrapFixture(participants)
		qc := unittest.QuorumCertificateFixture(unittest.QCWithBlockID(block.ID()))
		rootSnapshot, err := inmem.SnapshotFromBootstrapState(block, result, seal, qc)
		require.NoError(t, err)

		state, err := protocol.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)

		fullState, err := protocol.NewFullConsensusState(state, index, payloads, tracer, consumer,
			util.MockReceiptValidator(), util.MockSealValidator(seals))
		require.NoError(t, err)

		extend := unittest.BlockWithParentFixture(block.Header)
		extend.Payload.Guarantees = nil
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = fullState.Extend(&extend)
		require.NoError(t, err)

		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit)

		consumer.On("BlockFinalized", extend.Header).Once()
		err = fullState.Finalize(extend.ID())
		require.Nil(t, err)
		consumer.AssertExpectations(t)
	})
}

func TestExtendSealedBoundary(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)
		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "original commit should be root commit")

		// Create a first block on top of the snapshot
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block1)
		require.NoError(t, err)

		// Add a second block containing a receipt committing to the first block
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		err = state.Extend(&block2)
		require.NoError(t, err)

		// Add a third block containing a seal for the first block
		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})
		err = state.Extend(&block3)
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "commit should not change before finalizing")

		err = state.Finalize(block1.ID())
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "commit should not change after finalizing non-sealing block")

		err = state.Finalize(block2.ID())
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, seal.FinalState, finalCommit, "commit should not change after finalizing non-sealing block")

		err = state.Finalize(block3.ID())
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		require.Equal(t, block1Seal.FinalState, finalCommit, "commit should change after finalizing sealing block")
	})
}

func TestExtendMissingParent(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 2
		extend.Header.View = 2
		extend.Header.ParentID = unittest.BlockFixture().ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err := state.Extend(&extend)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
	})
}

func TestExtendHeightTooSmall(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.SetPayload(flow.EmptyPayload())
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = head.ID()

		err = state.Extend(&extend)
		require.NoError(t, err)

		// create another block with the same height and view, that is coming after
		extend.Header.ParentID = extend.Header.ID()
		extend.Header.Height = 1
		extend.Header.View = 2

		err = state.Extend(&extend)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
	})
}

func TestExtendHeightTooLarge(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())
		// set an invalid height
		block.Header.Height = head.Height + 2

		err = state.Extend(&block)
		require.Error(t, err)
	})
}

func TestExtendBlockNotConnected(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// add 2 blocks, the second finalizing/sealing the state of the first
		extend := unittest.BlockWithParentFixture(head)
		extend.SetPayload(flow.EmptyPayload())

		err = state.Extend(&extend)
		require.NoError(t, err)

		err = state.Finalize(extend.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		extend.Header.Timestamp = extend.Header.Timestamp.Add(time.Second)
		extend.Header.ParentID = head.ID()

		err = state.Extend(&extend)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
	})
}

func TestExtendInvalidChainID(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block := unittest.BlockWithParentFixture(head)
		block.SetPayload(flow.EmptyPayload())
		// use an invalid chain ID
		block.Header.ChainID = head.ChainID + "-invalid"

		err = state.Extend(&block)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

// Test that Extend will refuse payloads that contain duplicate receipts, where
// duplicates can be in another block on the fork, or within the payload.
func TestExtendReceiptsDuplicate(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block2)
		require.Nil(t, err)

		receipt := unittest.ReceiptForBlockFixture(&block2)

		// B1 <- B2 <- B3{R(B2)} <- B4{R(B2)}
		t.Run("duplicate receipt in different block", func(t *testing.T) {
			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(flow.Payload{
				Receipts: []*flow.ExecutionReceipt{receipt},
			})
			err = state.Extend(&block3)
			require.Nil(t, err)

			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(flow.Payload{
				Receipts: []*flow.ExecutionReceipt{receipt},
			})
			err = state.Extend(&block4)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})

		// B1 <- B2 <- B3{R(B2), R(B2)}
		t.Run("duplicate receipt in same block", func(t *testing.T) {
			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(flow.Payload{
				Receipts: []*flow.ExecutionReceipt{
					receipt,
					receipt,
				},
			})
			err = state.Extend(&block3)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})

	})
}

// Test that Extend will refuse payloads that contain receipts for blocks that
// are already sealed on the fork, but will accept receipts for blocks that are
// sealed on another fork.
func TestExtendReceiptsSealedBlock(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// create block2
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block2)
		require.Nil(t, err)

		block2Receipt := unittest.ReceiptForBlockFixture(&block2)

		// B1<--B2<--B3{R{B2)}<--B4{S(R(B2))}<--B5{R'(B2)}

		// create block3 with a receipt for block2
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block2Receipt},
		})
		err = state.Extend(&block3)
		require.Nil(t, err)

		// create a seal for block2
		seal2 := unittest.Seal.Fixture(unittest.Seal.WithResult(&block2Receipt.ExecutionResult))

		// create block4 containing a seal for block2
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal2},
		})
		err = state.Extend(&block4)
		require.Nil(t, err)

		// insert another receipt for block 2, which is now the highest sealed
		// block, and ensure that the receipt is rejected
		receipt := unittest.ReceiptForBlockFixture(&block2)
		block5 := unittest.BlockWithParentFixture(block4.Header)
		block5.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{receipt},
		})
		err = state.Extend(&block5)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// B1<--B2<--B3{R{B2)}<--B4{S(R(B2))}<--B5{R'(B2)}
		//       |
		//       +---B6{R''(B2)}

		// insert another receipt for B2 but in a separate fork. The fact that
		// B2 is sealed on a separate fork should not cause the receipt to be
		// rejected
		block6 := unittest.BlockWithParentFixture(block2.Header)
		block6.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{receipt},
		})
		err = state.Extend(&block6)
		require.Nil(t, err)
	})
}

// Test that Extend will reject payloads that contain receipts for blocks that
// are not on the fork
//
// B1<--B2<--B3
//      |
//      +----B4{R(B3)}
func TestExtendReceiptsBlockNotOnFork(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		// create block2
		block2 := unittest.BlockWithParentFixture(head)
		block2.Payload.Guarantees = nil
		block2.Header.PayloadHash = block2.Payload.Hash()
		err := state.Extend(&block2)
		require.Nil(t, err)

		// create block3
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block3)
		require.Nil(t, err)

		block3Receipt := unittest.ReceiptForBlockFixture(&block3)

		block4 := unittest.BlockWithParentFixture(block2.Header)
		block4.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block3Receipt},
		})
		err = state.Extend(&block4)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsNotSorted(t *testing.T) {
	// Todo: this test needs to be updated:
	// We don't require the receipts to be sorted by height anymore
	// We could require an "parent first" ordering, which is less strict than
	// a full ordering by height
	t.Skip()

	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		// create block2 and block3
		block2 := unittest.BlockWithParentFixture(head)
		block2.Payload.Guarantees = nil
		block2.Header.PayloadHash = block2.Payload.Hash()
		err := state.Extend(&block2)
		require.Nil(t, err)

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.Payload.Guarantees = nil
		block3.Header.PayloadHash = block3.Payload.Hash()
		err = state.Extend(&block3)
		require.Nil(t, err)

		// insert a block with payload receipts not sorted by block height.
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.Payload.Guarantees = nil
		block4.Payload.Receipts = append(block4.Payload.Receipts,
			unittest.ReceiptForBlockFixture(&block3),
			unittest.ReceiptForBlockFixture(&block2),
		)
		block4.Header.PayloadHash = block4.Payload.Hash()
		err = state.Extend(&block4)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsInvalid(t *testing.T) {
	validator := &mockmodule.ReceiptValidator{}

	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// create block2 and block3
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block2)
		require.Nil(t, err)

		// Add a receipt for block 2
		receipt := unittest.ExecutionReceiptFixture()

		// force the receipt validator to refuse this receipt
		validator.On("Validate", mock.Anything).Return(engine.NewInvalidInputError(""))

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{receipt},
		})
		err = state.Extend(&block3)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)
	})
}

func TestExtendReceiptsValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block2)
		require.Nil(t, err)

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block3)
		require.Nil(t, err)

		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block4)
		require.Nil(t, err)

		receipt3a := unittest.ReceiptForBlockFixture(&block3)
		receipt3b := unittest.ReceiptForBlockFixture(&block3)

		block5 := unittest.BlockWithParentFixture(block4.Header)
		block5.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{
				receipt3a,
				receipt3b,
				unittest.ReceiptForBlockFixture(&block4),
			},
		})
		err = state.Extend(&block5)
		require.Nil(t, err)
	})
}

// Tests the full flow of transitioning between epochs by finalizing a setup
// event, then a commit event, then finalizing the first block of the next epoch.
// Also tests that appropriate epoch transition events are fired.
func TestExtendEpochTransitionValid(t *testing.T) {
	// create a event consumer to test epoch transition events
	consumer := new(mockprotocol.Consumer)
	consumer.On("BlockFinalized", mock.Anything)
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolStateAndConsumer(t, rootSnapshot, consumer, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// we should begin the epoch in the staking phase
		phase, err := state.AtBlockID(head.ID()).Phase()
		assert.Nil(t, err)
		require.Equal(t, flow.EpochPhaseStaking, phase)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block1)
		require.Nil(t, err)
		err = state.Finalize(block1.ID())
		require.Nil(t, err)

		// create a receipt for block 1
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)

		// add a second block with a receipt committing to the first block
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		err = state.Extend(&block2)
		require.Nil(t, err)
		err = state.Finalize(block2.ID())
		require.Nil(t, err)

		epoch1Setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Order(order.ByNodeIDAsc)

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
			unittest.WithFirstView(epoch1FinalView+1),
		)

		// create the seal referencing block1 and including the setup event
		seal1 := unittest.Seal.Fixture(
			unittest.Seal.WithResult(&block1Receipt.ExecutionResult),
			unittest.Seal.WithServiceEvents(epoch2Setup.ServiceEvent()),
		)

		// create a receipt for block2
		block2Receipt := unittest.ReceiptForBlockFixture(&block2)

		// block 3 contains the epoch setup service event, as well as a receipt
		// for block 2
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block2Receipt},
			Seals:    []*flow.Seal{seal1},
		})

		// insert the block containing the seal containing the setup event
		err = state.Extend(&block3)
		require.Nil(t, err)

		// now that the setup event has been emitted, we should be in the setup phase
		phase, err = state.AtBlockID(block3.ID()).Phase()
		assert.Nil(t, err)
		require.Equal(t, flow.EpochPhaseSetup, phase)

		// we should NOT be able to query epoch 2 wrt block 1
		_, err = state.AtBlockID(block1.ID()).Epochs().Next().InitialIdentities()
		require.Error(t, err)
		_, err = state.AtBlockID(block1.ID()).Epochs().Next().Clustering()
		require.Error(t, err)

		// we should be able to query epoch 2 wrt block 3
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().InitialIdentities()
		assert.Nil(t, err)
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().Clustering()
		assert.Nil(t, err)

		// only setup event is finalized, not commit, so shouldn't be able to get certain info
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().DKG()
		require.Error(t, err)

		// ensure an epoch phase transition when we finalize the event
		consumer.On("EpochSetupPhaseStarted", epoch2Setup.Counter-1, block3.Header).Once()
		err = state.Finalize(block3.ID())
		require.Nil(t, err)
		consumer.AssertCalled(t, "EpochSetupPhaseStarted", epoch2Setup.Counter-1, block3.Header)

		epoch2Commit := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(epoch2Setup.Counter),
			unittest.WithDKGFromParticipants(epoch2Participants),
		)

		// create a seal for block 2 with epoch2 service event
		seal2 := unittest.Seal.Fixture(
			unittest.Seal.WithResult(&block2Receipt.ExecutionResult),
			unittest.Seal.WithServiceEvents(epoch2Commit.ServiceEvent()),
		)

		// create a receipt for block 3
		block3Receipt := unittest.ReceiptForBlockFixture(&block3)

		// block 4 contains the epoch commit service event, as well as a receipt
		// for block 3
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block3Receipt},
			Seals:    []*flow.Seal{seal2},
		})

		err = state.Extend(&block4)
		require.Nil(t, err)

		// we should NOT be able to query epoch 2 commit info wrt block 3
		_, err = state.AtBlockID(block3.ID()).Epochs().Next().DKG()
		require.Error(t, err)

		// now epoch 2 is fully ready, we can query anything we want about it wrt block 4 (or later)
		_, err = state.AtBlockID(block4.ID()).Epochs().Next().InitialIdentities()
		require.Nil(t, err)
		_, err = state.AtBlockID(block4.ID()).Epochs().Next().Clustering()
		require.Nil(t, err)
		_, err = state.AtBlockID(block4.ID()).Epochs().Next().DKG()
		assert.Nil(t, err)

		// how that the commit event has been emitted, we should be in the committed phase
		phase, err = state.AtBlockID(block4.ID()).Phase()
		assert.Nil(t, err)
		require.Equal(t, flow.EpochPhaseCommitted, phase)

		// expect epoch phase transition once we finalize block 4
		consumer.On("EpochCommittedPhaseStarted", epoch2Setup.Counter-1, block4.Header)
		err = state.Finalize(block4.ID())
		require.Nil(t, err)
		consumer.AssertCalled(t, "EpochCommittedPhaseStarted", epoch2Setup.Counter-1, block4.Header)

		// we should still be in epoch 1
		epochCounter, err := state.AtBlockID(block4.ID()).Epochs().Current().Counter()
		require.Nil(t, err)
		require.Equal(t, epoch1Setup.Counter, epochCounter)

		// block 5 has the final view of the epoch
		block5 := unittest.BlockWithParentFixture(block4.Header)
		block5.SetPayload(flow.EmptyPayload())
		block5.Header.View = epoch1FinalView

		err = state.Extend(&block5)
		require.Nil(t, err)

		// we should still be in epoch 1, since epochs are inclusive of final view
		epochCounter, err = state.AtBlockID(block5.ID()).Epochs().Current().Counter()
		require.Nil(t, err)
		require.Equal(t, epoch1Setup.Counter, epochCounter)

		// block 6 has a view > final view of epoch 1, it will be considered the first block of epoch 2
		block6 := unittest.BlockWithParentFixture(block5.Header)
		block6.SetPayload(flow.EmptyPayload())
		// we should handle view that aren't exactly the first valid view of the epoch
		block6.Header.View = epoch1FinalView + uint64(1+rand.Intn(10))

		err = state.Extend(&block6)
		require.Nil(t, err)

		// now, at long last, we are in epoch 2
		epochCounter, err = state.AtBlockID(block6.ID()).Epochs().Current().Counter()
		require.Nil(t, err)
		require.Equal(t, epoch2Setup.Counter, epochCounter)

		// we should begin epoch 2 in staking phase
		// how that the commit event has been emitted, we should be in the committed phase
		phase, err = state.AtBlockID(block6.ID()).Phase()
		assert.Nil(t, err)
		require.Equal(t, flow.EpochPhaseStaking, phase)

		// expect epoch transition once we finalize block 6
		consumer.On("EpochTransition", epoch2Setup.Counter, block6.Header).Once()
		err = state.Finalize(block5.ID())
		require.Nil(t, err)
		err = state.Finalize(block6.ID())
		require.Nil(t, err)
		consumer.AssertCalled(t, "EpochTransition", epoch2Setup.Counter, block6.Header)
	})
}

// we should be able to have conflicting forks with two different instances of
// the same service event for the same epoch
//
//        /-->BLOCK1-->BLOCK3-->BLOCK5
// ROOT --+
//        \-->BLOCK2-->BLOCK4-->BLOCK6
//
func TestExtendConflictingEpochEvents(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add two conflicting blocks for each service event to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block1)
		require.Nil(t, err)

		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block2)
		require.Nil(t, err)

		// add blocks containing receipts for block1 and block2 (necessary for
		// sealing)
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block3 := unittest.BlockWithParentFixture(block1.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		err = state.Extend(&block3)
		require.Nil(t, err)

		block2Receipt := unittest.ReceiptForBlockFixture(&block2)
		block4 := unittest.BlockWithParentFixture(block2.Header)
		block4.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block2Receipt},
		})
		err = state.Extend(&block4)
		require.Nil(t, err)

		rootSetup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)

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

		// create one seal containing the first setup event
		seal1 := unittest.Seal.Fixture(
			unittest.Seal.WithResult(&block1Receipt.ExecutionResult),
			unittest.Seal.WithServiceEvents(nextEpochSetup1.ServiceEvent()),
		)

		// create another seal containing the second setup event
		seal2 := unittest.Seal.Fixture(
			unittest.Seal.WithResult(&block2Receipt.ExecutionResult),
			unittest.Seal.WithServiceEvents(nextEpochSetup2.ServiceEvent()),
		)

		// block 5 builds on block 3, contains setup event 1
		block5 := unittest.BlockWithParentFixture(block3.Header)
		block5.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal1},
		})
		err = state.Extend(&block5)
		require.Nil(t, err)

		// block 6 builds on block 4, contains setup event 2
		block6 := unittest.BlockWithParentFixture(block4.Header)
		block6.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal2},
		})
		err = state.Extend(&block6)
		require.Nil(t, err)

		// should be able query each epoch from the appropriate reference block
		setup1FinalView, err := state.AtBlockID(block5.ID()).Epochs().Next().FinalView()
		assert.Nil(t, err)
		require.Equal(t, nextEpochSetup1.FinalView, setup1FinalView)

		setup2FinalView, err := state.AtBlockID(block6.ID()).Epochs().Next().FinalView()
		assert.Nil(t, err)
		require.Equal(t, nextEpochSetup2.FinalView, setup2FinalView)
	})
}

// extending protocol state with an invalid epoch setup service event should cause an error
func TestExtendEpochSetupInvalid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block1)
		require.Nil(t, err)
		err = state.Finalize(block1.ID())
		require.Nil(t, err)

		epoch1Setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Order(order.ByNodeIDAsc)

		// this function will return a VALID setup event and seal, we will modify
		// in different ways in each test case
		createSetup := func() (*flow.EpochSetup, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			seal := unittest.Seal.Fixture(
				unittest.Seal.WithBlockID(block1.ID()),
				unittest.Seal.WithServiceEvents(setup.ServiceEvent()),
			)
			return setup, seal
		}

		t.Run("wrong counter", func(t *testing.T) {
			setup, seal := createSetup()
			setup.Counter = epoch1Setup.Counter

			block := unittest.BlockWithParentFixture(block1.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})

			err = state.Extend(&block)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})

		t.Run("invalid final view", func(t *testing.T) {
			setup, seal := createSetup()

			block := unittest.BlockWithParentFixture(block1.Header)
			setup.FinalView = block.Header.View
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err = state.Extend(&block)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})

		t.Run("empty seed", func(t *testing.T) {
			setup, seal := createSetup()
			setup.RandomSource = nil

			block := unittest.BlockWithParentFixture(block1.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})

			err = state.Extend(&block)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})
	})
}

// extending protocol state with an invalid epoch commit service event should cause an error
func TestExtendEpochCommitInvalid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block1)
		require.Nil(t, err)
		err = state.Finalize(block1.ID())
		require.Nil(t, err)

		// add a block with a receipt for block1
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		err = state.Extend(&block2)
		require.Nil(t, err)
		err = state.Finalize(block2.ID())
		require.Nil(t, err)

		epoch1Setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)

		// swap consensus node for a new one for epoch 2
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		epoch2Participants := append(
			participants.Filter(filter.Not(filter.HasRole(flow.RoleConsensus))),
			epoch2NewParticipant,
		).Order(order.ByNodeIDAsc)

		createSetup := func(sealedResult *flow.ExecutionResult) (*flow.EpochSetup, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
				unittest.WithFirstView(epoch1Setup.FinalView+1),
			)
			seal := unittest.Seal.Fixture(
				unittest.Seal.WithResult(sealedResult),
				unittest.Seal.WithServiceEvents(setup.ServiceEvent()),
			)
			return setup, seal
		}

		createCommit := func(sealedResult *flow.ExecutionResult) (*flow.EpochCommit, *flow.Seal) {
			commit := unittest.EpochCommitFixture(
				unittest.CommitWithCounter(epoch1Setup.Counter+1),
				unittest.WithDKGFromParticipants(epoch2Participants),
			)
			seal := unittest.Seal.Fixture(
				unittest.Seal.WithResult(sealedResult),
				unittest.Seal.WithServiceEvents(commit.ServiceEvent()),
			)
			return commit, seal
		}

		t.Run("without setup", func(t *testing.T) {
			_, seal := createCommit(&block1Receipt.ExecutionResult)

			block := unittest.BlockWithParentFixture(block2.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err = state.Extend(&block)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})

		// insert the epoch setup
		epoch2Setup, setupSeal := createSetup(&block1Receipt.ExecutionResult)
		block2Receipt := unittest.ReceiptForBlockFixture(&block2)
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block2Receipt},
			Seals:    []*flow.Seal{setupSeal},
		})
		err = state.Extend(&block3)
		require.Nil(t, err)
		err = state.Finalize(block3.ID())
		require.Nil(t, err)
		_ = epoch2Setup

		t.Run("inconsistent counter", func(t *testing.T) {
			commit, seal := createCommit(&block2Receipt.ExecutionResult)
			commit.Counter = epoch2Setup.Counter + 1

			block := unittest.BlockWithParentFixture(block3.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Extend(&block)
			require.Error(t, err)
			require.True(t, st.IsInvalidExtensionError(err), err)
		})

		t.Run("inconsistent cluster QCs", func(t *testing.T) {
			commit, seal := createCommit(&block2Receipt.ExecutionResult)
			commit.ClusterQCs = append(commit.ClusterQCs, unittest.QuorumCertificateFixture())

			block := unittest.BlockWithParentFixture(block3.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Extend(&block)
			require.Error(t, err)
		})

		t.Run("missing dkg group key", func(t *testing.T) {
			commit, seal := createCommit(&block2Receipt.ExecutionResult)
			commit.DKGGroupKey = nil

			block := unittest.BlockWithParentFixture(block3.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Extend(&block)
			require.Error(t, err)
		})

		t.Run("inconsistent DKG participants", func(t *testing.T) {
			commit, seal := createCommit(&block2Receipt.ExecutionResult)

			// add the consensus node from epoch *1*, which was removed for epoch 2
			epoch1CONNode := participants.Filter(filter.HasRole(flow.RoleConsensus))[0]
			commit.DKGParticipants[epoch1CONNode.NodeID] = flow.DKGParticipant{
				KeyShare: unittest.KeyFixture(crypto.BLSBLS12381).PublicKey(),
				Index:    1,
			}

			block := unittest.BlockWithParentFixture(block3.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Extend(&block)
			require.Error(t, err)
		})
	})
}

// if we reach the first block of the next epoch before both setup and commit
// service events are finalized, the chain should halt
func TestExtendEpochTransitionWithoutCommit(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block1)
		require.Nil(t, err)
		err = state.Finalize(block1.ID())
		require.Nil(t, err)

		// add a block containing a receipt for block1
		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})
		err = state.Extend(&block2)
		require.Nil(t, err)
		err = state.Finalize(block2.ID())
		require.Nil(t, err)

		epoch1Setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Order(order.ByNodeIDAsc)

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
			unittest.WithFirstView(epoch1FinalView+1),
		)

		// create the seal referencing block1 and including the setup event
		seal1 := unittest.Seal.Fixture(
			unittest.Seal.WithResult(&block1Receipt.ExecutionResult),
			unittest.Seal.WithServiceEvents(epoch2Setup.ServiceEvent()),
		)

		// block 3 contains the epoch setup service event
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal1},
		})

		// insert the block containing the seal containing the setup event
		err = state.Extend(&block3)
		require.Nil(t, err)

		// block 4 will be the first block for epoch 2
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.Header.View = epoch2Setup.FinalView + 1

		err = state.Extend(&block4)
		require.Error(t, err)
	})
}

func TestExtendInvalidSealsInBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := storeutil.StorageLayer(t, db)

		// create a event consumer to test epoch transition events
		distributor := events.NewDistributor()
		consumer := new(mockprotocol.Consumer)
		distributor.AddConsumer(consumer)

		rootSnapshot := unittest.RootSnapshotFixture(participants)

		state, err := protocol.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)

		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentFixture(head)
		block1.Payload.Guarantees = nil
		block1.Header.PayloadHash = block1.Payload.Hash()

		block1Receipt := unittest.ReceiptForBlockFixture(&block1)
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Receipts: []*flow.ExecutionReceipt{block1Receipt},
		})

		block1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(&block1Receipt.ExecutionResult))
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{block1Seal},
		})

		sealValidator := &mockmodule.SealValidator{}
		sealValidator.On("Validate", mock.Anything).
			Return(func(candidate *flow.Block) *flow.Seal {
				if candidate.ID() == block3.ID() {
					return nil
				}
				seal, _ := seals.ByBlockID(candidate.Header.ParentID)
				return seal
			}, func(candidate *flow.Block) error {
				if candidate.ID() == block3.ID() {
					return engine.NewInvalidInputError("")
				}
				_, err := seals.ByBlockID(candidate.Header.ParentID)
				return err
			}).
			Times(3)

		fullState, err := protocol.NewFullConsensusState(state, index, payloads, tracer, consumer,
			util.MockReceiptValidator(), sealValidator)
		require.NoError(t, err)

		err = fullState.Extend(&block1)
		require.NoError(t, err)
		err = fullState.Extend(&block2)
		require.NoError(t, err)
		err = fullState.Extend(&block3)

		sealValidator.AssertExpectations(t)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err))
	})
}

func TestHeaderExtendValid(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		_, seal, err := rootSnapshot.SealedResult()
		require.NoError(t, err)

		extend := unittest.BlockWithParentFixture(head)
		extend.SetPayload(flow.EmptyPayload())

		err = state.Extend(&extend)
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

		err := state.Extend(&extend)
		require.Error(t, err)
		require.True(t, st.IsInvalidExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
	})
}

func TestHeaderExtendHeightTooSmall(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		block1 := unittest.BlockWithParentFixture(head)
		err = state.Extend(&block1)
		require.NoError(t, err)

		// create another block that points to the previous block `extend` as parent
		// but has _same_ height as parent. This violates the condition that a child's
		// height must increment the parent's height by one, i.e. it should be rejected
		// by the follower right away
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.Header.Height = block1.Header.Height

		err = state.Extend(&block2)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(block2.ID(), &sealID))
		require.Error(t, err)
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
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

		err = state.Extend(&block)
		require.Error(t, err)
	})
}

func TestHeaderExtendBlockNotConnected(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// add 2 blocks, where:
		// first block is added and then finalized;
		// second block is a sibling to the finalized block
		// The Follower should reject this block as an outdated chain extension
		block1 := unittest.BlockWithParentFixture(head)
		err = state.Extend(&block1)
		require.NoError(t, err)

		err = state.Finalize(block1.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		block2 := unittest.BlockWithParentFixture(head)
		err = state.Extend(&block2)
		require.Error(t, err)
		require.True(t, st.IsOutdatedExtensionError(err), err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(block2.ID(), &sealID))
		require.Error(t, err)
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
	})
}

func TestHeaderExtendHighestSeal(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		// create block2 and block3
		block2 := unittest.BlockWithParentFixture(head)
		block2.SetPayload(flow.EmptyPayload())
		err := state.Extend(&block2)
		require.Nil(t, err)

		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.EmptyPayload())
		err = state.Extend(&block3)
		require.Nil(t, err)

		// create seals for block2 and block3
		seal2 := unittest.Seal.Fixture(
			unittest.Seal.WithBlockID(block2.ID()),
		)
		seal3 := unittest.Seal.Fixture(
			unittest.Seal.WithBlockID(block3.ID()),
		)

		// include the seals in block4
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{
			// placing seals in the reversed order to test
			// Extend will pick the highest sealed block
			Seals:      []*flow.Seal{seal3, seal2},
			Guarantees: nil,
		})
		err = state.Extend(&block4)
		require.Nil(t, err)

		finalCommit, err := state.AtBlockID(block4.ID()).Commit()
		require.NoError(t, err)
		require.Equal(t, seal3.FinalState, finalCommit)
	})
}

func TestMakeValid(t *testing.T) {
	t.Run("should trigger BlockProcessable with parent block", func(t *testing.T) {
		consumer := &mockprotocol.Consumer{}
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		head, err := rootSnapshot.Head()
		require.NoError(t, err)
		util.RunWithFullProtocolStateAndConsumer(t, rootSnapshot, consumer, func(db *badger.DB, state *protocol.MutableState) {
			// create block2 and block3
			block2 := unittest.BlockWithParentFixture(head)
			block2.SetPayload(flow.EmptyPayload())
			err := state.Extend(&block2)
			require.Nil(t, err)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(flow.EmptyPayload())
			err = state.Extend(&block3)
			require.Nil(t, err)

			consumer.On("BlockProcessable", mock.Anything).Return()

			// make valid on block2
			err = state.MarkValid(block2.ID())
			require.NoError(t, err)

			// because the parent block is the root block,
			// BlockProcessable is not triggered on root block.
			consumer.AssertNotCalled(t, "BlockProcessable")

			err = state.MarkValid(block3.ID())
			require.NoError(t, err)

			// because the parent is not a root block, BlockProcessable event should be emitted
			// block3's parent is block2
			consumer.AssertCalled(t, "BlockProcessable", block2.Header)
		})
	})
}

// If block A is finalized and contains a seal to block B, then B is the last sealed block
func TestSealed(t *testing.T) {
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.FollowerState) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		// A <- B <- C <- D <- E <- F <- G
		blockA := unittest.BlockWithParentAndSeal(head, nil)
		blockB := unittest.BlockWithParentAndSeal(blockA.Header, nil)
		blockC := unittest.BlockWithParentAndSeal(blockB.Header, blockA.Header)
		blockD := unittest.BlockWithParentAndSeal(blockC.Header, blockB.Header)
		blockE := unittest.BlockWithParentAndSeal(blockD.Header, nil)
		blockF := unittest.BlockWithParentAndSeal(blockE.Header, nil)
		blockG := unittest.BlockWithParentAndSeal(blockF.Header, nil)
		blockH := unittest.BlockWithParentAndSeal(blockG.Header, nil)

		saveBlock(t, blockA, nil, state)
		saveBlock(t, blockB, nil, state)
		saveBlock(t, blockC, nil, state)
		saveBlock(t, blockD, blockA, state)
		saveBlock(t, blockE, blockB, state)
		saveBlock(t, blockF, blockC, state)
		saveBlock(t, blockG, blockD, state)
		saveBlock(t, blockH, blockE, state)

		sealed, err := state.Sealed().Head()
		require.NoError(t, err)
		require.Equal(t, blockB.Header.Height, sealed.Height)
	})
}

func saveBlock(t *testing.T, block *flow.Block, finalizes *flow.Block, state *protocol.FollowerState) {
	err := state.Extend(block)
	require.NoError(t, err)

	if finalizes != nil {
		err = state.Finalize(finalizes.ID())
		require.NoError(t, err)
	}

	err = state.MarkValid(block.Header.ID())
	require.NoError(t, err)
}
