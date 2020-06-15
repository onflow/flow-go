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
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestHappyPath_TwoEngine considers the happy path of Ingest/LightIngest and Verify engines.
// It evaluates the happy path scenario of
// concurrently sending an execution receipt with
// `chunkCount`-many chunks to `verNodeCount`-many verification nodes
// the happy path should result in dissemination of a result approval for each
// distinct chunk by each verification node. The result approvals should be
// sent to the consensus nodes
//
// NOTE: some test cases are meant to solely run locally when FLOWLOCAL environmental
// variable is set to TRUE
func TestHappyPath_TwoEngine(t *testing.T) {
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
			// sets last parameter to false to indicate using two engine configuration
			testHappyPath(t, tc.verNodeCount, tc.chunkCount, tc.lightIngest, false)
		})
	}
}

// TestHappyPath_ThreeEngine considers the happy path of Finder-Match-Verify engines.
// It evaluates the happy path scenario of
// concurrently sending an execution receipt with
// `chunkCount`-many chunks to `verNodeCount`-many verification nodes
// the happy path should result in dissemination of a result approval for each
// distinct chunk by each verification node. The result approvals should be
// sent to the consensus nodes
func TestHappyPath_ThreeEngine(t *testing.T) {
	var mu sync.Mutex
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
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%d-verification node %d-chunk number", tc.verNodeCount, tc.chunkCount), func(t *testing.T) {
			mu.Lock()
			defer mu.Unlock()
			testHappyPath(t, tc.verNodeCount, tc.chunkCount, true, true)
		})
	}
}

// testHappyPath runs `verNodeCount`-many verification nodes
// and checks that concurrently received execution receipts with the same result part that
// by each verification node results in:
// - the selection of the assigned chunks by the ingest engine
// - request of the associated chunk data pack to the assigned chunks
// - formation of a complete verifiable chunk by the ingest engine for each assigned chunk
// - submitting a verifiable chunk locally to the verify engine by the ingest engine
// - dropping the ingestion of the ERs that share the same result once the verifiable chunk is submitted to verify engine
// - broadcast of a matching result approval to consensus nodes for each assigned chunk
// lightIngest indicates whether to use the LightIngestEngine or the original ingest engine
// threeEngine indicates whether to use the threeEngine version or the twoEngine version of architecture.
func testHappyPath(t *testing.T, verNodeCount int, chunkNum int, lightIngest, threeEngine bool) {
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
	conIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

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
		verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner, requestInterval, failureThreshold)

		// starts all the engines
		<-verNode.FinderEngine.Ready()
		<-verNode.MatchEngine.(module.ReadyDoneAware).Ready()
		<-verNode.VerifierEngine.(module.ReadyDoneAware).Ready()

		// assumes the verification node has received the block
		err := verNode.Blocks.Store(completeER.Block)
		assert.Nil(t, err)

		verNodes = append(verNodes, verNode)
	}

	// mock execution node
	exeNode, exeEngine := setupMockExeNode(t, hub, exeIdentity, verIdentities, identities, completeER)

	// mock consensus node
	conNode, conEngine, conWG := setupMockConsensusNode(t, hub, conIdentity, verIdentities, identities, completeER)

	// sends execution receipt to each of verification nodes
	verWG := sync.WaitGroup{}
	for _, verNode := range verNodes {
		verWG.Add(1)
		go func(vn mock2.VerificationNode, receipt *flow.ExecutionReceipt) {
			defer verWG.Done()
			err := vn.FinderEngine.Process(exeIdentity.NodeID, receipt)
			require.NoError(t, err)
		}(verNode, completeER.Receipt)
	}

	// requires all verification nodes process the receipt
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

	// requires all verification nodes send a result approval per assigned chunk
	unittest.RequireReturnsBefore(t, conWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second)
	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// stops verification nodes
	// Note: this should be done prior to any evaluation to make sure that
	// the process method of Ingest engines is done working.
	for _, verNode := range verNodes {
		// stops all the engines
		<-verNode.FinderEngine.Done()
		<-verNode.MatchEngine.(module.ReadyDoneAware).Done()
		<-verNode.VerifierEngine.(module.ReadyDoneAware).Done()
	}

	// stops continuous delivery of nodes
	for _, verNet := range verNets {
		verNet.StopConDev()
	}

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
	verNode := testutil.VerificationNode(t, hub, verIdentity, identities, assigner, requestInterval, failureThreshold)
	// inject block
	err := verNode.Blocks.Store(completeER.Block)
	assert.Nil(t, err)

	// starts all the engines
	<-verNode.FinderEngine.Ready()
	<-verNode.MatchEngine.(module.ReadyDoneAware).Ready()
	<-verNode.VerifierEngine.(module.ReadyDoneAware).Ready()

	// starts verification node's network in continuous mode
	verNet, ok := hub.GetNetwork(verIdentity.NodeID)
	assert.True(t, ok)
	verNet.StartConDev(100, true)

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
	err = verNode.FinderEngine.Process(exeIdentity.NodeID, completeER.Receipt)
	assert.Nil(t, err)

	unittest.RequireReturnsBefore(t, approvalWG.Wait, 5*time.Second)

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// stop continuous delivery mode of the network
	verNet.StopConDev()

	// stops verification node
	// Note: this should be done prior to any evaluation to make sure that
	// the process method of Ingest engines is done working.
	<-verNode.FinderEngine.Done()
	<-verNode.MatchEngine.(module.ReadyDoneAware).Done()
	<-verNode.VerifierEngine.(module.ReadyDoneAware).Done()

	// receipt ID should be added to the ingested results mempool
	assert.True(t, verNode.IngestedResultIDs.Has(completeER.Receipt.ExecutionResult.ID()))

	verNode.Done()
	conNode.Done()
	exeNode.Done()

}
