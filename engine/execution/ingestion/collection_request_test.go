package ingestion

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	realStorage "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCollectionRequests(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()
		//To make sure we always have collection if the block fixture changes
		guarantees := unittest.CollectionGuaranteesFixture(3)

		guarantees[0].SignerIDs = []flow.Identifier{collection1Identity.NodeID, collection3Identity.NodeID}
		guarantees[1].SignerIDs = []flow.Identifier{collection2Identity.NodeID}
		guarantees[2].SignerIDs = []flow.Identifier{collection2Identity.NodeID, collection3Identity.NodeID}

		block.Guarantees = guarantees
		block.PayloadHash = block.Payload.Hash()

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		ctx.state.On("AtBlockID", block.ID()).Return(ctx.snapshot).Maybe()

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: guarantees[0].ID()}), gomock.Eq([]flow.Identifier{collection1Identity.NodeID, collection3Identity.NodeID}))

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: guarantees[1].ID()}), gomock.Eq(collection2Identity.NodeID))

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: guarantees[2].ID()}), gomock.Eq([]flow.Identifier{collection2Identity.NodeID, collection3Identity.NodeID}))

		ctx.executionState.On("StateCommitmentByBlockID", block.ParentID).Return(unittest.StateCommitmentFixture(), nil)

		err := ctx.engine.ProcessLocal(&block)

		require.NoError(t, err)
	})
}

func TestNoCollectionRequestsIfParentMissing(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()
		//To make sure we always have collection if the block fixture changes
		guarantees := unittest.CollectionGuaranteesFixture(3)

		guarantees[0].SignerIDs = []flow.Identifier{collection1Identity.NodeID, collection3Identity.NodeID}
		guarantees[1].SignerIDs = []flow.Identifier{collection2Identity.NodeID}
		guarantees[2].SignerIDs = []flow.Identifier{collection2Identity.NodeID, collection3Identity.NodeID}

		block.Guarantees = guarantees
		block.PayloadHash = block.Payload.Hash()

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		ctx.state.On("AtBlockID", block.ID()).Return(ctx.snapshot).Maybe()

		ctx.collectionConduit.EXPECT().Submit(gomock.Any(), gomock.Any()).Times(0)

		ctx.executionState.On("StateCommitmentByBlockID", block.ParentID).Return(nil, realStorage.ErrNotFound)

		err := ctx.engine.ProcessLocal(&block)

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

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: id}), gomock.Eq(collection1Identity.NodeID)).Return(nil)
		ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ParentID).Return(executableBlock.StartState, nil)

		err := ctx.engine.ProcessLocal(executableBlock.Block)
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

		//ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ParentID).Return(unittest.StateCommitmentFixture(), realStorage.ErrNotFound)

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

		for _, col := range executableBlock.Block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collection1Identity.NodeID))
		}

		ctx.state.On("AtBlockID", executableBlock.ID()).Return(ctx.snapshot)

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlock.Block))
		ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ParentID).Return(unittest.StateCommitmentFixture(), nil)

		err := ctx.engine.ProcessLocal(executableBlock.Block)
		require.NoError(t, err)

		// Expected no calls so test should fail if any occurs
		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: executableBlock.Collections()[1].Transactions},
		}

		err = ctx.engine.ProcessLocal(&rightResponse)

		require.NoError(t, err)
	})
}
