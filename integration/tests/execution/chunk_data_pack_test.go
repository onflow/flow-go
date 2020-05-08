package execution

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionChunkDataPacks(t *testing.T) {
	suite.Run(t, new(ExecutionChunkDataPacksSuite))
}

type ExecutionChunkDataPacksSuite struct {
	Suite
}

func (s *ExecutionChunkDataPacksSuite) SetupTest() {

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

func (gs *ExecutionChunkDataPacksSuite) TestVerificationNodesRequestChunkDataPacks() {

	// wait for first finalized block, called blockA
	blockA := gs.BlockState.WaitForFirstFinalized(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockA.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockA with SC %x", erExe1BlockA.ExecutionResult.FinalStateCommit)

	// assert there were no ChunkDataPackRequests from the verification node yet
	require.Equal(gs.T(), 0, gs.MsgState.LenFrom(gs.verID),
		"expected no ChunkDataPackRequest to be sent before a transaction existed")

	// send transaction
	err := gs.AccessClient().DeployContract(context.Background(), gs.net.Genesis().ID(), common.CounterContract)
	require.NoError(gs.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(gs.T(), &gs.BlockState, &gs.ReceiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockB.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockB with SC %x", erExe1BlockB.ExecutionResult.FinalStateCommit)

	// extract chunk ID from execution receipt
	require.Len(gs.T(), erExe1BlockB.ExecutionResult.Chunks, 1)
	chunkID := erExe1BlockB.ExecutionResult.Chunks[0].ID()

	// TODO the following is extremely flaky, investigate why and re-activate.
	// wait for ChunkDataPack pushed from execution node
	// msg := gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackResponse, gs.exe1ID)
	// pack := msg.(*messages.ChunkDataPackResponse)
	// require.Equal(gs.T(), erExe1BlockB.ExecutionResult.Chunks[0].ID(), pack.Data.ChunkID
	// TODO clear messages

	// send a ChunkDataPackRequest from Ghost node
	err = gs.Ghost().Send(context.Background(), engine.ExecutionReceiptProvider, []flow.Identifier{gs.exe1ID},
		&messages.ChunkDataPackRequest{ChunkID: chunkID})
	require.NoError(gs.T(), err)

	// wait for ChunkDataPackResponse
	msg2 := gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackResponse, gs.exe1ID)
	pack2 := msg2.(*messages.ChunkDataPackResponse)
	require.Equal(gs.T(), chunkID, pack2.Data.ChunkID)
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.Chunks[0].StartState, pack2.Data.StartState)

	// verify state proofs
	v := ledger.NewTrieVerifier(257)
	isValid, err := v.VerifyRegistersProof(pack2.Data.Registers(), pack2.Data.Values(), pack2.Data.Proofs(),
		erExe1BlockB.ExecutionResult.Chunks[0].StartState)
	require.NoError(gs.T(), err, "error verifying chunk trie proofs")
	require.True(gs.T(), isValid, "chunk trie proofs are not valid, but must be")

	_, err = trie.NewPSMT(pack2.Data.StartState, 257, pack2.Data.Registers(), pack2.Data.Values(), pack2.Data.Proofs())
	require.NoError(gs.T(), err, "error building PSMT")
}
