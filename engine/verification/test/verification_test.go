package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// checks that an execution result received by the verification node results in:
// - request of the appropriate collection
// - selection of the assigned chunks by the ingest engine
// - formation of a complete verifiable chunk by the ingest engine for each assigned chunk
// - submitting a verifiable chunk locally to the verify engine by the ingest engine
// - broadcast of a matching result approval to consensus nodes fo each assigned chunk
func TestHappyPath(t *testing.T) {
	// number of chunks in an ER
	chunkNum := 10
	hub := stub.NewNetworkHub()

	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]

	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity, verIdentity}

	assigner := &mock.ChunkAssigner{}
	verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner)
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)

	completeER := unittest.CompleteExecutionResultFixture(chunkNum)

	// assigns half of the chunks to this verifier
	a := chunkassignment.NewAssignment()
	for i := 0; i < chunkNum; i++ {
		if isAssigned(i, chunkNum) {
			a.Assign(completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(i)), []flow.Identifier{verNode.Me.NodeID()})
		}
	}
	assigner.On("Assigner",
		testifymock.Anything,
		completeER.Receipt.ExecutionResult.Chunks,
		testifymock.Anything).
		Return(a, nil)

	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, identities)
	exeEngine := new(network.Engine)
	exeConduit, err := exeNode.Net.Register(engine.ExecutionStateProvider, exeEngine)
	exeChunkSeen := make(map[flow.Identifier]struct{})
	assert.Nil(t, err)
	exeEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			req, ok := args[1].(*messages.ExecutionStateRequest)
			require.True(t, ok)
			for i := 0; i < chunkNum; i++ {
				chunkID := completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(i)).ID()
				if isAssigned(i, chunkNum) && chunkID == req.ChunkID {
					// each assigned chunk should be requested only once
					_, ok := exeChunkSeen[chunkID]
					require.False(t, ok)
					exeChunkSeen[chunkID] = struct{}{}

					// publishes the execution state response to the network
					res := &messages.ExecutionStateResponse{
						State: *completeER.ChunkStates[i],
					}
					err := exeConduit.Submit(res, verIdentity.NodeID)
					assert.Nil(t, err)
					return
				}
			}
			require.Error(t, fmt.Errorf(" requested an unidentifed chunk %v", req))
		}).
		Return(nil).
		Times(chunkNum)

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conIdentity, identities)
	conEngine := new(network.Engine)

	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			_, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
			// assert.Equal(t, completeER.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Return(nil).Times(chunkNum / 2)

	_, err = conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// assume the verification node has received the block
	err = verNode.Blocks.Add(completeER.Block)
	assert.Nil(t, err)

	// inject the collections into the collection node mempool
	for i := 0; i < chunkNum; i++ {
		err = colNode.Collections.Store(completeER.Collections[i])
		assert.Nil(t, err)
	}

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

	// only assigned chunks state should be added to the mempool
	for i := 0; i < chunkNum; i++ {
		if isAssigned(i, chunkNum) {
			assert.True(t, verNode.ChunkStates.Has(completeER.ChunkStates[i].ID()))
		} else {
			assert.False(t, verNode.ChunkStates.Has(completeER.ChunkStates[i].ID()))
		}
	}

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

	// associated resources should be removed from the mempool
	for i := 0; i < chunkNum; i++ {
		assert.False(t, verNode.Collections.Has(completeER.Collections[i].ID()))
	}
	// TODO adding complementary tests for claning other resources like the execution receipt
	// https://github.com/dapperlabs/flow-go/issues/2750
}

// isAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func isAssigned(index int, chunkNum int) bool {
	assigned := index >= 0 && index < chunkNum && index%2 == 0
	return answer
}
