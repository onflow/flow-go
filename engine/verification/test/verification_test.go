package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHappyPath considers the happy path of Finder-Match-Verify engines.
// It evaluates the happy path scenario of
// concurrently sending an execution receipt with
// `chunkCount`-many chunks to `verNodeCount`-many verification nodes
// the happy path should result in dissemination of a result approval for each
// distinct chunk by each verification node. The result approvals should be
// sent to the consensus nodes
func TestHappyPath(t *testing.T) {
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

			collector := metrics.NewNoopCollector()
			vertestutils.VerificationHappyPath(t, tc.verNodeCount, tc.chunkCount, collector, collector)
		})
	}
}

// TestSingleCollectionProcessing checks the full happy
// path assuming a single collection (including transactions on counter example)
// are submitted to the verification node.
func TestSingleCollectionProcessing(t *testing.T) {
	chainID := flow.Testnet
	chunkNum := 1

	// finder and match engine parameters
	// set based on following issue (3443)
	requestInterval := 1 * time.Second
	processInterval := 1 * time.Second
	failureThreshold := uint(2)

	// network identity setup
	hub := stub.NewNetworkHub()
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conIdentity := conIdentities[0]
	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity, verIdentity}

	// sets up verification node
	// assigner and assignment
	assigner := &mock.ChunkAssigner{}
	collector := metrics.NewNoopCollector()
	verNode := testutil.VerificationNode(t,
		hub,
		verIdentity,
		identities,
		assigner,
		requestInterval,
		processInterval,
		failureThreshold,
		uint(10),
		uint(10*chunkNum),
		chainID,
		collector,
		collector)

	// generate a child block out of root block of state in verification node,
	// and creates its corresponding execution result.
	root, err := verNode.State.Params().Root()
	require.NoError(t, err)

	completeER := vertestutils.CompleteExecutionReceiptFixture(t, chunkNum, chainID.Chain(), root)
	// stores block of execution result in state and mutate state accordingly
	err = verNode.State.Extend(completeER.ReceiptsData[0].ReferenceBlock)
	require.NoError(t, err)

	// mocks chunk assignment
	_, expectedChunkIDs := vertestutils.MockChunkAssignmentFixture(assigner, flow.IdentityList{verIdentity},
		[]*vertestutils.CompleteExecutionReceipt{completeER}, vertestutils.EvenChunkIndexAssigner)

	// starts all the engines
	unittest.RequireComponentsReadyBefore(t, 1*time.Second,
		verNode.FinderEngine,
		verNode.MatchEngine.(module.ReadyDoneAware),
		verNode.VerifierEngine)

	// starts verification node's network in continuous mode
	verNet, ok := hub.GetNetwork(verIdentity.NodeID)
	assert.True(t, ok)
	verNet.StartConDev(100, true)

	// execution node
	exeNode, exeEngine, _ := vertestutils.SetupChunkDataPackProvider(t,
		hub,
		exeIdentity,
		identities,
		chainID,
		[]*vertestutils.CompleteExecutionReceipt{completeER},
		expectedChunkIDs,
		vertestutils.RespondChunkDataPackRequestImmediately) // always responds to chunk data pack requests.

	// consensus node
	// mock consensus node
	conNode, conEngine, conWG := vertestutils.SetupMockConsensusNode(t,
		unittest.Logger(),
		hub,
		conIdentity,
		flow.IdentityList{verIdentity},
		identities,
		vertestutils.CompleteExecutionReceiptList{completeER},
		chainID,
		expectedChunkIDs)

	// send the ER from execution to verification node
	err = verNode.FinderEngine.Process(exeIdentity.NodeID, completeER.Receipts[0])
	assert.Nil(t, err)

	unittest.RequireReturnsBefore(t, conWG.Wait, 10*time.Second, "consensus nodes process")

	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// stop continuous delivery mode of the network
	verNet.StopConDev()

	// stops verification node
	// Note: this should be done prior to any evaluation to make sure that
	// the process method of Ingest engines is done working.
	unittest.RequireComponentsDoneBefore(t, 1*time.Second,
		verNode.FinderEngine,
		verNode.MatchEngine.(module.ReadyDoneAware),
		verNode.VerifierEngine)

	// receipt ID should be added to the ingested results mempool
	assert.True(t, verNode.ProcessedResultIDs.Has(completeER.Receipts[0].ExecutionResult.ID()))

	enginemock.RequireGenericNodesDoneBefore(t, 1*time.Second,
		conNode,
		exeNode,
		verNode.GenericNode)
}
