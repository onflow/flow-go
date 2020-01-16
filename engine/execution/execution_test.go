package execution_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFlow(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleCollection
	})
	conID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleConsensus
	})
	exeID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleExecution
	})

	identities := flow.IdentityList{colID, conID, exeID}

	genesis := mock.Genesis(identities)

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
		Content: flow.Content{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	// submit block from consensus node
	exeNode.BlocksEngine.Submit(conID.NodeID, block)

	// wait for blocks engine to finish processing
	exeNode.BlocksEngine.Wait()

	// submit collection from collection node
	exeNode.BlocksEngine.Submit(colID.NodeID, &messages.CollectionResponse{
		Collection: col,
	})

	// wait for blocks engine to finish processing
	exeNode.BlocksEngine.Wait()

	// wait for execution engine to finish processing
	exeNode.ExecutionEngine.Wait()

	// wait for receipt engine to finish processing
	exeNode.ReceiptsEngine.Wait()

	net, ok := hub.GetNetwork(exeNode.Me.NodeID())
	require.True(t, ok)

	var receipt *flow.ExecutionReceipt

	// intercept execution receipt
	net.FlushAllExcept(func(message *stub.PendingMessage) bool {
		event, ok := message.Event.(*flow.ExecutionReceipt)
		if ok {
			receipt = event
			return false
		}

		return true
	})

	require.NotNil(t, receipt)

	// view := exeNode.State.NewView(receipt.ExecutionResult.FinalStateCommitment)

	// _, err = exeNode.VM.NewBlockContext(block).ExecuteScript(view, []byte(`
	// 	pub fun main(): Int {
	// 	  return 5
	// 	}
	// `))
}
