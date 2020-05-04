package execution

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/tests/common"
)

func (gs *ExecutionSuite) TestVerificationNodesRequestChunkDataPacks() {

	// pause execution node 2
	err := gs.net.ContainerByID(gs.exe2ID).Pause()
	require.NoError(gs.T(), err, "could not pause execution node 2")

	// wait for first finalized block, called blockA
	blockA := gs.BlockState.WaitForFirstFinalized(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockA.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockA with SC %x", erExe1BlockA.ExecutionResult.FinalStateCommit)

	// assert there were no ChunkDataPackRequests from the verification node yet
	require.Equal(gs.T(), 0, gs.MsgState.LenFrom(gs.verID), "expected no ChunkDataPackRequest to be sent before a transaction existed")

	// send transaction
	err = common.DeployCounter(context.Background(), gs.AccessClient())

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(gs.T(), &gs.BlockState, &gs.ReceiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockB.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockB with SC %x", erExe1BlockB.ExecutionResult.FinalStateCommit)

	// wait for ChunkDataPackRequests
	gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackRequest, gs.verID)
}
