package verification

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

// TestVerificationStreamNegotiation enables gzip stream compression only between execution and verification nodes, while the
// rest of network runs on plain libp2p streams. It evaluates that network operates on its happy path concerning verification functionality.
func TestVerificationStreamNegotiation(t *testing.T) {
	s := new(VerificationStreamNegotiationSuite)
	s.PreferredUnicasts = string(unicast.GzipCompressionUnicast) // enables gzip stream compression between execution and verification node
	suite.Run(t, s)
}

type VerificationStreamNegotiationSuite struct {
	Suite
}

// TestVerificationNodeHappyPath verifies the integration of verification and execution nodes over the
// happy path of successfully issuing a result approval for the first chunk of the first block of the testnet.
// Note that gzip stream compression is enabled between verification and execution nodes.
func (suite *VerificationStreamNegotiationSuite) TestVerificationNodeHappyPath() {
	testVerificationNodeHappyPath(suite.T(), suite.exe1ID, suite.verID, suite.BlockState, suite.ReceiptState, suite.ApprovalState)
}

func testVerificationNodeHappyPath(t *testing.T,
	exeID flow.Identifier,
	verID flow.Identifier,
	blocks *blockstate.BlockState,
	receipts *receiptstate.ReceiptState,
	approvals *approvalstate.ResultApprovalState,
) {

	// wait for next height finalized (potentially first height), called blockA
	blockA := blocks.WaitForHighestFinalizedProgress(t)
	t.Logf("blockA generated, height: %v ID: %v\n", blockA.Header.Height, blockA.Header.ID())

	// waits for execution receipt for blockA from execution node, called receiptA
	receiptA := receipts.WaitForReceiptFrom(t, blockA.Header.ID(), exeID)
	resultID := receiptA.ExecutionResult.ID()
	t.Logf("receipt for blockA generated: result ID: %x\n", resultID)

	// wait for a result approval from verification node
	approvals.WaitForResultApproval(t, verID, resultID, uint64(0))
}
