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

func TestHappyPath(t *testing.T) {
	testcases := []struct {
		verNodeCount,
		chunkCount int
	}{
		{
			verNodeCount: 1,
			chunkCount:   2,
		},
		{
			verNodeCount: 1,
			chunkCount:   10,
		},
		{
			verNodeCount: 2,
			chunkCount:   2,
		},
		{
			verNodeCount: 2,
			chunkCount:   10,
		},
		{
			verNodeCount: 10,
			chunkCount:   2,
		},
		{
			verNodeCount: 10,
			chunkCount:   10,
		},
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%d-verification node %d-chunk number", tc.verNodeCount, tc.chunkCount), func(t *testing.T) {
			testHappyPath(t, tc.verNodeCount, tc.chunkCount)
		})
	}
}

// TestHappyPath checks that concurrently received execution receipts with the same result part that
// received by the verification node results in:
// - the selection of the assigned chunks by the ingest engine
// - request of the associated collections to the assigned chunks
// - formation of a complete verifiable chunk by the ingest engine for each assigned chunk
// - submitting a verifiable chunk locally to the verify engine by the ingest engine
// - dropping the ingestion of the ERs that share the same result once the verifiable chunk is submitted to verify engine
// - broadcast of a matching result approval to consensus nodes for each assigned chunk
func testHappyPath(t *testing.T, verNodeCount int, chunkNum int) {
	// generates network hub
	hub := stub.NewNetworkHub()

	// generates identities of nodes, one of each type, `verNodeCount` many of verification nodes
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentities := unittest.IdentityListFixture(verNodeCount, unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]

	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity}
	identities = append(identities, verIdentities...)

	// Execution receipt and chunk assignment
	//
	// creates an execution receipt and its associated data
	// with `chunkNum` chunks
	completeER := CompleteExecutionResultFixture(t, chunkNum)

	// mocks the assignment to only assign "some" chunks to the verIdentity
	// the assignment is done based on `isAssgined` function
	assigner := &mock.ChunkAssigner{}
	a := chmodel.NewAssignment()
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		assignees := make([]flow.Identifier, 0)
		for _, verIdentity := range verIdentities {
			if isAssigned(int(chunk.Index), chunkNum) {
				assignees = append(assignees, verIdentity.NodeID)
			}
		}
		a.Add(chunk, assignees)
	}
	assigner.On("Assign",
		testifymock.Anything,
		completeER.Receipt.ExecutionResult.Chunks,
		testifymock.Anything).
		Return(a, nil)

	// nodes and engines
	//
	// verification node
	verNodes := make([]mock2.VerificationNode, 0)
	for _, verIdentity := range verIdentities {
		verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner)

		// assumes the verification node has received the block
		err := verNode.BlockStorage.Store(completeER.Block)
		assert.Nil(t, err)

		verNodes = append(verNodes, verNode)
	}

	// collection node
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)
	// injects the assigned collections into the collection node mempool
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if isAssigned(int(chunk.Index), chunkNum) {
			err := colNode.Collections.Store(completeER.Collections[chunk.Index])
			assert.Nil(t, err)
		}
	}

	// mock execution node
	exeNode, exeEngine := setupMockExeNode(t, hub, exeIdentity, verIdentities, identities, completeER)

	// mock consensus node
	conNode, conEngine := setupMockConsensusNode(t, hub, conIdentity, verIdentities, identities, completeER)

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

	for _, verNode := range verNodes {
		verWG.Add(2)

		// receipt1
		go func(vn mock2.VerificationNode) {
			defer verWG.Done()
			err := vn.IngestEngine.Process(exeIdentity.NodeID, receipt1)
			assert.Nil(t, err)

		}(verNode)

		// receipt2
		go func(vn mock2.VerificationNode) {
			defer verWG.Done()
			err := vn.IngestEngine.Process(exeIdentity.NodeID, receipt2)
			assert.Nil(t, err)
		}(verNode)
	}

	unittest.AssertReturnsBefore(t, verWG.Wait, 3*time.Second)

	for _, verNode := range verNodes {
		// both receipts should be added to the authenticated mempool of verification node
		require.True(t, verNode.AuthReceipts.Has(receipt1.ID()) && verNode.AuthReceipts.Has(receipt2.ID()))
		// and do not reside in pending pool
		require.False(t, verNode.PendingReceipts.Has(receipt1.ID()) || verNode.PendingReceipts.Has(receipt2.ID()))
	}

	// flush the collection requests
	verNets := make([]*stub.Network, 0)
	for _, verIdentity := range verIdentities {
		verNet, ok := hub.GetNetwork(verIdentity.NodeID)
		assert.True(t, ok)
		verNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
			return m.ChannelID == engine.CollectionProvider
		})

		verNets = append(verNets, verNet)
	}

	// flush the collection response
	colNet, ok := hub.GetNetwork(colIdentity.NodeID)
	assert.True(t, ok)
	colNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
		return m.ChannelID == engine.CollectionProvider
	})

	// flush the result approval broadcast
	for _, verNet := range verNets {
		verNet.DeliverAll(true)
	}

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// resource cleanup
	//
	for _, verNode := range verNodes {
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
	}
	colNode.Done()
	conNode.Done()
	exeNode.Done()
}

