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

func (s *TestSealingHappyPathTestSuite) TestSealingHappyPath() {
	// waits for first finalized block, called blockA
	blockA := s.BlockState.WaitForFirstFinalized(s.T())
	s.T().Logf("blockA generated, height: %v ID: %v", blockA.Header.Height, blockA.Header.ID())

	//// waits for execution receipt for blockA from execution node, called receiptA
	//receiptA := r.ReceiptState.WaitForReceiptFrom(r.T(), blockA.Header.ID(), r.exeID)
	//r.T().Logf("receipt for blockA generated: result ID: %x", resultID)

	// send transaction
	err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, receiptB := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), &s.BlockState, &s.ReceiptState)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	resultID := receiptB.ExecutionResult.ID()
	require.Greater(s.T(), len(receiptB.ExecutionResult.Chunks), 1)
	s.T().Logf("receipt for blockB generated: result ID: %x with %d chunks", resultID, len(receiptB.ExecutionResult.Chunks))

	// wait for a result approval from verification node
	s.ApprovalState.WaitForResultApproval(s.T(), s.verID, resultID, uint64(0))
	s.T().Logf("result approval generated for blockB: result ID: %x chunk index: 0", resultID)
	s.ApprovalState.WaitForResultApproval(s.T(), s.verID, resultID, uint64(1))
	s.T().Logf("result approval generated for blockB: result ID: %x chunk index: 1", resultID)

	s.BlockState.WaitForSealed(s.T(), blockB.Header.Height)
}
