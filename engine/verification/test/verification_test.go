package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
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

	completeER := CompleteExecutionResultFixture(t, chunkNum)

	//
	// assigns half of the chunks to this verifier
	a := chmodel.NewAssignment()
	for i := 0; i < chunkNum; i++ {
		if isAssigned(i, chunkNum) {
			chunk, ok := completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(i))
			require.True(t, ok, "chunk out of range requested")
			a.Add(chunk, []flow.Identifier{verNode.Me.NodeID()})
		}
	}
	assigner.On("Assign",
		testifymock.Anything,
		completeER.Receipt.ExecutionResult.Chunks,
		testifymock.Anything).
		Return(a, nil)

	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, identities)
	exeEngine := new(network.Engine)

	exeChunkDataConduit, err := exeNode.Net.Register(engine.ChunkDataPackProvider, exeEngine)
	assert.Nil(t, err)
	exeChunkDataSeen := make(map[flow.Identifier]struct{})

	exeEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if req, ok := args[1].(*messages.ChunkDataPackRequest); ok {
				require.True(t, ok)
				for i := 0; i < chunkNum; i++ {
					chunk, ok := completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(i))
					require.True(t, ok, "chunk out of range requested")
					chunkID := chunk.ID()
					if isAssigned(i, chunkNum) && chunkID == req.ChunkID {
						// each assigned chunk data pack should be requested only once
						_, ok := exeChunkDataSeen[chunkID]
						require.False(t, ok)
						exeChunkDataSeen[chunkID] = struct{}{}

						// publishes the chunk data pack response to the network
						res := &messages.ChunkDataPackResponse{
							Data: *completeER.ChunkDataPacks[i],
						}
						err := exeChunkDataConduit.Submit(res, verIdentity.NodeID)
						assert.Nil(t, err)
						return
					}
				}
				require.Error(t, fmt.Errorf(" requested an unidentifed chunk data pack %v", req))
			}

			require.Error(t, fmt.Errorf("unknown request to execution node %v", args[1]))

		}).
		Return(nil).
		// half of the chunks assigned to the verification node
		// for each chunk the verification node contacts execution node
		// once for chunk data pack
		Times(chunkNum / 2)

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conIdentity, identities)
	conEngine := new(network.Engine)

	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			_, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
		}).
		// half of the chunks are assigned to the verification node
		// for each chunk there is one result approval emitted from verification node
		// to consensus node, hence total number of calls is chunkNum/2
		Return(nil).Times(chunkNum / 2)

	_, err = conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// assumes the verification node has received the block
	err = verNode.BlockStorage.Store(completeER.Block)
	assert.Nil(t, err)

	// injects the collections into the collection node mempool
	for i := 0; i < chunkNum; i++ {
		err = colNode.Collections.Store(completeER.Collections[i])
		assert.Nil(t, err)
	}

	// send the ER from execution to verification node
	receipt1 := completeER.Receipt
	receipt2 := &flow.ExecutionReceipt{
		ExecutorID:        exeIdentity.NodeID,
		ExecutionResult:   completeER.Receipt.ExecutionResult,
		ExecutorSignature: unittest.SignatureFixture(),
	}

	verWG := sync.WaitGroup{}
	verWG.Add(2)

	go func() {
		err := verNode.IngestEngine.Process(exeIdentity.NodeID, receipt1)
		assert.Nil(t, err)
		go verWG.Done()
	}()

	go func() {
		err := verNode.IngestEngine.Process(exeIdentity.NodeID, receipt2)
		assert.Nil(t, err)
		go verWG.Done()
	}()

	// the receipt should be added to the mempool
	// sleep for 1 second to make sure that receipt finds its way to
	// authReceipts pool
	assert.Eventually(t, func() bool {
		return verNode.AuthReceipts.Has(completeER.Receipt.ID())
	}, time.Second, 50*time.Millisecond)

	// flush the collection request
	verNet, ok := hub.GetNetwork(verIdentity.NodeID)
	assert.True(t, ok)
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

	unittest.AssertReturnsBefore(t, verWG.Wait, 3*time.Second)

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// resource cleanup
	//
	for i := 0; i < chunkNum; i++ {
		// associated resources for each chunk should be removed from the mempool
		assert.False(t, verNode.AuthCollections.Has(completeER.Collections[i].ID()))
		assert.False(t, verNode.PendingCollections.Has(completeER.Collections[i].ID()))
		assert.False(t, verNode.ChunkDataPacks.Has(completeER.ChunkDataPacks[i].ID()))
		if isAssigned(i, chunkNum) {
			// chunk ID of assigned chunks should be added to ingested chunks mempool
			assert.True(t, verNode.IngestedChunkIDs.Has(completeER.Receipt.ExecutionResult.Chunks[i].ID()))
		}
	}
	// since all chunks have been ingested, execution receipt should be removed from mempool
	assert.False(t, verNode.PendingReceipts.Has(completeER.Receipt.ID()))
	assert.False(t, verNode.AuthReceipts.Has(completeER.Receipt.ID()))

	// receipt ID should be added to the ingested results mempool
	assert.True(t, verNode.IngestedResultIDs.Has(completeER.Receipt.ExecutionResult.ID()))

	verNode.Done()
	colNode.Done()
	conNode.Done()
	exeNode.Done()
}

// isAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func isAssigned(index int, chunkNum int) bool {
	answer := index >= 0 && index < chunkNum && index%2 == 0
	return answer
}
