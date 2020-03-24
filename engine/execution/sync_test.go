package execution_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	testutil2 "github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSyncFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exe1ID, exe2ID, verID}

	genesis := flow.Genesis(identities)

	tx1 := testutil2.DeployCounterContractTransaction()
	tx2 := testutil2.CreateCounterTransaction()
	//tx3 := testutil2.AddToCounterTransaction()

	col1 := flow.Collection{Transactions: []*flow.TransactionBody{&tx1}}
	col2 := flow.Collection{Transactions: []*flow.TransactionBody{&tx2}}
	col3 := flow.Collection{Transactions: []*flow.TransactionBody{&tx2}}

	//collections := map[flow.Identifier]flow.Collection{
	//	col1.ID(): col1,
	//	col2.ID(): col2,
	//	col3.ID(): col3,
	//}

	//Create two blocks, with one tx each
	block1 := &flow.Block{
		Header: flow.Header{
			ParentID: genesis.ID(),
			View:     42,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					CollectionID: col1.ID(),
				},
			},
		},
	}

	block2 := &flow.Block{
		Header: flow.Header{
			ParentID: block1.ParentID,
			View:     44,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					CollectionID: col2.ID(),
				},
			},
		},
	}

	//block3 := &flow.Block{
	//	Header: flow.Header{
	//		ParentID: block2.ParentID,
	//		View:     45,
	//	},
	//	Payload: flow.Payload{
	//		Guarantees: []*flow.CollectionGuarantee{
	//			{
	//				CollectionID: col3.ID(),
	//			},
	//		},
	//	},
	//}

	// exe1 will sync from exe2 after exe2 executes transactions
	exeNode1 := testutil.ExecutionNode(t, hub, exe1ID, identities)
	exeNode2 := testutil.ExecutionNode(t, hub, exe2ID, identities)
	defer exeNode1.Done()
	defer exeNode2.Done()

	collectionNode := testutil.GenericNode(t, hub, colID, identities)
	verificationNode := testutil.GenericNode(t, hub, verID, identities)
	//consensusNode := testutil.GenericNode(t, hub, conID, identities)

	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.CollectionProvider, collectionEngine)

	collectionEngine.On("Submit", exe2ID.NodeID, &messages.CollectionRequest{ID: col1.ID()}).Run(func(args mock.Arguments) {
		colConduit.Submit(&messages.CollectionResponse{Collection: col1,}, exe2ID.NodeID)
	})
	collectionEngine.On("Submit", exe2ID.NodeID, &messages.CollectionRequest{ID: col2.ID()}).Run(func(args mock.Arguments) {
		colConduit.Submit(&messages.CollectionResponse{Collection: col2,}, exe2ID.NodeID)
	})
	collectionEngine.On("Submit", exe1ID.NodeID, &messages.CollectionRequest{ID: col3.ID()}).Run(func(args mock.Arguments) {
		colConduit.Submit(&messages.CollectionResponse{Collection: col3,}, exe2ID.NodeID)
	})

	var receipt *flow.ExecutionReceipt

	// submit block from consensus node
	exeNode2.IngestionEngine.Submit(conID.NodeID, block1)
	time.Sleep(1*time.Second)
	exeNode2.IngestionEngine.Submit(conID.NodeID, block2)

	exeNode2.Net.DeliverAll(true)

	isBlock2Executed := false

	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	verificationEngine.On("Submit", exe2ID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			if receipt.ExecutionResult.BlockID == block2.ID() {
				isBlock2Executed = true
			}
		}).
		Return(nil)

	// wait for block2 to be executed on execNode2
	hub.Eventually(t, func() bool {
		return isBlock2Executed
	})

	head, err := exeNode1.State.Final().Head()
	require.NoError(t, err)
	fmt.Printf("%#v", head)

	collectionEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
}
