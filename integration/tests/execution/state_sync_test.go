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
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionStateSync(t *testing.T) {
	suite.Run(t, new(StateSyncSuite))
}

type StateSyncSuite struct {
	Suite
	exe2ID flow.Identifier
}

func (s *StateSyncSuite) SetupTest() {

	// need second execution nodes
	s.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(s.exe2ID),
		testnet.WithLogLevel(zerolog.ErrorLevel))
	s.nodeConfigs = append(s.nodeConfigs, exe2Config)

	s.SetupTest()
}

func (s *StateSyncSuite) TestStateSyncAfterNetworkPartition() {

	// pause execution node 2
	err := s.net.ContainerByID(s.exe2ID).Pause()
	require.NoError(s.T(), err, "could not pause execution node 2")

	// wait for first finalized block, called blockA
	blockA := s.BlockState.WaitForFirstFinalized(s.T())
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := s.ReceiptState.WaitForReceiptFrom(s.T(), blockA.Header.ID(), s.exe1ID)
	s.T().Logf("got erExe1BlockA with SC %x", erExe1BlockA.ExecutionResult.FinalStateCommit)

	// send transaction
	err = s.AccessClient().DeployContract(context.Background(), s.net.Genesis().ID(), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), &s.BlockState, &s.ReceiptState)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := s.ReceiptState.WaitForReceiptFrom(s.T(), blockB.Header.ID(), s.exe1ID)
	s.T().Logf("got erExe1BlockB with SC %x", erExe1BlockB.ExecutionResult.FinalStateCommit)

	// require that state between blockA and blockB has changed
	require.NotEqual(s.T(), erExe1BlockA.ExecutionResult.FinalStateCommit,
		erExe1BlockB.ExecutionResult.FinalStateCommit)

	// unpause execution node 2
	err = s.net.ContainerByID(s.exe2ID).Start()
	require.NoError(s.T(), err)

	// wait until the next proposed block is finalized, called blockC
	blockC := s.BlockState.WaitUntilNextHeightFinalized(s.T())
	s.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erExe1BlockC := s.ReceiptState.WaitForReceiptFrom(s.T(), blockC.Header.ID(), s.exe1ID)
	s.T().Logf("got erExe1BlockC with SC %x", erExe1BlockC.ExecutionResult.FinalStateCommit)

	// require that state between blockB and blockC has not changed
	require.Equal(s.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe1BlockC.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockA from execution node 2 (this one must have been synced)
	erExe2BlockA := s.ReceiptState.WaitForReceiptFrom(s.T(), blockA.Header.ID(), s.exe2ID)
	s.T().Logf("got erExe2BlockA with SC %x", erExe2BlockA.ExecutionResult.FinalStateCommit)

	// require that state for blockA is the same for execution node 1 and 2
	require.Equal(s.T(), erExe1BlockA.ExecutionResult.FinalStateCommit, erExe2BlockA.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockB from execution node 2 (this one must have been synced)
	erExe2BlockB := s.ReceiptState.WaitForReceiptFrom(s.T(), blockB.Header.ID(), s.exe2ID)
	s.T().Logf("got erExe2BlockB with SC %x", erExe2BlockB.ExecutionResult.FinalStateCommit)

	// require that state for blockB is the same for execution node 1 and 2
	require.Equal(s.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, erExe2BlockB.ExecutionResult.FinalStateCommit)

	// wait for execution receipt for blockC from execution node 2 (this one must have been synced)
	erExe2BlockC := s.ReceiptState.WaitForReceiptFrom(s.T(), blockC.Header.ID(), s.exe2ID)
	s.T().Logf("got erExe2BlockC with SC %x", erExe2BlockC.ExecutionResult.FinalStateCommit)

	// require that state for blockC is the same for execution node 1 and 2
	require.Equal(s.T(), erExe1BlockC.ExecutionResult.FinalStateCommit, erExe2BlockC.ExecutionResult.FinalStateCommit)

	// send a ExecutionStateSyncRequest from Ghost node
	err = s.Ghost().Send(context.Background(), engine.ExecutionSync, []flow.Identifier{s.exe1ID},
		&messages.ExecutionStateSyncRequest{CurrentBlockID: blockA.Header.ID(), TargetBlockID: blockB.Header.ID()})
	require.NoError(s.T(), err)

	// wait for ExecutionStateDelta
	msg2 := s.MsgState.WaitForMsgFrom(s.T(), common.MsgIsExecutionStateDeltaWithChanges, s.exe1ID)
	executionStateDelta := msg2.(*messages.ExecutionStateDelta)
	require.Equal(s.T(), erExe1BlockB.ExecutionResult.FinalStateCommit, executionStateDelta.EndState)
}
