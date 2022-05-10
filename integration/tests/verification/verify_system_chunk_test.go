package verification

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestVerifySystemChunk(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "active-pacemaker")
	suite.Run(t, new(VerifySystemChunkSuite))
}

type VerifySystemChunkSuite struct {
	Suite
}

// TestSystemChunkIDsShouldBeDifferent evaluates that system chunk of consecutive blocks that
// do not cause state change have different chunk Ids.
func (suite *VerifySystemChunkSuite) TestSystemChunkIDsShouldBeDifferent() {
	// // wait for next height finalized (potentially first height), called blockA
	blockA := suite.BlockState.WaitForHighestFinalizedProgress(suite.T())
	suite.T().Logf("blockA generated, height: %v ID: %v\n", blockA.Header.Height, blockA.Header.ID())

	// waits for the next finalized block after blockA, called blockB.
	blockB := suite.BlockState.WaitForFinalizedChild(suite.T(), blockA)
	suite.T().Logf("blockB generated, height: %v ID: %v\n", blockB.Header.Height, blockB.Header.ID())

	// waits for execution receipt for blockA from execution node, called receiptA.
	receiptA := suite.ReceiptState.WaitForReceiptFrom(suite.T(), blockA.Header.ID(), suite.exe1ID)
	resultAId := receiptA.ExecutionResult.ID()
	suite.T().Logf("receipt for blockA generated: result ID: %x\n", resultAId)

	// waits for execution receipt for blockB from execution node, called receiptB.
	receiptB := suite.ReceiptState.WaitForReceiptFrom(suite.T(), blockB.Header.ID(), suite.exe1ID)
	resultBId := receiptB.ExecutionResult.ID()
	suite.T().Logf("receipt for blockB generated: result ID: %x\n", resultBId)

	// Todo: drop this part once system chunk changes the state
	// requires that execution state is not changed between block A and B
	stateA, err := receiptA.ExecutionResult.FinalStateCommitment()
	require.NoError(suite.T(), err)
	stateB, err := receiptB.ExecutionResult.FinalStateCommitment()
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), stateA, stateB)

	// computes ids of system chunk for result A and B
	systemChunkAId := receiptA.ExecutionResult.Chunks[0].ID()
	systemChunkBId := receiptB.ExecutionResult.Chunks[0].ID()

	// requires that system chunk Id of execution results be different
	require.NotEqual(suite.T(), systemChunkAId, systemChunkBId)
}
