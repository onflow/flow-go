package verification

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/common"
)

func TestHappyPath(t *testing.T) {
	suite.Run(t, new(VerificationTestSuite))
}

type VerificationTestSuite struct {
	Suite
}

// TestSealingAndVerificationHappyPath evaluates the health of the happy path of verification and sealing. It
// deploys a transaction into the testnet hence causing an execution result with more than
// one chunk, assigns all chunks to the same single verification node in this testnet, and then verifies whether verification node
// generates a result approval for all chunks of that execution result.
// It also enables sealing based on result approvals and verifies whether the block of that specific multi-chunk execution result is sealed
// affected by the emitted result approvals.
func (v *VerificationTestSuite) TestSealingAndVerificationHappyPath() {
	common.SealingAndVerificationHappyPathTest(
		v.T(),
		v.BlockState,
		v.ReceiptState,
		v.ApprovalState,
		v.AccessClient(),
		v.exe1ID,
		v.exe2ID,
		v.verID,
		v.net.Root().ID())
}
