package test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	mock2 "github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/logging"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestHappyPath evaluates the happy path scenario of
// concurrently sending two execution receipts of the same result each
// with `chunkCount`-many chunks to `verNodeCount`-many verification nodes
// the happy path should result in dissemination of a result approval for each
// distinct chunk by each verification node. The result approvals should be
// sent to the consensus nodes
//
// NOTE: some test cases are meant to solely run locally when FLOWLOCAL environmental
// variable is set to TRUE
func TestHappyPath(t *testing.T) {
	// TODO broken test
	// will be addressed in
	// https://github.com/dapperlabs/flow-go/issues/3613
	t.SkipNow()
	testcases := []struct {
		verNodeCount,
		chunkCount int
		ci bool // indicates if this test should run on CI
	}{
		{
			verNodeCount: 1,
			chunkCount:   2,
			ci:           true,
		},
		{
			verNodeCount: 1,
			chunkCount:   10,
			ci:           true,
		},
		{
			verNodeCount: 2,
			chunkCount:   2,
			ci:           true,
		},
		{
			verNodeCount: 2,
			chunkCount:   10,
			ci:           true,
		},
		{
			verNodeCount: 5,
			chunkCount:   2,
			ci:           false,
		},
		{
			verNodeCount: 5,
			chunkCount:   10,
			ci:           false,
		},
	}

	for _, tc := range testcases {
		if os.Getenv("FLOWLOCAL") != "TRUE" && !tc.ci {
			// skip the test case if it is not meant for CI
			continue
		}
		t.Run(fmt.Sprintf("%d-verification node %d-chunk number", tc.verNodeCount, tc.chunkCount), func(t *testing.T) {
			testHappyPath(t, tc.verNodeCount, tc.chunkCount)
		})
	}
}

// testHappyPath runs `verNodeCount`-many verification nodes
// and checks that concurrently received execution receipts with the same result part that
// by each verification node results in:
// - the selection of the assigned chunks by the ingest engine
// - request of the associated collections to the assigned chunks
// - formation of a complete verifiable chunk by the ingest engine for each assigned chunk
// - submitting a verifiable chunk locally to the verify engine by the ingest engine
// - dropping the ingestion of the ERs that share the same result once the verifiable chunk is submitted to verify engine
// - broadcast of a matching result approval to consensus nodes for each assigned chunk
func testHappyPath(t *testing.T, verNodeCount int, chunkNum int) {
	// to demarcate the debug logs
	log.Debug().
		Int("verification_nodes_count", verNodeCount).
		Int("chunk_num", chunkNum).
		Msg("TestHappyPath started")

	// ingest engine parameters
	// set based on following issue
	// https://github.com/dapperlabs/flow-go/issues/3443
	requestInterval := uint(1000)
	failureThreshold := uint(2)

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
			if IsAssigned(chunk.Index) {
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
		verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner, requestInterval, failureThreshold)

		// starts the ingest engine
		<-verNode.IngestEngine.Ready()

		// assumes the verification node has received the block
		err := verNode.Blocks.Store(completeER.Block)
		assert.Nil(t, err)

		verNodes = append(verNodes, verNode)
	}

	// collection node
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)
	// injects the assigned collections into the collection node mempool
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if IsAssigned(chunk.Index) {
			err := colNode.Collections.Store(completeER.Collections[chunk.Index])
			assert.Nil(t, err)
		}
	}
	colNet, ok := hub.GetNetwork(colIdentity.NodeID)
	assert.True(t, ok)
	colNet.StartConDev(100, true)

	// mock execution node
	exeNode, exeEngine := setupMockExeNode(t, hub, exeIdentity, verIdentities, identities, completeER)

	// mock consensus node
	conNode, conEngine, conWG := setupMockConsensusNode(t, hub, conIdentity, verIdentities, identities, completeER)

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

	unittest.RequireReturnsBefore(t, verWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second)

	// creates a network instance for each verification node
	// and sets it in continuous delivery mode
	// then flushes the collection requests
	verNets := make([]*stub.Network, 0)
	for _, verIdentity := range verIdentities {
		verNet, ok := hub.GetNetwork(verIdentity.NodeID)
		assert.True(t, ok)
		verNet.StartConDev(requestInterval, true)
		verNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
			return m.ChannelID == engine.CollectionProvider
		})

		verNets = append(verNets, verNet)
	}

	unittest.RequireReturnsBefore(t, conWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second)
	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// stops verification nodes
	// Note: this should be done prior to any evaluation to make sure that
	// the process method of Ingest engines is done working.
	for _, verNode := range verNodes {
		<-verNode.IngestEngine.Done()
	}

	// stops continuous delivery of verification nodes
	for _, verNet := range verNets {
		verNet.StopConDev()
	}
	colNet.StopConDev()

	// resource cleanup
	//
	for _, verNode := range verNodes {
		for i := 0; i < chunkNum; i++ {
			// associated resources for each chunk should be removed from the mempool
			assert.False(t, verNode.AuthCollections.Has(completeER.Collections[i].ID()))
			assert.False(t, verNode.PendingCollections.Has(completeER.Collections[i].ID()))
			assert.False(t, verNode.ChunkDataPacks.Has(completeER.ChunkDataPacks[i].ID()))
			if IsAssigned(completeER.Receipt.ExecutionResult.Chunks[i].Index) {
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

	// to demarcate the debug logs
	log.Debug().
		Int("verification_nodes_count", verNodeCount).
		Int("chunk_num", chunkNum).
		Msg("TestHappyPath finishes")
}

// TestSingleCollectionProcessing checks the full happy
// path assuming a single collection (including transactions on counter example)
// are submited to the verification node.
func TestSingleCollectionProcessing(t *testing.T) {
	// TODO unskip this :(
	t.Skip()

	// ingest engine parameters
	requestInterval := uint(1)
	failureThreshold := uint(1)

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
	verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner, requestInterval, failureThreshold)
	// inject block
	err := verNode.Blocks.Store(completeER.Block)
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
		}).Return(nil).Once()

	// consensus node
	conNode := testutil.GenericNode(t, hub, conIdentity, identities)
	conEngine := new(network.Engine)
	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			_, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
		}).Return(nil).Once()

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

}

