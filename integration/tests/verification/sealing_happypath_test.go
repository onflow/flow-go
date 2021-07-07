package verification

import (
	"context"
	"testing"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/common"
)

func TestSealingHappyPathTest(t *testing.T) {
	suite.Run(t, new(TestSealingHappyPathTestSuite))
}

type TestSealingHappyPathTestSuite struct {
	Suite
}

// TestSealingAndVerificationHappyPath evaluates the health of the happy path of verification and sealing. It
//deploys a transaction into the testnet hence causing an execution result with more than
// one chunk, assigns all chunks to the same single verification node in this testnet, and then verifies whether verification node
// generates a result approval for all chunks of that execution result.
// It also enables sealing based on result approvals and verifies whether the block of that specific multi-chunk execution result is sealed
// affected by the emitted result approvals.
func (s *TestSealingHappyPathTestSuite) TestSealingAndVerificationHappyPath() {
	// captures one finalized block called blockA, just to make sure that the finalization progresses.
	blockA := s.BlockState.WaitForNextFinalized(s.T())
	s.T().Logf("blockA generated, height: %v ID: %v", blockA.Header.Height, blockA.Header.ID())

	// sends a transaction
	err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// waits until for a different state commitment for a finalized block, call that block blockB,
	// which has more than one chunk on its execution result.
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), &s.BlockState, &s.ReceiptState, common.WithMinimumChunks(2))
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// waits for the execution receipt of blockB from both execution nodes, and makes sure that there is no execution fork.
	receiptB1 := s.ReceiptState.WaitForReceiptFrom(s.T(), blockB.Header.ID(), s.exe1ID)
	s.T().Logf("receipt for blockB generated by execution node-1: %x result ID: %x", s.exe1ID, receiptB1.ExecutionResult.ID())
	receiptB2 := s.ReceiptState.WaitForReceiptFrom(s.T(), blockB.Header.ID(), s.exe2ID)
	s.T().Logf("receipt for blockB generated by execution node-2: %x result ID: %x", s.exe2ID, receiptB2.ExecutionResult.ID())

	require.Equal(s.T(), receiptB1.ExecutionResult.ID(), receiptB2.ExecutionResult.ID(), "execution fork happened at blockB")
	resultB := receiptB1.ExecutionResult
	resultBId := resultB.ID()
	// re-evaluates that resultB has more than one chunk.
	require.Greater(s.T(), len(resultB.Chunks), 1)
	s.T().Logf("receipt for blockB generated: result ID: %x with %d chunks", resultBId, len(resultB.Chunks))

	// wait for a result approval from verification node for the chunks of resultB.
	for i := 0; i < len(resultB.Chunks); i++ {
		s.ApprovalState.WaitForResultApproval(s.T(), s.verID, resultBId, uint64(i))
		s.T().Logf("result approval generated for blockB: result ID: %x chunk index: %d", resultBId, i)
	}

	// waits until blockB is sealed by consensus nodes after result approvals for all of its chunks emitted.
	s.BlockState.WaitForSealed(s.T(), blockB.Header.Height)
}
