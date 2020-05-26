package verification

import (
	"fmt"
	"math/rand"
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
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
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

	var mu sync.Mutex
	testcases := []struct {
		verNodeCount,
		chunkCount int
		lightIngest bool // indicates if light ingest engine should replace the original one
	}{
		{
			verNodeCount: 1,
			chunkCount:   2,
			lightIngest:  true,
		},
		{
			verNodeCount: 1,
			chunkCount:   10,
			lightIngest:  true,
		},
		{
			verNodeCount: 2,
			chunkCount:   2,
			lightIngest:  true,
		},
		{
			verNodeCount: 1,
			chunkCount:   2,
			lightIngest:  false,
		},
		{
			verNodeCount: 1,
			chunkCount:   10,
			lightIngest:  false,
		},
		{
			verNodeCount: 2,
			chunkCount:   2,
			lightIngest:  false,
		},
	}

	for _, tc := range testcases {
		// skips tests of original ingest over CI
		if !tc.lightIngest && os.Getenv("FLOWLOCAL") != "TRUE" {
			continue
		}
		t.Run(fmt.Sprintf("%d-verification node %d-chunk number %t-light ingest", tc.verNodeCount, tc.chunkCount, tc.lightIngest), func(t *testing.T) {
			mu.Lock()
			defer mu.Unlock()
			testHappyPath(t, tc.verNodeCount, tc.chunkCount, tc.lightIngest)
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
// lightIngest indicates whether to use the LightIngestEngine or the original ingest engine
func testHappyPath(t *testing.T, verNodeCount int, chunkNum int, lightIngest bool) {
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
	completeER := utils.CompleteExecutionResultFixture(t, chunkNum)

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
		verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner, requestInterval, failureThreshold, lightIngest)

		// starts the ingest engine
		if lightIngest {
			<-verNode.LightIngestEngine.Ready()
		} else {
			<-verNode.IngestEngine.Ready()
		}

		// assumes the verification node has received the block
		err := verNode.Blocks.Store(completeER.Block)
		assert.Nil(t, err)

		verNodes = append(verNodes, verNode)
	}

	// light ingest engine does not interact with collection node
	// but the current version of original ingest engine does
	// TODO removing collection request from ORIGINAL ingest engine
	// https://github.com/dapperlabs/flow-go/issues/3008
	var colNode mock2.CollectionNode
	var colNet *stub.Network
	if !lightIngest {
		// collection node
		colNode = testutil.CollectionNode(t, hub, colIdentity, identities)
		// injects the assigned collections into the collection node mempool
		for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
			if IsAssigned(chunk.Index) {
				err := colNode.Collections.Store(completeER.Collections[chunk.Index])
				assert.Nil(t, err)
			}
		}
		net, ok := hub.GetNetwork(colIdentity.NodeID)
		assert.True(t, ok)
		colNet = net
		colNet.StartConDev(100, true)
	}

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

	// invoking verification node with two receipts concurrently
	verWG := sync.WaitGroup{}
	for _, verNode := range verNodes {
		verWG.Add(2)
		for _, er := range []*flow.ExecutionReceipt{receipt1, receipt2} {
			go func(vn mock2.VerificationNode, receipt *flow.ExecutionReceipt) {
				defer verWG.Done()
				// routes the receipt to either light or original ingest engines based
				// on the test type
				if lightIngest {
					err := vn.LightIngestEngine.Process(exeIdentity.NodeID, receipt)
					assert.Nil(t, err)
				} else {
					err := vn.IngestEngine.Process(exeIdentity.NodeID, receipt)
					assert.Nil(t, err)
				}
			}(verNode, er)
		}
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
		if lightIngest {
			<-verNode.LightIngestEngine.Done()
		} else {
			<-verNode.IngestEngine.Done()
		}
	}

	// stops continuous delivery of verification nodes
	for _, verNet := range verNets {
		verNet.StopConDev()
	}

	// light ingest engine does not interact with collection node
	// but the current version of original ingest engine does
	// TODO removing collection request from ORIGINAL ingest engine
	// https://github.com/dapperlabs/flow-go/issues/3008
	if !lightIngest {
		colNet.StopConDev()
	}

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

		// LightIngestEngine does the cleaning of ingested receipts slower and passively
		// hence it is discarded to check the receipts clean up in lightIngest mode.
		if !lightIngest {
			// since all chunks have been ingested, neither of execution receipts should reside on any mempool
			assert.False(t, verNode.PendingReceipts.Has(receipt1.ID()))
			assert.False(t, verNode.AuthReceipts.Has(receipt1.ID()))
			assert.False(t, verNode.PendingReceipts.Has(receipt2.ID()))
			assert.False(t, verNode.AuthReceipts.Has(receipt2.ID()))
		}

		// result ID should be added to the ingested results mempool
		assert.True(t, verNode.IngestedResultIDs.Has(completeER.Receipt.ExecutionResult.ID()))

		verNode.Done()
	}

	conNode.Done()
	exeNode.Done()

	// light ingest engine does not interact with collection node
	// but the current version of original ingest engine does
	// TODO removing collection request from ORIGINAL ingest engine
	// https://github.com/dapperlabs/flow-go/issues/3008
	if !lightIngest {
		colNode.Done()
	}

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
	// ingest engine parameters
	// set based on following issue
	// https://github.com/dapperlabs/flow-go/issues/3443
	requestInterval := uint(1000)
	failureThreshold := uint(2)

	// network identity setup
	hub := stub.NewNetworkHub()
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]
	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity, verIdentity}

	// complete ER counter example
	completeER := utils.CompleteExecutionResultFixture(t, 1)
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
	//
	// verification node
	verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner, requestInterval, failureThreshold, true)
	// inject block
	err := verNode.Blocks.Store(completeER.Block)
	assert.Nil(t, err)
	// starts the ingest engine
	<-verNode.LightIngestEngine.Ready()
	// starts verification node's network in continuous mode
	verNet, ok := hub.GetNetwork(verIdentity.NodeID)
	assert.True(t, ok)
	verNet.StartConDev(100, true)

	// collection node
	colNode := testutil.CollectionNode(t, hub, colIdentity, identities)
	// inject the collection
	err = colNode.Collections.Store(completeER.Collections[0])
	assert.Nil(t, err)
	// starts collection node's network in continuous mode
	colNet, ok := hub.GetNetwork(colIdentity.NodeID)
	assert.True(t, ok)
	colNet.StartConDev(100, true)

	// execution node
	exeNode := testutil.GenericNode(t, hub, exeIdentity, identities)
	exeEngine := new(network.Engine)
	exeChunkDataConduit, err := exeNode.Net.Register(engine.ChunkDataPackProvider, exeEngine)
	assert.Nil(t, err)
	exeEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if _, ok := args[1].(*messages.ChunkDataRequest); ok {
				// publishes the chunk data pack response to the network
				res := &messages.ChunkDataResponse{
					ChunkDataPack: *completeER.ChunkDataPacks[0],
					Collection:    *completeER.Collections[0],
					Nonce:         rand.Uint64(),
				}
				err := exeChunkDataConduit.Submit(res, verIdentity.NodeID)
				assert.Nil(t, err)
			}
		}).Return(nil).Once()

	// consensus node
	conNode := testutil.GenericNode(t, hub, conIdentity, identities)
	conEngine := new(network.Engine)
	approvalWG := sync.WaitGroup{}
	approvalWG.Add(1)
	conEngine.On("Process", verIdentity.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			_, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
			approvalWG.Done()
		}).Return(nil).Once()

	_, err = conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// send the ER from execution to verification node
	err = verNode.LightIngestEngine.Process(exeIdentity.NodeID, completeER.Receipt)
	assert.Nil(t, err)

	unittest.RequireReturnsBefore(t, approvalWG.Wait, 5*time.Second)

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// stop continuous delivery mode of the network
	verNet.StopConDev()
	colNet.StopConDev()

	// stops verification node
	// Note: this should be done prior to any evaluation to make sure that
	// the process method of Ingest engines is done working.
	<-verNode.LightIngestEngine.Done()

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
		ingestHappyPath(b, 10, 100, true)
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
// lightIngest indicates whether to use the LightIngestEngine or the original ingest engine
func ingestHappyPath(tb testing.TB, receiptCount int, chunkCount int, lightIngest bool) {
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
		ers[i] = utils.LightExecutionResultFixture(chunkCount)
	}

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
		lightIngest,
		testutil.WithVerifierEngine(verifierEng))

	// starts the ingest engine
	if lightIngest {
		<-verNode.LightIngestEngine.Ready()
	} else {
		<-verNode.IngestEngine.Ready()
	}

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
			if lightIngest {
				err := verNode.LightIngestEngine.Process(exeIdentity.NodeID, receipt)
				require.NoError(tb, err)
			} else {
				err := verNode.IngestEngine.Process(exeIdentity.NodeID, receipt)
				require.NoError(tb, err)
			}
		}(er.Receipt)
	}

	unittest.RequireReturnsBefore(tb, verifierEngWG.Wait, time.Duration(receiptCount)*time.Second)
	verNode.Done()
}
