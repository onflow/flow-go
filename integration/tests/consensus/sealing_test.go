package consensus

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionStateSealing(t *testing.T) {
	suite.Run(t, new(SealingSuite))
}

type SealingSuite struct {
	suite.Suite
	cancel context.CancelFunc
	net    *testnet.FlowNetwork
	conIDs []flow.Identifier
	exeID  flow.Identifier
	verID  flow.Identifier
	reader *client.FlowMessageStreamReader
}

func (ss *SealingSuite) Execution() *client.GhostClient {
	ghost := ss.net.ContainerByID(ss.exeID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(ss.T(), err, "could not get ghost client")
	return client
}

func (ss *SealingSuite) Verification() *client.GhostClient {
	ghost := ss.net.ContainerByID(ss.verID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(ss.T(), err, "could not get ghost client")
	return client
}

func (ss *SealingSuite) SetupTest() {

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// to collect node confiss...
	var nodeConfigs []testnet.NodeConfig

	// need one dummy collection node (unused ghost)
	colConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, colConfig)

	// need three real consensus nodes
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithID(conID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
		ss.conIDs = append(ss.conIDs, conID)
	}

	// need one controllable execution node (used ghost)
	ss.exeID = unittest.IdentifierFixture()
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithID(ss.exeID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one controllable verification node (used ghost)
	ss.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithID(ss.verID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("consensus_execution_state_sealing", nodeConfigs)

	// initialize the network
	ss.net = testnet.PrepareFlowNetwork(ss.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	ss.cancel = cancel
	ss.net.Start(ctx)

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		var err error
		ss.reader, err = ss.Execution().Subscribe(context.Background()) // doesn't matter which one
		if err == nil {
			break
		}
		if attempts >= 10 {
			require.NoError(ss.T(), err, "could not subscribe to ghost (%d attempts)", attempts)
		}
	}
}

func (ss *SealingSuite) TearDownTest() {
	ss.net.Remove()
	ss.cancel()
}

func (ss *SealingSuite) TestBlockSealCreation() {

	// fix the deadline of the entire test
	deadline := time.Now().Add(20 * time.Second)

	// first, we listen to see which block proposal is the first one to be
	// confirmed three times (finalized)
	var targetID flow.Identifier
	parents := make(map[flow.Identifier]flow.Identifier)
	confirmations := make(map[flow.Identifier]uint)
SearchLoop:
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := ss.reader.Next()
		if err != nil {
			ss.T().Logf("could not read next message: %s", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		// make sure we skip duplicates
		proposalID := proposal.Header.ID()
		_, processed := confirmations[proposalID]
		if processed {
			continue
		}
		confirmations[proposalID] = 0

		// we map the proposal to its parent for later
		parentID := proposal.Header.ParentID
		parents[proposalID] = parentID

		// we add one confirmation for each ancestor
		for {

			// check if we keep track of the parent
			_, tracked := confirmations[parentID]
			if !tracked {
				break
			}

			// add one confirmation to parent
			confirmations[parentID]++

			// check if we reached three confirmations
			if confirmations[parentID] >= 3 {
				targetID = parentID
				break SearchLoop
			}

			// check if there is another ancestor
			parentID, tracked = parents[parentID]
			if !tracked {
				break
			}
		}
	}

	// make sure we found a target block to seal
	require.NotEqual(ss.T(), flow.ZeroID, targetID, "should have found target block")

	ss.T().Logf("target for sealing found (block: %x)", targetID)

	// create a chunk for the execution result
	chunk := flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      0,                         // irrelevant for consensus node
			StartState:           flow.DummyStateCommitment, // irrelevant for consensus node
			EventCollection:      flow.ZeroID,               // irrelevant for consensus node
			BlockID:              targetID,
			TotalComputationUsed: 0, // irrelevant for consensus node
			NumberOfTransactions: 0, // irrelevant for consensus node
		},
		Index:    0,                                 // should start at zero
		EndState: unittest.StateCommitmentFixture(), // random end execution state
	}

	resultID := ss.net.Seal().ResultID

	// create the execution result for the target block
	result := flow.ExecutionResult{
		PreviousResultID: resultID,               // need genesis result
		BlockID:          targetID,               // refer the target block
		Chunks:           flow.ChunkList{&chunk}, // include only chunk
	}

	ss.T().Logf("execution result generated (result: %x)", result.ID())

	// create the execution receipt for the only execution node
	receipt := flow.ExecutionReceipt{
		ExecutorID:        ss.exeID, // our fake execution node
		ExecutionResult:   result,   // result for target block
		Spocks:            nil,      // ignored
		ExecutorSignature: nil,      // ignored
	}

	// keep trying to send execution receipt to the first consensus node
ReceiptLoop:
	for time.Now().Before(deadline) {
		conID := ss.conIDs[0]
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := ss.Execution().Send(ctx, engine.PushReceipts, &receipt, conID)
		cancel()
		if err != nil {
			ss.T().Logf("could not send execution receipt: %s", err)
			continue
		}
		break ReceiptLoop
	}

	ss.T().Logf("execution receipt submitted (receipt: %x, result: %x)", receipt.ID(), receipt.ExecutionResult.ID())

	// create the result approval for the only verification node
	approval := flow.ResultApproval{
		Body: flow.ResultApprovalBody{
			Attestation: flow.Attestation{
				BlockID:           result.BlockID, // the block for the result
				ExecutionResultID: result.ID(),    // the actual execution result
				ChunkIndex:        chunk.Index,    // the chunk of this approval
			},
			ApproverID:           ss.verID, // our fake verification node
			AttestationSignature: nil,      // ignared
			Spock:                nil,      // ignared
		},
		VerifierSignature: nil, // ignored
	}

	// keep trying to send result approval to the first consensus node
ApprovalLoop:
	for time.Now().Before(deadline) {
		conID := ss.conIDs[0]
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := ss.Verification().Send(ctx, engine.PushApprovals, &approval, conID)
		cancel()
		if err != nil {
			ss.T().Logf("could not send result approval: %s", err)
			continue
		}
		break ApprovalLoop
	}

	ss.T().Logf("result approval submitted (approval: %x, result: %x)", approval.ID(), approval.Body.ExecutionResultID)

	// we try to find a block with the guarantee included and three confirmations
	found := false
SealingLoop:
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := ss.reader.Next()
		if err != nil {
			ss.T().Logf("could not read message: %s", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		// log the proposal details
		proposalID := proposal.Header.ID()
		seals := proposal.Payload.Seals

		// if the block seal is included, we add the block to those we
		// monitor for confirmations
		for _, seal := range seals {
			if seal.ResultID == result.ID() {
				found = true
				ss.T().Logf("%x: block seal included!", proposalID)
				break SealingLoop
			}
		}
	}

	// make sure we found the guarantee in at least one block proposal
	require.True(ss.T(), found, "block seal should have been included in at least one block")
}
