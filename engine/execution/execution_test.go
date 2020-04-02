package execution_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exeID, verID}

	genesis := flow.Genesis(identities)

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(1) } }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(2) } }"),
	}

	tx3 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(3) } }"),
	}

	tx4 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(4) } }"),
	}

	col1 := flow.Collection{Transactions: []*flow.TransactionBody{&tx1, &tx2}}
	col2 := flow.Collection{Transactions: []*flow.TransactionBody{&tx3, &tx4}}

	collections := map[flow.Identifier]flow.Collection{
		col1.ID(): col1,
		col2.ID(): col2,
	}

	block := &flow.Block{
		Header: flow.Header{
			ParentID: genesis.ID(),
			View:     42,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					CollectionID: col1.ID(),
				},
				{
					CollectionID: col2.ID(),
				},
			},
		},
	}

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21)
	defer exeNode.Done()

	collectionNode := testutil.GenericNode(t, hub, colID, identities)
	verificationNode := testutil.GenericNode(t, hub, verID, identities)
	consensusNode := testutil.GenericNode(t, hub, conID, identities)

	collectionEngine := new(network.Engine)
	colConduit, _ := collectionNode.Net.Register(engine.CollectionProvider, collectionEngine)
	collectionEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			originID, _ := args[0].(flow.Identifier)
			req, _ := args[1].(*messages.CollectionRequest)

			col, exists := collections[req.ID]
			assert.True(t, exists)

			res := &messages.CollectionResponse{
				Collection: col,
			}

			err := colConduit.Submit(res, originID)
			assert.NoError(t, err)
		}).
		Return(nil).
		Times(len(collections))

	var receipt *flow.ExecutionReceipt

	verificationEngine := new(network.Engine)
	_, _ = verificationNode.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	consensusEngine := new(network.Engine)
	_, _ = consensusNode.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	consensusEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
			assert.Equal(t, len(collections), len(receipt.ExecutionResult.Chunks))

			for i, chunk := range receipt.ExecutionResult.Chunks {
				assert.EqualValues(t, i, chunk.CollectionIndex)
			}
		}).
		Return(nil).
		Once()

	// submit block from consensus node
	exeNode.IngestionEngine.Submit(conID.NodeID, block)

	assert.Eventually(t, func() bool {
		hub.DeliverAll()
		return receipt != nil
	}, time.Second*10, time.Millisecond*500)

	collectionEngine.AssertExpectations(t)
	verificationEngine.AssertExpectations(t)
	consensusEngine.AssertExpectations(t)

	collectionNode.Done()
	verificationNode.Done()
	consensusNode.Done()
	exeNode.Done()
}

func TestBlockIngestionMultipleConsensusNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	con1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	con2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := flow.IdentityList{colID, con1ID, con2ID, exeID}

	genesis := flow.Genesis(identities)

	block2 := &flow.Block{
		Header: flow.Header{
			ParentID:   genesis.ID(),
			View:       2,
			Height:     2,
			ProposerID: con1ID.ID(),
		},
	}
	//fork := &flow.Block{
	//	Header: flow.Header{
	//		ParentID:   genesis.ID(),
	//		View:       2,
	//		Height:     2,
	//		ProposerID: con2ID.ID(),
	//	},
	//}
	block3 := &flow.Block{
		Header: flow.Header{
			ParentID:   block2.ID(),
			View:       3,
			Height:     3,
			ProposerID: con2ID.ID(),
		},
	}

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21)
	defer exeNode.Done()

	consensus1Node := testutil.GenericNode(t, hub, con1ID, identities)
	consensus2Node := testutil.GenericNode(t, hub, con2ID, identities)

	actualCalls := 0

	consensusEngine := new(network.Engine)
	_, _ = consensus1Node.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	_, _ = consensus2Node.Net.Register(engine.ExecutionReceiptProvider, consensusEngine)
	consensusEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) { actualCalls++ }).
		Return(nil)

	// TODO Execution Engine is able to work on forks, but full test cannot be enabled
	// due to Consensus Follower not fully implemented
	exeNode.IngestionEngine.Submit(con1ID.NodeID, block2)
	//exeNode.IngestionEngine.Submit(con1ID.NodeID, block3) // block 3 cannot be executed if parent (block2 is missing)
	//exeNode.IngestionEngine.Submit(con1ID.NodeID, fork) // block 3 cannot be executed if parent (block2 is missing)
	hub.Eventually(t, equal(2, &actualCalls))

	exeNode.IngestionEngine.Submit(con2ID.NodeID, block3)
	hub.Eventually(t, equal(4, &actualCalls))

	var res flow.Identifier
	err := exeNode.BadgerDB.View(operation.RetrieveNumber(2, &res))
	require.NoError(t, err)
	require.Equal(t, block2.ID(), res)

	err = exeNode.BadgerDB.View(operation.RetrieveNumber(3, &res))
	require.NoError(t, err)
	require.Equal(t, block3.ID(), res)

	consensusEngine.AssertExpectations(t)

	consensus1Node.Done()
	consensus2Node.Done()
	exeNode.Done()
}

func TestBroadcastToMultipleVerificationNodes(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	ver2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, exeID, ver1ID, ver2ID}

	genesis := flow.Genesis(identities)

	block := &flow.Block{
		Header: flow.Header{
			ParentID: genesis.ID(),
			View:     42,
		},
	}

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities, 21)
	defer exeNode.Done()

	verification1Node := testutil.GenericNode(t, hub, ver1ID, identities)
	verification2Node := testutil.GenericNode(t, hub, ver2ID, identities)

	actualCalls := 0

	var receipt *flow.ExecutionReceipt

	verificationEngine := new(network.Engine)
	_, _ = verification1Node.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	_, _ = verification2Node.Net.Register(engine.ExecutionReceiptProvider, verificationEngine)
	verificationEngine.On("Submit", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			actualCalls++

			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil)

	exeNode.IngestionEngine.SubmitLocal(block)

	hub.Eventually(t, equal(2, &actualCalls))

	verificationEngine.AssertExpectations(t)

	verification1Node.Done()
	verification2Node.Done()
	exeNode.Done()
}

func equal(expected int, actual *int) func() bool {
	return func() bool {
		fmt.Printf("expect %d got %d\n", expected, *actual)
		return expected == *actual
	}
}
