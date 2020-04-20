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
	mock2 "github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestHappyPath checks that concurrently received execution receipts with the same result part that
// received by the verification node results in:
// - the selection of the assigned chunks by the ingest engine
// - request of the associated collections to the assigned chunks
// - formation of a complete verifiable chunk by the ingest engine for each assigned chunk
// - submitting a verifiable chunk locally to the verify engine by the ingest engine
// - dropping the ingestion of the ERs that share the same result once the verifiable chunk is submitted to verify engine
// - broadcast of a matching result approval to consensus nodes for each assigned chunk
func TestHappyPath(t *testing.T) {
	// generates network hub
	hub := stub.NewNetworkHub()

	// generates identities of nodes, one of each type
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]

	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity, verIdentity}

	// Execution receipt and chunk assignment
	//
	// creates an execution receipt and its associated data
	// with `chunkNum` chunks
	chunkNum := 10
	completeER := CompleteExecutionResultFixture(t, chunkNum)

	// mocks the assignment to only assign "some" chunks to the verIdentity
	// the assignment is done based on `isAssgined` function
	assigner := &mock.ChunkAssigner{}
	a := chmodel.NewAssignment()
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if isAssigned(int(chunk.Index), chunkNum) {
			a.Add(chunk, []flow.Identifier{verIdentity.NodeID})
		}
	}
	assigner.On("Assign",
		testifymock.Anything,
		completeER.Receipt.ExecutionResult.Chunks,
		testifymock.Anything).
		Return(a, nil)

	// nodes and engines
	//
	// verification node
	verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner)
	// assumes the verification node has received the block
	err := verNode.BlockStorage.Store(completeER.Block)
	assert.Nil(t, err)

	// collection node
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)
	// injects the assigned collections into the collection node mempool
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if isAssigned(int(chunk.Index), chunkNum) {
			err = colNode.Collections.Store(completeER.Collections[chunk.Index])
			assert.Nil(t, err)
		}
	}

	// mock execution node
	exeNode, exeEngine := setupMockExeNode(t, hub, exeIdentity, verIdentity, identities, completeER)

	// mock consensus node
	conNode, conEngine := setupMockConsensusNode(t, hub, conIdentity, verIdentity, identities, completeER)

	// duplicates the ER (receipt1) into another ER (receipt2)
	// that share their result part
	receipt1 := completeER.Receipt
	receipt2 := &flow.ExecutionReceipt{
		ExecutorID:        exeIdentity.NodeID,
		ExecutionResult:   completeER.Receipt.ExecutionResult,
		ExecutorSignature: unittest.SignatureFixture(),
	}

	// invoking verification node with two receipts
	// concurrently
	//
	verWG := sync.WaitGroup{}
	verWG.Add(2)

	// receipt1
	go func() {
		err := verNode.IngestEngine.Process(exeIdentity.NodeID, receipt1)
		assert.Nil(t, err)
		go verWG.Done()
	}()
	// receipt2
	go func() {
		err := verNode.IngestEngine.Process(exeIdentity.NodeID, receipt2)
		assert.Nil(t, err)
		go verWG.Done()
	}()

	// both receipts should be added to the authenticated mempool of verification node
	// and do not reside in pending pool
	// sleeps for 50 milliseconds to make sure that receipt finds its way to authReceipts pool
	assert.Eventually(t, func() bool {
		return verNode.AuthReceipts.Has(receipt1.ID()) &&
			verNode.AuthReceipts.Has(receipt2.ID()) &&
			!verNode.PendingReceipts.Has(receipt1.ID()) &&
			!verNode.PendingReceipts.Has(receipt2.ID())
	}, time.Second, 50*time.Millisecond)

	// flush the collection requests
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
	// since all chunks have been ingested, neither of execution receipts should reside on any mempool
	assert.False(t, verNode.PendingReceipts.Has(receipt1.ID()))
	assert.False(t, verNode.AuthReceipts.Has(receipt1.ID()))
	assert.False(t, verNode.PendingReceipts.Has(receipt2.ID()))
	assert.False(t, verNode.AuthReceipts.Has(receipt2.ID()))

	// result ID should be added to the ingested results mempool
	assert.True(t, verNode.IngestedResultIDs.Has(completeER.Receipt.ExecutionResult.ID()))

	verNode.Done()
	colNode.Done()
	conNode.Done()
	exeNode.Done()
}

