package verification

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestVerifySystemChunk(t *testing.T) {
	suite.Run(t, new(VerifySystemChunkSuite))
}

type VerifySystemChunkSuite struct {
	Suite
}

// TestSystemChunkIDsShouldBeDifferent evaluates that system chunk of consecutive blocks that
// do not cause state change have different chunk Ids.
func (suite *VerifySystemChunkSuite) TestSystemChunkIDsShouldBeDifferent() {
	// // wait for next height finalized (potentially first height), called blockA
	currentFinalized := suite.BlockState.HighestFinalizedHeight()
	blockA := suite.BlockState.WaitForHighestFinalizedProgress(suite.T(), currentFinalized)
	suite.T().Logf("blockA generated, height: %v ID: %v\n", blockA.Height, blockA.Hash())

	// waits for the next finalized block after blockA, called blockB.
	blockB := suite.BlockState.WaitForFinalizedChild(suite.T(), blockA)
	suite.T().Logf("blockB generated, height: %v ID: %v\n", blockB.Height, blockB.Hash())

	// waits for execution receipt for blockA from execution node, called receiptA.
	receiptA := suite.ReceiptState.WaitForReceiptFrom(suite.T(), blockA.Hash(), suite.exe1ID)
	resultAId := receiptA.ExecutionResult.Hash()
	suite.T().Logf("receipt for blockA generated: result ID: %x\n", resultAId)

	// waits for execution receipt for blockB from execution node, called receiptB.
	receiptB := suite.ReceiptState.WaitForReceiptFrom(suite.T(), blockB.Hash(), suite.exe1ID)
	resultBId := receiptB.ExecutionResult.Hash()
	suite.T().Logf("receipt for blockB generated: result ID: %x\n", resultBId)

	// computes ids of system chunk for result A and B
	systemChunkA := receiptA.ExecutionResult.Chunks[0]
	systemChunkAId := systemChunkA.Hash()
	suite.T().Logf("system chunk for blockA: %v\n", *systemChunkA)

	systemChunkB := receiptB.ExecutionResult.Chunks[0]
	systemChunkBId := systemChunkB.Hash()
	suite.T().Logf("system chunk for blockB: %v\n", *systemChunkB)

	// requires that system chunk Id of execution results be different
	require.NotEqual(suite.T(), systemChunkAId, systemChunkBId)
}
