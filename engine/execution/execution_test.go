package execution_test

import (
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
	"github.com/dapperlabs/flow-go/storage/badger"
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

	// TODO: index guarantees when receiving a block
	// this is a temporary patch to prevent a storage error
	// https://github.com/dapperlabs/flow-go/issues/2355
	guarantees := badger.NewGuarantees(exeNode.DB)
	for _, g := range block.Guarantees {
		err := guarantees.Store(g)
		require.NoError(t, err)
	}

	// submit block from consensus node
	exeNode.BlocksEngine.Submit(conID.NodeID, block)

	assert.Eventually(t, func() bool { return receipt != nil }, time.Second*3, time.Millisecond*500)
}