// setupMockExeNode creates and returns an execution node and its registered engine in the network (hub)
// it mocks the process method of execution node that on receiving a chunk data pack request from
// a certain verifier node (verIdentity) about a chunk that is assigned to it, replies the chunk back
// data pack back to the node. Otherwise, if the request is not a chunk data pack request, or if the
// requested chunk data pack is not about an assigned chunk to the verifier node (verIdentity), it fails the
// test.
func setupMockExeNode(t *testing.T,
	hub *stub.Hub,
	exeIdentity *flow.Identity,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	completeER verification.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine) {
	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, othersIdentity)
	exeEngine := new(network.Engine)

	// determines the expected number of result approvals this node should receive
	approvalsCount := 0
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if isAssigned(len(completeER.ChunkDataPacks), int(chunk.Index)) {
			approvalsCount++
		}
	}

	// map form verIds --> chunks they asked
	exeChunkDataSeen := make(map[flow.Identifier]map[flow.Identifier]struct{})
	for _, verIdentity := range verIdentities {
		exeChunkDataSeen[verIdentity.NodeID] = make(map[flow.Identifier]struct{})
	}

	exeChunkDataConduit, err := exeNode.Net.Register(engine.ChunkDataPackProvider, exeEngine)
	assert.Nil(t, err)

	chunkNum := len(completeER.ChunkDataPacks)

	exeEngine.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if originID, ok := args[0].(flow.Identifier); ok {
				if req, ok := args[1].(*messages.ChunkDataPackRequest); ok {
					require.True(t, ok)
					for i := 0; i < chunkNum; i++ {
						chunk, ok := completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(i))
						require.True(t, ok, "chunk out of range requested")
						chunkID := chunk.ID()
						if isAssigned(i, chunkNum) && chunkID == req.ChunkID {
							// each assigned chunk data pack should be requested only once
							_, ok := exeChunkDataSeen[originID][chunkID]
							require.False(t, ok)

							// marks execution chunk data pack request as seen
							exeChunkDataSeen[originID][chunkID] = struct{}{}

							// publishes the chunk data pack response to the network
							res := &messages.ChunkDataPackResponse{
								Data: *completeER.ChunkDataPacks[i],
							}
							err := exeChunkDataConduit.Submit(res, originID)
							assert.Nil(t, err)
							return
						}
					}
					require.Error(t, fmt.Errorf(" requested an unidentifed chunk data pack %v", req))
				}
			}

			require.Error(t, fmt.Errorf("unknown request to execution node %v", args[1]))

		}).
		Return(nil).
		// half of the chunks assigned to the verification node
		// for each chunk the verification node contacts execution node
		// once for chunk data pack
		Times(len(verIdentities) * approvalsCount)

	return &exeNode, exeEngine
}

// setupMockConsensusNode creates and returns a mock consensus node (conIdentity) and its registered engine in the
// network (hub). It mocks the process method of the consensus engine to receive a message from a certain
// verification node (verIdentity) evaluates whether it is a result approval about an assigned chunk to that verifier node.
func setupMockConsensusNode(t *testing.T,
	hub *stub.Hub,
	conIdentity *flow.Identity,
	verIdentities flow.IdentityList,
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

	// map form verIds --> result approval ID
	resultApprovalSeen := make(map[flow.Identifier]map[flow.Identifier]struct{})
	for _, verIdentity := range verIdentities {
		resultApprovalSeen[verIdentity.NodeID] = make(map[flow.Identifier]struct{})
	}

	conEngine.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			originID, ok := args[0].(flow.Identifier)
			assert.True(t, ok)

			resultApproval, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)

			// asserts that result approval has not been seen from this
			_, ok = resultApprovalSeen[originID][resultApproval.ID()]
			assert.False(t, ok)

			// marks result approval as seen
			resultApprovalSeen[originID][resultApproval.ID()] = struct{}{}

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