// BenchmarkIngestEngine benchmarks the happy path of ingest engine with sending
// 10 execution receipts simultaneously where each receipt has 100 chunks in it.
func BenchmarkIngestEngine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ingestHappyPath(b, 10, 100)
	}

}

// ingestHappyPath is used for benchmarking the happy path performance of ingest engine
// on receiving `receiptCount`-many receipts each with `chunkCount`-many chunks.
// It runs a single instance of verification node, with an actual ingest engine and a mocked
// verify engine.
// The execution receipts are sent to the node simultaneously assuming that it already has all the other
// resources for them, i.e., collections, blocks, and chunk data packs.
// The benchmark finishes when a verifiable chunk is sent for each assigned chunk from the ingest engine
// to the verify engine.
func ingestHappyPath(tb testing.TB, receiptCount int, chunkCount int) {
	// ingest engine parameters
	// set based on following issue
	// https://github.com/dapperlabs/flow-go/issues/3443
	requestInterval := uint(1000)
	failureThreshold := uint(2)

	// generates network hub
	hub := stub.NewNetworkHub()

	// generates identities of nodes, one of each type and `verCount` many verification node
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	verIdentity := identities.Filter(filter.HasRole(flow.RoleVerification))[0]
	exeIdentity := identities.Filter(filter.HasRole(flow.RoleExecution))[0]

	// Execution receipt and chunk assignment
	//
	ers := make([]verification.CompleteExecutionResult, receiptCount)
	for i := 0; i < receiptCount; i++ {
		ers[i] = CompleteExecutionResultFixture(tb, chunkCount)
	}

	fmt.Println("Chunks have been made")

	// mocks the assignment to assign the single chunk to this verifier node
	assigner := NewMockAssigner(verIdentity.NodeID)

	vChunks := make([]*verification.VerifiableChunk, 0)

	// collects assigned chunks to verification node in vChunks
	for _, er := range ers {
		for _, chunk := range er.Receipt.ExecutionResult.Chunks {
			if IsAssigned(chunk.Index) {
				vChunks = append(vChunks, VerifiableChunk(chunk.Index, er))
			}
		}
	}

	// nodes and engines
	//
	// verification node
	verifierEng, verifierEngWG := SetupMockVerifierEng(tb, vChunks)
	verNode := testutil.VerificationNode(tb, hub, verIdentity, identities, assigner, requestInterval, failureThreshold,
		testutil.WithVerifierEngine(verifierEng))

	// starts the ingest engine
	<-verNode.IngestEngine.Ready()

	// assumes the verification node has received the block, collections, and chunk data pack associated
	// with each receipt
	for _, er := range ers {
		// block
		err := verNode.Blocks.Store(er.Block)
		require.NoError(tb, err)

		for _, chunk := range er.Receipt.ExecutionResult.Chunks {
			// collection
			added := verNode.AuthCollections.Add(er.Collections[chunk.Index])
			require.True(tb, added)

			// chunk
			added = verNode.ChunkDataPacks.Add(er.ChunkDataPacks[chunk.Index])
			require.True(tb, added)
		}
	}

	for _, er := range ers {
		go func(receipt *flow.ExecutionReceipt) {
			err := verNode.IngestEngine.Process(exeIdentity.NodeID, receipt)
			require.NoError(tb, err)
		}(er.Receipt)
	}

	unittest.RequireReturnsBefore(tb, verifierEngWG.Wait, time.Duration(receiptCount)*time.Second)
	verNode.Done()
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

	// determines the expected number of result chunk data pack requests
	chunkDataPackCount := 0
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if IsAssigned(chunk.Index) {
			chunkDataPackCount++
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
						if chunkID == req.ChunkID {
							if !IsAssigned(chunk.Index) {
								require.Error(t, fmt.Errorf(" requested an unassigned chunk data pack %x", req))
							}
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

							log.Debug().
								Hex("origin_id", logging.ID(originID)).
								Hex("chunk_id", logging.ID(chunkID)).
								Msg("chunk data pack request answered by execution node")

							return
						}
					}
					require.Error(t, fmt.Errorf(" requested an unidentifed chunk data pack %v", req))
				}
			}

			require.Error(t, fmt.Errorf("unknown request to execution node %v", args[1]))

		}).
		Return(nil).
		// each verification node is assigned to `chunkDataPackCount`-many independent chunks
		// and there are `len(verIdentities)`-many verification nodes
		// so there is a total of len(verIdentities) * chunkDataPackCount expected
		// chunk data pack requests
		Times(len(verIdentities) * chunkDataPackCount)

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
	completeER verification.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine, *sync.WaitGroup) {
	// determines the expected number of result approvals this node should receive
	approvalsCount := 0
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if IsAssigned(chunk.Index) {
			approvalsCount++
		}
	}

	wg := &sync.WaitGroup{}
	// each verification node is assigned to `approvalsCount`-many independent chunks
	// and there are `len(verIdentities)`-many verification nodes
	// so there is a total of len(verIdentities) * approvalsCount expected
	// result approvals
	wg.Add(len(verIdentities) * approvalsCount)

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
			assert.True(t, IsAssigned(resultApproval.Body.ChunkIndex))

			wg.Done()
		}).Return(nil)

	_, err := conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	return &conNode, conEngine, wg
}
