package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	suite.Run(t, &FailingTxRevertedSuite{
		chainID: flow.Testnet,
	})
}

type FailingTxRevertedSuite struct {
	Suite
	chainID flow.ChainID
}

func (s *FailingTxRevertedSuite) TestExecutionFailingTxReverted() {

	chain := s.net.Root().Header.ChainID.Chain()

	// wait for next height finalized (potentially first height), called blockA
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T())
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// send transaction
	err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), common.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, erBlockB := common.WaitUntilFinalizedStateCommitmentChanged(s.T(), s.BlockState, &s.ReceiptState)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// final states
	finalStateBlockB, err := erBlockB.ExecutionResult.FinalStateCommitment()
	require.NoError(s.T(), err)

	// send transaction that panics and should revert
	tx := common.SDKTransactionFixture(
		common.WithTransactionDSL(common.CreateCounterPanicTx(chain)),
		common.WithReferenceBlock(sdk.Identifier(s.net.Root().ID())),
	)
	err = s.AccessClient().SendTransaction(context.Background(), &tx)
	require.NoError(s.T(), err, "could not send tx to create counter that should panic")

	// send transaction that has no sigs and should not execute
	tx = common.SDKTransactionFixture(
		common.WithTransactionDSL(common.CreateCounterTx(sdk.Address(chain.ServiceAddress()))),
		common.WithReferenceBlock(sdk.Identifier(s.net.Root().ID())),
	)
	tx.PayloadSignatures = nil
	tx.EnvelopeSignatures = nil
	err = s.AccessClient().SendTransaction(context.Background(), &tx)
	require.NoError(s.T(), err, "could not send tx to create counter with wrong sig")

	// wait until the next proposed block is finalized, called blockC
	blockC := s.BlockState.WaitUntilNextHeightFinalized(s.T())
	s.T().Logf("got blockC height %v ID %v", blockC.Header.Height, blockC.Header.ID())

	// wait for execution receipt for blockC from execution node 1
	erBlockC := s.ReceiptState.WaitForReceiptFrom(s.T(), blockC.Header.ID(), s.exe1ID)
	finalStateBlockC, err := erBlockC.ExecutionResult.FinalStateCommitment()
	require.NoError(s.T(), err)

	s.T().Logf("got erBlockC with SC %x", finalStateBlockC)

	// assert that state did not change between blockB and blockC
	require.Equal(s.T(), finalStateBlockB, finalStateBlockC)
}
