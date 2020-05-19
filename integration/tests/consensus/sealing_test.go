package consensus

import (
	"context"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/utils/unittest"
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
	colConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, colConfig)

	// need three real consensus nodes
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(conID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
		ss.conIDs = append(ss.conIDs, conID)
	}

	// need one controllable execution node (used ghost)
	ss.exeID = unittest.IdentifierFixture()
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(ss.exeID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one controllable verification node (used ghost)
	ss.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(ss.verID), testnet.AsGhost())
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

func (ss *SealingSuite) TestBlockSealCreationEmptyBlock() {

	// fix the deadline of the entire test
	deadline := time.Now().Add(20 * time.Second)

	// listen for the first block proposal
	var targetID flow.Identifier
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

		targetID = proposal.Header.ID()
		break
	}

	// make sure we found a target block to seal
	require.NotEqual(ss.T(), flow.ZeroID, targetID, "should have found target block")

	ss.T().Logf("target block for seal generation found (block: %x)", targetID)

	// get the genesis block
	genesis := ss.net.Genesis().Header

	// decode hard-coded initial commit
	commit, _ := hex.DecodeString("c95f9c6a9e5cc270af0502a740fee65ccad451356038a5219b6440d13ee10161")

	// rebuild the genesis result to get the ID
	previous := flow.ExecutionResult{ExecutionResultBody: flow.ExecutionResultBody{
		PreviousResultID: flow.ZeroID,
		BlockID:          genesis.ID(),
		FinalStateCommit: commit,
	}}

	// create the execution result for the target block
	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID:          targetID,                          // refer the target block
			PreviousResultID: previous.ID(),                     // need genesis result
			FinalStateCommit: unittest.StateCommitmentFixture(), // end state is unchanged
			Chunks:           nil,                               // no chunks for this test
		},
		Signatures: nil,
	}

	ss.T().Logf("execution result for target block generated (result: %x)", result.ID())

	// create the execution receipt for the result
	receipt := flow.ExecutionReceipt{
		ExecutorID:        ss.exeID, // our fake execution node
		ExecutionResult:   result,   // result for target block
		Spocks:            nil,      // ignored
		ExecutorSignature: nil,      // ignored
	}

	ss.T().Logf("execution receipt for execution result generated (receipt: %x", receipt.ID())

	// send the execution receipt to the consensus node
	conID := ss.conIDs[0]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := ss.Execution().Send(ctx, engine.ExecutionReceiptProvider, &receipt, conID)
	ss.Require().NoError(err, "should be able to send execution receipt")

	// go through block proposals until seal is included
	var proposalID flow.Identifier
	var sealID flow.Identifier
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
		seals := proposal.Payload.Seals
		for _, seal := range seals {
			if seal.ResultID == result.ID() {
				proposalID = proposal.Header.ID()
				sealID = seal.ID()
				break
			}
		}
	}

	ss.Assert().NotEqual(flow.ZeroID, proposalID, "should find proposal including seal")
	ss.Assert().NotEqual(flow.ZeroID, sealID, "should find seal for result")
}
