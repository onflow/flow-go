package execution_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	identities := flow.IdentityList{colID, conID, exeID}

	genesis := flow.Genesis(identities)

	exeNode := testutil.ExecutionNode(t, hub, exeID, genesis)

	defer func() {
		exeNode.BadgerDB.Close()
		exeNode.LevelDB.SafeClose()
	}()

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

	net, ok := hub.GetNetwork(exeNode.Me.NodeID())
	require.True(t, ok)

	// intercept collection request message
	net.
		OnMessage(
			stub.FromID(exeNode.Me.NodeID()),
			stub.ChannelID(engine.CollectionProvider),
		).
		Do(func(m *stub.PendingMessage) {
			// submit collection from collection node
			exeNode.BlocksEngine.Submit(colID.NodeID, &messages.CollectionResponse{
				Collection: col,
			})
		})

	var receipt *flow.ExecutionReceipt

	// intercept receipt broadcast message
	net.
		OnMessage(
			stub.FromID(exeNode.Me.NodeID()),
			stub.ChannelID(engine.ReceiptProvider),
		).
		Do(func(m *stub.PendingMessage) {
			event, ok := m.Event.(*flow.ExecutionReceipt)
			if ok {
				receipt = event
			}
		})

	// submit block from consensus node
	exeNode.BlocksEngine.Submit(conID.NodeID, block)

	assert.Eventually(t, func() bool {
		return receipt != nil
	}, time.Second*3, time.Millisecond*500)

	assert.Equal(t, block.ID(), receipt.ExecutionResult.BlockID)
}