// TestSingleCollectionProcessing checks the full happy
// path assuming a single collection (including transactions on counter example)
// are submited to the verification node.
func TestSingleCollectionProcessing(t *testing.T) {

	// network identity setup
	hub := stub.NewNetworkHub()
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]
	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity, verIdentity}

	// complete ER counter example
	completeER := GetCompleteExecutionResultForCounter(t)
	chunk, ok := completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(0))
	assert.True(t, ok)

	// assigner and assignment
	assigner := &mock.ChunkAssigner{}
	assignment := chmodel.NewAssignment()
	assignment.Add(chunk, []flow.Identifier{verIdentity.NodeID})
	assigner.On("Assign",
		testifymock.Anything,
		completeER.Receipt.ExecutionResult.Chunks,
		testifymock.Anything).
		Return(assignment, nil)

	// setup nodes

	// verification node
	verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner)
	// inject block
	err := verNode.BlockStorage.Store(completeER.Block)
	assert.Nil(t, err)

	// collection node
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)
	// inject the collection
	err = colNode.Collections.Store(completeER.Collections[0])
	assert.Nil(t, err)

	// execution node
	exeNode := testutil.GenericNode(t, hub, exeIdentity, identities)
	exeEngine := new(network.Engine)
	exeChunkDataConduit, err := exeNode.Net.Register(engine.ChunkDataPackProvider, exeEngine)
	assert.Nil(t, err)
	exeEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if _, ok := args[1].(*messages.ChunkDataPackRequest); ok {
				// publishes the chunk data pack response to the network
				res := &messages.ChunkDataPackResponse{
					Data: *completeER.ChunkDataPacks[0],
				}
				err := exeChunkDataConduit.Submit(res, verIdentity.NodeID)
				assert.Nil(t, err)
			}
		}).Return(nil).Times(1)

	// consensus node
	conNode := testutil.GenericNode(t, hub, conIdentity, identities)
	conEngine := new(network.Engine)
	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			_, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
		}).Return(nil).Times(1)

	_, err = conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// send the ER from execution to verification node
	err = verNode.IngestEngine.Process(exeIdentity.NodeID, completeER.Receipt)
	assert.Nil(t, err)

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

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// receipt ID should be added to the ingested results mempool
	assert.True(t, verNode.IngestedResultIDs.Has(completeER.Receipt.ExecutionResult.ID()))

	verNode.Done()
	colNode.Done()
	conNode.Done()
	exeNode.Done()
// setupMockExeNode creates and returns an execution node and its registered engine in the network (hub)
// it mocks the process method of execution node that on receiving a chunk data pack request from
// a certain verifier node (verIdentity) about a chunk that is assigned to it, replies the chunk back
// data pack back to the node. Otherwise, if the request is not a chunk data pack request, or if the
// requested chunk data pack is not about an assigned chunk to the verifier node (verIdentity), it fails the
// test.
func setupMockExeNode(t *testing.T,
	hub *stub.Hub,
	exeIdentity *flow.Identity,
	verIdentity *flow.Identity,
	othersIdentity flow.IdentityList,
	completeER verification.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine) {
	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, othersIdentity)
	exeEngine := new(network.Engine)

	exeChunkDataSeen := make(map[flow.Identifier]struct{})

	exeChunkDataConduit, err := exeNode.Net.Register(engine.ChunkDataPackProvider, exeEngine)
	assert.Nil(t, err)

	chunkNum := len(completeER.ChunkDataPacks)
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

	return &exeNode, exeEngine
}

// setupMockConsensusNode creates and returns a mock consensus node (conIdentity) and its registered engine in the
// network (hub). It mocks the process method of the consensus engine to receive a message from a certain
// verification node (verIdentity) evaluates whether it is a result approval about an assigned chunk to that verifier node.
func setupMockConsensusNode(t *testing.T,
	hub *stub.Hub,
	conIdentity *flow.Identity,
	verIdentity *flow.Identity,
	othersIdentity flow.IdentityList,
	completeER verification.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine) {

	// determines the expected number of result approvals this node should receive
	approvalsCount := 0
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if isAssigned(len(completeER.ChunkDataPacks), int(chunk.Index)) {
			approvalsCount++
		}
	}

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conIdentity, othersIdentity)
	conEngine := new(network.Engine)

	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			resultApproval, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)

			// asserts that the result approval is assigned to the verifier
			assert.True(t, isAssigned(int(resultApproval.Body.ChunkIndex), len(completeER.ChunkDataPacks)))
		}).Return(nil).Times(approvalsCount)

	_, err := conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	return &conNode, conEngine
}

// isAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func isAssigned(index int, chunkNum int) bool {
	answer := index >= 0 && index < chunkNum && index%2 == 0
	return answer
}
