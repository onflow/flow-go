package execution

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	suite.Run(t, new(FailingTxRevertedSuite))
}

type FailingTxRevertedSuite struct {
	Suite
}

func (s *FailingTxRevertedSuite) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// need one access node
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	nodeConfigs = append(nodeConfigs, acsConfig)

	// generate the three consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need one execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, exe1Config)

	// need one verification node
	s.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(s.verID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// add the ghost node config
	s.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(s.ghostID), testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("execution_tests", nodeConfigs)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())
}

func (s *FailingTxRevertedSuite) TestExecutionFailingTxReverted() {

	// wait for first finalized block, called blockA
	blockA := s.BlockState.WaitForFirstFinalized(s.T())
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// send transaction
	err := s.AccessClient().DeployContract(context.Background(), s.net.Genesis().ID(), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, erBlockB := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), &s.BlockState, &s.ReceiptState)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// send transaction that panics and should revert
	tx := unittest.TransactionBodyFixture(
		unittest.WithTransactionDSL(common.CreateCounterPanicTx),
		unittest.WithReferenceBlock(s.net.Genesis().ID()),
	)
	err = s.AccessClient().SendTransaction(context.Background(), tx)
	require.NoError(s.T(), err, "could not send tx to create counter that should panic")

	// send transaction that has no sigs and should not execute
	tx = unittest.TransactionBodyFixture(
		unittest.WithTransactionDSL(common.CreateCounterTx),
		unittest.WithReferenceBlock(s.net.Genesis().ID()),
	)
	tx.PayloadSignatures = nil
	tx.EnvelopeSignatures = nil
	err = s.AccessClient().SendTransaction(context.Background(), tx)
	require.NoError(s.T(), err, "could not send tx to create counter with wrong sig")

	// wait until the next proposed block is finalized, called blockC
	blockC := s.BlockState.WaitUntilNextHeightFinalized(s.T())
	s.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erBlockC := s.ReceiptState.WaitForReceiptFrom(s.T(), blockC.Header.ID(), s.exe1ID)
	s.T().Logf("got erBlockC with SC %x", erBlockC.ExecutionResult.FinalStateCommit)

	// assert that state did not change between blockB and blockC
	require.Equal(s.T(), erBlockB.ExecutionResult.FinalStateCommit, erBlockC.ExecutionResult.FinalStateCommit)
}
