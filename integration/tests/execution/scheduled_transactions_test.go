package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
)

func TestScheduledTransactions(t *testing.T) {
	suite.Run(t, new(ScheduledTransactionsSuite))
}

type ScheduledTransactionsSuite struct {
	Suite
}

func (s *ScheduledTransactionsSuite) TestScheduleTransaction_DeployAndGetStatus() {
	// wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.HeaderBody.Height, blockA.ID())

	// Execute script to call getStatus(id: 10) on the contract
	result, ok := s.getTransactionStatus(10)
	s.T().Logf("result: %v, ok: %v", result, ok)
	require.False(s.T(), ok, "getStatus(10) should return false for non-existent scheduled transaction")

	// Wait for a block to be executed to ensure everything is processed
	blockB := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.HeaderBody.Height)
	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockB.ID()), s.exe1ID)
	s.T().Logf("got block result ID %v", erBlock.ExecutionResult.BlockID)
}

func (s *ScheduledTransactionsSuite) TestScheduleTransaction_ScheduledAndExecuted() {
	sc := systemcontracts.SystemContractsForChain(s.net.Root().HeaderBody.ChainID)

	// Wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.HeaderBody.Height, blockA.ID())

	// Deploy the test contract first
	err := lib.DeployScheduledTransactionTestContract(
		s.AccessClient(),
		sdk.Address(sc.FlowCallbackScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
		sdk.Identifier(s.net.Root().ID()),
	)
	require.NoError(s.T(), err, "could not deploy test contract")

	// Wait for next height finalized before scheduling transaction
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), s.BlockState.HighestFinalizedHeight())

	// Schedule a transaction for 10 seconds in the future
	scheduleDelta := int64(10)
	futureTimestamp := time.Now().Unix() + scheduleDelta

	s.T().Logf("scheduling transaction at timestamp: %v, current timestamp: %v", futureTimestamp, time.Now().Unix())
	transactionID, err := lib.ScheduleTransactionAtTimestamp(
		futureTimestamp,
		s.AccessClient(),
		sdk.Address(sc.FlowTransactionScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
	)
	require.NoError(s.T(), err, "could not schedule transaction transaction")
	s.T().Logf("scheduled transaction with ID: %d", transactionID)

	const scheduledStatus = 1
	const executedStatus = 2

	// Check the status of the transaction right after scheduling
	status, ok := s.getTransactionStatus(transactionID)
	require.True(s.T(), ok, "transaction status should not be nil after scheduling")
	require.Equal(s.T(), scheduledStatus, status, "status should be equal to scheduled")
	s.T().Logf("transaction status after scheduling: %v", status)

	// Verify the transaction is scheduled (not executed yet)
	executedTransactions := s.getExecutedTransactions()
	require.NotContains(s.T(), executedTransactions, transactionID, "transaction should not be executed immediately")

	// Wait to ensure the transaction has time to be executed
	s.T().Log("waiting for transaction execution...")
	time.Sleep(time.Duration(scheduleDelta)*time.Second + 2)

	// Wait for blocks to be processed after the transaction execution time
	// blockC := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height+2)
	// erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockC.Header.ID()), s.exe1ID)
	// s.T().Logf("got block result ID %v after waiting", erBlock.ExecutionResult.BlockID)

	// Check the status again - it should still exist but be marked as executed
	statusAfter, ok := s.getTransactionStatus(transactionID)
	require.True(s.T(), ok, "transaction status should not be nil after scheduling")
	require.Equal(s.T(), executedStatus, statusAfter, "status should be equal to executed")

	// Verify the transaction was executed by checking our test contract
	executedTransactionsAfter := s.getExecutedTransactions()
	s.T().Logf("executed transactions: %v", executedTransactionsAfter)
	require.Len(s.T(), executedTransactionsAfter, 1, "should have exactly one executed transaction")
	require.Contains(s.T(), executedTransactionsAfter, transactionID, "transaction should have been executed")
}

