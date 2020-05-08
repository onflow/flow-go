package ingestion

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	realStorage "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type colReqMatcher struct {
	req *messages.CollectionRequest
}

func (c *colReqMatcher) Matches(x interface{}) bool {
	other := x.(*messages.CollectionRequest)
	return c.req.ID == other.ID
}
func (c *colReqMatcher) String() string {
	return fmt.Sprintf("ID %x", c.req.ID)
}

func TestCollectionRequests(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()
		// To make sure we always have collection if the block fixture changes
		guarantees := unittest.CollectionGuaranteesFixture(3)

		guarantees[0].SignerIDs = []flow.Identifier{collection1Identity.NodeID, collection3Identity.NodeID}
		guarantees[1].SignerIDs = []flow.Identifier{collection2Identity.NodeID}
		guarantees[2].SignerIDs = []flow.Identifier{collection2Identity.NodeID, collection3Identity.NodeID}

		block.Payload.Guarantees = guarantees
		block.Header.PayloadHash = block.Payload.Hash()

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		ctx.state.On("AtBlockID", block.ID()).Return(ctx.snapshot).Maybe()

		ctx.collectionConduit.EXPECT().Submit(
			&colReqMatcher{req: &messages.CollectionRequest{ID: guarantees[0].ID(), Nonce: rand.Uint64()}},
			gomock.Eq([]flow.Identifier{collection1Identity.NodeID, collection3Identity.NodeID}),
		)

		ctx.collectionConduit.EXPECT().Submit(
			&colReqMatcher{req: &messages.CollectionRequest{ID: guarantees[1].ID(), Nonce: rand.Uint64()}},
			gomock.Eq(collection2Identity.NodeID),
		)

		ctx.collectionConduit.EXPECT().Submit(
			&colReqMatcher{req: &messages.CollectionRequest{ID: guarantees[2].ID(), Nonce: rand.Uint64()}},
			gomock.Eq([]flow.Identifier{collection2Identity.NodeID, collection3Identity.NodeID}),
		)

		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, block.Header.ParentID).
			Return(unittest.StateCommitmentFixture(), nil)

		proposal := unittest.ProposalFromBlock(&block)
		err := ctx.engine.ProcessLocal(proposal)

		require.NoError(t, err)
	})
}

func TestNoCollectionRequestsIfParentMissing(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()
		// To make sure we always have collection if the block fixture changes
		guarantees := unittest.CollectionGuaranteesFixture(3)

		guarantees[0].SignerIDs = []flow.Identifier{collection1Identity.NodeID, collection3Identity.NodeID}
		guarantees[1].SignerIDs = []flow.Identifier{collection2Identity.NodeID}
		guarantees[2].SignerIDs = []flow.Identifier{collection2Identity.NodeID, collection3Identity.NodeID}

		block.Payload.Guarantees = guarantees
		block.Header.PayloadHash = block.Payload.Hash()

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		ctx.state.On("AtBlockID", block.ID()).Return(ctx.snapshot).Maybe()

		ctx.collectionConduit.EXPECT().Submit(gomock.Any(), gomock.Any()).Times(0)

		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, block.Header.ParentID).
			Return(nil, realStorage.ErrNotFound)

		proposal := unittest.ProposalFromBlock(&block)
		err := ctx.engine.ProcessLocal(proposal)

		require.NoError(t, err)
	})
}

func TestValidatingCollectionResponse(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		executableBlock := unittest.ExecutableBlockFixture([][]flow.Identifier{{collection1Identity.NodeID}})
		executableBlock.StartState = unittest.StateCommitmentFixture()

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlock.Block))

		id := executableBlock.Collections()[0].Guarantee.ID()

		ctx.state.On("AtBlockID", executableBlock.Block.ID()).Return(ctx.snapshot).Maybe()

		ctx.collectionConduit.EXPECT().Submit(
			&colReqMatcher{req: &messages.CollectionRequest{ID: id, Nonce: rand.Uint64()}},
			gomock.Eq(collection1Identity.NodeID),
		).Return(nil)

		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, executableBlock.Block.Header.ParentID).
			Return(executableBlock.StartState, nil)

		proposal := unittest.ProposalFromBlock(executableBlock.Block)
		err := ctx.engine.ProcessLocal(proposal)
		require.NoError(t, err)

		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: executableBlock.Collections()[0].Transactions},
		}

		// TODO Enable wrong response sending once we have a way to hash collection

		// wrongResponse := provider.CollectionResponse{
		//	Fingerprint:  fingerprint,
		//	Transactions: []flow.TransactionBody{tx},
		// }

		// engine.Submit(collectionIdentity.NodeID, wrongResponse)

		// no interaction with conduit for finished executableBlock
		// </TODO enable>

		//ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.Header.ParentID).Return(unittest.StateCommitmentFixture(), realStorage.ErrNotFound)

		ctx.assertSuccessfulBlockComputation(executableBlock, unittest.IdentifierFixture())

		err = ctx.engine.ProcessLocal(&rightResponse)
		require.NoError(t, err)
	})
}

