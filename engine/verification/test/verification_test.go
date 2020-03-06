package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// checks that an execution result received by the verification node results in:
// - request of the appropriate collection
// - formation of a complete execution result by the ingest engine
// - broadcast of a matching result approval to consensus nodes
func TestHappyPath(t *testing.T) {
	hub := stub.NewNetworkHub()

	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]

	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity, verIdentity}

	verNode := testutil.VerificationNode(t, hub, verIdentity, identities)
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)

	completeER := unittest.CompleteExecutionResultFixture()

	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, identities)
	exeEngine := new(network.Engine)
	exeConduit, err := exeNode.Net.Register(engine.ExecutionStateProvider, exeEngine)
	assert.Nil(t, err)
	exeEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			req, ok := args[1].(*messages.ExecutionStateRequest)
			require.True(t, ok)
			assert.Equal(t, completeER.Receipt.ExecutionResult.Chunks.ByIndex(0).ID(), req.ChunkID)

			res := &messages.ExecutionStateResponse{
				State: *completeER.ChunkStates[0],
			}
			err := exeConduit.Submit(res, verIdentity.NodeID)
			assert.Nil(t, err)
		}).
		Return(nil).
		Once()

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conIdentity, identities)
	conEngine := new(network.Engine)
	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			ra, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
			assert.Equal(t, completeER.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Return(nil).
		Once()
	_, err = conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// assume the verification node has received the block
	err = verNode.Blocks.Add(completeER.Block)
	assert.Nil(t, err)

	// inject the collection into the collection node mempool
	err = colNode.Collections.Store(completeER.Collections[0])
	assert.Nil(t, err)

	// send the ER from execution to verification node
	err = verNode.IngestEngine.Process(exeIdentity.NodeID, completeER.Receipt)
	assert.Nil(t, err)

	// the receipt should be added to the mempool
	assert.True(t, verNode.Receipts.Has(completeER.Receipt.ID()))

	// flush the chunk state request
	verNet, ok := hub.GetNetwork(verIdentity.NodeID)
	assert.True(t, ok)
	verNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.ExecutionStateProvider
	})

	// flush the chunk state response
	exeNet, ok := hub.GetNetwork(exeIdentity.NodeID)
	assert.True(t, ok)
	exeNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.ExecutionStateProvider
	})

	// the chunk state should be added to the mempool
	assert.True(t, verNode.ChunkStates.Has(completeER.ChunkStates[0].ID()))

	// flush the collection request
	verNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.CollectionProvider
	})

	// flush the collection response
	colNet, ok := hub.GetNetwork(colIdentity.NodeID)
	assert.True(t, ok)
	colNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.CollectionProvider
	})

	// flush the result approval broadcast
	verNet.DeliverAll(true)

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// the receipt should be removed from the mempool
	assert.False(t, verNode.Receipts.Has(completeER.Receipt.ID()))
	// associated resources should be removed from the mempool
	assert.False(t, verNode.Collections.Has(completeER.Collections[0].ID()))
	assert.False(t, verNode.ChunkStates.Has(completeER.ChunkStates[0].ID()))
	assert.False(t, verNode.Blocks.Has(completeER.Block.ID()))
}