func (s *ScheduledTransactionsSuite) TestScheduleTransaction_ScheduleAndCancelTransaction() {
	sc := systemcontracts.SystemContractsForChain(s.net.Root().HeaderBody.ChainID)

	// Wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.HeaderBody.Height, blockA.ID())

	// Deploy the test contract first
	err := lib.DeployScheduledTransactionTestContract(
		s.AccessClient(),
		sdk.Address(sc.FlowTransactionScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
		sdk.Identifier(s.net.Root().ID()),
	)
	require.NoError(s.T(), err, "could not deploy test contract")

	// Wait for next height finalized before scheduling transaction
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), s.BlockState.HighestFinalizedHeight())

	// Schedule a transaction for 10 seconds in the future
	scheduleDelta := int64(10)
	futureTimestamp := time.Now().Unix() + scheduleDelta

	s.T().Logf("scheduling transaction at timestamp: %v, current timestamp: %v", futureTimestamp, time.Now().Unix())
	transactionID, err := lib.ScheduleTransactionAtTimestamp(
		futureTimestamp,
		s.AccessClient(),
		sdk.Address(sc.FlowTransactionScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
	)
	require.NoError(s.T(), err, "could not schedule transaction transaction")
	s.T().Logf("scheduled transaction with ID: %d", transactionID)

	const scheduledStatus = 1
	const canceledStatus = 3

	// Wait fraction of the scheduled time
	s.T().Log("waiting for transaction execution...")
	time.Sleep(time.Second + 2)

	// Check the status of the transaction
	status, ok := s.getTransactionStatus(transactionID)
	require.True(s.T(), ok, "transaction status should not be nil after scheduling")
	require.Equal(s.T(), scheduledStatus, status, "status should be equal to scheduled")
	s.T().Logf("transaction status after scheduling: %v", status)

	// Verify the transaction is scheduled (not executed yet)
	executedTransactions := s.getExecutedTransactions()
	require.NotContains(s.T(), executedTransactions, transactionID, "transaction should not be executed immediately")

	// Cancel the transaction
	canceledID, err := lib.CancelTransactionByID(
		transactionID,
		s.AccessClient(),
		sdk.Address(sc.FlowTransactionScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
	)
	require.NoError(s.T(), err, "could not cancel transaction transaction")
	require.Equal(s.T(), transactionID, canceledID, "canceled transaction ID should be the same as scheduled")

	// Wait for transaction scheduled time to make sure it was not executed
	time.Sleep(time.Duration(scheduleDelta) * time.Second)

	// Check the status of the transaction
	status, ok = s.getTransactionStatus(transactionID)
	require.True(s.T(), ok, "transaction status should not be nil after scheduling")
	require.Equal(s.T(), canceledStatus, status, "status should be equal to canceled")
}

func (s *ScheduledTransactionsSuite) getTransactionStatus(transactionID uint64) (int, bool) {
	getStatusScript := dsl.Main{
		Import: dsl.Import{
			Address: s.AccessClient().SDKServiceAddress(),
			Names:   []string{"FlowTransactionScheduler"},
		},
		ReturnType: "FlowTransactionScheduler.Status?",
		Code:       fmt.Sprintf("return FlowTransactionScheduler.getStatus(id: %d)", transactionID),
	}

	latest, err := s.AccessClient().GetLatestFinalizedBlockHeader(context.Background())
	require.NoError(s.T(), err, "could not get latest finalized block header")

	result, err := s.AccessClient().ExecuteScriptAtBlock(context.Background(), getStatusScript, latest.ID)
	require.NoError(s.T(), err, "could not execute getStatus script")

	optionalResult, ok := result.(cadence.Optional)
	require.True(s.T(), ok, "result should be a cadence.Optional")

	if optionalResult.Value == nil {
		return 0, false
	}

	enumValue, ok := optionalResult.Value.(cadence.Enum)
	require.True(s.T(), ok, "status should be a cadence.Enum")

	raw := enumValue.FieldsMappedByName()["rawValue"]
	val, ok := raw.(cadence.UInt8)
	require.True(s.T(), ok, "status should be a cadence.UInt8")

	return int(val), true
}

func (s *ScheduledTransactionsSuite) getExecutedTransactions() []uint64 {
	getExecutedScript := dsl.Main{
		Import: dsl.Import{
			Address: s.AccessClient().SDKServiceAddress(),
			Names:   []string{"TestFlowTransactionHandler"},
		},
		ReturnType: "[UInt64]",
		Code:       "return TestFlowTransactionHandler.getExecutedTransactions()",
	}

	latest, err := s.AccessClient().GetLatestFinalizedBlockHeader(context.Background())
	require.NoError(s.T(), err, "could not get latest finalized block header")

	result, err := s.AccessClient().ExecuteScriptAtBlock(context.Background(), getExecutedScript, latest.ID)
	require.NoError(s.T(), err, "could not execute getStatus script")

	// Convert cadence array to Go slice
	cadenceArray, ok := result.(cadence.Array)
	require.True(s.T(), ok, "result should be a cadence array")

	var executedIDs []uint64
	for _, value := range cadenceArray.Values {
		if id, ok := value.(cadence.UInt64); ok {
			executedIDs = append(executedIDs, uint64(id))
		}
	}

	return executedIDs
}
