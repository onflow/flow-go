package execution_test

import (
	"fmt"
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
		Script: []byte("transaction { execute { log(1) } }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(2) } }"),
	}

	tx3 := flow.TransactionBody{
		Script: []byte("transaction { execute { log(3) } }"),
	}

	tx4 := flow.TransactionBody{
		Script: []byte("transaction { execute { Account(publicKeys: [[248,98,184,91,48,89,48,19,6,7,42,134,72,206,61,2,1,6,8,42,134,72,206,61,3,1,7,3,66,0,4,114,176,116,164,82,208,167,100,161,218,52,49,143,68,203,22,116,13,241,207,171,30,107,80,229,228,20,93,192,110,93,21,28,156,37,36,79,18,62,83,201,182,254,35,117,4,163,126,119,121,144,10,173,83,202,38,227,181,124,92,61,112,48,196,2,3,130,3,232]], code: [112,117,98,32,99,111,110,116,114,97,99,116,32,84,101,115,116,105,110,103,32,123,32,112,117,98,32,114,101,115,111,117,114,99,101,32,67,111,117,110,116,101,114,32,123,32,10,9,9,112,117,98,32,118,97,114,32,99,111,117,110,116,58,32,73,110,116,10,10,9,9,105,110,105,116,40,41,32,123,10,9,9,9,115,101,108,102,46,99,111,117,110,116,32,61,32,48,10,9,9,125,10,9,9,112,117,98,32,102,117,110,32,97,100,100,40,95,32,99,111,117,110,116,58,32,73,110,116,41,32,123,10,9,9,9,115,101,108,102,46,99,111,117,110,116,32,61,32,115,101,108,102,46,99,111,117,110,116,32,43,32,99,111,117,110,116,10,9,9,125,32,125,10,10,9,9,112,117,98,32,102,117,110,32,99,114,101,97,116,101,67,111,117,110,116,101,114,40,41,58,32,64,67,111,117,110,116,101,114,32,123,10,9,9,9,32,32,114,101,116,117,114,110,32,60,45,99,114,101,97,116,101,32,67,111,117,110,116,101,114,40,41,10,9,9,32,32,125,32,125]) } }"),
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
			Number:   42,
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

	verEngine := new(network.Engine)
	_, _ = verNode.Net.Register(engine.ReceiptProvider, verEngine)
	verEngine.On("Process", exeID.NodeID, mock.Anything).
		Run(func(args mock.Arguments) {
			receipt, _ = args[1].(*flow.ExecutionReceipt)

			assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
		}).
		Return(nil).
		Once()

	conEngine := new(network.Engine)
	_, _ = conNode.Net.Register(engine.ReceiptProvider, conEngine)
	conEngine.On("Process", exeID.NodeID, mock.Anything).
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
	exeNode.BlocksEngine.Submit(conID.NodeID, block)

	assert.Eventually(t, func() bool { return receipt != nil }, time.Second*3, time.Millisecond*500)

	res, err := exeNode.ExecutionEngine.ExecuteScript([]byte(
		"import 0x01\npub fun main(): Int { return getAccount(0x01).published[&Testing.Counter]?.count ?? -3 }",
	))
	fmt.Println(res)
	assert.NoError(t, err)

}
