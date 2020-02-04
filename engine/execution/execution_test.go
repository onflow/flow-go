package execution_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()
	hub.EnableSyncDelivery()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exeID, verID}

	genesis := flow.Genesis(identities)

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signatures:   nil,
	}

	block := &flow.Block{
		Header: flow.Header{
			ParentID: genesis.ID(),
			Number:   42,
		},
		Payload: flow.Payload{
			Identities: nil,
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	exeNode := testutil.ExecutionNode(t, hub, exeID, identities)
	defer exeNode.Done()

	colNode := testutil.GenericNode(t, hub, colID, identities)
	verNode := testutil.GenericNode(t, hub, verID, identities)
	conNode := testutil.GenericNode(t, hub, conID, identities)

	colEngine := new(network.Engine)
	colConduit, _ := colNode.Net.Register(engine.CollectionProvider, colEngine)
	colEngine.On("Process", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			originID, _ := args[0].(flow.Identifier)
			req, _ := args[1].(*messages.CollectionRequest)

			assert.Equal(t, col.ID(), req.ID)

			res := &messages.CollectionResponse{
				Collection: col,
			}

			err := colConduit.Submit(res, originID)
			assert.NoError(t, err)
		}).
		Return(nil).
		Once()

	var receipt *flow.ExecutionReceipt

	verEngine := new(network.Engine)
	_, _ = verNode.Net.Register(engine.ReceiptProvider, verEngine)
	verEngine.On("Process", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			req, _ := args[1].(*flow.ExecutionReceipt)

			receipt = req
			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	conEngine := new(network.Engine)
	_, _ = conNode.Net.Register(engine.ReceiptProvider, conEngine)
	conEngine.On("Process", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			req, _ := args[1].(*flow.ExecutionReceipt)

			receipt = req
			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	// submit block from consensus node
	exeNode.BlocksEngine.Submit(conID.NodeID, block)

	assert.Eventually(t, func() bool { return receipt != nil }, time.Second*3, time.Millisecond*500)
}
