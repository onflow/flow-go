package execution

import (
	"context"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

type StateSyncSuite struct {
	Suite
}

func (s *StateSyncSuite) TestStateSyncAfterNetworkPartition() {
	// wait for next height finalized (potentially first height), called blockA
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T())
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := s.ReceiptState.WaitForReceiptFrom(s.T(), blockA.Header.ID(), s.exe1ID)
	finalStateExe1BlockA, err := erExe1BlockA.ExecutionResult.FinalStateCommitment()
	require.NoError(s.T(), err)
	s.T().Logf("got erExe1BlockA with SC %x", finalStateExe1BlockA)

	// send transaction
	err = s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), s.BlockState, s.ReceiptState)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := s.ReceiptState.WaitForReceiptFrom(s.T(), blockB.Header.ID(), s.exe1ID)
	finalStateExe1BlockB, err := erExe1BlockB.ExecutionResult.FinalStateCommitment()
	require.NoError(s.T(), err)
	s.T().Logf("got erExe1BlockB with SC %x", finalStateExe1BlockB)

	// require that state between blockA and blockB has changed
	require.NotEqual(s.T(), finalStateExe1BlockA, finalStateExe1BlockB)

	// wait until the next proposed block is finalized, called blockC
	blockC := s.BlockState.WaitUntilNextHeightFinalized(s.T())
	s.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erExe1BlockC := s.ReceiptState.WaitForReceiptFrom(s.T(), blockC.Header.ID(), s.exe1ID)
	finalStateExe1BlockC, err := erExe1BlockC.ExecutionResult.FinalStateCommitment()
	require.NoError(s.T(), err)
	s.T().Logf("got erExe1BlockC with SC %x", finalStateExe1BlockC)

	// require that state between blockB and blockC has not changed
	require.Equal(s.T(), finalStateExe1BlockB, finalStateExe1BlockC)

	// wait for block C has been sealed
	sealed := s.BlockState.WaitForSealed(s.T(), blockC.Header.Height)
	s.T().Logf("block C has been sealed: %v", sealed.Header.ID())

	// send a ExecutionStateSyncRequest from Ghost node
	err = s.Ghost().Send(context.Background(), engine.SyncExecution,
		&messages.ExecutionStateSyncRequest{FromHeight: blockA.Header.Height, ToHeight: blockC.Header.Height},
		[]flow.Identifier{s.exe1ID}...)
	require.NoError(s.T(), err)

	// wait for ExecutionStateDelta
	msg2 := s.MsgState.WaitForMsgFrom(s.T(), common.MsgIsExecutionStateDeltaWithChanges, s.exe1ID, "state delta from execution node")
	executionStateDelta := msg2.(*messages.ExecutionStateDelta)
	require.Equal(s.T(), finalStateExe1BlockB, executionStateDelta.EndState)
}
