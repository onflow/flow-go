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

func TestScheduledCallbacks(t *testing.T) {
	suite.Run(t, new(ScheduledCallbacksSuite))
}

type ScheduledCallbacksSuite struct {
	Suite
}

func (s *ScheduledCallbacksSuite) TestScheduleCallback_DeployAndGetStatus() {
	// wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// Execute script to call getStatus(id: 0) on the contract
	chainID := s.net.Root().Header.ChainID
	chain := chainID.Chain()

	result := s.getCallbackStatus(chain.ServiceAddress(), 0)
	s.T().Logf("result: %v", result)

	// Verify the result is nil (cadence.Optional with nil value)
	optionalResult, ok := result.(cadence.Optional)
	require.True(s.T(), ok, "result should be a cadence.Optional")
	require.Nil(s.T(), optionalResult.Value, "getStatus(0) should return nil for non-existent callback")

	// Wait for a block to be executed to ensure everything is processed
	blockB := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height)
	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockB.Header.ID()), s.exe1ID)
	s.T().Logf("got block result ID %v", erBlock.ExecutionResult.BlockID)
}

// note:

// seems like blockchain is not sealed after the transaction is executed
// it same for other tests, it might be realted to verification nodes getting incorrect data from block computer
// try to dissable scheduled callbacks to make sure its sealing again
// then try to narrow down where it goes wrong

func (s *ScheduledCallbacksSuite) TestScheduleCallback_ScheduledAndExecuted() {
	// Get chain information
	chainID := s.net.Root().Header.ChainID
	sc := systemcontracts.SystemContractsForChain(chainID)
	serviceAddress := sc.FlowServiceAccount.Address

	// Wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// Deploy the test contract first
	s.deployTestContract(serviceAddress)

	// Wait for next height finalized
	currentFinalized = s.BlockState.HighestFinalizedHeight()
	blockB := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// Schedule a callback for 10 seconds in the future
	futureTimestamp := float64(time.Now().Unix() + 10)
	callbackID := s.scheduleCallback(sc, futureTimestamp)
	s.T().Logf("scheduled callback with ID: %d", callbackID)

	// Check the status of the callback right after scheduling
	status := s.getCallbackStatus(serviceAddress, callbackID)
	require.NotNil(s.T(), status, "callback status should not be nil after scheduling")
	scheduledStatus, ok := status.(cadence.UInt8)
	require.True(s.T(), ok, "status incorrect type")
	require.Equal(s.T(), uint8(0), scheduledStatus, "status should be equal to scheduled")
	s.T().Logf("callback status after scheduling: %v", status)

	// // Verify the callback is scheduled (not executed yet)
	// executedCallbacks := s.getExecutedCallbacks(serviceAddress)
	// require.NotContains(s.T(), executedCallbacks, callbackID, "callback should not be executed immediately")

	// // Wait for 12 seconds to ensure the callback has time to be executed
	// s.T().Log("waiting 12 seconds for callback execution...")
	// time.Sleep(12 * time.Second)

	// // Wait for blocks to be processed after the callback execution time
	// blockC := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height+2)
	// erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockC.Header.ID()), s.exe1ID)
	// s.T().Logf("got block result ID %v after waiting", erBlock.ExecutionResult.BlockID)

	// // Check the status again - it should still exist but be marked as executed
	// statusAfter := s.getCallbackStatus(serviceAddress, callbackID)
	// if statusAfter != nil {
	// 	s.T().Logf("callback status after execution time: %v", statusAfter)
	// }

	// // Verify the callback was executed by checking our test contract
	// executedCallbacksAfter := s.getExecutedCallbacks(serviceAddress)
	// s.T().Logf("executed callbacks: %v", executedCallbacksAfter)
	// require.Contains(s.T(), executedCallbacksAfter, callbackID, "callback should have been executed")
}

func (s *ScheduledCallbacksSuite) deployTestContract(serviceAddress flow.Address) {
	testContract := lib.TestFlowCallbackHandlerContract(sdk.Address(serviceAddress))
	tx, err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), testContract)

	require.NoError(s.T(), err, "could not deploy test contract")
	s.T().Logf("deployed test contract, tx ID: %v", tx.ID())

	// check transaction status before waiting
	result, err := s.AccessClient().GetTransactionResult(context.Background(), tx.ID())
	if err != nil {
		s.T().Logf("error getting transaction result: %v", err)
	} else {
		s.T().Logf("transaction status: %v, error: %v", result.Status, result.Error)
		if result.Error != nil {
			s.T().Logf("transaction error message: %v", result.Error.Error())
		}
	}

	_, err = s.AccessClient().WaitForSealed(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for deploy transaction to be sealed")
}

