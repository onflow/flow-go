package verification

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestResultApprovalTestSuit(t *testing.T) {
	suite.Run(t, new(ResultApprovalTestSuite))
}

type ResultApprovalTestSuite struct {
	Suite
}

// TestVerificationNodeHappyPath verifies the integration of verification and execution nodes over the
// happy path of successfully issuing a result approval for the first chunk of the first block of the testnet.
func (r *ResultApprovalTestSuite) TestVerificationNodeHappyPath() {
	// wait for next height finalized (potentially first height), called blockA
	blockA := r.BlockState.WaitForHighestFinalizedProgress(r.T())
	r.T().Logf("blockA generated, height: %v ID: %v", blockA.Header.Height, blockA.Header.ID())

	// waits for execution receipt for blockA from execution node, called receiptA
	receiptA := r.ReceiptState.WaitForReceiptFrom(r.T(), blockA.Header.ID(), r.exe1ID)
	resultID := receiptA.ExecutionResult.ID()
	r.T().Logf("receipt for blockA generated: result ID: %x", resultID)

	// wait for a result approval from verification node
	r.ApprovalState.WaitForResultApproval(r.T(), r.verID, resultID, uint64(0))
}
