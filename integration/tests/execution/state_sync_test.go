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

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionSuite))
}

type ExecutionSuite struct {
	suite.Suite
	common.TestnetStateTracker
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	nodeIDs []flow.Identifier
	ghostID flow.Identifier
	exe1ID  flow.Identifier
	exe2ID  flow.Identifier
	verID   flow.Identifier
}

func (gs *ExecutionSuite) Ghost() *client.GhostClient {
	ghost := gs.net.ContainerByID(gs.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *ExecutionSuite) AccessClient() *testnet.Client {
	client, err := testnet.NewClient(fmt.Sprintf(":%s", gs.net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(gs.T(), err, "could not get access client")
	return client
}

func (gs *ExecutionSuite) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// need one access node
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	nodeConfigs = append(nodeConfigs, acsConfig)

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.ErrorLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need two execution nodes
	gs.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe1ID),
		testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, exe1Config)
	gs.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe2ID),
		testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, exe2Config)

	// need one verification node
	gs.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(gs.verID),
		testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.ErrorLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.WithID(gs.ghostID), testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.ErrorLevel))
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

func (gs *ExecutionSuite) TearDownTest() {
	gs.net.Remove()
	gs.cancel()
}

func (gs *ExecutionSuite) xTestStateSyncAfterNetworkPartition() {

	// pause execution node 2
	err := gs.net.ContainerByID(gs.exe2ID).Pause()
	require.NoError(gs.T(), err, "could not pause execution node 2")

	// wait for first finalized block, called blockA
	blockA := gs.BlockState.WaitForFirstFinalized(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockA.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockA with SC %x", erExe1BlockA.ExecutionResult.FinalStateCommit)

	// send transaction
	err = common.DeployCounter(context.Background(), gs.AccessClient())

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(gs.T(), &gs.BlockState, &gs.ReceiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockB.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockB with SC %x", erExe1BlockB.ExecutionResult.FinalStateCommit)

	// require that state between blockA and blockB has changed
	require.NotEqual(gs.T(), erExe1BlockA.ExecutionResult.FinalStateCommit,
		erExe1BlockB.ExecutionResult.FinalStateCommit)

	// unpause execution node 2
	err = gs.net.ContainerByID(gs.exe2ID).Start()
	require.NoError(gs.T(), err)

	// wait until the next proposed block is finalized, called blockC
	blockC := gs.BlockState.WaitUntilNextHeightFinalized(gs.T())
	gs.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erExe1BlockC := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockC.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockC with SC %x", erExe1BlockC.ExecutionResult.FinalStateCommit)

	// require that state between blockB and blockC has not changed
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe1BlockC.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockA from execution node 2 (this one must have been synced)
	erExe2BlockA := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockA.Header.ID(), gs.exe2ID)
	gs.T().Logf("got erExe2BlockA with SC %x", erExe2BlockA.ExecutionResult.FinalStateCommit)

	// require that state for blockA is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockA.ExecutionResult.FinalStateCommit, erExe2BlockA.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockB from execution node 2 (this one must have been synced)
	erExe2BlockB := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockB.Header.ID(), gs.exe2ID)
	gs.T().Logf("got erExe2BlockB with SC %x", erExe2BlockB.ExecutionResult.FinalStateCommit)

	// require that state for blockB is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe2BlockB.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockC from execution node 2 (this one must have been synced)
	erExe2BlockC := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockC.Header.ID(), gs.exe2ID)
	gs.T().Logf("got erExe2BlockC with SC %x", erExe2BlockC.ExecutionResult.FinalStateCommit)

	// require that state for blockC is the same for execution node 1 and 2
	require.Equal(gs.T(), erExe1BlockC.ExecutionResult.FinalStateCommit, erExe2BlockC.ExecutionResult.FinalStateCommit)
}