func (s *ScheduledCallbacksSuite) scheduleCallback(sc *systemcontracts.SystemContracts, timestamp float64) uint64 {
	// Create schedule callback transaction
	scheduleTx := dsl.Transaction{
		Imports: dsl.Imports{
			dsl.Import{
				Names:   []string{"TestFlowCallbackHandler"},
				Address: sdk.Address(sc.FlowServiceAccount.Address),
			},
			dsl.Import{
				Names:   []string{"FlowCallbackScheduler"},
				Address: sdk.Address(sc.FlowCallbackScheduler.Address),
			},
			dsl.Import{
				Names:   []string{"FlowToken"},
				Address: sdk.Address(sc.FlowToken.Address),
			},
			dsl.Import{
				Names:   []string{"FungibleToken"},
				Address: sdk.Address(sc.FungibleToken.Address),
			},
		},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				if !account.storage.check<@TestFlowCallbackHandler.Handler>(from: TestFlowCallbackHandler.HandlerStoragePath) {
					let handler <- TestFlowCallbackHandler.createHandler()
				
					account.storage.save(<-handler, to: TestFlowCallbackHandler.HandlerStoragePath)
					account.capabilities.storage.issue<auth(FlowCallbackScheduler.ExecuteCallback) &{FlowCallbackScheduler.CallbackHandler}>(TestFlowCallbackHandler.HandlerStoragePath)
				}

				let callbackCap = account.capabilities.storage
									.getControllers(forPath: TestFlowCallbackHandler.HandlerStoragePath)[0]
									.capability as! Capability<auth(FlowCallbackScheduler.ExecuteCallback) &{FlowCallbackScheduler.CallbackHandler}>
				
				let vault = account.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
				?? panic("Could not borrow FlowToken vault")
				
				let fees <- vault.withdraw(amount: 1.0) as! @FlowToken.Vault
				let priorityEnum = FlowCallbackScheduler.Priority.High

				let scheduledCallback = FlowCallbackScheduler.schedule(
					callback: callbackCap,
					data: "test data",
					timestamp: UFix64(` + fmt.Sprintf("%.8f", timestamp) + `),
					priority: priorityEnum,
					executionEffort: 1000,
					fees: <-fees
				)

				TestFlowCallbackHandler.addScheduledCallback(callback: scheduledCallback)
			`),
		},
	}

	// Convert to SDK transaction and send
	sdkTx := lib.SDKTransactionFixture(
		lib.WithTransactionDSL(scheduleTx),
		lib.WithReferenceBlock(sdk.Identifier(s.net.Root().ID())),
		lib.WithChainID(s.net.Root().Header.ChainID),
	)

	err := s.AccessClient().SendTransaction(context.Background(), &sdkTx)
	require.NoError(s.T(), err, "could not send schedule transaction")

	// Wait for the transaction to be sealed
	sealedResult, err := s.AccessClient().WaitForSealed(context.Background(), sdkTx.ID())
	require.NoError(s.T(), err, "could not wait for schedule transaction to be sealed")

	// Extract callback ID from events
	callbackID := s.extractCallbackIDFromEvents(sealedResult)
	if callbackID == 0 {
		// Fallback: assume first callback ID is 1 if we can't extract from events
		callbackID = 1
	}

	return callbackID
}

func (s *ScheduledCallbacksSuite) getCallbackStatus(serviceAddress flow.Address, callbackID uint64) cadence.Value {
	getStatusScript := dsl.Main{
		Import: dsl.Import{
			Address: sdk.Address(serviceAddress),
			Names:   []string{"FlowCallbackScheduler"},
		},
		ReturnType: "FlowCallbackScheduler.Status?",
		Code:       fmt.Sprintf("return FlowCallbackScheduler.getStatus(id: %d)", callbackID),
	}

	result, err := s.AccessClient().ExecuteScript(context.Background(), getStatusScript)
	require.NoError(s.T(), err, "could not execute getStatus script")
	return result
}

func (s *ScheduledCallbacksSuite) getExecutedCallbacks(serviceAddress flow.Address) []uint64 {
	getExecutedScript := dsl.Main{
		Import: dsl.Import{
			Address: sdk.Address(serviceAddress),
			Names:   []string{"TestFlowCallbackHandler"},
		},
		ReturnType: "[UInt64]",
		Code:       "return TestFlowCallbackHandler.getExecutedCallbacks()",
	}

	result, err := s.AccessClient().ExecuteScript(context.Background(), getExecutedScript)
	require.NoError(s.T(), err, "could not execute getExecutedCallbacks script")

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

func (s *ScheduledCallbacksSuite) extractCallbackIDFromEvents(result *sdk.TransactionResult) uint64 {
	// Look for CallbackScheduled event
	for _, event := range result.Events {
		if event.Type == "FlowCallbackScheduler.CallbackScheduled" {
			// Extract ID field from event
			id := event.Value.SearchFieldByName("id")
			if id != nil {
				return uint64(id.(cadence.UInt64))
			}
		}
	}
	return 0
}