func TestNoBlockExecutedUntilAllCollectionsArePosted(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		executableBlock := unittest.ExecutableBlockFixture(
			[][]flow.Identifier{
				{collection1Identity.NodeID},
				{collection1Identity.NodeID},
				{collection1Identity.NodeID},
			},
		)

		for _, col := range executableBlock.Block.Payload.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(
				&colReqMatcher{req: &messages.CollectionRequest{ID: col.ID(), Nonce: rand.Uint64()}},
				gomock.Eq(collection1Identity.NodeID),
			)
		}

		ctx.state.On("AtBlockID", executableBlock.ID()).Return(ctx.snapshot)

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlock.Block))
		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, executableBlock.Block.Header.ParentID).
			Return(unittest.StateCommitmentFixture(), nil)

		proposal := unittest.ProposalFromBlock(executableBlock.Block)
		err := ctx.engine.ProcessLocal(proposal)
		require.NoError(t, err)

		// Expected no calls so test should fail if any occurs
		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: executableBlock.Collections()[1].Transactions},
		}

		err = ctx.engine.ProcessLocal(&rightResponse)

		require.NoError(t, err)
	})
}

func TestCollectionSharedByMultipleBlocks(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		blockA := unittest.BlockFixture()
		blockB := unittest.BlockFixture()

		completeCollection := unittest.CompleteCollectionFixture()
		completeCollection.Guarantee.SignerIDs = []flow.Identifier{collection1Identity.NodeID}

		blockA.Payload.Guarantees = []*flow.CollectionGuarantee{completeCollection.Guarantee}
		blockA.Header.PayloadHash = blockA.Payload.Hash()
		executableBlockA := entity.ExecutableBlock{
			Block:               &blockA,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{completeCollection.Guarantee.CollectionID: completeCollection},
			StartState:          unittest.StateCommitmentFixture(),
		}

		blockB.Payload.Guarantees = []*flow.CollectionGuarantee{completeCollection.Guarantee}
		blockB.Header.PayloadHash = blockB.Payload.Hash()
		executableBlockB := entity.ExecutableBlock{
			Block:               &blockB,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{completeCollection.Guarantee.CollectionID: completeCollection},
			StartState:          unittest.StateCommitmentFixture(),
		}

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockA.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockB.Block))
		ctx.state.On("AtBlockID", blockA.ID()).Return(ctx.snapshot).Maybe()
		ctx.state.On("AtBlockID", blockB.ID()).Return(ctx.snapshot).Maybe()

		ctx.collectionConduit.EXPECT().Submit(
			&colReqMatcher{req: &messages.CollectionRequest{ID: completeCollection.Guarantee.ID(), Nonce: rand.Uint64()}},
			gomock.Any(),
		).Times(1)

		ctx.executionState.On("StateCommitmentByBlockID", blockA.Header.ParentID).Return(executableBlockA.StartState, nil)
		ctx.executionState.On("StateCommitmentByBlockID", blockB.Header.ParentID).Return(executableBlockB.StartState, nil)

		proposalA := unittest.ProposalFromBlock(executableBlockA.Block)
		err := ctx.engine.ProcessLocal(proposalA)
		require.NoError(t, err)

		proposalB := unittest.ProposalFromBlock(executableBlockB.Block)
		err = ctx.engine.ProcessLocal(proposalB)
		require.NoError(t, err)

		ctx.assertSuccessfulBlockComputation(&executableBlockA, unittest.IdentifierFixture())
		ctx.assertSuccessfulBlockComputation(&executableBlockB, unittest.IdentifierFixture())

		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: completeCollection.Transactions},
		}

		err = ctx.engine.ProcessLocal(&rightResponse)

		require.NoError(t, err)
	})
}
