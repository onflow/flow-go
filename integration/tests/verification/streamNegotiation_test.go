package verification

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestVerificationStreamNegotiationSuite(t *testing.T) {
	s := new(VerificationStreamNegotiationSuite)
	s.preferredUnicasts = "gzip-compression"
	suite.Run(t, s)
}

type VerificationStreamNegotiationSuite struct {
	Suite
}

// TestVerificationNodeHappyPath verifies the integration of verification and execution nodes over the
// happy path of successfully issuing a result approval for the first chunk of the first block of the testnet.
func (suite *VerificationStreamNegotiationSuite) TestVerificationNodeHappyPath() {
	// wait for next height finalized (potentially first height), called blockA
	blockA := suite.BlockState.WaitForHighestFinalizedProgress(suite.T())
	suite.T().Logf("blockA generated, height: %v ID: %v\n", blockA.Header.Height, blockA.Header.ID())

	// waits for execution receipt for blockA from execution node, called receiptA
	receiptA := suite.ReceiptState.WaitForReceiptFrom(suite.T(), blockA.Header.ID(), suite.exe1ID)
	resultID := receiptA.ExecutionResult.ID()
	suite.T().Logf("receipt for blockA generated: result ID: %x\n", resultID)

	// wait for a result approval from verification node
	suite.ApprovalState.WaitForResultApproval(suite.T(), suite.verID, resultID, uint64(0))
}
