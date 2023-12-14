package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	suite.Run(t, new(FailingTxRevertedSuite))
}

type FailingTxRevertedSuite struct {
	Suite
}

func (s *FailingTxRevertedSuite) TestExecutionFailingTxReverted() {

	chainID := s.net.Root().Header.ChainID
	chain := chainID.Chain()
	serviceAddress := chain.ServiceAddress()

	// wait for next height finalized (potentially first height), called blockA
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v\n", blockA.Header.Height, blockA.Header.ID())

	// send transaction
	tx, err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), lib.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	_, err = s.AccessClient().WaitForExecuted(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for tx to be executed")

	// send transaction that panics and should revert
	failingTx := lib.SDKTransactionFixture(
		lib.WithTransactionDSL(lib.CreateCounterPanicTx(chain)),
		lib.WithReferenceBlock(sdk.Identifier(s.net.Root().ID())),
		lib.WithChainID(chainID),
	)

	err = s.AccessClient().SendTransaction(context.Background(), &failingTx)
	require.NoError(s.T(), err, "could not send tx to create counter that should panic")

	txResult, err := s.AccessClient().WaitForExecuted(context.Background(), failingTx.ID())
	require.NoError(s.T(), err, "could not wait for tx to be executed")

	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(txResult.BlockID), s.exe1ID)
	s.T().Logf("got blockB result ID %v\n", erBlock.ExecutionResult.BlockID)

	// expected two chunks (one for the transaction, one for the system)
	require.Len(s.T(), erBlock.Chunks, 2)

	//// assert that state did not change in the first chunk
	require.Equal(s.T(), erBlock.Chunks[0].StartState, erBlock.Chunks[0].EndState)

	// send transaction that has no sigs and should not execute
	failingTx = lib.SDKTransactionFixture(
		lib.WithTransactionDSL(lib.CreateCounterTx(sdk.Address(serviceAddress))),
		lib.WithReferenceBlock(sdk.Identifier(s.net.Root().ID())),
		lib.WithChainID(chainID),
	)
	failingTx.PayloadSignatures = nil
	failingTx.EnvelopeSignatures = nil

	err = s.AccessClient().SendTransaction(context.Background(), &failingTx)
	require.NoError(s.T(), err, "could not send tx to create counter with wrong sig")

	txResult, err = s.AccessClient().WaitForExecuted(context.Background(), failingTx.ID())
	require.NoError(s.T(), err, "could not wait for tx to be executed")

	erBlock = s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(txResult.BlockID), s.exe1ID)
	s.T().Logf("got blockB result ID %v\n", erBlock.ExecutionResult.BlockID)

	// expected two chunks (one for the transaction, one for the system)
	require.Len(s.T(), erBlock.Chunks, 2)

	//// assert that state did not change in the first chunk
	require.Equal(s.T(), erBlock.Chunks[0].StartState, erBlock.Chunks[0].EndState)
}
