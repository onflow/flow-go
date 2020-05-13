package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	suite.Run(t, new(FailingTxRevertedSuite))
}

type FailingTxRevertedSuite struct {
	Suite
}

func (s *FailingTxRevertedSuite) TestExecutionFailingTxReverted() {

	// wait for first finalized block, called blockA
	blockA := s.BlockState.WaitForFirstFinalized(s.T())
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// send transaction
	err := s.AccessClient().DeployContract(context.Background(), s.net.Genesis().ID(), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, erBlockB := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), &s.BlockState, &s.ReceiptState)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// send transaction that panics and should revert
	tx := unittest.TransactionBodyFixture(
		unittest.WithTransactionDSL(common.CreateCounterPanicTx),
		unittest.WithReferenceBlock(s.net.Genesis().ID()),
	)
	err = s.AccessClient().SendTransaction(context.Background(), tx)
	require.NoError(s.T(), err, "could not send tx to create counter that should panic")

	// send transaction that has no sigs and should not execute
	tx = unittest.TransactionBodyFixture(
		unittest.WithTransactionDSL(common.CreateCounterTx),
		unittest.WithReferenceBlock(s.net.Genesis().ID()),
	)
	tx.PayloadSignatures = nil
	tx.EnvelopeSignatures = nil
	err = s.AccessClient().SendTransaction(context.Background(), tx)
	require.NoError(s.T(), err, "could not send tx to create counter with wrong sig")

	// wait until the next proposed block is finalized, called blockC
	blockC := s.BlockState.WaitUntilNextHeightFinalized(s.T())
	s.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erBlockC := s.ReceiptState.WaitForReceiptFrom(s.T(), blockC.Header.ID(), s.exe1ID)
	s.T().Logf("got erBlockC with SC %x", erBlockC.ExecutionResult.FinalStateCommit)

	// assert that state did not change between blockB and blockC
	require.Equal(s.T(), erBlockB.ExecutionResult.FinalStateCommit, erBlockC.ExecutionResult.FinalStateCommit)
}
