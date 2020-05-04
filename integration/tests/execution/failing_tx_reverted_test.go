package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	suite.Run(t, new(ExecutionSuite3))
}

type ExecutionSuite3 struct {
	suite.Suite
	common.TestnetStateTracker
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	nodeIDs []flow.Identifier
	ghostID flow.Identifier
	exe1ID  flow.Identifier
	verID   flow.Identifier
}

func (gs *ExecutionSuite3) Ghost() *client.GhostClient {
	ghost := gs.net.ContainerByID(gs.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *ExecutionSuite3) AccessClient() *testnet.Client {
	client, err := testnet.NewClient(fmt.Sprintf(":%s", gs.net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(gs.T(), err, "could not get access client")
	return client
}

func (gs *ExecutionSuite3) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// need one access node
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	nodeConfigs = append(nodeConfigs, acsConfig)

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need one execution nodes
	gs.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, exe1Config)

	// need one verification node
	gs.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(gs.verID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(gs.ghostID), testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("execution_tests", nodeConfigs)

	// initialize the network
	gs.net = testnet.PrepareFlowNetwork(gs.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	gs.cancel = cancel
	gs.net.Start(ctx)

	// start tracking blocks
	gs.Track(gs.T(), ctx, gs.Ghost())
}

func (gs *ExecutionSuite3) TearDownTest() {
	gs.net.Remove()
	gs.cancel()
}

func (gs *ExecutionSuite3) TestExecutionFailingTxReverted() {

	// wait for first finalized block, called blockA
	blockA := gs.BlockState.WaitForFirstFinalized(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// send transaction
	err := common.DeployCounter(context.Background(), gs.AccessClient())
	require.NoError(gs.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, erBlockB := common.WaitUntilFinalizedStateCommitmentChanged(gs.T(), &gs.BlockState, &gs.ReceiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// send transaction that panics and should revert
	err = common.CreateCounterPanic(context.Background(), gs.AccessClient())
	require.NoError(gs.T(), err, "could not send tx to create counter that should panic")

	// send transaction that has no sigs and should not execute
	err = common.CreateCounterWrongSig(context.Background(), gs.AccessClient())
	require.NoError(gs.T(), err, "could not send tx to create counter with wrong sig")

	// wait until the next proposed block is finalized, called blockC
	blockC := gs.BlockState.WaitUntilNextHeightFinalized(gs.T())
	gs.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erBlockC := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockC.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erBlockC with SC %x", erBlockC.ExecutionResult.FinalStateCommit)

	// assert that state did not change between blockB and blockC
	require.Equal(gs.T(), erBlockB.ExecutionResult.FinalStateCommit, erBlockC.ExecutionResult.FinalStateCommit)
}
